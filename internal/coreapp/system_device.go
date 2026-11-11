package coreapp

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/coremsg"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	deviceReadyTimeout       = 6 * time.Second
	deviceReadyStatusGrace   = 350 * time.Millisecond
	deviceReadyQueryInterval = 750 * time.Millisecond
	deviceReadyPollInterval  = 100 * time.Millisecond

	// 启动下发前等待首个有效温度的上限与轮询间隔：开机自启动时温度桥接
	// 往往还要十几秒才出第一帧数据。
	startupControlTempWait         = 25 * time.Second
	startupControlTempPollInterval = 500 * time.Millisecond
)

// onShowWindowRequest 显示窗口请求回调
func (a *CoreApp) onShowWindowRequest() {
	a.logInfo("收到显示窗口请求")

	// 通知所有已连接的 GUI 客户端显示窗口
	if a.ipcServer != nil && a.ipcServer.HasClients() {
		a.ipcServer.BroadcastEvent("show-window", nil)
	} else {
		// 没有 GUI 连接，启动 GUI
		a.logInfo("没有 GUI 连接，尝试启动 GUI")
		if err := launchGUI(); err != nil {
			a.logError("启动 GUI 失败: %v", err)
		}
	}
}

// onQuitRequest 退出请求回调
func (a *CoreApp) onQuitRequest() {
	a.logInfo("收到退出请求")
	if a.stopping.Load() {
		return
	}

	// 通知所有 GUI 客户端退出
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent("quit", nil)
	}

	// 发送退出信号
	select {
	case a.quitChan <- true:
	default:
	}
}

func didDeviceSwitchToManualMode(previousMode, currentMode string) bool {
	if currentMode != "挡位工作模式" {
		return false
	}
	if previousMode == "" {
		return false
	}
	return previousMode != currentMode
}

// onFanDataUpdate 风扇数据更新回调
func (a *CoreApp) onFanDataUpdate(fanData *types.FanData) {
	if fanData == nil {
		return
	}

	// 本回调运行在 HID 读协程上，任何阻塞都会让读协程收不到正在等待的命令 ACK，
	// 发送方只能等到超时失败。因此这里不调用会拿设备锁的 IsConnected（设备层刚确认过
	// 连接有效），不拿 a.mutex 写锁，也不深拷贝配置；只有真正检测到模式切换这种罕见情况，
	// 才把读配置、落盘和广播交给后台协程。
	//
	// A newly opened HID handle can deliver the device's default gear-mode
	// report before the connection has passed its readiness gate. Do not treat
	// that bootstrap report as a user-requested mode transition.
	a.mutex.RLock()
	coreConnected := a.isConnected
	a.mutex.RUnlock()
	controlReady := automaticControlAllowed(a.stopping.Load(), a.systemSuspended.Load(), coreConnected, true)
	if controlReady && a.rtssPublisher != nil {
		a.rtssPublisher.Publish(fanData.CurrentRPM)
	}

	a.deviceModeMu.Lock()
	deviceSwitchedToManual := didDeviceSwitchToManualMode(a.lastDeviceMode, fanData.WorkMode)
	userSetAutoControl := a.userSetAutoControl
	a.lastDeviceMode = fanData.WorkMode
	a.userSetAutoControl = false
	a.deviceModeMu.Unlock()

	if deviceSwitchedToManual && controlReady && !userSetAutoControl {
		a.safeGo("device-switched-to-manual", a.handleDeviceSwitchedToManual)
	}

	// 广播风扇数据更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFanDataUpdate, fanData)
	}
}

// handleDeviceSwitchedToManual 处理用户在设备上按键切回挡位模式。
// 如果开启了"断连保持配置模式"，则忽略设备状态变化，避免误判。
func (a *CoreApp) handleDeviceSwitchedToManual() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	ignoreDevice := false
	cfg, changed := a.configManager.Mutate(func(cfg *types.AppConfig) bool {
		if !cfg.AutoControl {
			return false
		}
		if cfg.IgnoreDeviceOnReconnect {
			ignoreDevice = true
			return false
		}
		cfg.AutoControl = false
		return true
	})
	if ignoreDevice {
		a.logInfo("检测到设备模式变化，但已开启断连保持配置模式，保持APP配置不变")
		return
	}
	if !changed {
		return
	}

	a.logInfo("检测到设备切换到挡位工作模式，自动关闭智能变频")
	if err := a.configManager.Save(); err != nil {
		a.logError("保存配置失败: %v", err)
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
}

// resetDeviceModeTracking 在连接失效时清空上一次的设备工作模式：
// 否则重连后的首个挡位模式上报会被拿去和旧连接的"实时转速模式"比较，被误判为用户手动切挡。
func (a *CoreApp) resetDeviceModeTracking() {
	a.deviceModeMu.Lock()
	a.lastDeviceMode = ""
	a.deviceModeMu.Unlock()
}

// onDeviceDisconnect 设备断开回调
func (a *CoreApp) onDeviceDisconnect() {
	if a.rtssPublisher != nil {
		a.rtssPublisher.Close()
	}

	a.mutex.Lock()
	wasConnected := a.isConnected
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	a.resetDeviceModeTracking()

	if wasConnected {
		// 刚断开：健康检查的重连兜底回到快速节奏，瞬时掉线仍能像以前一样尽快恢复。
		a.healthReconnectStreak.Store(0)
		a.logInfo("设备连接已断开，将在健康检查时尝试自动重连")
		a.recordTimelineEvent(types.TimelineEventTypeDisconnect, types.TimelineKeyDeviceDisconnected)
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	if a.autoReconnectSuppressed.Load() {
		a.logInfo("设备已手动断开，跳过自动重连")
		return
	}

	a.requestReconnect("device-disconnect", nil)
}

func defaultReconnectDelays() []time.Duration {
	return []time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
		20 * time.Second,
		30 * time.Second,
		30 * time.Second,
		60 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
}

func cloneReconnectDelays(delays []time.Duration) []time.Duration {
	if len(delays) == 0 {
		return defaultReconnectDelays()
	}
	cloned := make([]time.Duration, len(delays))
	copy(cloned, delays)
	return cloned
}

func (a *CoreApp) requestReconnect(reason string, retryDelays []time.Duration) {
	a.requestReconnectWithLogging(reason, retryDelays, false)
}

// requestReconnectWithLogging 与 requestReconnect 相同；quiet=true 时把重连流程中
// 例行的进度/失败日志降到 Debug。用于设备长期不在时健康检查的反复重试，
// 避免每 30 秒写一组相同的错误日志。
func (a *CoreApp) requestReconnectWithLogging(reason string, retryDelays []time.Duration, quiet bool) {
	if a.shutdownInProgress.Load() {
		return
	}
	if a.autoReconnectSuppressed.Load() {
		if quiet {
			a.logDebug("自动重连已被手动断开抑制，忽略请求: %s", reason)
			return
		}
		a.logInfo("自动重连已被手动断开抑制，忽略请求: %s", reason)
		return
	}

	a.reconnectMutex.Lock()
	if !a.reconnectInProgress.CompareAndSwap(false, true) {
		a.reconnectMutex.Unlock()
		a.logDebug("重连流程已在进行中，忽略新的请求: %s", reason)
		return
	}
	cancel := make(chan struct{})
	done := make(chan struct{})
	wake := make(chan struct{}, 1)
	a.reconnectCancel = cancel
	a.reconnectDone = done
	a.reconnectWake = wake
	a.reconnectMutex.Unlock()

	delays := cloneReconnectDelays(retryDelays)
	a.safeGo("reconnect@"+reason, func() {
		defer a.finishReconnect(cancel, done, wake)
		a.runReconnectLoop(reason, delays, cancel, wake, quiet)
	})
}

// cancelReconnect 会立即唤醒正在退避等待的重连流程。
// 正在执行底层连接调用时无法强行打断；其返回后会检查取消信号并退出。
func (a *CoreApp) cancelReconnect() {
	a.reconnectMutex.Lock()
	if a.reconnectCancel != nil {
		close(a.reconnectCancel)
		a.reconnectCancel = nil
	}
	a.reconnectMutex.Unlock()
}

func (a *CoreApp) finishReconnect(cancel, done, wake chan struct{}) {
	a.reconnectMutex.Lock()
	if a.reconnectCancel == cancel {
		a.reconnectCancel = nil
	}
	if a.reconnectDone == done {
		a.reconnectDone = nil
	}
	if a.reconnectWake == wake {
		a.reconnectWake = nil
	}
	a.reconnectInProgress.Store(false)
	// 唤醒恢复可能正等待此通道；先在互斥保护下清除进行中标记，再通知等待者，
	// 以免新请求被旧流程的 reconnectInProgress=true 错误拒绝。
	close(done)
	a.reconnectMutex.Unlock()
}

// requestReconnectAfterCurrent 确保唤醒恢复不会被休眠前遗留的重连流程吞掉。
// 旧流程已通过 cancelReconnect 收到取消信号；这里等待它释放设备操作后再启动恢复重连，
// 避免两个 goroutine 同时连接同一 HID/BLE 设备。
func (a *CoreApp) requestReconnectAfterCurrent(reason string, retryDelays []time.Duration) {
	a.reconnectMutex.Lock()
	done := a.reconnectDone
	a.reconnectMutex.Unlock()

	if done == nil {
		a.requestReconnect(reason, retryDelays)
		return
	}

	a.safeGo("reconnect-after-current@"+reason, func() {
		<-done
		a.requestReconnect(reason, retryDelays)
	})
}

func reconnectDelayElapsed(delay time.Duration, cancel, wake <-chan struct{}) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-cancel:
		return false
	case <-wake:
		return true
	}
}

func (a *CoreApp) wakeReconnect() bool {
	a.reconnectMutex.Lock()
	wake := a.reconnectWake
	a.reconnectMutex.Unlock()
	if wake == nil {
		return false
	}
	select {
	case wake <- struct{}{}:
	default:
	}
	return true
}

// runReconnectLoop 安排设备重连
func (a *CoreApp) runReconnectLoop(reason string, retryDelays []time.Duration, cancel, wake <-chan struct{}, quiet bool) {
	logProgress, logFailure := a.logInfo, a.logError
	if quiet {
		logProgress, logFailure = a.logDebug, a.logDebug
	}
	logProgress("启动设备重连流程: %s", reason)

	for i, delay := range retryDelays {
		select {
		case <-cancel:
			a.logInfo("重连流程已取消: %s", reason)
			return
		default:
		}

		if a.autoReconnectSuppressed.Load() {
			a.logInfo("自动重连已被手动断开抑制，停止重连流程: %s", reason)
			return
		}

		// 检查是否已经连接（可能其他途径已重连）
		a.mutex.RLock()
		connected := a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}

		if delay > 0 {
			logProgress("等待 %v 后尝试第 %d 次重连...", delay, i+1)
			if !reconnectDelayElapsed(delay, cancel, wake) {
				a.logInfo("重连流程已取消: %s", reason)
				return
			}
		}

		select {
		case <-cancel:
			a.logInfo("重连流程已取消: %s", reason)
			return
		default:
		}

		if a.autoReconnectSuppressed.Load() {
			a.logInfo("自动重连已被手动断开抑制，停止重连流程: %s", reason)
			return
		}

		// 再次检查连接状态
		a.mutex.RLock()
		connected = a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}

		logProgress("尝试第 %d 次重连设备...", i+1)
		if a.connectDevice(false) {
			a.logInfo("设备重连成功")

			// 如果开启了断连保持配置模式，重新应用APP配置
			cfg := a.configManager.Get()
			if cfg.IgnoreDeviceOnReconnect {
				a.logInfo("断连保持配置模式已开启，重新应用APP配置")
				a.reapplyConfigAfterReconnect()
			}

			return
		}
		logFailure("第 %d 次重连失败", i+1)
	}

	logFailure("所有重连尝试均失败，等待下次健康检查")
}

// notifyHIDUnresponsive 在 BS1 的 USB 通道"设备在系统里但不再通信"时提醒用户。
//
// 这种状态软件重连永远修不好：实测新句柄、旧句柄、连 0x05 握手都无应答，
// 只有拔插 USB 线或重启系统能恢复。
//
// 只在用户手动点「连接」失败时调用。不做自动提醒是因为时机无法把握：自动重连
// 发生在刚唤醒、桌面还没出来的时候，实测通知会落在锁屏阶段，用户根本看不见
// （只能事后去通知中心翻）。与其猜"用户什么时候在看屏幕"，不如等他主动来点——
// 那时的反馈一定落在眼前。
func (a *CoreApp) notifyHIDUnresponsive() {
	if !a.deviceManager.HIDUnresponsive() {
		return
	}
	a.logError("BS1 USB 通道无响应：设备仍在系统中但不再通信，已提示用户重新插拔 USB-C 线")
	if a.notifier == nil {
		return
	}
	a.notifier.Notify(
		coremsg.T(coremsg.NotifyDeviceUnresponsiveTitle),
		coremsg.T(coremsg.NotifyDeviceUnresponsiveBody),
	)
}

func shouldReconnectAfterResume(proactivelySuspended, resumeReconnectWanted, autoReconnectSuppressed, forceReconnect bool) bool {
	if proactivelySuspended {
		return resumeReconnectWanted
	}
	return forceReconnect && !autoReconnectSuppressed
}

// resumeReconnectWantedOnSuspend 判断休眠时是否应记住“唤醒后自动重连”的意图。
//
// coreConnected/deviceConnected 反映当前实时连接状态；reconnectInProgress 表示上一次
// 唤醒排定的重连仍在退避等待中——此时设备虽未连上，但用户确实希望设备保持连接，
// 必须视为“已连接”，否则紧接着的再次休眠会把意图误清成 false，最终唤醒便会当作
// 用户手动断开而放弃重连。autoReconnectSuppressed 为真表示用户已手动断开，任何情况
// 下都不应记住重连意图。
func resumeReconnectWantedOnSuspend(coreConnected, deviceConnected, reconnectInProgress, autoReconnectSuppressed bool) bool {
	if autoReconnectSuppressed {
		return false
	}
	return coreConnected || deviceConnected || reconnectInProgress
}

func (a *CoreApp) maybeRecoverFromSystemResume(source string, gap, expectedInterval time.Duration) bool {
	if !shouldRecoverFromSystemResumeGap(gap, expectedInterval) {
		return false
	}
	// 收到过挂起通知、还没等到用户在场的唤醒时，计时间隔变长不代表用户回来了：
	// 现代待机与无人值守的自动唤醒期间进程也会被零星调度。此时按间隔"恢复"会重连
	// 设备、启动智能控温，风扇就在睡眠中转起来。只有确认用户有过输入才恢复，
	// 其余交给电源通知（或 waitForUserAfterAutomaticResume）处理。
	if a.systemSuspended.Load() && !a.userInputSinceSuspend() {
		a.logDebug("挂起期间检测到计时间隔 %s（来源=%s），用户尚未操作，继续等待唤醒通知", gap.Round(time.Second), source)
		return true
	}
	a.triggerResumeRecovery(source, gap, false)
	return true
}

// userInputSinceSuspend 判断收到挂起通知之后用户是否有过键盘/鼠标输入。
// 拿不到输入时刻（非 Windows 或调用失败）时返回 false。
func (a *CoreApp) userInputSinceSuspend() bool {
	if !a.hasSuspendInputTick.Load() {
		return false
	}
	last, ok := lastUserInputTick()
	if !ok {
		return false
	}
	return tickAfter(last, a.suspendInputTick.Load())
}

// tickAfter 判断 GetTickCount 时钟上 a 是否晚于 b，按差值的符号比较以容忍约 49.7 天的回绕。
func tickAfter(a, b uint32) bool {
	return int32(a-b) > 0
}

// onSystemSuspend 收到系统挂起（睡眠/休眠）通知时调用。
func (a *CoreApp) onSystemSuspend() {
	// 自动唤醒后系统又睡回去时 systemSuspended 仍为 true，下面的 CAS 会直接返回；
	// 先让等待用户的观察协程失效，它不能在下一次睡眠里继续计时、到点恢复设备。
	a.automaticResumeGeneration.Add(1)
	if !a.systemSuspended.CompareAndSwap(false, true) {
		return
	}
	if tick, ok := currentInputTick(); ok {
		a.suspendInputTick.Store(tick)
		a.hasSuspendInputTick.Store(true)
	} else {
		a.hasSuspendInputTick.Store(false)
	}
	generation := a.suspendGeneration.Add(1)
	if a.rtssPublisher != nil {
		a.rtssPublisher.Close()
	}
	start := time.Now()
	a.logInfo("收到系统挂起通知：提前停止监控并断开设备/桥接，避免唤醒后失效句柄导致崩溃")
	// 趁进程还活着先记下挂起点：唤醒标记只能说明"这里恢复了"，配上挂起标记
	// 才能在图上把整段没有采样的空窗圈出来。
	a.recordTimelineEvent(types.TimelineEventTypeResume, types.TimelineKeySystemSuspended)

	a.mutex.RLock()
	coreConnected := a.isConnected
	a.mutex.RUnlock()
	a.resumeReconnectWanted.Store(resumeReconnectWantedOnSuspend(
		coreConnected,
		a.deviceManager.IsConnected(),
		a.reconnectInProgress.Load(),
		a.autoReconnectSuppressed.Load(),
	))
	a.cancelReconnect()
	a.connectGeneration.Add(1)
	a.suppressedBeforeSuspend.Store(a.autoReconnectSuppressed.Load())
	a.autoReconnectSuppressed.Store(true)
	// Make the core connection state unavailable before any asynchronous
	// cleanup starts. A monitor tick that races with the suspend callback will
	// now fail its readiness check instead of sending a realtime RPM command.
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	a.resetDeviceModeTracking()
	// Wait for an in-flight automatic/configuration write to finish. New writes
	// are blocked by systemSuspended above, so the suspend power-off/disconnect
	// sequence cannot be followed by a stale realtime RPM write.
	a.waitForDeviceControlIdle()
	a.stopTemperatureMonitoring()
	// 学习偏移的周期落盘间隔较长，挂起前补写一次：睡眠中电池耗尽或直接断电时内存里的结果会丢失。
	a.saveLearnedOffsetsIfDirty("suspend")

	done := make(chan struct{})
	a.safeGo("suspend-cleanup", func() {
		defer close(done)
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		// 断开前按配置关闭风扇与灯效，让设备在睡眠期间不转不亮；唤醒重连后由
		// reapplyDeviceConfigLocked 自动恢复。
		a.powerOffDeviceOutputsForSuspend()
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		a.safeRun("suspend-device-disconnect", func() {
			a.deviceManager.DisconnectSilently()
		})
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}
		a.safeRun("suspend-bridge-stop", func() {
			a.bridgeManager.Stop()
		})
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
		}
		// 把挂起标记和最后一段采样写盘：睡眠中电池耗尽或直接断电时，内存里的这些都会丢失。
		a.flushTemperatureHistory("suspend")
		a.logInfo("挂起前清理完成，耗时 %s", time.Since(start).Round(time.Millisecond))
	})

	select {
	case <-done:
	case <-time.After(suspendCleanupGrace):
		a.logError("挂起前清理超过 %s 仍未完成，转入后台继续执行，避免阻塞系统电源回调", suspendCleanupGrace)
	}
}

func (a *CoreApp) isCurrentSuspendGeneration(generation uint64) bool {
	return a.systemSuspended.Load() && a.suspendGeneration.Load() == generation && !a.stopping.Load()
}

// powerOffDeviceOutputsForSuspend 在系统挂起断开设备之前关闭风扇与灯效，让设备在睡眠
// 期间不转不亮。仅在配置开启且当前仍连接时执行；唤醒重连后 reapplyDeviceConfigLocked 会
// 按用户配置自动恢复（风扇走修复后的可靠起转路径）。尽力而为，任何失败都不阻塞挂起。
func (a *CoreApp) powerOffDeviceOutputsForSuspend() {
	if !a.configManager.Get().SuspendTurnOff {
		return
	}
	if !a.deviceManager.IsConnected() {
		return
	}
	a.logInfo("休眠：按配置关闭风扇与灯效，唤醒后自动恢复")
	a.safeRun("suspend-fan-off", func() { a.deviceManager.SetCustomFanSpeed(0) })
	// 灯效与挡位灯仅 BS2/BS2PRO/BS3PRO 支持。
	if !a.deviceManager.IsBS1() {
		a.safeRun("suspend-rgb-off", func() { a.deviceManager.SetRGBOff() })
		a.safeRun("suspend-gearlight-off", func() { a.deviceManager.SetGearLight(false) })
	}
}

// onSystemResume 收到系统唤醒通知时调用，触发设备与监控的恢复。
//
// userPresent=false 对应 PBT_APMRESUMEAUTOMATIC：计划任务、Windows 更新、网络唤醒
// 定时器等无人值守的自动唤醒也会发它，系统做完事几分钟后又睡回去。这时若照常重连并
// 启动智能控温，风扇会在"睡眠中"转起来，睡回去后还停在最后的转速。所以只有用户在场
// （PBT_APMRESUMESUSPEND 或检测到输入），或者系统持续醒着超过 automaticResumeGrace，
// 才恢复设备控制。
func (a *CoreApp) onSystemResume(userPresent bool) {
	if userPresent || !a.systemSuspended.Load() {
		// 没收到过挂起通知时无从判断，保持原来的立即恢复。
		a.automaticResumeGeneration.Add(1)
		if userPresent {
			a.logInfo("收到系统唤醒通知（用户在场）")
		} else {
			a.logInfo("收到系统唤醒通知")
		}
		a.triggerResumeRecovery("power-event", 0, true)
		return
	}
	a.logInfo("收到系统自动唤醒通知：可能是无人值守的定时唤醒，暂不重连设备，等待用户操作（持续唤醒超过 %s 后再恢复）", automaticResumeGrace)
	generation := a.automaticResumeGeneration.Add(1)
	a.activeAutomaticWake.Store(generation)
	a.safeGo("automatic-resume-wait", func() {
		a.waitForUserAfterAutomaticResume(generation)
	})
}

// waitForUserAfterAutomaticResume 在自动唤醒后等待用户出现。
//
// 正常情况下用户在场时 PBT_APMRESUMESUSPEND 紧随而来，会让本协程失效；这里的输入
// 检测与超时只是兜底，防止某些机器（例如现代待机）不发该通知导致设备一直不恢复。
func (a *CoreApp) waitForUserAfterAutomaticResume(generation uint64) {
	start := time.Now()
	ticker := time.NewTicker(automaticResumePollInterval)
	defer ticker.Stop()
	// 蓝牙可能在唤醒前就已重连（HID 到达通知不会再来），先主动压一次风扇；失败（设备
	// 还没出现）就隔一段时间再试，直到成功一次为止。之后的断线重连由 HID 到达通知负责。
	held := false
	var nextHoldAt time.Time
	for range ticker.C {
		if a.stopping.Load() || a.automaticResumeGeneration.Load() != generation || !a.systemSuspended.Load() {
			return
		}
		if !held && !time.Now().Before(nextHoldAt) {
			held = a.holdFanQuietAfterAutomaticWake(generation, "automatic-resume")
			nextHoldAt = time.Now().Add(quietHoldRetryInterval)
		}
		if a.userInputSinceSuspend() {
			a.logInfo("自动唤醒后检测到用户操作，开始恢复设备")
			a.triggerResumeRecovery("power-event-user-input", 0, true)
			return
		}
		if time.Since(start) >= automaticResumeGrace {
			a.logInfo("自动唤醒后系统已持续运行 %s，恢复设备控制", automaticResumeGrace)
			a.triggerResumeRecovery("power-event-automatic", 0, true)
			return
		}
	}
}

func (a *CoreApp) onSupportedHIDArrival(devicePath string) {
	if a.stopping.Load() {
		return
	}
	if a.systemSuspended.Load() {
		// 自动唤醒期间 Windows 会自己重连已配对的蓝牙散热器，固件一连上就会起转。
		// 这里只把风扇压住，不做常规重连，常规恢复要等用户回来。
		generation := a.automaticResumeGeneration.Load()
		a.safeGo("automatic-wake-quiet-hold@hid-arrival", func() {
			a.holdFanQuietAfterAutomaticWake(generation, "hid-arrival")
		})
		return
	}
	if a.autoReconnectSuppressed.Load() {
		return
	}
	a.mutex.RLock()
	connected := a.isConnected
	a.mutex.RUnlock()
	if connected || a.deviceManager.IsConnected() {
		return
	}

	a.logInfo("检测到飞智 HID 接口已就绪，立即触发设备重连: %s", devicePath)
	// 接口刚出现时设备可能还没准备好，这次重连若失败，后续健康检查按快速节奏继续重试。
	a.healthReconnectStreak.Store(0)
	if !a.wakeReconnect() {
		a.requestReconnect("hid-interface-arrival", []time.Duration{0})
	}
}

// triggerResumeRecovery 以节流方式触发唤醒恢复，避免电源事件与基于时间间隔的检测重复执行。
func (a *CoreApp) triggerResumeRecovery(source string, gap time.Duration, forceReconnect bool) {
	nowUnix := time.Now().UnixNano()
	lastUnix := atomic.LoadInt64(&a.lastResumeRecoveryUnix)
	if lastUnix > 0 && time.Duration(nowUnix-lastUnix) < systemResumeRecoveryCooldown {
		a.refreshTrayAfterResume()
		return
	}
	if !a.resumeRecoveryRunning.CompareAndSwap(false, true) {
		a.refreshTrayAfterResume()
		return
	}
	atomic.StoreInt64(&a.lastResumeRecoveryUnix, nowUnix)

	a.safeGo("systemResumeRecovery@"+source, func() {
		defer a.resumeRecoveryRunning.Store(false)
		a.handleSystemResume(source, gap, forceReconnect)
	})
}

func (a *CoreApp) handleSystemResume(source string, gap time.Duration, forceReconnect bool) {
	start := time.Now()
	defer func() {
		a.logInfo("系统唤醒恢复流程结束（来源=%s，耗时=%s）", source, time.Since(start).Round(time.Millisecond))
	}()

	// 若之前收到过挂起通知，休眠清理可能仍在后台阻塞。先使该清理失效，避免它在
	// 新连接建立后再停止桥接或覆盖连接状态。
	proactivelySuspended := a.systemSuspended.Swap(false)
	resumeReconnectWanted := a.resumeReconnectWanted.Swap(false)
	if proactivelySuspended {
		a.suspendGeneration.Add(1)
	}
	a.cancelReconnect()
	// 唤醒后设备可能需要一段时间重新枚举，健康检查的重连兜底先回到快速节奏。
	a.healthReconnectStreak.Store(0)
	forceReconnect = shouldReconnectAfterResume(
		proactivelySuspended,
		resumeReconnectWanted,
		a.autoReconnectSuppressed.Load(),
		forceReconnect,
	)

	a.logInfo("检测到系统从睡眠/休眠恢复，来源=%s，挂起时长约=%s，主动挂起=%v，开始执行连接自愈",
		source, gap.Round(time.Second), proactivelySuspended)
	a.recordTimelineEvent(types.TimelineEventTypeResume, types.TimelineKeyResumeFromSleep)

	// 唤醒后 Explorer 可能重启或通知区域被重建，主动刷新托盘图标避免图标丢失/无响应。
	a.refreshTrayAfterResume()

	wasConnected := a.deviceManager.IsConnected()
	a.mutex.RLock()
	if !wasConnected {
		wasConnected = a.isConnected
	}
	a.mutex.RUnlock()
	// 休眠前用户已手动断开时，即使旧 HID 监控协程尚未退出而仍报“已连接”，
	// 也不能据此重新连接。
	if (proactivelySuspended && !resumeReconnectWanted) ||
		(!proactivelySuspended && a.autoReconnectSuppressed.Load()) {
		wasConnected = false
	}

	// 桥接停止与设备断开都涉及外部进程/cgo 调用，唤醒后句柄可能失效，统一兜底防止崩溃。
	a.safeRun("resume-bridge-stop", func() {
		a.bridgeManager.Stop()
	})

	// 主动挂起时温度监控已停止，唤醒后需重新启动（与设备连接解耦）。
	if proactivelySuspended {
		// 挂起时置位抑制只是为了挡住休眠期间的重连，唤醒后恢复挂起前的状态。
		// 只在"需要重连"时才清除的话，休眠时散热器本来就没接的情况会永久失去自动重连：
		// 唤醒后再插上，HID 到达与健康检查的重连都会被当作"已手动断开"忽略。
		a.autoReconnectSuppressed.Store(a.suppressedBeforeSuspend.Load() && !resumeReconnectWanted)
		a.ensureTemperatureMonitoring("system-resume")
	}

	if !wasConnected && !forceReconnect {
		a.logInfo("系统恢复时设备原本未连接，仅重置桥接状态")
		return
	}

	// 自动唤醒期间的压风扇可能还在收尾：systemSuspended 已清，它很快会放弃并断开自己
	// 打开的连接。等它结束再开始恢复，免得它的断开把下面新建的连接断掉。
	a.quietHoldMutex.Lock()
	a.quietHoldMutex.Unlock() //nolint:staticcheck // 仅作屏障
	a.safeRun("resume-device-disconnect", func() {
		a.deviceManager.DisconnectForRecovery()
	})
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	a.resetDeviceModeTracking()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	a.requestReconnectAfterCurrent("system-resume", []time.Duration{
		0,
		15 * time.Second,
		15 * time.Second,
		20 * time.Second,
		30 * time.Second,
		30 * time.Second,
		60 * time.Second,
		60 * time.Second,
	})
}

// refreshTrayAfterResume 在系统唤醒后刷新托盘图标。
//
// 由于唤醒后 Explorer/通知区域可能尚未完全恢复，这里立即刷新一次，并在数秒后再刷新一次，
// 以提高托盘图标恢复成功率。
func (a *CoreApp) refreshTrayAfterResume() {
	if a.trayManager == nil {
		return
	}
	a.trayManager.RefreshIcon()
	a.safeGo("resume-tray-refresh-delayed", func() {
		time.Sleep(5 * time.Second)
		a.trayManager.RefreshIcon()
	})
}

// safeRun 在当前协程内执行 fn，并捕获其 panic，避免影响调用方的后续清理流程。
func (a *CoreApp) safeRun(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			a.logError("[%s] 执行时发生panic，已恢复: %v", name, r)
		}
	}()
	fn()
}

// ConnectDevice handles a user-initiated connection request.
func (a *CoreApp) ConnectDevice() bool {
	ok := a.connectDevice(true)
	if !ok {
		// 用户主动点了「连接」却失败，立刻告诉他为什么、该怎么办。
		a.notifyHIDUnresponsive()
	}
	return ok
}

// connectDevice collapses concurrent IPC, startup, health-check, and reconnect
// requests into one physical connection attempt. A disconnect, suspend, or stop
// increments connectGeneration; an older attempt then discards its result rather
// than resurrecting a connection the caller explicitly cancelled.
func (a *CoreApp) connectDevice(manual bool) bool {
	if a.shutdownInProgress.Load() {
		return false
	}
	if manual {
		a.autoReconnectSuppressed.Store(false)
	}

	a.connectMutex.Lock()
	if done := a.connectDone; done != nil {
		a.connectMutex.Unlock()
		if manual {
			// 唤醒后设备不响应时，进行中的那次连接可能卡在系统 HID 调用里迟迟不返回。
			// 原先手动连接会无限期排队等它：用户怎么点「重新连接」都没反应，只能重启软件。
			// 现在最多等 manualConnectWaitTimeout，超时就如实报告失败。
			select {
			case <-done:
			case <-time.After(manualConnectWaitTimeout):
				a.logError("手动连接：上一次连接尝试已超过 %s 仍未返回（可能卡在系统 HID 调用），本次放弃等待", manualConnectWaitTimeout)
				return false
			}
		} else {
			<-done
		}
		a.connectMutex.Lock()
		result := a.connectResult
		a.connectMutex.Unlock()
		return result
	}

	generation := a.connectGeneration.Load()
	done := make(chan struct{})
	a.connectDone = done
	a.connectMutex.Unlock()

	// 连接尝试本身无法从外部打断（底层是 cgo/系统调用），至少在它迟迟不返回时留下日志，
	// 唤醒后"怎么都连不上"的反馈才有据可查。
	slowWarning := time.AfterFunc(slowConnectAttemptWarning, func() {
		a.logError("设备连接尝试已持续 %s 仍未返回，可能卡在打开/读取 HID 设备的系统调用中", slowConnectAttemptWarning)
	})
	success := a.connectDeviceOnce(generation, manual)
	slowWarning.Stop()

	a.connectMutex.Lock()
	a.connectResult = success
	if a.connectDone == done {
		a.connectDone = nil
	}
	close(done)
	a.connectMutex.Unlock()
	return success
}

func (a *CoreApp) connectDeviceOnce(generation uint64, manual bool) bool {
	var success bool
	var deviceInfo map[string]string
	if manual {
		success, deviceInfo = a.deviceManager.Connect()
	} else {
		success, deviceInfo = a.deviceManager.Reconnect()
	}
	if success && !a.isConnectionAttemptCurrent(generation) {
		a.logInfo("连接结果已过期，关闭新建立的设备连接")
		a.deviceManager.DisconnectSilently()
		return false
	}
	if success {
		settings, readyErr := a.waitForDeviceReady(generation)
		if readyErr != nil {
			a.logError("设备 HID 句柄已打开，但在就绪等待期内未收到有效响应：%v", readyErr)
			// Do not retain a half-initialized HID handle. Recovery disconnects
			// safely even if its reader is stuck, allowing the next retry to open
			// a fresh handle.
			a.deviceManager.DisconnectForRecovery()
			a.mutex.Lock()
			a.deviceSettings = nil
			a.mutex.Unlock()
			return false
		}
		// Serialize the final readiness check and Core state publication with the
		// suspend barrier. This closes the small window where suspend could begin
		// after the check but before isConnected is published.
		a.deviceControlMutex.Lock()
		if !a.isConnectionAttemptCurrent(generation) {
			a.deviceControlMutex.Unlock()
			a.logInfo("设备就绪结果已过期，关闭新建立的设备连接")
			a.deviceManager.DisconnectSilently()
			return false
		}
		a.mutex.Lock()
		a.isConnected = true
		if settings != nil {
			a.deviceSettings = settings
		}
		a.mutex.Unlock()
		a.deviceControlMutex.Unlock()

		// 只记当前转速：0xEF 里的"目标转速"是会话级回显，新会话恒为 0，
		// 拿它判断"上次关机停没停"会得出错误结论。转速是物理量，可信。
		if fanData := a.deviceManager.GetCurrentFanData(); fanData != nil {
			a.logInfo("连接时设备转速：%d RPM", fanData.CurrentRPM)
		}

		if !a.isConnectionAttemptCurrent(generation) {
			return false
		}

		// BS1 的灯光输出在关机时被关掉了，这里只重新打开输出开关，
		// 沿用设备 flash 里已经存好的灯效程序——不重传那 19 帧，也就不会多写一次闪存。
		// BS2/BS3 由下面的 applyConfiguredLightStrip 完整重放，不需要这一步。
		if a.deviceManager.IsBS1() && a.configManager.Get().LightStrip.Mode != "off" && a.lockDeviceControlIfReady() {
			if a.deviceManager.SetRGBOn() {
				a.logInfo("已恢复灯光输出")
			} else {
				a.logError("恢复灯光输出失败")
			}
			a.deviceControlMutex.Unlock()
		}

		a.persistLastDeviceTransport(a.deviceManager.GetDeviceType())

		if settings != nil && a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceSettingsUpdate, *settings)
		}
		if deviceInfo != nil && a.ipcServer != nil {
			eventPayload := map[string]any{}
			for key, value := range deviceInfo {
				eventPayload[key] = value
			}
			if settings != nil {
				eventPayload["deviceSettings"] = settings
			}
			a.ipcServer.BroadcastEvent(ipc.EventDeviceConnected, eventPayload)
		}
		// A spontaneous 0xEF often proves readiness before the detailed query is
		// attempted. In that path settings is nil and the GUI previously never
		// received firmwareVersion even though a manual debug query could read it.
		// Complete the readback after isConnected has been published so the normal
		// settings event updates both an already-open GUI and future status calls.
		if settings == nil || settings.FirmwareVersion == "" {
			a.safeGo("refreshDeviceSettings@connect", func() {
				time.Sleep(100 * time.Millisecond)
				if _, err := a.RefreshDeviceSettings(); err != nil {
					a.logError("连接后补充读取固件详细信息失败: %v", err)
				}
			})
		}
		a.recordTimelineEvent(types.TimelineEventTypeMode, types.TimelineKeyDeviceConnected)

		// BS1 不支持灯带。配置写入只能在连接已就绪时发生，
		// 以避免对休眠后仍在初始化的 HID 句柄发包。
		if !a.deviceManager.IsBS1() && a.lockDeviceControlIfReady() {
			if err := a.applyConfiguredLightStrip(); err != nil {
				a.logError("应用灯带配置失败: %v", err)
			}
			a.deviceControlMutex.Unlock()
		}
		// 本进程第一次连上设备时，把 App 侧的转速控制也下发一次（灯带上面已经发过）。
		if a.startupConfigApplied.CompareAndSwap(false, true) {
			a.applyStartupDeviceConfig(generation)
		}
		if a.isConnectionAttemptCurrent(generation) {
			a.ensureTemperatureMonitoring("device-connect")
		}
	} else if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "连接失败")
	}
	return success
}

// persistLastDeviceTransport 将本次成功连接的传输方式写入配置，
// 使自动重连的 BLE 扫描跳过/限频策略在进程重启后依然成立。
func (a *CoreApp) persistLastDeviceTransport(transport string) {
	if transport != types.DeviceTypeHID && transport != types.DeviceTypeBLE {
		return
	}
	cfg := a.configManager.Get()
	if cfg.LastDeviceTransport == transport {
		return
	}
	cfg.LastDeviceTransport = transport
	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		a.logError("保存上次设备连接方式失败: %v", err)
	}
}

// DisconnectDevice 断开设备连接
func (a *CoreApp) DisconnectDevice() {
	a.autoReconnectSuppressed.Store(true)
	a.connectGeneration.Add(1)
	a.cancelReconnect()

	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()

	a.deviceManager.DisconnectSilently()

	a.recordTimelineEvent(types.TimelineEventTypeDisconnect, types.TimelineKeyDeviceDisconnected)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// applyStartupDeviceConfig 在本进程首次连上设备后把 App 侧配置下发一次。
//
// 软件启动之前散热器一直按固件自己保留的状态运行，App 配置里的智能变频/挡位都还
// 没生效过。原先启动连接只下发灯带，转速则完全交给温度监控循环的下一拍——而开机
// 自启动时温度桥接往往还在启动，那几拍读不到温度就什么都不会下发，散热器要等十几
// 秒才被接管，表现就是"软件设置的智能控温开机没有自动下发"。
func (a *CoreApp) applyStartupDeviceConfig(generation uint64) {
	if a.configManager.Get().AutoControl && a.latestControlTemp() <= 0 {
		// 智能变频的目标转速必须由实测温度查曲线得出，等桥接出第一个有效读数
		// 再下发；等不到就按 automaticReplayTargetRPM 的曲线首点兜底。
		a.safeGo("applyStartupDeviceConfig@await-temperature", func() {
			a.awaitFirstControlTemp(startupControlTempWait)
			a.replayStartupDeviceConfig(generation)
		})
		return
	}
	a.replayStartupDeviceConfig(generation)
}

func (a *CoreApp) replayStartupDeviceConfig(generation uint64) {
	if !a.isConnectionAttemptCurrent(generation) {
		a.startupConfigApplied.Store(false)
		return
	}
	if !a.lockDeviceControlIfReady() {
		// 没能下发就把标记退回去，下一次连接成功时重试。
		a.startupConfigApplied.Store(false)
		a.logInfo("设备尚未就绪或正在挂起，跳过启动配置下发")
		return
	}
	defer a.deviceControlMutex.Unlock()

	// 灯带在连接成功时已经单独下发过，这里不再重复上传灯效帧。
	if err := a.replayDeviceConfigLocked("启动", false); err != nil {
		a.logError("启动后部分配置下发失败: %v", err)
	}
}

func (a *CoreApp) latestControlTemp() int {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.currentTemp.ControlTemp > 0 {
		return a.currentTemp.ControlTemp
	}
	return a.currentTemp.MaxTemp
}

// awaitFirstControlTemp 等待温度监控循环写入第一个有效读数，超时即返回。
func (a *CoreApp) awaitFirstControlTemp(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		if a.stopping.Load() || a.systemSuspended.Load() {
			return
		}
		if a.latestControlTemp() > 0 {
			return
		}
		if !time.Now().Before(deadline) {
			return
		}
		time.Sleep(startupControlTempPollInterval)
	}
}

// reapplyConfigAfterReconnect 重连后重新应用APP配置
func (a *CoreApp) reapplyConfigAfterReconnect() {
	if !a.lockDeviceControlIfReady() {
		a.logInfo("设备尚未就绪或正在挂起，跳过本次重连配置重放")
		return
	}
	defer a.deviceControlMutex.Unlock()

	if err := a.reapplyDeviceConfigLocked("重连"); err != nil {
		a.logError("重连后部分配置重放失败: %v", err)
	}
}

// reapplyDeviceConfigLocked reapplies every App-owned device setting. Callers
// must hold deviceControlMutex; this is shared by reconnect and firmware
// maintenance so 0x03/0x06 side effects cannot leak into the visible state.
func (a *CoreApp) reapplyDeviceConfigLocked(reason string) error {
	return a.replayDeviceConfigLocked(reason, true)
}

// replayDeviceConfigLocked 与 reapplyDeviceConfigLocked 相同，但允许跳过灯带：
// 启动路径已经在连接成功时单独下发过灯带，重复上传灯效帧毫无意义。
func (a *CoreApp) replayDeviceConfigLocked(reason string, includeLightStrip bool) error {
	cfg := a.configManager.Get()
	var replayErrors []error

	// 重新应用智能变频配置
	if cfg.AutoControl {
		targetRPM, exact := a.automaticReplayTargetRPM(cfg)
		if exact {
			a.logInfo("%s后恢复智能变频实时目标: %d RPM", reason, targetRPM)
		} else {
			a.logInfo("%s后按当前温度恢复智能变频目标: %d RPM", reason, targetRPM)
		}
		if !a.deviceManager.SetCustomFanSpeed(targetRPM) {
			replayErrors = append(replayErrors, fmt.Errorf("重新应用智能变频目标失败: %d RPM", targetRPM))
		}
	} else if cfg.CustomSpeedEnabled {
		// 重新应用自定义转速
		a.logInfo("重新应用自定义转速: %d RPM", cfg.CustomSpeedRPM)
		if !a.deviceManager.SetCustomFanSpeed(cfg.CustomSpeedRPM) {
			replayErrors = append(replayErrors, fmt.Errorf("重新应用自定义转速失败"))
		}
	} else {
		level := a.getRememberedManualLevel(cfg.ManualGear, cfg.ManualLevel)
		rpm := cfg.ResolveGearRPM(cfg.ManualGear, level)
		a.logInfo("重新应用手动挡位: %s %s (%d RPM)", cfg.ManualGear, level, rpm)
		if !a.deviceManager.SetManualGearRPM(cfg.ManualGear, level, rpm) {
			replayErrors = append(replayErrors, fmt.Errorf("重新应用手动挡位失败"))
		}
	}

	// 以下功能仅 BS2/BS2PRO 支持
	if !a.deviceManager.IsBS1() {
		// true 和 false 都必须重放，否则固件保留的旧状态会与 App 配置相反。
		a.logInfo("重新应用挡位灯: %t", cfg.GearLight)
		if !a.deviceManager.SetGearLight(cfg.GearLight) {
			replayErrors = append(replayErrors, fmt.Errorf("重新应用挡位灯失败"))
		}
		if !a.deviceManager.SetSmartStartStop(cfg.SmartStartStop) {
			replayErrors = append(replayErrors, fmt.Errorf("重新应用智能启停失败: %s", cfg.SmartStartStop))
		}

		if includeLightStrip {
			if err := a.applyConfiguredLightStrip(); err != nil {
				replayErrors = append(replayErrors, fmt.Errorf("重新应用灯带配置: %w", err))
			}
		}
	}

	// 重新应用通电自启动配置（BS1 和 BS2/BS2PRO 都支持）
	a.logInfo("重新应用通电自启动: %t", cfg.PowerOnStart)
	if !a.deviceManager.SetPowerOnStart(cfg.PowerOnStart) {
		replayErrors = append(replayErrors, fmt.Errorf("重新应用通电自启动失败"))
	}
	return errors.Join(replayErrors...)
}

func (a *CoreApp) automaticReplayTargetRPM(cfg types.AppConfig) (targetRPM int, exact bool) {
	// Before 0x03/0x06 the latest 0xEF target is the exact result of the full
	// automatic controller (including adaptive offsets and ramp limiting). The
	// maintenance notification is normally suppressed while the command lock is
	// held, so prefer this value and preserve a legitimate 0 RPM target.
	if fanData := a.deviceManager.GetCurrentFanData(); fanData != nil && fanData.CurrentMode&0x01 == 1 {
		return int(fanData.TargetRPM), true
	}
	a.mutex.RLock()
	controlTemp := a.currentTemp.ControlTemp
	if controlTemp <= 0 {
		controlTemp = a.currentTemp.MaxTemp
	}
	a.mutex.RUnlock()
	if controlTemp <= 0 {
		if len(cfg.FanCurve) > 0 {
			return cfg.FanCurve[0].RPM, false
		}
		defaultCurve := types.GetDefaultFanCurve()
		return defaultCurve[0].RPM, false
	}
	return temperature.CalculateTargetRPM(controlTemp, cfg.FanCurve), false
}

// GetDeviceStatus 获取设备状态
func (a *CoreApp) GetDeviceStatus() map[string]any {
	// 先拷出核心字段再调用设备 getter，不在持有 a.mutex 时去等设备锁：
	// 发送方持设备锁等待 ACK 时，这会把需要 a.mutex 的路径一起拖住。
	a.mutex.RLock()
	settings := a.deviceSettings
	connected := a.isConnected
	currentTemp := a.currentTemp
	a.mutex.RUnlock()

	productID := a.deviceManager.GetProductID()
	productIDHex := ""
	if productID != 0 {
		productIDHex = fmt.Sprintf("0x%04X", productID)
	}

	model := a.deviceManager.GetModelName()

	return map[string]any{
		"connected":      connected,
		"monitoring":     a.monitoringTemp.Load(),
		"currentData":    a.deviceManager.GetCurrentFanData(),
		"temperature":    currentTemp,
		"productId":      productIDHex,
		"model":          model,
		"deviceSettings": settings,
	}
}

func (a *CoreApp) RefreshDeviceSettings() (*types.DeviceSettings, error) {
	if !a.lockDeviceControlIfReady() {
		return nil, fmt.Errorf("设备尚未就绪或正在挂起")
	}
	defer a.deviceControlMutex.Unlock()

	settings, err := a.deviceManager.QueryDeviceSettings()
	if err != nil && !settings.Available {
		return nil, err
	}

	a.mutex.Lock()
	a.deviceSettings = &settings
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceSettingsUpdate, settings)
	}
	return &settings, err
}

// ReinitializeDeviceFirmware runs a maintenance command, restores every
// App-owned control/light setting affected by it, then returns verified readback.
// 0x03 selects fixed gear 1 on first initialization; 0x06 additionally resets
// the four RPM slots and rebuilds runtime, startup and LED/RGB state.
func (a *CoreApp) ReinitializeDeviceFirmware(factoryReset bool) (*types.DeviceSettings, error) {
	if !a.lockDeviceControlIfReady() {
		return nil, fmt.Errorf("设备尚未就绪或正在挂起")
	}
	defer a.deviceControlMutex.Unlock()

	var err error
	if factoryReset {
		err = a.deviceManager.FactoryReinitializeFirmware()
	} else {
		err = a.deviceManager.InitializeFirmwareController()
	}
	if err != nil {
		return nil, err
	}
	replayErr := a.reapplyDeviceConfigLocked("固件维护")

	settings, queryErr := a.deviceManager.QueryDeviceSettings()
	if queryErr != nil && !settings.Available {
		return nil, queryErr
	}
	if replayErr != nil {
		settings.ReadErrors = append(settings.ReadErrors, "配置重放: "+replayErr.Error())
	}
	a.mutex.Lock()
	a.deviceSettings = &settings
	a.mutex.Unlock()
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceSettingsUpdate, settings)
	}
	return &settings, errors.Join(replayErr, queryErr)
}

// isConnectionAttemptCurrent keeps an old connect/recovery attempt from
// publishing state after suspend, shutdown, or an explicit disconnect.
func isConnectionAttemptCurrent(stopping, suspended, autoReconnectSuppressed bool, generation, currentGeneration uint64) bool {
	return !stopping && !suspended && !autoReconnectSuppressed && generation == currentGeneration
}

func (a *CoreApp) isConnectionAttemptCurrent(generation uint64) bool {
	return !a.shutdownInProgress.Load() && isConnectionAttemptCurrent(
		a.stopping.Load(),
		a.systemSuspended.Load(),
		a.autoReconnectSuppressed.Load(),
		generation,
		a.connectGeneration.Load(),
	)
}

// automaticControlAllowed is deliberately pure so the suspend/control gate can
// be tested without a physical HID device.
func automaticControlAllowed(stopping, suspended, coreConnected, deviceConnected bool) bool {
	return !stopping && !suspended && coreConnected && deviceConnected
}

// isDeviceControlReady distinguishes an opened HID handle from a connection
// that has completed the protocol readiness handshake.
func (a *CoreApp) isDeviceControlReady() bool {
	a.mutex.RLock()
	coreConnected := a.isConnected
	a.mutex.RUnlock()
	return automaticControlAllowed(
		a.stopping.Load(),
		a.systemSuspended.Load(),
		coreConnected,
		a.deviceManager.IsConnected(),
	) && !a.shutdownInProgress.Load()
}

// lockDeviceControlIfReady serializes all automatic/recovery writes with the
// suspend barrier. The caller must unlock deviceControlMutex on success.
func (a *CoreApp) lockDeviceControlIfReady() bool {
	a.deviceControlMutex.Lock()
	if a.isDeviceControlReady() {
		return true
	}
	a.deviceControlMutex.Unlock()
	return false
}

// waitForDeviceControlIdle forms the suspend barrier. Acquiring this mutex
// waits for an in-flight HID write/query to finish; the debug record makes the
// synchronization point explicit instead of leaving an empty critical
// section that static analysis cannot distinguish from a locking bug.
func (a *CoreApp) waitForDeviceControlIdle() {
	a.deviceControlMutex.Lock()
	defer a.deviceControlMutex.Unlock()
	a.logDebug("挂起前设备控制写入屏障已清空")
}

// setAutomaticFanSpeed makes the readiness check and the HID write one
// suspend-serialized operation. The first return value tells callers whether
// a write was attempted; the second is the device write result.
func (a *CoreApp) setAutomaticFanSpeed(rpm int) (ready, written bool) {
	if !a.lockDeviceControlIfReady() {
		return false, false
	}
	defer a.deviceControlMutex.Unlock()
	// 调用方依据的是本周期开头的配置快照。期间用户若已切到手动挡位/自定义转速，
	// 这次写入会把设备拉回实时模式，而下个周期因智能控温已关闭不会再管它，
	// 风扇就停在一个过期的实时转速上。模式切换都在本锁之外先改配置，这里复查即可。
	autoControl := false
	a.configManager.View(func(cfg *types.AppConfig) { autoControl = cfg.AutoControl })
	if !autoControl {
		return false, false
	}
	return true, a.deviceManager.SetFanSpeed(rpm)
}

// waitForDeviceReady requires protocol-level evidence before exposing a newly
// opened HID handle to the rest of Core. A valid status report is sufficient;
// otherwise a successful settings query with usable data proves readiness.
func (a *CoreApp) waitForDeviceReady(generation uint64) (*types.DeviceSettings, error) {
	return a.waitForDeviceReadyWhile(func() bool { return a.isConnectionAttemptCurrent(generation) })
}

// automaticWakeQuietHoldWanted 判断自动唤醒期间是否还需要把风扇压住：仍处于这一轮
// 等待用户的阶段、挂起前设备是连着的（用户没有手动断开），且上次走的是 HID——
// 只有系统托管的蓝牙/USB HID 才会被 Windows 自己重连；BS1 的 BLE 由 THRM 主动扫描
// 连接，自动唤醒时不会自己连上，也就无需去碰。
func (a *CoreApp) automaticWakeQuietHoldWanted(generation uint64) bool {
	return !a.stopping.Load() &&
		a.systemSuspended.Load() &&
		a.automaticResumeGeneration.Load() == generation &&
		generation != 0 && a.activeAutomaticWake.Load() == generation &&
		a.resumeReconnectWanted.Load() &&
		a.configManager.Get().LastDeviceTransport == types.DeviceTypeHID
}

// holdFanQuietAfterAutomaticWake 在无人值守的自动唤醒期间把风扇压到 0。
//
// 固件在蓝牙一连上就会自己起转，而 Windows 在自动唤醒时会主动重连已配对的散热器，
// 用户看到的就是"睡眠中风扇不定时转"。这里短暂打开 HID、等设备开始应答后下发 0 转，
// 然后断开（断开不会向设备发任何命令，风扇保持 0；系统睡回去蓝牙一断，固件按智能
// 启停自己停）。用户回来后常规的唤醒恢复会重连并按配置恢复控温。
func (a *CoreApp) holdFanQuietAfterAutomaticWake(generation uint64, reason string) bool {
	a.quietHoldMutex.Lock()
	defer a.quietHoldMutex.Unlock()

	if !a.automaticWakeQuietHoldWanted(generation) {
		return false
	}
	ok, _ := a.deviceManager.Reconnect()
	if !ok {
		a.logDebug("自动唤醒期间未连上设备（来源=%s），稍后重试", reason)
		return false
	}
	defer a.safeRun("automatic-wake-quiet-disconnect", a.deviceManager.DisconnectSilently)

	stillWanted := func() bool { return a.automaticWakeQuietHoldWanted(generation) }
	if _, err := a.waitForDeviceReadyWhile(stillWanted); err != nil {
		a.logError("自动唤醒期间设备未就绪，未能压住风扇（来源=%s）: %v", reason, err)
		return false
	}
	a.deviceControlMutex.Lock()
	defer a.deviceControlMutex.Unlock()
	if !stillWanted() {
		return false
	}
	if !a.deviceManager.SetCustomFanSpeed(0) {
		a.logError("自动唤醒期间下发 0 转失败（来源=%s）", reason)
		return false
	}
	a.logInfo("系统自动唤醒并重连了散热器（来源=%s），已把风扇压到 0，等用户回来再恢复控温", reason)
	return true
}

// waitForDeviceReadyWhile 等待刚打开的设备开始应答：收到状态帧，或设置查询返回有效数据。
// current 返回 false 时视为取消。
func (a *CoreApp) waitForDeviceReadyWhile(current func() bool) (*types.DeviceSettings, error) {
	timeout := time.NewTimer(deviceReadyTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(deviceReadyPollInterval)
	defer ticker.Stop()

	nextQueryAt := time.Now().Add(deviceReadyStatusGrace)
	var lastErr error
	for {
		if !current() || !a.deviceManager.IsConnected() {
			return nil, fmt.Errorf("连接尝试已被取消")
		}
		if a.deviceManager.GetCurrentFanData() != nil {
			return nil, nil
		}

		if !time.Now().Before(nextQueryAt) {
			a.deviceControlMutex.Lock()
			if !current() || !a.deviceManager.IsConnected() {
				a.deviceControlMutex.Unlock()
				return nil, fmt.Errorf("连接尝试已被取消")
			}
			settings, err := a.deviceManager.QueryDeviceSettings()
			a.deviceControlMutex.Unlock()
			if err == nil && settings.Available {
				return &settings, nil
			}
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("设备设置查询未返回有效数据")
			}
			nextQueryAt = time.Now().Add(deviceReadyQueryInterval)
		}

		select {
		case <-timeout.C:
			if lastErr != nil {
				return nil, fmt.Errorf("就绪超时: %w", lastErr)
			}
			return nil, fmt.Errorf("就绪超时，未收到有效状态帧")
		case <-ticker.C:
		}
	}
}

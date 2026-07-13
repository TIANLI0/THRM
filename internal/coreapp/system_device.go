package coreapp

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	deviceReadyTimeout       = 6 * time.Second
	deviceReadyStatusGrace   = 350 * time.Millisecond
	deviceReadyQueryInterval = 750 * time.Millisecond
	deviceReadyPollInterval  = 100 * time.Millisecond
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
	// A newly opened HID handle can deliver the device's default gear-mode
	// report before the connection has passed its readiness gate. Do not treat
	// that bootstrap report as a user-requested mode transition.
	controlReady := a.isDeviceControlReady()
	a.mutex.Lock()
	cfg := a.configManager.Get()
	deviceSwitchedToManual := didDeviceSwitchToManualMode(a.lastDeviceMode, fanData.WorkMode)

	// 检查工作模式变化
	// 如果开启了"断连保持配置模式"，则忽略设备状态变化，避免误判
	if deviceSwitchedToManual &&
		controlReady &&
		cfg.AutoControl &&
		!a.userSetAutoControl &&
		!cfg.IgnoreDeviceOnReconnect {

		a.logInfo("检测到设备切换到挡位工作模式，自动关闭智能变频")
		cfg.AutoControl = false

		a.configManager.Set(cfg)
		a.configManager.Save()

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	} else if deviceSwitchedToManual &&
		controlReady &&
		cfg.AutoControl &&
		!a.userSetAutoControl &&
		cfg.IgnoreDeviceOnReconnect {
		a.logInfo("检测到设备模式变化，但已开启断连保持配置模式，保持APP配置不变")
	}

	a.lastDeviceMode = fanData.WorkMode

	if a.userSetAutoControl {
		a.userSetAutoControl = false
	}

	a.mutex.Unlock()

	// 广播风扇数据更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFanDataUpdate, fanData)
	}
}

// onDeviceDisconnect 设备断开回调
func (a *CoreApp) onDeviceDisconnect() {
	a.mutex.Lock()
	wasConnected := a.isConnected
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()

	if wasConnected {
		a.logInfo("设备连接已断开，将在健康检查时尝试自动重连")
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
	if a.autoReconnectSuppressed.Load() {
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
	a.reconnectCancel = cancel
	a.reconnectDone = done
	a.reconnectMutex.Unlock()

	delays := cloneReconnectDelays(retryDelays)
	a.safeGo("reconnect@"+reason, func() {
		defer a.finishReconnect(cancel, done)
		a.runReconnectLoop(reason, delays, cancel)
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

func (a *CoreApp) finishReconnect(cancel, done chan struct{}) {
	a.reconnectMutex.Lock()
	if a.reconnectCancel == cancel {
		a.reconnectCancel = nil
	}
	if a.reconnectDone == done {
		a.reconnectDone = nil
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

func reconnectDelayElapsed(delay time.Duration, cancel <-chan struct{}) bool {
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
	}
}

// runReconnectLoop 安排设备重连
func (a *CoreApp) runReconnectLoop(reason string, retryDelays []time.Duration, cancel <-chan struct{}) {
	a.logInfo("启动设备重连流程: %s", reason)

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
			a.logInfo("等待 %v 后尝试第 %d 次重连...", delay, i+1)
			if !reconnectDelayElapsed(delay, cancel) {
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

		a.logInfo("尝试第 %d 次重连设备...", i+1)
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
		a.logError("第 %d 次重连失败", i+1)
	}

	a.logError("所有重连尝试均失败，等待下次健康检查")
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
	a.triggerResumeRecovery(source, gap, false)
	return true
}

// onSystemSuspend 收到系统挂起（睡眠/休眠）通知时调用。
func (a *CoreApp) onSystemSuspend() {
	if !a.systemSuspended.CompareAndSwap(false, true) {
		return
	}
	generation := a.suspendGeneration.Add(1)
	start := time.Now()
	a.logInfo("收到系统挂起通知：提前停止监控并断开设备/桥接，避免唤醒后失效句柄导致崩溃")

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
	a.autoReconnectSuppressed.Store(true)
	// Make the core connection state unavailable before any asynchronous
	// cleanup starts. A monitor tick that races with the suspend callback will
	// now fail its readiness check instead of sending a realtime RPM command.
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()
	// Wait for an in-flight automatic/configuration write to finish. New writes
	// are blocked by systemSuspended above, so the suspend power-off/disconnect
	// sequence cannot be followed by a stale realtime RPM write.
	a.waitForDeviceControlIdle()
	suspendFanOff := a.configManager.Get().SuspendFanOff
	// Windows may enter sleep as soon as this callback returns. Execute the
	// user-requested fan/light shutdown synchronously while the HID handle is
	// still valid; the remaining disconnect and bridge cleanup can continue in
	// the bounded background phase below.
	if suspendFanOff {
		a.safeRun("suspend-power-off", a.powerOffDeviceForSuspend)
	}
	a.stopTemperatureMonitoring()

	done := make(chan struct{})
	a.safeGo("suspend-cleanup", func() {
		defer close(done)
		if !a.isCurrentSuspendGeneration(generation) {
			return
		}

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

// powerOffDeviceForSuspend 在系统挂起前将风扇降到 0 转速，并(非 BS1)关闭挡位灯与 RGB。
// 唤醒重连后由 ConnectDevice/reapplyConfigAfterReconnect 重新应用转速、挡位灯与灯带配置。
func (a *CoreApp) powerOffDeviceForSuspend() {
	if !a.deviceManager.IsConnected() {
		return
	}

	a.logInfo("系统挂起前：风扇转速归零并关闭挡位灯/RGB")
	if !a.deviceManager.SetFanSpeed(0) {
		a.logError("挂起前归零转速失败")
	}

	// BS1 不支持挡位灯与 RGB 关闭。
	if a.deviceManager.IsBS1() {
		return
	}
	if !a.deviceManager.SetGearLight(false) {
		a.logError("挂起前关闭挡位灯失败")
	}
	if !a.deviceManager.SetRGBOff() {
		a.logError("挂起前关闭 RGB 失败")
	}
}

// onSystemResume 收到系统唤醒通知时调用，触发设备与监控的恢复。
func (a *CoreApp) onSystemResume() {
	a.logInfo("收到系统唤醒通知")
	a.triggerResumeRecovery("power-event", 0, true)
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
	forceReconnect = shouldReconnectAfterResume(
		proactivelySuspended,
		resumeReconnectWanted,
		a.autoReconnectSuppressed.Load(),
		forceReconnect,
	)

	a.logInfo("检测到系统从睡眠/休眠恢复，来源=%s，挂起时长约=%s，主动挂起=%v，开始执行连接自愈",
		source, gap.Round(time.Second), proactivelySuspended)

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
		if resumeReconnectWanted {
			a.autoReconnectSuppressed.Store(false)
		}
		a.ensureTemperatureMonitoring("system-resume")
	}

	if !wasConnected && !forceReconnect {
		a.logInfo("系统恢复时设备原本未连接，仅重置桥接状态")
		return
	}

	a.safeRun("resume-device-disconnect", func() {
		a.deviceManager.DisconnectForRecovery()
	})
	a.mutex.Lock()
	a.isConnected = false
	a.deviceSettings = nil
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	a.requestReconnectAfterCurrent("system-resume", []time.Duration{
		systemResumeReconnectDelay,
		8 * time.Second,
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
	return a.connectDevice(true)
}

// connectDevice collapses concurrent IPC, startup, health-check, and reconnect
// requests into one physical connection attempt. A disconnect, suspend, or stop
// increments connectGeneration; an older attempt then discards its result rather
// than resurrecting a connection the caller explicitly cancelled.
func (a *CoreApp) connectDevice(manual bool) bool {
	if manual {
		a.autoReconnectSuppressed.Store(false)
	}

	a.connectMutex.Lock()
	if done := a.connectDone; done != nil {
		a.connectMutex.Unlock()
		<-done
		a.connectMutex.Lock()
		result := a.connectResult
		a.connectMutex.Unlock()
		return result
	}

	generation := a.connectGeneration.Load()
	done := make(chan struct{})
	a.connectDone = done
	a.connectMutex.Unlock()

	success := a.connectDeviceOnce(generation)

	a.connectMutex.Lock()
	a.connectResult = success
	if a.connectDone == done {
		a.connectDone = nil
	}
	close(done)
	a.connectMutex.Unlock()
	return success
}

func (a *CoreApp) connectDeviceOnce(generation uint64) bool {
	success, deviceInfo := a.deviceManager.Connect()
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
		if !a.isConnectionAttemptCurrent(generation) {
			return false
		}

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

		// BS1 不支持灯带。配置写入只能在连接已就绪时发生，
		// 以避免对休眠后仍在初始化的 HID 句柄发包。
		if !a.deviceManager.IsBS1() && a.lockDeviceControlIfReady() {
			if err := a.applyConfiguredLightStrip(); err != nil {
				a.logError("应用灯带配置失败: %v", err)
			}
			a.deviceControlMutex.Unlock()
		}
		if a.isConnectionAttemptCurrent(generation) {
			a.ensureTemperatureMonitoring("device-connect")
		}
	} else if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "连接失败")
	}
	return success
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

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// reapplyConfigAfterReconnect 重连后重新应用APP配置
func (a *CoreApp) reapplyConfigAfterReconnect() {
	if !a.lockDeviceControlIfReady() {
		a.logInfo("设备尚未就绪或正在挂起，跳过本次重连配置重放")
		return
	}
	defer a.deviceControlMutex.Unlock()

	cfg := a.configManager.Get()

	// 重新应用智能变频配置
	if cfg.AutoControl {
		a.logInfo("重新启动智能变频")
	} else if cfg.CustomSpeedEnabled {
		// 重新应用自定义转速
		a.logInfo("重新应用自定义转速: %d RPM", cfg.CustomSpeedRPM)
		if !a.deviceManager.SetCustomFanSpeed(cfg.CustomSpeedRPM) {
			a.logError("重新应用自定义转速失败")
		}
	}

	// 以下功能仅 BS2/BS2PRO 支持
	if !a.deviceManager.IsBS1() {
		// 重新应用挡位灯配置
		if cfg.GearLight {
			a.logInfo("重新开启挡位灯")
			if !a.deviceManager.SetGearLight(true) {
				a.logError("重新开启挡位灯失败")
			}
		}

		if err := a.applyConfiguredLightStrip(); err != nil {
			a.logError("重连后重新应用灯带配置失败: %v", err)
		}
	}

	// 重新应用通电自启动配置（BS1 和 BS2/BS2PRO 都支持）
	if cfg.PowerOnStart {
		a.logInfo("重新开启通电自启动")
		if !a.deviceManager.SetPowerOnStart(true) {
			a.logError("重新开启通电自启动失败")
		}
	}
}

// GetDeviceStatus 获取设备状态
func (a *CoreApp) GetDeviceStatus() map[string]any {
	a.mutex.RLock()
	settings := a.deviceSettings
	defer a.mutex.RUnlock()

	productID := a.deviceManager.GetProductID()
	productIDHex := ""
	if productID != 0 {
		productIDHex = fmt.Sprintf("0x%04X", productID)
	}

	model := a.deviceManager.GetModelName()

	return map[string]any{
		"connected":      a.isConnected,
		"monitoring":     a.monitoringTemp.Load(),
		"currentData":    a.deviceManager.GetCurrentFanData(),
		"temperature":    a.currentTemp,
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

// isConnectionAttemptCurrent keeps an old connect/recovery attempt from
// publishing state after suspend, shutdown, or an explicit disconnect.
func isConnectionAttemptCurrent(stopping, suspended, autoReconnectSuppressed bool, generation, currentGeneration uint64) bool {
	return !stopping && !suspended && !autoReconnectSuppressed && generation == currentGeneration
}

func (a *CoreApp) isConnectionAttemptCurrent(generation uint64) bool {
	return isConnectionAttemptCurrent(
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
	)
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
	return true, a.deviceManager.SetFanSpeed(rpm)
}

// waitForDeviceReady requires protocol-level evidence before exposing a newly
// opened HID handle to the rest of Core. A valid status report is sufficient;
// otherwise a successful settings query with usable data proves readiness.
func (a *CoreApp) waitForDeviceReady(generation uint64) (*types.DeviceSettings, error) {
	timeout := time.NewTimer(deviceReadyTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(deviceReadyPollInterval)
	defer ticker.Stop()

	nextQueryAt := time.Now().Add(deviceReadyStatusGrace)
	var lastErr error
	for {
		if !a.isConnectionAttemptCurrent(generation) || !a.deviceManager.IsConnected() {
			return nil, fmt.Errorf("连接尝试已被取消")
		}
		if a.deviceManager.GetCurrentFanData() != nil {
			return nil, nil
		}

		if !time.Now().Before(nextQueryAt) {
			a.deviceControlMutex.Lock()
			if !a.isConnectionAttemptCurrent(generation) || !a.deviceManager.IsConnected() {
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

package coreapp

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
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
	a.mutex.Lock()
	cfg := a.configManager.Get()
	deviceSwitchedToManual := didDeviceSwitchToManualMode(a.lastDeviceMode, fanData.WorkMode)

	// 检查工作模式变化
	// 如果开启了"断连保持配置模式"，则忽略设备状态变化，避免误判
	if deviceSwitchedToManual &&
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

	if !a.reconnectInProgress.CompareAndSwap(false, true) {
		a.logDebug("重连流程已在进行中，忽略新的请求: %s", reason)
		return
	}

	delays := cloneReconnectDelays(retryDelays)
	a.safeGo("reconnect@"+reason, func() {
		defer a.reconnectInProgress.Store(false)
		a.runReconnectLoop(reason, delays)
	})
}

// runReconnectLoop 安排设备重连
func (a *CoreApp) runReconnectLoop(reason string, retryDelays []time.Duration) {
	a.logInfo("启动设备重连流程: %s", reason)

	for i, delay := range retryDelays {
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
			time.Sleep(delay)
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
		if a.ConnectDevice() {
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
	start := time.Now()
	a.logInfo("收到系统挂起通知：提前停止监控并断开设备/桥接，避免唤醒后失效句柄导致崩溃")

	a.autoReconnectSuppressed.Store(true)
	a.stopTemperatureMonitoring()

	suspendFanOff := a.configManager.Get().SuspendFanOff

	done := make(chan struct{})
	a.safeGo("suspend-cleanup", func() {
		defer close(done)

		// 断开设备前(句柄仍有效)先归零转速并关闭挡位灯/RGB，避免休眠期间风扇/灯光保持运行。
		if suspendFanOff {
			a.safeRun("suspend-power-off", a.powerOffDeviceForSuspend)
		}

		a.safeRun("suspend-device-disconnect", func() {
			a.deviceManager.DisconnectSilently()
		})
		a.mutex.Lock()
		a.isConnected = false
		a.mutex.Unlock()

		a.safeRun("suspend-bridge-stop", func() {
			a.bridgeManager.Stop()
		})

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

	// 若之前收到过挂起通知（主动断开），唤醒后必须强制重连，且需要重启已停止的温度监控。
	proactivelySuspended := a.systemSuspended.Swap(false)
	forceReconnect = forceReconnect || proactivelySuspended

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

	// 桥接停止与设备断开都涉及外部进程/cgo 调用，唤醒后句柄可能失效，统一兜底防止崩溃。
	a.safeRun("resume-bridge-stop", func() {
		a.bridgeManager.Stop()
	})

	// 主动挂起时温度监控已停止，唤醒后需重新启动（与设备连接解耦）。
	if proactivelySuspended {
		a.autoReconnectSuppressed.Store(false)
		a.safeGo("resume-temp-monitor", func() {
			a.startTemperatureMonitoring()
		})
	}

	if !wasConnected && !forceReconnect {
		a.logInfo("系统恢复时设备原本未连接，仅重置桥接状态")
		return
	}

	a.safeRun("resume-device-disconnect", func() {
		a.deviceManager.DisconnectSilently()
	})
	a.mutex.Lock()
	a.isConnected = false
	a.mutex.Unlock()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	a.requestReconnect("system-resume", []time.Duration{
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

// ConnectDevice 连接设备
func (a *CoreApp) ConnectDevice() bool {
	a.autoReconnectSuppressed.Store(false)

	success, deviceInfo := a.deviceManager.Connect()
	if success {
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()

		settings, settingsErr := a.RefreshDeviceSettings()
		if settingsErr != nil {
			a.logError("读取设备设置失败: %v", settingsErr)
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

		// BS1 不支持灯带
		if !a.deviceManager.IsBS1() {
			if err := a.applyConfiguredLightStrip(); err != nil {
				a.logError("应用灯带配置失败: %v", err)
			}
		}
		a.safeGo("startTemperatureMonitoring@ConnectDevice", func() {
			a.startTemperatureMonitoring()
		})
	} else if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "连接失败")
	}
	return success
}

// DisconnectDevice 断开设备连接
func (a *CoreApp) DisconnectDevice() {
	a.autoReconnectSuppressed.Store(true)

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

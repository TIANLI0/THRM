package coreapp

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/types"
)

func runtimeDebugInfo() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	toMB := func(value uint64) float64 {
		return float64(value) / (1024 * 1024)
	}

	lastGC := ""
	if mem.LastGC > 0 {
		lastGC = time.Unix(0, int64(mem.LastGC)).Format("2006-01-02 15:04:05")
	}

	return map[string]any{
		"goroutines":     runtime.NumGoroutine(),
		"allocMB":        toMB(mem.Alloc),
		"heapAllocMB":    toMB(mem.HeapAlloc),
		"heapInUseMB":    toMB(mem.HeapInuse),
		"heapIdleMB":     toMB(mem.HeapIdle),
		"heapReleasedMB": toMB(mem.HeapReleased),
		"stackInUseMB":   toMB(mem.StackInuse),
		"sysMB":          toMB(mem.Sys),
		"heapObjects":    mem.HeapObjects,
		"nextGCMB":       toMB(mem.NextGC),
		"numGC":          mem.NumGC,
		"lastGC":         lastGC,
		"gccpFraction":   mem.GCCPUFraction,
		"pauseTotalMs":   float64(mem.PauseTotalNs) / 1_000_000,
	}
}

// UpdateConfig 更新配置
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	oldCfg := a.configManager.Get()
	if len(cfg.FanCurveProfiles) == 0 && len(oldCfg.FanCurveProfiles) > 0 {
		cfg.FanCurveProfiles = curveprofiles.CloneProfiles(oldCfg.FanCurveProfiles)
		cfg.ActiveFanCurveProfileID = oldCfg.ActiveFanCurveProfileID
	}
	cfg.LegionFnQSupport = oldCfg.LegionFnQSupport
	cfg.ManualGearLevels = cloneManualGearLevels(oldCfg.ManualGearLevels)
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
	curveprofiles.NormalizeConfig(&cfg)
	if idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}
	syncSmartControlOffsetsForActiveProfile(&cfg)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	storeSmartControlOffsetsForActiveProfile(&cfg)
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
	if a.legionFnQSupportChecked.Load() && !a.legionFnQSupported.Load() && (cfg.LegionFnQ.Enabled || cfg.LegionFnQ.TakeOverFan) {
		return fmt.Errorf("Lenovo Legion Fn+Q 仅支持拯救者设备")
	}
	normalizeHotkeyConfig(&cfg)
	normalizeManualGearMemoryConfig(&cfg)
	types.NormalizeManualGearRPM(&cfg)

	cfg.ConfigPath = oldCfg.ConfigPath
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	a.syncManualGearLevelMemoryLocked(cfg)
	a.applyHotkeyBindings(cfg)
	a.applyPluginConfig(cfg)
	return nil
}

func (a *CoreApp) SetTemperatureHistoryEnabled(enabled bool) error {
	if err := a.tempHistory.SetEnabled(enabled); err != nil {
		return err
	}
	return nil
}

// SetFanCurve 设置风扇曲线
func (a *CoreApp) SetFanCurve(curve []types.FanCurvePoint) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if err := config.ValidateFanCurve(curve); err != nil {
		return err
	}

	cfg := a.configManager.Get()
	curveprofiles.NormalizeConfig(&cfg)
	cfg.FanCurve = curveprofiles.CloneCurve(curve)
	idx := curveprofiles.FindIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = curveprofiles.CloneCurve(cfg.FanCurve)
	}
	syncSmartControlOffsetsForActiveProfile(&cfg)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	storeSmartControlOffsetsForActiveProfile(&cfg)
	return a.configManager.Update(cfg)
}

// ResetLearnedOffsets 清空学习到的曲线偏移。
func (a *CoreApp) ResetLearnedOffsets() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.SmartControl = smartcontrol.ResetLearnedState(cfg.SmartControl, cfg.FanCurve)
	storeSmartControlOffsetsForActiveProfile(&cfg)
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	a.logInfo("已重置学习偏移")
	return nil
}

// SetAutoControl 设置智能变频
func (a *CoreApp) SetAutoControl(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled && cfg.CustomSpeedEnabled {
		return fmt.Errorf("自定义转速模式下无法开启智能变频")
	}

	cfg.AutoControl = enabled

	if enabled {
		a.userSetAutoControl = true
	}

	if !enabled && a.isConnected {
		a.safeGo("applyCurrentGearSetting", func() {
			a.applyCurrentGearSetting()
		})
	}

	a.configManager.Set(cfg)
	err := a.configManager.Save()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return err
}

// applyCurrentGearSetting 应用当前挡位设置
func (a *CoreApp) applyCurrentGearSetting() {
	fanData := a.deviceManager.GetCurrentFanData()
	if fanData == nil {
		return
	}

	cfg := a.configManager.Get()
	setGear := fanData.SetGear
	if setGear == "" {
		setGear = cfg.ManualGear
	}
	level := a.getRememberedManualLevel(setGear, cfg.ManualLevel)
	rpm := cfg.ResolveGearRPM(setGear, level)

	a.logInfo("应用当前挡位设置: %s %s (%d RPM)", setGear, level, rpm)
	a.deviceManager.SetManualGearRPM(setGear, level, rpm)
}

// SetManualGear 设置手动挡位
func (a *CoreApp) SetManualGear(gear, level string) bool {
	cfg := a.configManager.Get()
	cfg.ManualGear = gear
	cfg.ManualLevel = level
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
	}
	cfg.ManualGearLevels[gear] = normalizeManualLevel(level)
	types.NormalizeManualGearRPM(&cfg)
	rpm := cfg.ResolveGearRPM(gear, level)
	a.configManager.Update(cfg)
	a.rememberManualGearLevel(gear, level)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return a.deviceManager.SetManualGearRPM(gear, level, rpm)
}

// SetCustomSpeed 设置自定义转速
func (a *CoreApp) SetCustomSpeed(enabled bool, rpm int) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled {
		if cfg.AutoControl {
			cfg.AutoControl = false
		}

		cfg.CustomSpeedEnabled = true
		cfg.CustomSpeedRPM = rpm

		if a.isConnected {
			a.safeGo("setCustomFanSpeed", func() {
				a.deviceManager.SetCustomFanSpeed(rpm)
			})
		}
	} else {
		cfg.CustomSpeedEnabled = false
	}

	a.configManager.Set(cfg)
	err := a.configManager.Save()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return err
}

// SetGearLight 设置挡位灯
func (a *CoreApp) SetGearLight(enabled bool) bool {
	if !a.deviceManager.SetGearLight(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.GearLight = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetPowerOnStart 设置通电自启动
func (a *CoreApp) SetPowerOnStart(enabled bool) bool {
	if !a.deviceManager.SetPowerOnStart(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.PowerOnStart = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetSmartStartStop 设置智能启停
func (a *CoreApp) SetSmartStartStop(mode string) bool {
	if !a.deviceManager.SetSmartStartStop(mode) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.SmartStartStop = mode
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetBrightness 设置亮度
func (a *CoreApp) SetBrightness(percentage int) bool {
	if !a.deviceManager.SetBrightness(percentage) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.Brightness = percentage
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetLightStrip 设置灯带
func (a *CoreApp) SetLightStrip(lightCfg types.LightStripConfig) error {
	lightCfg, _ = normalizeLightStripConfig(lightCfg)

	cfg := a.configManager.Get()
	cfg.LightStrip = lightCfg
	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.isConnected {
		if err := a.deviceManager.SetLightStrip(lightCfg); err != nil {
			return err
		}
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
}

func (a *CoreApp) applyConfiguredLightStrip() error {
	cfg := a.configManager.Get()
	lightCfg, changed := normalizeLightStripConfig(cfg.LightStrip)

	if changed {
		cfg.LightStrip = lightCfg
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存灯带默认配置失败: %v", err)
		}
	}

	return a.deviceManager.SetLightStrip(lightCfg)
}

func normalizeLightStripConfig(cfg types.LightStripConfig) (types.LightStripConfig, bool) {
	defaults := types.GetDefaultLightStripConfig()
	changed := false

	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
		changed = true
	}
	if cfg.Speed == "" {
		cfg.Speed = defaults.Speed
		changed = true
	}
	if cfg.Brightness < 0 || cfg.Brightness > 100 {
		cfg.Brightness = defaults.Brightness
		changed = true
	}
	if len(cfg.Colors) == 0 {
		cfg.Colors = defaults.Colors
		changed = true
	}

	return cfg, changed
}

// SetWindowsAutoStart 设置Windows自启动
func (a *CoreApp) SetWindowsAutoStart(enable bool) error {
	err := a.autostartManager.SetWindowsAutoStart(enable)
	if err == nil {
		cfg := a.configManager.Get()
		cfg.WindowsAutoStart = enable
		a.configManager.Update(cfg)

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	}
	return err
}

// GetDebugInfo 获取调试信息
func (a *CoreApp) GetDebugInfo() map[string]any {
	info := map[string]any{
		"debugMode":               a.debugMode,
		"trayReady":               a.trayManager.IsReady(),
		"trayInitialized":         a.trayManager.IsInitialized(),
		"isConnected":             a.isConnected,
		"autoReconnectSuppressed": a.autoReconnectSuppressed.Load(),
		"legionFnQSupported":      a.legionFnQSupported.Load(),
		"guiLastResponse":         time.Unix(atomic.LoadInt64(&a.guiLastResponse), 0).Format("2006-01-02 15:04:05"),
		"monitoringTemp":          a.monitoringTemp.Load(),
		"autoStartLaunch":         a.isAutoStartLaunch,
		"hasGUIClients":           a.ipcServer != nil && a.ipcServer.HasClients(),
		"pawnIOInstallerPath":     appmeta.FirstExistingPath(appmeta.PawnIOInstallerCandidates(config.GetInstallDir())),
		"runtime":                 runtimeDebugInfo(),
	}
	if a.pluginManager != nil {
		info["plugins"] = a.pluginManager.Statuses()
	}
	return info
}

// SetDebugMode 设置调试模式
func (a *CoreApp) SetDebugMode(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.DebugMode = enabled
	syncSmartControlOffsetsForActiveProfile(&cfg)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, enabled)
	storeSmartControlOffsetsForActiveProfile(&cfg)
	a.debugMode = enabled

	if a.logger != nil {
		a.logger.SetDebugMode(enabled)
		if enabled {
			a.logger.Info("调试模式已开启，后续日志将包含调试级别")
		} else {
			a.logger.Info("调试模式已关闭，调试级别日志将被忽略")
		}
	}

	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
}

func (a *CoreApp) SendDeviceDebugCommand(hexCommand string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if !a.debugMode {
		return types.DeviceDebugCommandResult{}, fmt.Errorf("请先开启调试模式")
	}
	return a.deviceManager.SendDebugCommand(hexCommand, waitMs)
}

func (a *CoreApp) GetDeviceDebugFrames() []types.DeviceDebugFrame {
	return a.deviceManager.GetDebugFrames()
}

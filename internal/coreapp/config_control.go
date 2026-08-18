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
	// 飞智兼容开关由 SetFlydigiCompat 独占维护：它要连带改写设备安全描述符，
	// 让通用的 UpdateConfig 也能改会让配置和系统实际状态脱节。
	cfg.FlydigiCompat = oldCfg.FlydigiCompat
	cfg.ManualGearLevels = cloneManualGearLevels(oldCfg.ManualGearLevels)
	// 保留时长由专用接口独占维护，以记录器的实际状态为准：GUI 手上的配置副本可能还是
	// 改动之前的值，采信入参会让"改完保留时长后又动了别的设置"把用户的选择静默回退。
	cfg.TemperatureHistoryRetentionHours = a.historyRetentionHours(oldCfg)
	// 前端模型可能未携带按曲线方案记忆的映射字段，避免整表被清空
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = oldCfg.SmartControl.LearnedOffsetsByProfile
	}
	if cfg.SmartControl.TargetTempByProfile == nil {
		cfg.SmartControl.TargetTempByProfile = oldCfg.SmartControl.TargetTempByProfile
	}
	if cfg.SmartControl.LearningBiasByProfile == nil {
		cfg.SmartControl.LearningBiasByProfile = oldCfg.SmartControl.LearningBiasByProfile
	}
	cfg.RTSS = normalizeUpdatedRTSSConfig(cfg.RTSS, oldCfg.RTSS)
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	cfg.ThemeMode = types.NormalizeThemeMode(cfg.ThemeMode)
	cfg.TempSource = types.NormalizeTempSource(cfg.TempSource)
	cfg.GpuDevice = types.NormalizeDeviceSelection(cfg.GpuDevice)
	cfg.CpuSensor = types.NormalizeSensorSelection(cfg.CpuSensor)
	cfg.CpuSensors = types.NormalizeSensorSelections(cfg.CpuSensors)
	cfg.GpuSensor = types.NormalizeSensorSelection(cfg.GpuSensor)
	cfg.WindowBlur = types.NormalizeWindowBlur(cfg.WindowBlur)
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
	normalizeFanFeatureConfig(&cfg)

	cfg.ConfigPath = oldCfg.ConfigPath
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	a.syncManualGearLevelMemoryLocked(cfg)
	a.applyTrayVisibility(cfg)
	a.applyHotkeyBindings(cfg)
	a.applyPluginConfig(cfg)
	if a.rtssPublisher != nil {
		a.rtssPublisher.Configure(cfg.RTSS.Enabled, time.Duration(cfg.RTSS.UpdateIntervalMS)*time.Millisecond)
		a.rtssPublisher.SetPosition(cfg.RTSS.PositionMode, cfg.RTSS.PositionX, cfg.RTSS.PositionY)
	}
	return nil
}

func normalizeUpdatedRTSSConfig(next, previous types.RTSSConfig) types.RTSSConfig {
	// Older GUI clients do not send the nested RTSS object. Preserve the stored
	// settings instead of disabling OSD output when they update another field.
	if next.UpdateIntervalMS == 0 {
		next = previous
	}
	if next.PositionMode == "" {
		next.PositionMode = previous.PositionMode
		next.PositionX = previous.PositionX
		next.PositionY = previous.PositionY
	}
	next, _ = types.NormalizeRTSSConfig(next)
	return next
}

// PreviewRTSSPosition updates the active OSD cursor without persisting the
// configuration. The GUI uses it while a pointer or key is held, then commits
// the final coordinates through UpdateConfig once the interaction stops.
func (a *CoreApp) PreviewRTSSPosition(mode string, x, y int) {
	cfg := a.configManager.Get().RTSS
	cfg.PositionMode = mode
	cfg.PositionX = x
	cfg.PositionY = y
	cfg, _ = types.NormalizeRTSSConfig(cfg)
	if a.rtssPublisher != nil {
		a.rtssPublisher.SetPosition(cfg.PositionMode, cfg.PositionX, cfg.PositionY)
	}
}

func (a *CoreApp) SetTemperatureHistoryEnabled(enabled bool) error {
	if err := a.tempHistory.SetEnabled(enabled); err != nil {
		return err
	}
	return nil
}

// historyRetentionHours 返回当前生效的保留时长：优先取记录器的实际状态，
// 记录器缺失时（单测里直接构造的 CoreApp）退回配置里的值。
func (a *CoreApp) historyRetentionHours(cfg types.AppConfig) int {
	if a.tempHistory != nil {
		return a.tempHistory.RetentionHours()
	}
	return types.NormalizeTemperatureHistoryRetentionHours(cfg.TemperatureHistoryRetentionHours)
}

// SetTemperatureHistoryRetentionHours 调整温度历史后台保留时长(小时)并持久化到配置。
func (a *CoreApp) SetTemperatureHistoryRetentionHours(hours int) error {
	hours = types.NormalizeTemperatureHistoryRetentionHours(hours)
	if err := a.tempHistory.SetRetentionHours(hours); err != nil {
		return err
	}
	a.mutex.Lock()
	cfg := a.configManager.Get()
	cfg.TemperatureHistoryRetentionHours = hours
	err := a.configManager.Update(cfg)
	a.mutex.Unlock()

	// 广播出去，避免 GUI 手上的配置副本继续拿着旧值显示。
	if err == nil && a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return err
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

/* ── 自适应学习 2.0 ── */

// GetAdaptiveStatus 返回 2.0 的运行状态：倾向派生出的目标、模型学习进度、
// 以及当前（或预览的）自动曲线。
func (a *CoreApp) GetAdaptiveStatus() smartcontrol.AdaptiveStatus {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return smartcontrol.BuildAdaptiveStatus(a.configManager.Get().SmartControl)
}

// SetAdaptiveConfig 部分更新 2.0 配置（开关 / 倾向 / 安全红线）。
//
// 任何改动都会立刻重算一次自动曲线并写回配置：控温环最快也要到下一个采样周期
// 才会重算，而 GUI 在滑块松手的瞬间就要显示新曲线。这里同步算一遍既让界面即时
// 响应，也让重启后能从这条曲线继续，而不是退回种子曲线。
func (a *CoreApp) SetAdaptiveConfig(params ipc.SetAdaptiveConfigParams) (smartcontrol.AdaptiveStatus, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	adaptive := cfg.SmartControl.Adaptive

	if params.Enabled != nil {
		adaptive.Enabled = *params.Enabled
	}
	if params.Preference != nil {
		if *params.Preference < types.AdaptivePreferenceMin || *params.Preference > types.AdaptivePreferenceMax {
			return smartcontrol.AdaptiveStatus{}, fmt.Errorf("倾向取值需在 %d..%d 之间", types.AdaptivePreferenceMin, types.AdaptivePreferenceMax)
		}
		adaptive.Preference = *params.Preference
	}
	if params.TempLimit != nil {
		if *params.TempLimit < types.AdaptiveTempLimitMin || *params.TempLimit > types.AdaptiveTempLimitMax {
			return smartcontrol.AdaptiveStatus{}, fmt.Errorf("安全红线需在 %d..%d°C 之间", types.AdaptiveTempLimitMin, types.AdaptiveTempLimitMax)
		}
		adaptive.TempLimit = *params.TempLimit
	}

	adaptive, _ = smartcontrol.NormalizeAdaptiveConfig(adaptive)
	cfg.SmartControl.Adaptive = adaptive
	a.refreshAdaptiveCurve(&cfg)

	if err := a.configManager.Update(cfg); err != nil {
		return smartcontrol.AdaptiveStatus{}, err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return smartcontrol.BuildAdaptiveStatus(cfg.SmartControl), nil
}

// ResetAdaptiveModel 清空 2.0 学到的热模型，倾向与开关保持不变。
func (a *CoreApp) ResetAdaptiveModel() (smartcontrol.AdaptiveStatus, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.SmartControl.Adaptive = smartcontrol.ResetAdaptiveModel(cfg.SmartControl.Adaptive)
	a.refreshAdaptiveCurve(&cfg)

	if err := a.configManager.Update(cfg); err != nil {
		return smartcontrol.AdaptiveStatus{}, err
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	a.logInfo("已重置自适应学习 2.0 热模型")
	return smartcontrol.BuildAdaptiveStatus(cfg.SmartControl), nil
}

// refreshAdaptiveCurve 用当前配置重算自动曲线并写回。传 nil 作为上一版曲线：
// 这条路径只由显式的用户操作触发，应当一步到位，不受渐变限幅约束。
func (a *CoreApp) refreshAdaptiveCurve(cfg *types.AppConfig) {
	adaptive := cfg.SmartControl.Adaptive
	adaptive.AutoCurve = smartcontrol.SynthesizeAdaptiveCurve(
		adaptive.Model,
		smartcontrol.DeriveAdaptiveTuning(adaptive),
		cfg.SmartControl.NoiseProfile,
		nil,
	)
	adaptive.AutoCurveUpdatedAt = time.Now().Unix()
	cfg.SmartControl.Adaptive = adaptive
}

// SetAutoControl 设置智能变频
func (a *CoreApp) SetAutoControl(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled && cfg.CustomSpeedEnabled {
		return fmt.Errorf("自定义转速模式下无法开启智能变频")
	}

	changed := cfg.AutoControl != enabled
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

	if changed {
		a.recordSmartControlTimelineEvent(enabled)
	}

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

	// 配置的读-改-写必须在锁内完成，且 isConnected 也要在锁内取快照：
	// 托盘回调、快捷键与 IPC 请求都会并发调到这里，无锁时会丢更新。
	// 设备写入放到锁外，避免多帧 RGB 命令期间阻塞托盘状态刷新等其它持锁者。
	a.mutex.Lock()
	cfg := a.configManager.Get()
	cfg.LightStrip = lightCfg
	a.configManager.Set(cfg)
	saveErr := a.configManager.Save()
	connected := a.isConnected
	a.mutex.Unlock()

	if saveErr != nil {
		return saveErr
	}

	if connected {
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
		"trayEnabled":             a.trayManager.IsEnabled(),
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
	a.deviceManager.SetDebugCapture(enabled)

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

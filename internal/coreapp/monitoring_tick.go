package coreapp

import (
	"fmt"
	"math"
	"runtime/debug"
	"slices"
	"time"

	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/laptopfan"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

// temperatureMonitorState 汇集 startTemperatureMonitoring 循环里跨采样周期存活的状态。
// 它与一次监控会话同生共死，只由该会话的循环协程读写，不需要加锁。
// 定时器、lastTimerReset 与停止/订阅通道仍是 startTemperatureMonitoring 的局部变量。
type temperatureMonitorState struct {
	cfg              types.AppConfig
	cfgRevision      uint64
	hasClients       bool
	hasActiveClients bool
	updateInterval   time.Duration

	// 温度采样使用 EMA 平滑。
	sampleCount  int
	tempEMA      float64
	tempEMAReady bool

	rawTempHistory            []int
	recentAvgTemps            []int
	recentControlTemps        []int
	lastTargetRPM             int
	lastDeviceGeneration      uint64
	settingsRefreshGeneration uint64
	// 学习偏移每次合并都会立刻进入内存配置；这里只控制落盘与向 GUI 广播的频率。
	learningUnbroadcast     bool
	lastLearningSave        time.Time
	lastLearningBroadcast   time.Time
	lastLoggedTargetRPM     int
	lastControlLog          time.Time
	lastBridgeUpdateTime    int64
	staleBridgeUpdateCount  int
	bridgeFailureCount      int
	invalidControlTempCount int
	safeFallbackActive      bool
	lastBridgeRestart       time.Time
	smartCfg                types.SmartControlConfig
	smartCfgRevision        uint64
	// cachedCfg 是最近一次按修订号取到的配置深拷贝。配置没变的周期直接复用它，
	// 不再每个周期深拷贝整份配置（曲线方案、各方案学习偏移、散热收益报告……）。
	cachedCfg         types.AppConfig
	cachedCfgRevision uint64

	// 每个曲线点对应一个稳态采样桶。
	steadyObserver *smartcontrol.StableObserver
	// 观测器里的分桶样本与效率历史只对学到它们的那条曲线有意义。
	observerProfileID string
	observerCurve     []types.FanCurvePoint

	prevHasClients       bool
	prevHasActiveClients bool
	lastMemRelease       time.Time
	laptopSpeeds         laptopfan.FanSpeeds
	laptopSpeedsOK       bool
	lastLaptopFanRead    time.Time
}

// newTemperatureMonitorState 按 startTemperatureMonitoring 进入循环前的原有顺序初始化会话状态
// （取配置、查询客户端、首次读取温度、记录设备连接代次……）。
func (a *CoreApp) newTemperatureMonitorState() *temperatureMonitorState {
	s := &temperatureMonitorState{}

	s.cfg, s.cfgRevision = a.configManager.GetWithRevision()
	s.hasClients = a.ipcServer != nil && a.ipcServer.HasClients()
	s.hasActiveClients = a.ipcServer != nil && a.ipcServer.HasActiveClients()
	s.updateInterval = effectiveTemperatureMonitorInterval(s.cfg.TempUpdateRate, s.hasActiveClients, s.cfg.AutoControl)

	// 温度采样使用 EMA 平滑。
	s.sampleCount = max(s.cfg.TempSampleCount, 1)
	s.tempEMA = 0.0
	s.tempEMAReady = false

	s.rawTempHistory = make([]int, 0, 6)
	s.recentAvgTemps = make([]int, 0, 24)
	s.recentControlTemps = make([]int, 0, 24)
	initialTemp := a.tempReader.Read(a.temperatureSelection(s.cfg))
	if initialTemp.ControlTemp > 0 {
		s.rawTempHistory = append(s.rawTempHistory, initialTemp.ControlTemp)
	}
	s.lastTargetRPM = -1
	s.lastDeviceGeneration = a.deviceManager.ConnectionGeneration()
	s.settingsRefreshGeneration = uint64(0)
	// 学习偏移每次合并都会立刻进入内存配置；这里只控制落盘与向 GUI 广播的频率。
	s.learningUnbroadcast = false
	s.lastLearningSave = time.Now()
	s.lastLearningBroadcast = time.Now()
	s.lastLoggedTargetRPM = -1
	s.lastBridgeUpdateTime = initialTemp.UpdateTime
	s.staleBridgeUpdateCount = 0
	s.bridgeFailureCount = 0
	s.invalidControlTempCount = 0
	s.safeFallbackActive = false
	s.lastBridgeRestart = time.Time{}
	s.smartCfgRevision = s.cfgRevision - 1
	// cachedCfg 是最近一次按修订号取到的配置深拷贝。配置没变的周期直接复用它，
	// 不再每个周期深拷贝整份配置（曲线方案、各方案学习偏移、散热收益报告……）。
	s.cachedCfg, s.cachedCfgRevision = s.cfg, s.cfgRevision

	// 每个曲线点对应一个稳态采样桶。
	s.steadyObserver = smartcontrol.NewStableObserver(len(s.cfg.FanCurve))
	// 观测器里的分桶样本与效率历史只对学到它们的那条曲线有意义。
	s.observerProfileID = s.cfg.ActiveFanCurveProfileID
	s.observerCurve = slices.Clone(s.cfg.FanCurve)

	s.prevHasClients = s.hasClients
	s.prevHasActiveClients = s.hasActiveClients
	return s
}

// temperatureMonitorTick 执行一个采样周期：从设备重连检测到智能控温输出，各阶段顺序与原循环体一致。
// 休眠唤醒/挂起判定与定时器重置留在 startTemperatureMonitoring 的循环里。
func (a *CoreApp) temperatureMonitorTick(s *temperatureMonitorState, now time.Time) {
	deviceGeneration := a.monitorCheckDeviceGeneration(s)
	a.monitorRefreshConfig(s, now)
	clientsJustConnected := a.monitorRefreshClientState(s, now)

	temp := a.monitorSampleTemperature(s, now)
	a.monitorTrackControlTempValidity(s, temp)
	a.monitorCheckBridgeHealth(s, temp)

	a.monitorPublishTemperature(s, temp, clientsJustConnected)

	a.monitorNormalizeSmartConfig(s)

	controlReady := a.isDeviceControlReady()
	a.monitorUpdateSmartLight(s, temp, controlReady)
	a.monitorApplySafeFallback(s, controlReady)
	if s.cfg.AutoControl && temp.ControlTemp > 0 && controlReady {
		a.monitorAutoControl(s, temp, deviceGeneration)
	}

	if !s.cfg.AutoControl || !controlReady {
		s.lastTargetRPM = -1
	}
}

// monitorCheckDeviceGeneration 检测设备重连并重置智能控温输出状态，返回本周期的连接代次。
func (a *CoreApp) monitorCheckDeviceGeneration(s *temperatureMonitorState) uint64 {
	deviceGeneration := a.deviceManager.ConnectionGeneration()
	if deviceGeneration != s.lastDeviceGeneration {
		// A reconnect starts the device in its own default gear mode. Forget
		// the target sent through the old HID handle so the next valid
		// temperature sample re-enters realtime mode and writes a target.
		s.lastDeviceGeneration = deviceGeneration
		s.lastTargetRPM = -1
		s.steadyObserver.Reset()
		a.logInfo("检测到设备重连（连接代次=%d），重置智能控温输出状态", deviceGeneration)
	}
	return deviceGeneration
}

// monitorRefreshConfig 取本周期使用的配置快照，并应用按时间切换曲线方案的日程。
func (a *CoreApp) monitorRefreshConfig(s *temperatureMonitorState, now time.Time) {
	if a.configManager.Revision() != s.cachedCfgRevision {
		s.cachedCfg, s.cachedCfgRevision = a.configManager.GetWithRevision()
	}
	// 浅拷贝：本周期对 cfg 字段的重新赋值不会带进缓存；其中的切片/映射在循环里只读。
	s.cfg, s.cfgRevision = s.cachedCfg, s.cachedCfgRevision
	if a.applyTimeCurveSchedule(s.cfg, now) {
		// 刚切换了曲线方案：重取快照，本周期就按新曲线控温，后面也不会拿旧快照回写。
		s.cachedCfg, s.cachedCfgRevision = a.configManager.GetWithRevision()
		s.cfg, s.cfgRevision = s.cachedCfg, s.cachedCfgRevision
	}
}

// monitorRefreshClientState 刷新 GUI 连接状态及其派生设置（采样间隔、HID 空闲轮询、空闲内存归还、
// 扩展传感器释放），返回本周期是否刚有 GUI 连上或恢复订阅。
func (a *CoreApp) monitorRefreshClientState(s *temperatureMonitorState, now time.Time) bool {
	// 后台空闲（无前台 GUI 且未开启智能控温）时放慢采样：此时温度读取不驱动风扇，
	// 仅服务托盘提示与历史记录，降低采样频率可显著减少桥接传感器扫描带来的后台 CPU 占用。
	// 窗口隐藏的 GUI 已退订高频事件，不消费遥测，与没有 GUI 同样对待。
	s.hasClients = a.ipcServer != nil && a.ipcServer.HasClients()
	s.hasActiveClients = a.ipcServer != nil && a.ipcServer.HasActiveClients()
	s.updateInterval = effectiveTemperatureMonitorInterval(s.cfg.TempUpdateRate, s.hasActiveClients, s.cfg.AutoControl)
	a.deviceManager.SetIdlePolling(hidIdlePollingAllowed(s.hasActiveClients, s.cfg.RTSS))
	// GUI 断开瞬间把会话期间膨胀的堆内存归还操作系统，降低核心常驻后台时的 RSS。
	if s.prevHasClients && !s.hasClients && now.Sub(s.lastMemRelease) > idleMemoryReleaseCooldown {
		s.lastMemRelease = now
		a.safeGo("release-idle-memory", func() { debug.FreeOSMemory() })
	}
	// 扩展传感器只为散热收益测试临时开启，而测试跑在 GUI 里。
	a.releaseExtendedSensorsIfIdle(s.hasClients)
	// 刚有 GUI 连上或恢复订阅：它没收到过上一帧，差分掉的传感器清单对它来说就是缺失。
	clientsJustConnected := s.hasActiveClients && !s.prevHasActiveClients
	s.prevHasClients = s.hasClients
	s.prevHasActiveClients = s.hasActiveClients
	return clientsJustConnected
}

// monitorSampleTemperature 读取本周期的温度，并补充笔记本内置风扇转速。
func (a *CoreApp) monitorSampleTemperature(s *temperatureMonitorState, now time.Time) types.TemperatureData {
	selection := a.temperatureSelection(s.cfg)
	selection.Compact = useCompactBridgeSample(s.hasActiveClients, selection.ExtendedSensors)
	temp := a.tempReader.Read(selection)
	// 补充笔记本内置风扇转速（机械革命等 Uniwill/同方准系统），不支持时保持 0。
	// 每次读取是两次 WMI ExecMethod。没有 GUI 时它只服务于温度历史（托盘、RTSS 不显示它），
	// 因此只在历史本轮要记录样本、或上次读数已过旧时才读取，其余周期沿用上次读数。
	// 【读取判据集中于此】GUI 最小化时不算活跃，同样按无 GUI 处理。
	if laptopFanReadDue(s.hasActiveClients, a.tempHistory.SampleDue(temp), now.Sub(s.lastLaptopFanRead)) {
		s.laptopSpeeds, s.laptopSpeedsOK = a.laptopFanReader.Read()
		s.lastLaptopFanRead = now
	}
	if s.laptopSpeedsOK {
		temp.CPUFanRPM = s.laptopSpeeds.CPUFanRPM
		temp.GPUFanRPM = s.laptopSpeeds.GPUFanRPM
	}
	return temp
}

// monitorTrackControlTempValidity 统计控制温度连续无效的次数，并在温度源恢复时解除安全兜底。
func (a *CoreApp) monitorTrackControlTempValidity(s *temperatureMonitorState, temp types.TemperatureData) {
	if temp.ControlTemp <= 0 {
		// 桥接正在后台启动时读不到温度属于预期过渡态，不能计入"传感器失效"，
		// 否则每次启动/自愈窗口都会把风扇推到安全兜底转速。
		if !bridge.IsStarting(temp.BridgeMsg) {
			s.invalidControlTempCount++
		}
	} else {
		if s.safeFallbackActive {
			a.logInfo("Temperature source recovered; resuming normal automatic control")
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "Temperature source recovered; normal fan control resumed")
			}
		}
		s.invalidControlTempCount = 0
		s.safeFallbackActive = false
	}
}

// monitorCheckBridgeHealth 根据本次采样判断温度桥接是否卡死或连续失败，必要时触发自愈重启。
func (a *CoreApp) monitorCheckBridgeHealth(s *temperatureMonitorState, temp types.TemperatureData) {
	if temp.BridgeOk {
		s.bridgeFailureCount = 0
		staleBridge := false
		s.lastBridgeUpdateTime, s.staleBridgeUpdateCount, staleBridge = trackBridgeTemperatureStaleness(temp, s.lastBridgeUpdateTime, s.staleBridgeUpdateCount)
		if staleBridge && time.Since(s.lastBridgeRestart) >= temperatureBridgeRestartCooldown {
			a.logError("温度桥接返回的 updateTime 连续 %d 次未变化，触发桥接重连自愈", s.staleBridgeUpdateCount+1)
			a.recoverTemperatureBridge("stale-update")
			s.lastBridgeRestart = time.Now()
			s.lastBridgeUpdateTime = 0
			s.staleBridgeUpdateCount = 0
		}
	} else {
		s.lastBridgeUpdateTime = 0
		s.staleBridgeUpdateCount = 0
		if shouldRestartTemperatureBridge(temp) {
			s.bridgeFailureCount++
			if s.bridgeFailureCount >= consecutiveBridgeFailureRestartThreshold && time.Since(s.lastBridgeRestart) >= temperatureBridgeRestartCooldown {
				a.logError("温度桥接连续 %d 次读取失败，触发桥接重连自愈: %s", s.bridgeFailureCount, temp.BridgeMsg)
				a.recoverTemperatureBridge("read-failure")
				s.lastBridgeRestart = time.Now()
				s.bridgeFailureCount = 0
			}
		} else {
			s.bridgeFailureCount = 0
		}
	}
}

// monitorPublishTemperature 把本次采样写入当前温度与温度历史，并向前台 GUI 广播。
func (a *CoreApp) monitorPublishTemperature(s *temperatureMonitorState, temp types.TemperatureData, clientsJustConnected bool) {
	a.mutex.Lock()
	previousTemp := a.currentTemp
	a.currentTemp = temp
	a.mutex.Unlock()

	historyPoint, recorded := a.tempHistory.Add(temp, a.deviceManager.GetCurrentFanData())

	// 广播温度更新（无前台 GUI 时跳过差分与序列化，核心常驻后台时显著降低每秒开销）
	if s.hasActiveClients {
		sensorValues := a.ipcServer.ActiveClientsSupport(ipc.FeatureTemperatureSensorValues)
		a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, temperatureEventPayload(temp, previousTemp, clientsJustConnected, sensorValues))
		if recorded {
			a.ipcServer.BroadcastEvent(ipc.EventTemperatureHistoryUpdate, historyPoint)
		}
	}
}

// monitorNormalizeSmartConfig 在配置修订号变化时重新归一化智能控温配置，有改动则回写并落盘。
func (a *CoreApp) monitorNormalizeSmartConfig(s *temperatureMonitorState) {
	if s.cfgRevision != s.smartCfgRevision {
		smartChanged := false
		s.smartCfg, smartChanged = smartcontrol.NormalizeConfig(s.cfg.SmartControl, s.cfg.FanCurve, s.cfg.DebugMode)
		s.smartCfgRevision = s.cfgRevision
		if smartChanged {
			s.cfg.SmartControl = s.smartCfg
			// 只改写智能控温段，并以最新配置为准重新归一化，不回写本周期开头的整份快照。
			_, persisted := a.configManager.Mutate(func(cur *types.AppConfig) bool {
				next, changed := smartcontrol.NormalizeConfig(cur.SmartControl, cur.FanCurve, cur.DebugMode)
				if changed {
					cur.SmartControl = next
				}
				return changed
			})
			if persisted {
				if err := a.configManager.Save(); err != nil {
					a.logError("保存智能控温配置失败: %v", err)
				}
			}
		}
	}
}

// monitorUpdateSmartLight 在灯带处于 smart_temp 模式时按最高温度更新智能温控灯效。
func (a *CoreApp) monitorUpdateSmartLight(s *temperatureMonitorState, temp types.TemperatureData, controlReady bool) {
	if s.cfg.LightStrip.Mode == "smart_temp" && temp.MaxTemp > 0 && controlReady {
		// Smart lighting is host-driven: the cooler firmware exposes native
		// green/yellow/red animations through 0x44, but it cannot read the
		// computer's CPU/GPU temperature by itself. The device manager applies
		// hysteresis and suppresses duplicate writes.
		if a.lockDeviceControlIfReady() {
			err := a.deviceManager.UpdateSmartTemperatureLight(temp.MaxTemp)
			a.deviceControlMutex.Unlock()
			if err != nil {
				a.logError("智能温控灯效更新失败: %v", err)
			}
			a.broadcastSmartLightStatus()
		}
	}
}

// monitorApplySafeFallback 在控制温度连续无效时把风扇切到安全兜底转速。
func (a *CoreApp) monitorApplySafeFallback(s *temperatureMonitorState, controlReady bool) {
	if s.cfg.AutoControl && controlReady && s.invalidControlTempCount >= 3 && !s.safeFallbackActive {
		_, safeRPM := smartcontrol.GetCurveRPMBounds(s.cfg.FanCurve)
		if safeRPM <= 0 {
			safeRPM = 3000
		}
		ready, written := a.setAutomaticFanSpeed(safeRPM)
		if ready && written {
			s.safeFallbackActive = true
			s.lastTargetRPM = safeRPM
			message := fmt.Sprintf("Temperature readings are unavailable; fan switched to safe fallback at %d RPM", safeRPM)
			a.logError("%s", message)
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventDeviceError, message)
			}
		}
	}
}

// monitorAutoControl 是一个周期的智能控温：滤波、计算目标转速、下发、稳态学习与调试日志。
// 调用方已确认 cfg.AutoControl、temp.ControlTemp > 0 且设备可控。
func (a *CoreApp) monitorAutoControl(s *temperatureMonitorState, temp types.TemperatureData, deviceGeneration uint64) {
	// 采样窗口变化时重置 EMA，避免阶跃。
	newSampleCount := max(s.cfg.TempSampleCount, 1)
	if newSampleCount != s.sampleCount {
		s.sampleCount = newSampleCount
		s.tempEMAReady = false
	}

	// 切换方案或编辑曲线后点数通常不变，只比较点数会让新曲线沿用旧曲线的样本与效率历史。
	if s.steadyObserver == nil || s.cfg.ActiveFanCurveProfileID != s.observerProfileID || !slices.Equal(s.cfg.FanCurve, s.observerCurve) {
		s.steadyObserver = smartcontrol.NewStableObserver(len(s.cfg.FanCurve))
		s.observerProfileID = s.cfg.ActiveFanCurveProfileID
		s.observerCurve = slices.Clone(s.cfg.FanCurve)
	}

	activeCurve := s.cfg.FanCurve
	tickCfg := s.smartCfg

	avgTemp, controlTemp, spikeSuppressed := monitorFilterControlTemp(s, temp, tickCfg)

	prevTargetRPM := s.lastTargetRPM
	baseRPM, targetRPM := monitorComputeTargetRPM(controlTemp, activeCurve, tickCfg, prevTargetRPM)

	fanData := a.deviceManager.GetCurrentFanData()
	observedRPM := targetRPM
	if fanData != nil && fanData.CurrentRPM > 0 {
		observedRPM = int(fanData.CurrentRPM)
	}
	if shouldSendTargetRPM(targetRPM, prevTargetRPM, tickCfg.MinRPMChange, fanData) {
		a.monitorSendTargetRPM(s, targetRPM, deviceGeneration)
	}

	a.monitorLearnSteadyOffset(s, controlTemp, observedRPM, spikeSuppressed)
	a.monitorFlushLearnedOffsets(s)

	// 目标转速变化时立即记录；不变时按 controlDebugLogInterval 记一行，仍能看出温度走势。
	if baseRPM > 0 && (targetRPM != s.lastLoggedTargetRPM || time.Since(s.lastControlLog) >= controlDebugLogInterval) {
		s.lastLoggedTargetRPM = targetRPM
		s.lastControlLog = time.Now()
		a.logDebug("智能控温: 最高=%d°C 基准=%s 当前=%d°C 平均=%d°C 控制温度=%d°C 基础=%dRPM 目标=%dRPM", temp.MaxTemp, temp.ControlSource, temp.ControlTemp, avgTemp, controlTemp, baseRPM, targetRPM)
	}
}

// monitorFilterControlTemp 对本次控制温度做瞬时尖峰过滤与 EMA 平滑，并维护三段滑动历史。
// 返回 EMA 平均温度、用于查曲线的控制温度，以及本次采样是否被判为尖峰。
func monitorFilterControlTemp(s *temperatureMonitorState, temp types.TemperatureData, tickCfg types.SmartControlConfig) (avgTemp, controlTemp int, spikeSuppressed bool) {
	sampleTemp := temp.ControlTemp
	sampleSpikeSuppressed := false
	if tickCfg.FilterTransientSpike {
		sampleTemp, sampleSpikeSuppressed = smartcontrol.FilterTransientSample(temp.ControlTemp, s.rawTempHistory, tickCfg.Hysteresis)
	}
	s.rawTempHistory = append(s.rawTempHistory, temp.ControlTemp)
	if len(s.rawTempHistory) > 6 {
		s.rawTempHistory = s.rawTempHistory[len(s.rawTempHistory)-6:]
	}

	if !s.tempEMAReady {
		s.tempEMA = float64(sampleTemp)
		s.tempEMAReady = true
	} else {
		s.tempEMA = nextTemperatureEMA(s.tempEMA, sampleTemp, s.sampleCount)
	}
	avgTemp = int(math.Round(s.tempEMA))

	s.recentAvgTemps = append(s.recentAvgTemps, avgTemp)
	if len(s.recentAvgTemps) > 24 {
		s.recentAvgTemps = s.recentAvgTemps[len(s.recentAvgTemps)-24:]
	}

	controlTemp = avgTemp
	controlSpikeSuppressed := false
	if tickCfg.FilterTransientSpike {
		controlTemp, controlSpikeSuppressed = smartcontrol.FilterTransientSpike(avgTemp, s.recentAvgTemps, tickCfg.TargetTemp, tickCfg.Hysteresis)
	}
	spikeSuppressed = sampleSpikeSuppressed || controlSpikeSuppressed
	s.recentControlTemps = append(s.recentControlTemps, controlTemp)
	if len(s.recentControlTemps) > 24 {
		s.recentControlTemps = s.recentControlTemps[len(s.recentControlTemps)-24:]
	}
	return avgTemp, controlTemp, spikeSuppressed
}

// monitorComputeTargetRPM 由控制温度计算本周期的目标转速：曲线基础转速、智能控温修正、
// 曲线上下限钳制与升降速限幅。不读写任何循环状态。
func monitorComputeTargetRPM(controlTemp int, activeCurve []types.FanCurvePoint, tickCfg types.SmartControlConfig, prevTargetRPM int) (baseRPM, targetRPM int) {
	curveMinRPM, curveMaxRPM := smartcontrol.GetCurveRPMBounds(activeCurve)

	baseRPM = temperature.CalculateTargetRPM(controlTemp, activeCurve)

	targetRPM = smartcontrol.CalculateTargetRPM(controlTemp, activeCurve, tickCfg)
	if targetRPM <= 0 {
		targetRPM = baseRPM
	}

	if targetRPM > 0 {
		targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
	}

	if shouldApplyRampLimit(targetRPM, prevTargetRPM) {
		targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, tickCfg.RampUpLimit, tickCfg.RampDownLimit)
		if targetRPM > 0 {
			targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
		}
	}
	return baseRPM, targetRPM
}

// monitorSendTargetRPM 下发目标转速并记录结果；重连后首次写入成功时异步刷新设备状态。
func (a *CoreApp) monitorSendTargetRPM(s *temperatureMonitorState, targetRPM int, deviceGeneration uint64) {
	ready, written := a.setAutomaticFanSpeed(targetRPM)
	if written {
		s.lastTargetRPM = targetRPM
		if deviceGeneration != s.settingsRefreshGeneration {
			s.settingsRefreshGeneration = deviceGeneration
			a.safeGo("refreshDeviceSettings@realtime-mode", func() {
				// The device applies a mode change asynchronously. Query after a
				// short settle period so the cached status shown to the GUI is
				// the new realtime mode rather than the pre-reconnect gear mode.
				time.Sleep(200 * time.Millisecond)
				if _, err := a.RefreshDeviceSettings(); err != nil {
					a.logError("实时模式切换后刷新设备状态失败: %v", err)
				}
			})
		}
	} else if ready {
		s.lastTargetRPM = -1
		a.logError("智能控温转速下发失败，将在下个周期重试: %d RPM", targetRPM)
	}
}

// monitorLearnSteadyOffset 在开启学习时用稳态观测更新学习偏移并合并进内存配置；未开启时重置观测器。
func (a *CoreApp) monitorLearnSteadyOffset(s *temperatureMonitorState, controlTemp, observedRPM int, spikeSuppressed bool) {
	switch {
	case s.smartCfg.Learning:
		if !spikeSuppressed {
			steady := s.steadyObserver.Observe(controlTemp, observedRPM, s.cfg.FanCurve, s.smartCfg)
			if steady.Ready && steady.BucketIdx >= 0 {
				newOffsets, changed := smartcontrol.LearnSteadyOffset(
					steady.BucketIdx,
					steady.MeanTemp,
					steady.MeanRPM,
					steady.LocalEff,
					steady.HaveEff,
					s.cfg.FanCurve,
					s.smartCfg.LearnedOffsets,
					s.smartCfg,
				)
				if changed {
					s.smartCfg.LearnedOffsets = newOffsets
					s.cfg.SmartControl = s.smartCfg
					// 只合并学习偏移。期间若切换了方案或改了曲线，这批偏移是按旧曲线学的，丢弃。
					learnedProfileID := s.cfg.ActiveFanCurveProfileID
					learnedCurve := s.cfg.FanCurve
					_, merged := a.configManager.Mutate(func(cur *types.AppConfig) bool {
						if cur.ActiveFanCurveProfileID != learnedProfileID || !slices.Equal(cur.FanCurve, learnedCurve) {
							return false
						}
						cur.SmartControl.LearnedOffsets = cloneIntSlice(newOffsets)
						storeSmartControlOffsetsForActiveProfile(cur)
						return true
					})
					if merged {
						a.learnedOffsetsUnsaved.Store(true)
						s.learningUnbroadcast = true
					}
				}
			}
		}

	default:
		s.steadyObserver.Reset()
	}
}

// monitorFlushLearnedOffsets 按各自的节奏把学习偏移落盘并向 GUI 广播。
func (a *CoreApp) monitorFlushLearnedOffsets(s *temperatureMonitorState) {
	// 学习偏移在控温环里高频改写，每次都写盘既伤 SSD 也会拖慢采样周期。
	// 落盘放慢到 learnedOffsetsSaveInterval；退出、系统挂起前、重置学习与切换方案
	// 都会另行把内存中的配置整份落盘，不会因此丢失学习结果。
	if a.learnedOffsetsUnsaved.Load() && time.Since(s.lastLearningSave) >= learnedOffsetsSaveInterval {
		a.saveLearnedOffsetsIfDirty("periodic")
		s.lastLearningSave = time.Now()
	}
	// GUI 展示学习后的曲线，广播仍保持原来的 25 秒节奏，不随落盘一起放慢。
	if s.learningUnbroadcast && time.Since(s.lastLearningBroadcast) >= learnedOffsetsBroadcastInterval {
		s.learningUnbroadcast = false
		s.lastLearningBroadcast = time.Now()
		if a.ipcServer != nil && a.ipcServer.HasClients() {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, a.configManager.Get())
		}
	}
}

package coreapp

import (
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

const staleBridgeUpdateThreshold = 3

// idleTemperatureMonitorInterval 是后台空闲（无 GUI 连接且未开启智能控温）时的温度采样间隔下限。
// 此时温度读取仅用于托盘提示与历史记录，放慢采样可显著降低桥接进程的传感器扫描开销与后台 CPU 占用。
const idleTemperatureMonitorInterval = 10 * time.Second

// idleMemoryReleaseCooldown 限制 GUI 断开后归还内存的最小间隔，避免频繁开关 GUI 时反复触发 GC。
const idleMemoryReleaseCooldown = 30 * time.Second

const (
	consecutiveBridgeFailureRestartThreshold = 2
	temperatureBridgeRestartCooldown         = 10 * time.Second
)

func trackBridgeTemperatureStaleness(temp types.TemperatureData, lastUpdate int64, staleCount int) (int64, int, bool) {
	if !temp.BridgeOk || temp.UpdateTime <= 0 {
		return 0, 0, false
	}
	if temp.UpdateTime != lastUpdate {
		return temp.UpdateTime, 0, false
	}
	staleCount++
	return lastUpdate, staleCount, staleCount >= staleBridgeUpdateThreshold
}

func shouldRestartTemperatureBridge(temp types.TemperatureData) bool {
	if temp.BridgeOk {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(temp.BridgeMsg))
	if msg == "" {
		return true
	}

	restartHints := []string{
		"启动桥接程序失败",
		"桥接程序通信失败",
		"桥接程序未连接",
		"连接管道失败",
		"发送命令失败",
		"读取响应失败",
		"等待桥接程序启动超时",
		"未能获取管道名称",
		"pipe",
		"broken",
		"closed",
		"timeout",
		"bridge reconnect failed",
	}
	for _, hint := range restartHints {
		if strings.Contains(msg, strings.ToLower(hint)) {
			return true
		}
	}

	// 休眠恢复后硬件监控库偶尔会返回全 0 但进程仍能响应，重启桥接可重新初始化底层传感器。
	return temp.CPUTemp == 0 && temp.GPUTemp == 0
}

func (a *CoreApp) recoverTemperatureBridge(reason string) {
	a.safeRun("temperature-bridge-recover@"+reason, func() {
		a.bridgeManager.Stop()
		if err := a.bridgeManager.EnsureRunning(); err != nil {
			a.logError("温度桥接自愈重启失败[%s]: %v", reason, err)
			return
		}
		a.logInfo("温度桥接已完成自愈重启: %s", reason)
	})
}

func compactTemperatureEventPayload(current, previous types.TemperatureData) types.TemperatureData {
	compact := current
	if reflect.DeepEqual(current.CpuSensors, previous.CpuSensors) {
		compact.CpuSensors = nil
	}
	if reflect.DeepEqual(current.GpuSensors, previous.GpuSensors) {
		compact.GpuSensors = nil
	}
	if reflect.DeepEqual(current.CpuPowerSensors, previous.CpuPowerSensors) {
		compact.CpuPowerSensors = nil
	}
	if reflect.DeepEqual(current.GpuPowerSensors, previous.GpuPowerSensors) {
		compact.GpuPowerSensors = nil
	}
	if reflect.DeepEqual(current.GpuDevices, previous.GpuDevices) {
		compact.GpuDevices = nil
	}
	return compact
}

func (a *CoreApp) stopTemperatureMonitoring() <-chan struct{} {
	a.monitorMutex.Lock()
	defer a.monitorMutex.Unlock()

	if !a.monitoringTemp.Load() || a.monitorDone == nil {
		return nil
	}
	if !a.monitorStopping {
		close(a.monitorStop)
		a.monitorStopping = true
	}
	return a.monitorDone
}

// ensureTemperatureMonitoring starts a new monitoring session only after a
// previous stopping session has fully exited. This prevents suspend/resume and
// reconnect races from leaving monitoring permanently off until the next health
// check.
func (a *CoreApp) ensureTemperatureMonitoring(reason string) {
	if a.stopping.Load() {
		return
	}
	a.monitorMutex.Lock()
	running := a.monitoringTemp.Load()
	stopping := a.monitorStopping
	done := a.monitorDone
	a.monitorMutex.Unlock()

	if !running {
		a.safeGo("startTemperatureMonitoring@"+reason, func() {
			a.startTemperatureMonitoring()
		})
		return
	}
	if !stopping || done == nil {
		return
	}

	a.safeGo("restartTemperatureMonitoring@"+reason, func() {
		select {
		case <-done:
			a.startTemperatureMonitoring()
		case <-time.After(12 * time.Second):
			a.logError("等待旧温度监控退出超时，暂不启动新的监控会话（来源=%s）", reason)
		}
	})
}

// startTemperatureMonitoring 开始温度监控
func (a *CoreApp) startTemperatureMonitoring() {
	if a.stopping.Load() {
		return
	}
	a.monitorMutex.Lock()
	if a.monitoringTemp.Load() {
		a.monitorMutex.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	a.monitorStop = stop
	a.monitorDone = done
	a.monitorStopping = false
	a.monitoringTemp.Store(true)
	a.monitorMutex.Unlock()
	defer func() {
		a.monitorMutex.Lock()
		if a.monitorDone == done {
			a.monitorStop = nil
			a.monitorDone = nil
			a.monitorStopping = false
			a.monitoringTemp.Store(false)
		}
		a.monitorMutex.Unlock()
		close(done)
	}()

	// 注意：不在此处立即调用 EnterAutoMode，因为在启动时温度数据（桥接程序）可能尚未就绪。
	// 如果在温度读取成功之前切换到软件控制模式，设备将不会收到转速指令，导致风扇停转。
	// EnterAutoMode 和转速设置会在首次成功读取温度后，由 SetFanSpeed 内部统一完成。

	cfg, cfgRevision := a.configManager.GetWithRevision()
	hasClients := a.ipcServer != nil && a.ipcServer.HasClients()
	updateInterval := effectiveTemperatureMonitorInterval(cfg.TempUpdateRate, hasClients, cfg.AutoControl)

	// 温度采样使用 EMA 平滑。
	sampleCount := max(cfg.TempSampleCount, 1)
	tempEMA := 0
	tempEMAReady := false

	rawTempHistory := make([]int, 0, 6)
	recentAvgTemps := make([]int, 0, 24)
	recentControlTemps := make([]int, 0, 24)
	initialSelection := types.TemperatureSelection{
		TempSource: cfg.TempSource,
		GpuDevice:  cfg.GpuDevice,
		CpuSensor:  cfg.CpuSensor,
		CpuSensors: cfg.CpuSensors,
		GpuSensor:  cfg.GpuSensor,
	}
	initialTemp := a.tempReader.Read(initialSelection)
	if initialTemp.ControlTemp > 0 {
		rawTempHistory = append(rawTempHistory, initialTemp.ControlTemp)
	}
	lastTargetRPM := -1
	lastControlTemp := -1
	lastDeviceGeneration := a.deviceManager.ConnectionGeneration()
	settingsRefreshGeneration := uint64(0)
	learningDirty := false
	lastLearningSave := time.Now()
	lastMonitorTick := time.Now()
	lastBridgeUpdateTime := initialTemp.UpdateTime
	staleBridgeUpdateCount := 0
	bridgeFailureCount := 0
	lastBridgeRestart := time.Time{}
	var smartCfg types.SmartControlConfig
	smartCfgRevision := cfgRevision - 1

	// 每个曲线点对应一个稳态采样桶。
	steadyObserver := smartcontrol.NewStableObserver(len(cfg.FanCurve))
	thermalPredictor := smartcontrol.NewThermalPredictor()
	timer := time.NewTimer(updateInterval)
	defer timer.Stop()

	prevHasClients := hasClients
	var lastMemRelease time.Time

monitorLoop:
	for {
		select {
		case <-stop:
			break monitorLoop
		case <-timer.C:
			now := time.Now()
			gap := now.Sub(lastMonitorTick)
			lastMonitorTick = now
			if a.maybeRecoverFromSystemResume("temperature-monitor", gap, updateInterval) {
				thermalPredictor.Reset()
				timer.Reset(updateInterval)
				continue
			}
			deviceGeneration := a.deviceManager.ConnectionGeneration()
			if deviceGeneration != lastDeviceGeneration {
				// A reconnect starts the device in its own default gear mode. Forget
				// the target sent through the old HID handle so the next valid
				// temperature sample re-enters realtime mode and writes a target.
				lastDeviceGeneration = deviceGeneration
				lastTargetRPM = -1
				lastControlTemp = -1
				steadyObserver.Reset()
				thermalPredictor.Reset()
				a.logInfo("检测到设备重连（连接代次=%d），重置智能控温输出状态", deviceGeneration)
			}

			cfg, cfgRevision = a.configManager.GetWithRevision()
			a.applyTimeCurveSchedule(now)
			// 后台空闲（无 GUI 连接且未开启智能控温）时放慢采样：此时温度读取不驱动风扇，
			// 仅服务托盘提示与历史记录，降低采样频率可显著减少桥接传感器扫描带来的后台 CPU 占用。
			hasClients = a.ipcServer != nil && a.ipcServer.HasClients()
			updateInterval = effectiveTemperatureMonitorInterval(cfg.TempUpdateRate, hasClients, cfg.AutoControl)
			// GUI 断开瞬间把会话期间膨胀的堆内存归还操作系统，降低核心常驻后台时的 RSS。
			if prevHasClients && !hasClients && now.Sub(lastMemRelease) > idleMemoryReleaseCooldown {
				lastMemRelease = now
				a.safeGo("release-idle-memory", func() { debug.FreeOSMemory() })
			}
			prevHasClients = hasClients

			selection := types.TemperatureSelection{
				TempSource: cfg.TempSource,
				GpuDevice:  cfg.GpuDevice,
				CpuSensor:  cfg.CpuSensor,
				CpuSensors: cfg.CpuSensors,
				GpuSensor:  cfg.GpuSensor,
			}
			temp := a.tempReader.Read(selection)
			if temp.BridgeOk {
				bridgeFailureCount = 0
				staleBridge := false
				lastBridgeUpdateTime, staleBridgeUpdateCount, staleBridge = trackBridgeTemperatureStaleness(temp, lastBridgeUpdateTime, staleBridgeUpdateCount)
				if staleBridge && time.Since(lastBridgeRestart) >= temperatureBridgeRestartCooldown {
					a.logError("温度桥接返回的 updateTime 连续 %d 次未变化，触发桥接重连自愈", staleBridgeUpdateCount+1)
					a.recoverTemperatureBridge("stale-update")
					lastBridgeRestart = time.Now()
					lastBridgeUpdateTime = 0
					staleBridgeUpdateCount = 0
				}
			} else {
				lastBridgeUpdateTime = 0
				staleBridgeUpdateCount = 0
				if shouldRestartTemperatureBridge(temp) {
					bridgeFailureCount++
					if bridgeFailureCount >= consecutiveBridgeFailureRestartThreshold && time.Since(lastBridgeRestart) >= temperatureBridgeRestartCooldown {
						a.logError("温度桥接连续 %d 次读取失败，触发桥接重连自愈: %s", bridgeFailureCount, temp.BridgeMsg)
						a.recoverTemperatureBridge("read-failure")
						lastBridgeRestart = time.Now()
						bridgeFailureCount = 0
					}
				} else {
					bridgeFailureCount = 0
				}
			}

			a.mutex.Lock()
			previousTemp := a.currentTemp
			a.currentTemp = temp
			a.mutex.Unlock()

			historyPoint, recorded := a.tempHistory.Add(temp, a.deviceManager.GetCurrentFanData())

			// 广播温度更新（无 GUI 客户端时跳过差分与序列化，核心常驻后台时显著降低每秒开销）
			if hasClients {
				eventTemp := compactTemperatureEventPayload(temp, previousTemp)
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, eventTemp)
				if recorded {
					a.ipcServer.BroadcastEvent(ipc.EventTemperatureHistoryUpdate, historyPoint)
				}
			}

			if cfgRevision != smartCfgRevision {
				smartChanged := false
				smartCfg, smartChanged = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
				smartCfgRevision = cfgRevision
				if smartChanged {
					cfg.SmartControl = smartCfg
					a.configManager.Set(cfg)
					if err := a.configManager.Save(); err != nil {
						a.logError("保存智能控温配置失败: %v", err)
					}
				}
			}

			if cfg.AutoControl && temp.ControlTemp > 0 {
				// 采样窗口变化时重置 EMA，避免阶跃。
				newSampleCount := max(cfg.TempSampleCount, 1)
				if newSampleCount != sampleCount {
					sampleCount = newSampleCount
					tempEMAReady = false
				}

				if steadyObserver == nil || len(cfg.FanCurve) != steadyObserver.CurveLen() {
					steadyObserver = smartcontrol.NewStableObserver(len(cfg.FanCurve))
				}

				sampleTemp := temp.ControlTemp
				sampleSpikeSuppressed := false
				if smartCfg.FilterTransientSpike {
					sampleTemp, sampleSpikeSuppressed = smartcontrol.FilterTransientSample(temp.ControlTemp, rawTempHistory, smartCfg.Hysteresis)
				}
				rawTempHistory = append(rawTempHistory, temp.ControlTemp)
				if len(rawTempHistory) > 6 {
					rawTempHistory = rawTempHistory[len(rawTempHistory)-6:]
				}

				if !tempEMAReady {
					tempEMA = sampleTemp
					tempEMAReady = true
				} else {
					n := sampleCount
					tempEMA = (2*sampleTemp + (n-1)*tempEMA) / (n + 1)
				}
				avgTemp := tempEMA

				recentAvgTemps = append(recentAvgTemps, avgTemp)
				if len(recentAvgTemps) > 24 {
					recentAvgTemps = recentAvgTemps[len(recentAvgTemps)-24:]
				}

				controlTemp := avgTemp
				controlSpikeSuppressed := false
				if smartCfg.FilterTransientSpike {
					controlTemp, controlSpikeSuppressed = smartcontrol.FilterTransientSpike(avgTemp, recentAvgTemps, smartCfg.TargetTemp, smartCfg.Hysteresis)
				}
				spikeSuppressed := sampleSpikeSuppressed || controlSpikeSuppressed
				recentControlTemps = append(recentControlTemps, controlTemp)
				if len(recentControlTemps) > 24 {
					recentControlTemps = recentControlTemps[len(recentControlTemps)-24:]
				}

				// 预测只作为升温方向的前馈补偿：短窗口温度斜率叠加 CPU/GPU
				// 功耗突增，可在实测温度越过曲线点前提前升速。稳态学习仍使用
				// learningControlTemp，避免将预测值写入长期学习偏移。
				learningControlTemp := controlTemp
				if smartCfg.Learning {
					prediction := thermalPredictor.Observe(temp, now, cfg.TempSource, smartCfg.TrendGain)
					if prediction.ControlTemp > controlTemp {
						controlTemp = prediction.ControlTemp
						a.logDebug("预测控温: 实测=%d°C 预测=%d°C CPU+%.1f°C GPU+%.1f°C CPU功耗=%.1fW GPU功耗=%.1fW",
							learningControlTemp,
							controlTemp,
							prediction.CPURise,
							prediction.GPURise,
							temp.CPUPower,
							temp.GPUPower,
						)
					}
				} else {
					thermalPredictor.Reset()
				}

				curveMinRPM, curveMaxRPM := smartcontrol.GetCurveRPMBounds(cfg.FanCurve)

				baseRPM := temperature.CalculateTargetRPM(controlTemp, cfg.FanCurve)
				prevTargetRPM := lastTargetRPM

				targetRPM := smartcontrol.CalculateTargetRPM(controlTemp, cfg.FanCurve, smartCfg)
				if targetRPM <= 0 {
					targetRPM = baseRPM
				}

				if targetRPM > 0 {
					targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
				}

				if shouldApplyRampLimit(targetRPM, prevTargetRPM) {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
					if targetRPM > 0 {
						targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
					}
				}

				adjustedRPM, avoided := applySpeedAvoidance(targetRPM, curveMinRPM, curveMaxRPM, prevTargetRPM, controlTemp, lastControlTemp, cfg.SpeedAvoidance)
				if avoided {
					targetRPM = adjustedRPM
				}

				fanData := a.deviceManager.GetCurrentFanData()
				observedRPM := targetRPM
				if fanData != nil && fanData.CurrentRPM > 0 {
					observedRPM = int(fanData.CurrentRPM)
				}
				if shouldSendTargetRPM(targetRPM, prevTargetRPM, smartCfg.MinRPMChange, fanData) {
					if a.deviceManager.SetFanSpeed(targetRPM) {
						lastTargetRPM = targetRPM
						if deviceGeneration != settingsRefreshGeneration {
							settingsRefreshGeneration = deviceGeneration
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
					} else {
						lastTargetRPM = -1
						a.logError("智能控温转速下发失败，将在下个周期重试: %d RPM", targetRPM)
					}
				}

				if smartCfg.Learning && !spikeSuppressed {
					steady := steadyObserver.Observe(learningControlTemp, observedRPM, cfg.FanCurve, smartCfg)
					if steady.Ready && steady.BucketIdx >= 0 {
						newOffsets, changed := smartcontrol.LearnSteadyOffset(
							steady.BucketIdx,
							steady.MeanTemp,
							steady.MeanRPM,
							steady.LocalEff,
							steady.HaveEff,
							cfg.FanCurve,
							smartCfg.LearnedOffsets,
							smartCfg,
						)
						if changed {
							smartCfg.LearnedOffsets = newOffsets
							cfg.SmartControl = smartCfg
							storeSmartControlOffsetsForActiveProfile(&cfg)
							a.configManager.Set(cfg)
							learningDirty = true
						}
					}

					if learningDirty && time.Since(lastLearningSave) >= 25*time.Second {
						if err := a.configManager.Save(); err != nil {
							a.logError("保存学习偏移失败: %v", err)
						} else {
							lastLearningSave = time.Now()
							learningDirty = false
							if a.ipcServer != nil {
								a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
							}
						}
					}
				} else if !smartCfg.Learning {
					steadyObserver.Reset()
				}

				if baseRPM > 0 {
					a.logDebug("智能控温: 最高=%d°C 基准=%s 当前=%d°C 平均=%d°C 控制温度=%d°C 基础=%dRPM 目标=%dRPM", temp.MaxTemp, temp.ControlSource, temp.ControlTemp, avgTemp, controlTemp, baseRPM, targetRPM)
				}
				lastControlTemp = learningControlTemp
			}

			if !cfg.AutoControl {
				lastTargetRPM = -1
				lastControlTemp = -1
				thermalPredictor.Reset()
			}

			timer.Reset(updateInterval)
		}
	}

	if learningDirty {
		if err := a.configManager.Save(); err != nil {
			a.logError("退出监控时保存学习曲线失败: %v", err)
		}
	}
}

func temperatureMonitorInterval(updateRateSeconds int) time.Duration {
	if updateRateSeconds < 1 {
		updateRateSeconds = 1
	}
	return time.Duration(updateRateSeconds) * time.Second
}

// effectiveTemperatureMonitorInterval keeps configured responsiveness whenever
// the GUI is present or automatic control is active. Only a fully idle core
// session is slowed down, because those samples do not influence fan output.
func effectiveTemperatureMonitorInterval(updateRateSeconds int, hasClients, autoControl bool) time.Duration {
	interval := temperatureMonitorInterval(updateRateSeconds)
	if !hasClients && !autoControl && interval < idleTemperatureMonitorInterval {
		return idleTemperatureMonitorInterval
	}
	return interval
}

func shouldApplyRampLimit(targetRPM, prevTargetRPM int) bool {
	return prevTargetRPM > 0 || targetRPM == 0
}

func shouldSendTargetRPM(targetRPM, prevTargetRPM, minRPMChange int, fanData *types.FanData) bool {
	if targetRPM < 0 {
		return false
	}
	if prevTargetRPM < 0 {
		return true
	}
	if absRPMDelta(targetRPM, prevTargetRPM) >= minRPMChange {
		return true
	}
	if fanData == nil {
		return false
	}
	deviceTargetRPM := int(fanData.TargetRPM)
	if targetRPM > 0 && (deviceTargetRPM == 0 || fanData.CurrentRPM == 0) {
		return true
	}
	return absRPMDelta(targetRPM, deviceTargetRPM) >= minRPMChange
}

func absRPMDelta(a, b int) int {
	delta := a - b
	if delta < 0 {
		return -delta
	}
	return delta
}

// startHealthMonitoring 启动健康监控
func (a *CoreApp) startHealthMonitoring() {
	if a.stopping.Load() {
		return
	}
	a.healthMutex.Lock()
	if a.healthDone != nil {
		a.healthMutex.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	a.healthStop = stop
	a.healthDone = done
	a.healthStopping = false
	a.healthMutex.Unlock()

	a.logInfo("启动健康监控系统")
	a.safeGo("healthMonitoringLoop", func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer func() {
			a.healthMutex.Lock()
			if a.healthDone == done {
				a.healthStop = nil
				a.healthDone = nil
				a.healthStopping = false
			}
			a.healthMutex.Unlock()
			close(done)
		}()
		lastHealthCheck := time.Now()

		for {
			select {
			case <-ticker.C:
				now := time.Now()
				gap := now.Sub(lastHealthCheck)
				lastHealthCheck = now
				if a.maybeRecoverFromSystemResume("health-monitor", gap, 30*time.Second) {
					continue
				}

				a.performHealthCheck()
			case <-stop:
				a.logInfo("健康监控系统已停止")
				return
			}
		}
	})

	if a.logger != nil {
		a.safeGo("cleanOldLogs", func() {
			a.logger.CleanOldLogs()
		})
	}
}

func (a *CoreApp) stopHealthMonitoring(timeout time.Duration) {
	a.healthMutex.Lock()
	done := a.healthDone
	if done != nil && !a.healthStopping {
		close(a.healthStop)
		a.healthStopping = true
	}
	a.healthMutex.Unlock()

	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
		a.logError("等待健康监控停止超时")
	}
}

// performHealthCheck 执行健康检查
func (a *CoreApp) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			a.logError("健康检查中发生panic: %v", r)
		}
	}()

	a.trayManager.CheckHealth()
	a.ensureTemperatureMonitoringHealthy()
	a.checkDeviceHealth()

	a.logDebug("健康检查完成 - 托盘:%v 设备连接:%v",
		a.trayManager.IsInitialized(), a.isConnected)
}

func (a *CoreApp) ensureTemperatureMonitoringHealthy() {
	if a.systemSuspended.Load() {
		return
	}

	a.ensureTemperatureMonitoring("health-check")
}

// checkDeviceHealth 检查设备健康状态
func (a *CoreApp) checkDeviceHealth() {
	a.mutex.RLock()
	connected := a.isConnected
	a.mutex.RUnlock()

	if !connected {
		a.logInfo("健康检查: 设备未连接，尝试重新连接")
		a.requestReconnect("health-check", []time.Duration{0})
	} else {
		// 验证设备实际连接状态
		if !a.deviceManager.IsConnected() {
			a.logError("健康检查: 检测到设备状态不一致，触发断开回调")
			a.onDeviceDisconnect()
		}
	}
}

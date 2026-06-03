package coreapp

import (
	"os"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/curveprofiles"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/powernotify"
	"github.com/TIANLI0/THRM/internal/smartcontrol"
	"github.com/TIANLI0/THRM/internal/tray"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/TIANLI0/THRM/internal/version"
)

// Start 启动核心服务
func (a *CoreApp) Start() error {
	a.logInfo("=== THRM 核心服务启动 ===")
	a.logInfo("版本: %s", version.Get())
	a.logInfo("安装目录: %s", config.GetInstallDir())
	a.logInfo("调试模式: %v", a.debugMode)
	a.logInfo("当前工作目录: %s", config.GetCurrentWorkingDir())

	// 检测是否为自启动
	a.isAutoStartLaunch = autostart.DetectAutoStartLaunch(os.Args)
	a.logInfo("自启动模式: %v", a.isAutoStartLaunch)

	// 加载配置
	a.logInfo("开始加载配置文件")
	cfg := a.configManager.Load(a.isAutoStartLaunch)
	if normalizedLight, changed := normalizeLightStripConfig(cfg.LightStrip); changed {
		cfg.LightStrip = normalizedLight
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存灯带默认配置失败: %v", err)
		}
	}
	if normalizeHotkeyConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存快捷键默认配置失败: %v", err)
		}
	}
	if curveprofiles.NormalizeConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存温控曲线方案默认配置失败: %v", err)
		}
	}
	if normalizedSmart, changed := smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode); changed {
		cfg.SmartControl = normalizedSmart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存智能控温默认配置失败: %v", err)
		}
	}
	if normalizeManualGearMemoryConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存挡位记忆默认配置失败: %v", err)
		}
	}
	if types.NormalizeManualGearRPM(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存挡位转速默认配置失败: %v", err)
		}
	}
	if a.applyCachedLegionFnQSupport(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存 Lenovo Legion Fn+Q 缓存配置失败: %v", err)
		}
	}
	a.syncManualGearLevelMemory(cfg)
	a.logInfo("配置加载完成，配置路径: %s", cfg.ConfigPath)

	// 同步调试模式配置
	if cfg.DebugMode {
		a.debugMode = true
		if a.logger != nil {
			a.logger.SetDebugMode(true)
		}
		a.logInfo("从配置文件同步调试模式: 启用")
	}

	// 检查并同步Windows自启动状态
	a.logInfo("检查Windows自启动状态")
	actualAutoStart := a.autostartManager.CheckWindowsAutoStart()
	if actualAutoStart != cfg.WindowsAutoStart {
		cfg.WindowsAutoStart = actualAutoStart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("同步Windows自启动状态时保存配置失败: %v", err)
		} else {
			a.logInfo("已同步Windows自启动状态: %v", actualAutoStart)
		}
	}

	// 初始化HID
	a.logInfo("初始化HID库")
	if err := a.deviceManager.Init(); err != nil {
		a.logError("初始化HID库失败: %v", err)
		return err
	}
	a.logInfo("HID库初始化成功")

	// 设置设备回调
	a.deviceManager.SetCallbacks(a.onFanDataUpdate, a.onDeviceDisconnect)

	// 启动 IPC 服务器
	a.logInfo("启动 IPC 服务器")
	a.ipcServer = ipc.NewServer(a.handleIPCRequest, a.logger)
	if err := a.ipcServer.Start(); err != nil {
		a.logError("启动 IPC 服务器失败: %v", err)
		return err
	}
	if !a.legionFnQSupportChecked.Load() {
		a.startLegionFnQSupportDetection()
	}

	// 初始化系统托盘
	a.logInfo("开始初始化系统托盘")
	a.initSystemTray()
	a.applyHotkeyBindings(cfg)
	a.applyPluginConfig(cfg)

	// 注册系统睡眠/唤醒通知：睡眠前主动断开设备/桥接，唤醒后恢复，避免唤醒崩溃。
	if stop, err := powernotify.RegisterSuspendResumeNotifications(a.onSystemSuspend, a.onSystemResume); err != nil {
		a.logError("注册系统电源通知失败（将退化为基于时间间隔的唤醒检测）: %v", err)
	} else {
		a.powerNotifyStop = stop
		a.logInfo("已注册系统睡眠/唤醒通知")
	}

	// 启动健康监控
	if cfg.GuiMonitoring {
		a.logInfo("启动健康监控")
		a.safeGo("startHealthMonitoring", func() {
			a.startHealthMonitoring()
		})
	}

	a.logInfo("=== THRM 核心服务启动完成 ===")

	// 软件启动后立即开始温度监控（与智能控温开关解耦）
	a.safeGo("startTemperatureMonitoring@Start", func() {
		a.startTemperatureMonitoring()
	})

	// 尝试连接设备
	a.safeGo("delayedConnectDevice", func() {
		if a.isAutoStartLaunch {
			// 自启动时等待更长时间，让设备固件有足够时间完成初始化
			a.logInfo("自启动模式：等待设备初始化（3秒）")
			time.Sleep(3 * time.Second)
		} else {
			time.Sleep(1 * time.Second)
		}
		a.ConnectDevice()
	})

	return nil
}

// Stop 停止核心服务
func (a *CoreApp) Stop() {
	a.logInfo("核心服务正在停止...")
	if a.powerNotifyStop != nil {
		a.safeRun("power-notify-unregister", a.powerNotifyStop)
		a.powerNotifyStop = nil
	}
	a.stopTemperatureMonitoring()
	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
	}
	if a.pluginManager != nil {
		a.pluginManager.StopAll()
	}

	// 清理资源
	a.cleanup()

	// 停止所有监控
	a.DisconnectDevice()

	// 停止桥接程序
	a.bridgeManager.Stop()

	// 停止 IPC 服务器
	if a.ipcServer != nil {
		a.ipcServer.Stop()
	}

	// 停止托盘
	a.trayManager.Quit()

	a.logInfo("核心服务已停止")
}

// initSystemTray 初始化系统托盘
func (a *CoreApp) initSystemTray() {
	a.trayManager.SetCallbacks(
		a.onShowWindowRequest,
		a.onQuitRequest,
		a.onRestartRequest,
		func() bool {
			cfg := a.configManager.Get()
			newState := !cfg.AutoControl
			a.SetAutoControl(newState)
			return newState
		},
		func(profileID string) string {
			profile, err := a.SetActiveFanCurveProfile(profileID)
			if err != nil {
				a.logError("托盘设置温控曲线失败: %v", err)
				return ""
			}
			return profile.Name
		},
		func() ([]tray.CurveOption, string) {
			cfg := a.configManager.Get()
			options := make([]tray.CurveOption, 0, len(cfg.FanCurveProfiles))
			for _, p := range cfg.FanCurveProfiles {
				if p.ID == "" {
					continue
				}
				name := p.Name
				if strings.TrimSpace(name) == "" {
					name = "默认"
				}
				options = append(options, tray.CurveOption{ID: p.ID, Name: name})
			}
			return options, cfg.ActiveFanCurveProfileID
		},
		func() tray.Status {
			a.mutex.RLock()
			defer a.mutex.RUnlock()
			cfg := a.configManager.Get()
			fanData := a.deviceManager.GetCurrentFanData()
			var currentRPM uint16
			if fanData != nil {
				currentRPM = fanData.CurrentRPM
			}
			curveOptions := make([]tray.CurveOption, 0, len(cfg.FanCurveProfiles))
			for _, p := range cfg.FanCurveProfiles {
				if p.ID == "" {
					continue
				}
				name := p.Name
				if strings.TrimSpace(name) == "" {
					name = "默认"
				}
				curveOptions = append(curveOptions, tray.CurveOption{ID: p.ID, Name: name})
			}

			return tray.Status{
				Connected:            a.isConnected,
				CPUTemp:              a.currentTemp.CPUTemp,
				GPUTemp:              a.currentTemp.GPUTemp,
				CurrentRPM:           currentRPM,
				AutoControlState:     cfg.AutoControl,
				ActiveCurveProfileID: cfg.ActiveFanCurveProfileID,
				CurveProfiles:        curveOptions,
			}
		},
	)
	a.trayManager.Init()
}

// cleanup 清理资源
func (a *CoreApp) cleanup() {
	if a.healthCheckTicker != nil {
		a.healthCheckTicker.Stop()
	}

	select {
	case a.cleanupChan <- true:
	default:
	}

	if a.logger != nil {
		a.logger.Info("核心服务正在退出，清理资源")
		a.logger.Close()
	}
}

// 日志辅助方法
func (a *CoreApp) logInfo(format string, v ...any) {
	if a.logger != nil {
		a.logger.Info(format, v...)
	}
}

func (a *CoreApp) logError(format string, v ...any) {
	if a.logger != nil {
		a.logger.Error(format, v...)
	}
}

func (a *CoreApp) logDebug(format string, v ...any) {
	if a.logger != nil {
		a.logger.Debug(format, v...)
	}
}

func (a *CoreApp) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				CapturePanic(a, "goroutine:"+name, r)
			}
		}()

		fn()
	}()
}

// QuitChan exposes the internal quit signal for the thin cmd/core entrypoint.
func (a *CoreApp) QuitChan() <-chan bool {
	return a.quitChan
}

// LogInfo keeps logging available to the thin cmd/core entrypoint without exposing internals.
func (a *CoreApp) LogInfo(format string, v ...any) {
	a.logInfo(format, v...)
}

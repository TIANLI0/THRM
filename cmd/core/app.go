package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/appmeta"
	"github.com/TIANLI0/BS2PRO-Controller/internal/autostart"
	"github.com/TIANLI0/BS2PRO-Controller/internal/bridge"
	"github.com/TIANLI0/BS2PRO-Controller/internal/config"
	"github.com/TIANLI0/BS2PRO-Controller/internal/device"
	hotkeysvc "github.com/TIANLI0/BS2PRO-Controller/internal/hotkey"
	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/logger"
	"github.com/TIANLI0/BS2PRO-Controller/internal/notifier"
	"github.com/TIANLI0/BS2PRO-Controller/internal/plugins"
	"github.com/TIANLI0/BS2PRO-Controller/internal/plugins/fnqpowermode"
	"github.com/TIANLI0/BS2PRO-Controller/internal/smartcontrol"
	"github.com/TIANLI0/BS2PRO-Controller/internal/temperature"
	"github.com/TIANLI0/BS2PRO-Controller/internal/tray"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/TIANLI0/BS2PRO-Controller/internal/version"
	"golang.org/x/sys/windows/registry"
)

//go:embed icon.ico
var iconData []byte

// CoreApp 核心应用结构
type CoreApp struct {
	ctx context.Context

	// 管理器
	deviceManager    *device.Manager
	bridgeManager    *bridge.Manager
	tempReader       *temperature.Reader
	tempHistory      *temperature.HistoryRecorder
	configManager    *config.Manager
	trayManager      *tray.Manager
	hotkeyManager    *hotkeysvc.Manager
	notifier         *notifier.Manager
	autostartManager *autostart.Manager
	pluginManager    *plugins.Manager
	logger           *logger.CustomLogger
	ipcServer        *ipc.Server

	// 状态
	isConnected             bool
	monitoringTemp          atomic.Bool
	currentTemp             types.TemperatureData
	lastDeviceMode          string
	userSetAutoControl      bool
	isAutoStartLaunch       bool
	debugMode               bool
	legionFnQSupported      atomic.Bool
	legionFnQSupportChecked atomic.Bool
	legionFnQRegistered     atomic.Bool
	reconnectInProgress     atomic.Bool
	autoReconnectSuppressed atomic.Bool
	resumeRecoveryRunning   atomic.Bool
	systemSuspended         atomic.Bool
	lastResumeRecoveryUnix  int64

	// 系统电源（睡眠/唤醒）通知注销函数
	powerNotifyStop func()

	// 监控相关
	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	cleanupChan       chan bool
	quitChan          chan bool

	// 同步
	mutex                 sync.RWMutex
	stopMonitoring        chan bool
	manualGearLevelMemory map[string]string
}

const (
	systemResumeDetectionFloor   = 20 * time.Second
	systemResumeDetectionCeiling = 45 * time.Second
	systemResumeRecoveryCooldown = 15 * time.Second
	systemResumeReconnectDelay   = 3 * time.Second
	pawnIOInstallerTimeout       = 90 * time.Second
	pawnIOAlreadyExistsExitCode  = 183
	pawnIORegistryPath           = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO`
)

func systemResumeDetectionThreshold(expectedInterval time.Duration) time.Duration {
	threshold := min(max(expectedInterval*6, systemResumeDetectionFloor), systemResumeDetectionCeiling)
	return threshold
}

func shouldRecoverFromSystemResumeGap(gap, expectedInterval time.Duration) bool {
	return gap >= systemResumeDetectionThreshold(expectedInterval)
}

// NewCoreApp 创建核心应用实例
func NewCoreApp(debugMode, isAutoStart bool) *CoreApp {
	// 初始化日志系统
	installDir := config.GetInstallDir()
	customLogger, err := logger.NewCustomLogger(debugMode, installDir)
	if err != nil {
		// 如果初始化失败，无法记录，直接退出
		panic(fmt.Sprintf("初始化日志系统失败: %v", err))
	} else {
		customLogger.Info("核心服务启动")
		customLogger.Info("安装目录: %s", installDir)
		customLogger.Info("调试模式: %v", debugMode)
		customLogger.Info("自启动模式: %v", isAutoStart)
		customLogger.CleanOldLogs()
	}

	// 创建管理器
	bridgeMgr := bridge.NewManager(customLogger)
	deviceMgr := device.NewManager(customLogger)
	tempReader := temperature.NewReader(bridgeMgr, customLogger)
	configMgr := config.NewManager(installDir, customLogger)
	historyPath := filepath.Join(installDir, temperature.DefaultHistoryRelativePath)
	tempHistory := temperature.NewHistoryRecorder(historyPath, temperature.DefaultHistoryCapacity, temperature.DefaultHistorySampleInterval, customLogger)
	trayMgr := tray.NewManager(customLogger, iconData)
	autostartMgr := autostart.NewManager(customLogger)
	pluginMgr := plugins.NewManager(customLogger)

	app := &CoreApp{
		ctx:                context.Background(),
		deviceManager:      deviceMgr,
		bridgeManager:      bridgeMgr,
		tempReader:         tempReader,
		tempHistory:        tempHistory,
		currentTemp:        types.TemperatureData{BridgeOk: true},
		configManager:      configMgr,
		trayManager:        trayMgr,
		autostartManager:   autostartMgr,
		pluginManager:      pluginMgr,
		logger:             customLogger,
		isConnected:        false,
		stopMonitoring:     make(chan bool, 1),
		lastDeviceMode:     "",
		userSetAutoControl: false,
		isAutoStartLaunch:  isAutoStart,
		debugMode:          debugMode,
		guiLastResponse:    time.Now().Unix(),
		cleanupChan:        make(chan bool, 1),
		quitChan:           make(chan bool, 1),
		guiMonitorEnabled:  true,
		manualGearLevelMemory: map[string]string{
			"静音": "中",
			"标准": "中",
			"强劲": "中",
			"超频": "中",
		},
	}
	app.notifier = notifier.NewManager(customLogger, iconData)
	app.hotkeyManager = hotkeysvc.NewManager(customLogger, app.handleHotkeyAction)

	return app
}

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
	if normalizeCurveProfilesConfig(&cfg) {
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
	if stop, err := registerSuspendResumeNotifications(a.onSystemSuspend, a.onSystemResume); err != nil {
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

func (a *CoreApp) registerPlugins() {
	if a.pluginManager == nil {
		return
	}
	if !a.legionFnQRegistered.CompareAndSwap(false, true) {
		return
	}

	a.pluginManager.Register(fnqpowermode.New(fnqpowermode.Options{
		Logger: a.logger,
		OnModeChange: func(state fnqpowermode.PowerModeState) {
			a.handleLegionPowerModeChange(state)
		},
	}))
}

func (a *CoreApp) applyCachedLegionFnQSupport(cfg *types.AppConfig) bool {
	if cfg == nil || !cfg.LegionFnQSupport.Checked {
		return false
	}

	a.legionFnQSupportChecked.Store(true)
	a.legionFnQSupported.Store(cfg.LegionFnQSupport.Supported)
	a.logInfo("Lenovo Legion Fn+Q host support loaded from config cache: supported=%v", cfg.LegionFnQSupport.Supported)

	if cfg.LegionFnQSupport.Supported {
		a.registerPlugins()
		return false
	}

	return a.normalizeLegionFnQConfigForHost(cfg)
}

func (a *CoreApp) startLegionFnQSupportDetection() {
	a.safeGo("detectLegionFnQSupport", func() {
		supported, hostInfo, err := fnqpowermode.DetectSupport()
		if err != nil {
			a.logError("failed to detect Lenovo Legion Fn+Q host support: %v", err)
			return
		}

		a.cacheLegionFnQSupportResult(supported)
		a.legionFnQSupportChecked.Store(true)
		if !supported {
			a.logInfo("Lenovo Legion Fn+Q plugin skipped: unsupported host (manufacturer=%s model=%s family=%s product=%s)",
				hostInfo.Manufacturer, hostInfo.Model, hostInfo.Family, hostInfo.Product)
			a.disableLegionFnQConfigForUnsupportedHost()
			a.broadcastLegionFnQSupportUpdate(false)
			return
		}

		a.registerPlugins()
		a.legionFnQSupported.Store(true)
		a.broadcastLegionFnQSupportUpdate(true)

		a.mutex.RLock()
		cfg := a.configManager.Get()
		a.mutex.RUnlock()
		a.applyPluginConfig(cfg)
	})
}

func (a *CoreApp) cacheLegionFnQSupportResult(supported bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if cfg.LegionFnQSupport.Checked && cfg.LegionFnQSupport.Supported == supported {
		return
	}

	cfg.LegionFnQSupport = types.LegionFnQSupportCache{
		Checked:   true,
		Supported: supported,
	}
	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		a.logError("保存 Lenovo Legion Fn+Q 支持缓存失败: %v", err)
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
}

func (a *CoreApp) disableLegionFnQConfigForUnsupportedHost() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	if !a.normalizeLegionFnQConfigForHost(&cfg) {
		return
	}

	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		a.logError("保存 Lenovo Legion Fn+Q 配置失败: %v", err)
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
}

func (a *CoreApp) broadcastLegionFnQSupportUpdate(supported bool) {
	if a.ipcServer == nil {
		return
	}
	a.ipcServer.BroadcastEvent(ipc.EventLegionFnQSupportUpdate, map[string]any{
		"supported": supported,
	})
}

func (a *CoreApp) handleLegionPowerModeChange(state fnqpowermode.PowerModeState) {
	if !a.legionFnQSupported.Load() {
		return
	}

	a.logInfo("Lenovo Legion Fn+Q power mode changed: raw=%d mapped=%d mode=%s source=%s",
		state.Raw, state.Mapped, state.Mode, state.Source)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventLegionPowerModeUpdate, state)
	}

	a.applyLegionFnQFanMapping(state)
}

func (a *CoreApp) applyPluginConfig(cfg types.AppConfig) {
	if a.pluginManager == nil || !a.legionFnQSupported.Load() {
		return
	}

	if cfg.LegionFnQ.Enabled {
		if err := a.pluginManager.Start(fnqpowermode.PluginID); err != nil {
			a.logError("failed to start Lenovo Legion Fn+Q plugin: %v", err)
		}
		return
	}

	if err := a.pluginManager.Stop(fnqpowermode.PluginID); err != nil {
		a.logError("failed to stop Lenovo Legion Fn+Q plugin: %v", err)
	}
}

func (a *CoreApp) applyLegionFnQFanMapping(state fnqpowermode.PowerModeState) {
	cfg := a.configManager.Get()
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
	if !cfg.LegionFnQ.Enabled || !cfg.LegionFnQ.TakeOverFan {
		return
	}
	if !a.isConnected {
		a.logDebug("Lenovo Legion Fn+Q takeover skipped: device not connected")
		return
	}

	target, ok := cfg.LegionFnQ.ModeMapping[state.Mode]
	if !ok {
		a.logDebug("Lenovo Legion Fn+Q takeover skipped: no mapping for mode=%s", state.Mode)
		return
	}

	if cfg.AutoControl || cfg.CustomSpeedEnabled {
		cfg.AutoControl = false
		cfg.CustomSpeedEnabled = false
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("failed to save Lenovo Legion Fn+Q takeover config: %v", err)
		}
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	}

	a.safeGo("applyLegionFnQFanMapping", func() {
		if ok := a.SetManualGear(target.Gear, target.Level); !ok {
			a.logError("Lenovo Legion Fn+Q takeover failed: mode=%s gear=%s level=%s", state.Mode, target.Gear, target.Level)
			return
		}
		a.logInfo("Lenovo Legion Fn+Q takeover applied: mode=%s gear=%s level=%s", state.Mode, target.Gear, target.Level)
	})
}

func (a *CoreApp) normalizeLegionFnQConfigForHost(cfg *types.AppConfig) bool {
	if cfg == nil || !a.legionFnQSupportChecked.Load() || a.legionFnQSupported.Load() {
		return false
	}

	changed := false
	if cfg.LegionFnQ.Enabled {
		cfg.LegionFnQ.Enabled = false
		changed = true
	}
	if cfg.LegionFnQ.TakeOverFan {
		cfg.LegionFnQ.TakeOverFan = false
		changed = true
	}

	return changed
}

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

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	a.logDebug("处理 IPC 请求[%s] type=%s", req.RequestID, req.Type)

	switch req.Type {
	// 设备相关
	case ipc.ReqConnect:
		success := a.ConnectDevice()
		return a.successResponse(success)

	case ipc.ReqDisconnect:
		a.DisconnectDevice()
		return a.successResponse(true)

	case ipc.ReqGetDeviceStatus:
		status := a.GetDeviceStatus()
		return a.dataResponse(status)

	case ipc.ReqGetCurrentFanData:
		data := a.deviceManager.GetCurrentFanData()
		return a.dataResponse(data)

	// 配置相关
	case ipc.ReqGetConfig:
		cfg := a.configManager.Get()
		return a.dataResponse(cfg)

	case ipc.ReqUpdateConfig:
		var cfg types.AppConfig
		if err := json.Unmarshal(req.Data, &cfg); err != nil {
			return a.errorResponse("解析配置失败: " + err.Error())
		}
		if err := a.UpdateConfig(cfg); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetFanCurve:
		var curve []types.FanCurvePoint
		if err := json.Unmarshal(req.Data, &curve); err != nil {
			return a.errorResponse("解析风扇曲线失败: " + err.Error())
		}
		if err := a.SetFanCurve(curve); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqGetFanCurve:
		curve := a.configManager.Get().FanCurve
		return a.dataResponse(curve)

	case ipc.ReqResetLearnedOffsets:
		if err := a.ResetLearnedOffsets(); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(map[string]bool{"ok": true})

	case ipc.ReqGetFanCurveProfiles:
		return a.dataResponse(a.GetFanCurveProfiles())

	case ipc.ReqSetActiveFanCurveProfile:
		var params ipc.SetActiveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		profile, err := a.SetActiveFanCurveProfile(params.ID)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqSaveFanCurveProfile:
		var params ipc.SaveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		profile, err := a.SaveFanCurveProfile(params)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqDeleteFanCurveProfile:
		var params ipc.DeleteFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.DeleteFanCurveProfile(params.ID); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqExportFanCurveProfiles:
		code, err := a.ExportFanCurveProfiles()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(code)

	case ipc.ReqImportFanCurveProfiles:
		var params ipc.ImportFanCurveProfilesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.ImportFanCurveProfiles(params.Code); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 控制相关
	case ipc.ReqSetAutoControl:
		var params ipc.SetAutoControlParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetAutoControl(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetManualGear:
		var params ipc.SetManualGearParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetManualGear(params.Gear, params.Level)
		return a.successResponse(success)

	case ipc.ReqGetAvailableGears:
		gears := types.GearCommands
		return a.dataResponse(gears)

	case ipc.ReqSetCustomSpeed:
		var params ipc.SetCustomSpeedParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetGearLight:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetGearLight(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetPowerOnStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetPowerOnStart(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetSmartStartStop:
		var params ipc.SetStringParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetSmartStartStop(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetBrightness:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetBrightness(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetLightStrip:
		var params ipc.SetLightStripParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetLightStrip(params.Config); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 温度相关
	case ipc.ReqGetTemperature:
		a.mutex.RLock()
		temp := a.currentTemp
		a.mutex.RUnlock()
		return a.dataResponse(temp)

	case ipc.ReqGetTemperatureHistory:
		return a.dataResponse(a.tempHistory.Snapshot())

	case ipc.ReqSetTemperatureHistoryEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetTemperatureHistoryEnabled(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqTestTemperatureReading:
		cfg := a.configManager.Get()
		temp := a.tempReader.Read(types.TemperatureSelection{
			TempSource: cfg.TempSource,
			GpuDevice:  cfg.GpuDevice,
			CpuSensor:  cfg.CpuSensor,
			GpuSensor:  cfg.GpuSensor,
		})
		return a.dataResponse(temp)

	case ipc.ReqTestBridgeProgram:
		cfg := a.configManager.Get()
		data := a.bridgeManager.GetTemperature(types.TemperatureSelection{
			TempSource: cfg.TempSource,
			GpuDevice:  cfg.GpuDevice,
			CpuSensor:  cfg.CpuSensor,
			GpuSensor:  cfg.GpuSensor,
		})
		return a.dataResponse(data)

	case ipc.ReqGetBridgeProgramStatus:
		status := a.bridgeManager.GetStatus()
		return a.dataResponse(status)

	case ipc.ReqRestartPawnIO:
		result, err := a.bridgeManager.RestartPawnIO()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(result)

	case ipc.ReqReinstallPawnIO:
		result, err := a.ReinstallPawnIO()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(result)

	// 自启动相关
	case ipc.ReqSetWindowsAutoStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetWindowsAutoStart(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqCheckWindowsAutoStart:
		enabled := a.autostartManager.CheckWindowsAutoStart()
		return a.dataResponse(enabled)

	case ipc.ReqIsRunningAsAdmin:
		isAdmin := a.autostartManager.IsRunningAsAdmin()
		return a.dataResponse(isAdmin)

	case ipc.ReqGetAutoStartMethod:
		method := a.autostartManager.GetAutoStartMethod()
		return a.dataResponse(method)

	case ipc.ReqSetAutoStartWithMethod:
		var params ipc.SetAutoStartWithMethodParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.autostartManager.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 窗口相关
	case ipc.ReqShowWindow:
		a.onShowWindowRequest()
		return a.successResponse(true)

	case ipc.ReqHideWindow:
		// GUI 自己处理隐藏
		return a.successResponse(true)

	case ipc.ReqQuitApp:
		a.safeGo("onQuitRequest", func() {
			a.onQuitRequest()
		})
		return a.successResponse(true)

	// 调试相关
	case ipc.ReqGetDebugInfo:
		info := a.GetDebugInfo()
		return a.dataResponse(info)

	case ipc.ReqSetDebugMode:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetDebugMode(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqUpdateGuiResponseTime:
		atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
		return a.successResponse(true)

	// 系统相关
	case ipc.ReqPing:
		return a.dataResponse("pong")

	case ipc.ReqIsAutoStartLaunch:
		return a.dataResponse(a.isAutoStartLaunch)

	default:
		return a.errorResponse(fmt.Sprintf("未知的请求类型: %s", req.Type))
	}
}

// 响应辅助方法
func (a *CoreApp) successResponse(success bool) ipc.Response {
	data, _ := json.Marshal(success)
	return ipc.Response{Success: true, Data: data}
}

func (a *CoreApp) errorResponse(errMsg string) ipc.Response {
	return ipc.Response{Success: false, Error: errMsg}
}

func (a *CoreApp) dataResponse(data any) ipc.Response {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return a.errorResponse("序列化数据失败: " + err.Error())
	}
	return ipc.Response{Success: true, Data: dataBytes}
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
		30 * time.Second,
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
	a.logInfo("收到系统挂起通知：提前停止监控并断开设备/桥接，避免唤醒后失效句柄导致崩溃")

	a.autoReconnectSuppressed.Store(true)
	a.stopTemperatureMonitoring()

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
		30 * time.Second,
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

		if deviceInfo != nil && a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceConnected, deviceInfo)
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
	a.mutex.Unlock()

	a.deviceManager.DisconnectSilently()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// ReinstallPawnIO runs the bundled PawnIO installer stored under the install directory.
func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	installDir := config.GetInstallDir()
	installerPath := appmeta.FirstExistingPath(appmeta.PawnIOInstallerCandidates(installDir))
	if installerPath == "" {
		return nil, fmt.Errorf("未找到 PawnIO 安装包，已尝试路径: %v", appmeta.PawnIOInstallerCandidates(installDir))
	}

	result := map[string]any{
		"success": false,
		"path":    installerPath,
	}
	installedVersionBefore := readInstalledPawnIOVersion()
	if installedVersionBefore != "" {
		result["installedVersionBefore"] = installedVersionBefore
	}

	a.logInfo("开始重新安装 PawnIO: %s", installerPath)
	a.bridgeManager.Stop()

	if installedVersionBefore != "" {
		a.logInfo("检测到已安装 PawnIO (版本: %s)，先执行卸载再安装", installedVersionBefore)
		uninstallStep, uninstallErr := a.runPawnIOInstaller(installerPath, "uninstall", "-uninstall", "-silent")
		result["uninstall"] = uninstallStep
		if uninstallErr != nil {
			if isPawnIOInstallerTimeout(uninstallErr) {
				result["error"] = "PawnIO 卸载超时"
				return result, fmt.Errorf("PawnIO 卸载超时，请稍后检查驱动状态或手动运行 %s", installerPath)
			}
			result["uninstallWarning"] = uninstallErr.Error()
			a.logError("PawnIO 卸载返回错误，将继续尝试安装: %v", uninstallErr)
		}
	}

	installStep, installErr := a.runPawnIOInstaller(installerPath, "install", "-install", "-silent")
	result["install"] = installStep
	installedVersionAfter := readInstalledPawnIOVersion()
	if installedVersionAfter != "" {
		result["installedVersionAfter"] = installedVersionAfter
	}

	if installErr != nil {
		if isPawnIOInstallerTimeout(installErr) {
			result["error"] = "PawnIO 安装超时"
			return result, fmt.Errorf("PawnIO 安装超时，请稍后检查驱动状态或手动运行 %s", installerPath)
		}

		if pawnIOInstallerExitCode(installErr) == pawnIOAlreadyExistsExitCode && installedVersionAfter != "" {
			result["alreadyInstalled"] = true
			result["warning"] = "PawnIO 安装器返回 183（已存在），已确认系统中仍有 PawnIO 安装记录。"
			a.logInfo("PawnIO 安装器返回 183，检测到已安装版本 %s，按非致命结果处理", installedVersionAfter)
		} else {
			result["error"] = installErr.Error()
			return result, formatPawnIOInstallerError("PawnIO 安装失败", installErr, installStep)
		}
	} else {
		a.logInfo("PawnIO 安装程序执行完成")
	}

	result["success"] = true
	bridgeResult, bridgeErr := a.bridgeManager.RestartPawnIO()
	if bridgeErr != nil {
		result["bridgeWarning"] = bridgeErr.Error()
		a.logError("PawnIO 安装后重新初始化温度监控失败: %v", bridgeErr)
	} else {
		result["bridge"] = bridgeResult
	}

	return result, nil
}

func (a *CoreApp) runPawnIOInstaller(installerPath, action string, args ...string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(a.ctx, pawnIOInstallerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, installerPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	outputText := strings.TrimSpace(string(output))
	step := map[string]any{
		"action":   action,
		"args":     args,
		"success":  err == nil,
		"exitCode": pawnIOInstallerExitCode(err),
	}
	if outputText != "" {
		step["output"] = outputText
	}
	if ctx.Err() == context.DeadlineExceeded {
		step["timeout"] = true
		step["success"] = false
		return step, ctx.Err()
	}
	if err != nil {
		step["error"] = err.Error()
	}
	return step, err
}

func readInstalledPawnIOVersion() string {
	for _, access := range []uint32{registry.QUERY_VALUE | registry.WOW64_64KEY, registry.QUERY_VALUE | registry.WOW64_32KEY} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, pawnIORegistryPath, access)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("DisplayVersion")
		_ = key.Close()
		if err == nil && strings.TrimSpace(version) != "" {
			return strings.TrimSpace(version)
		}
	}
	return ""
}

func pawnIOInstallerExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func isPawnIOInstallerTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func formatPawnIOInstallerError(prefix string, err error, step map[string]any) error {
	if output, ok := step["output"].(string); ok && output != "" {
		return fmt.Errorf("%s: %v；输出: %s", prefix, err, output)
	}
	return fmt.Errorf("%s: %v", prefix, err)
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
	defer a.mutex.RUnlock()

	productID := a.deviceManager.GetProductID()
	productIDHex := ""
	if productID != 0 {
		productIDHex = fmt.Sprintf("0x%04X", productID)
	}

	model := a.deviceManager.GetModelName()

	return map[string]any{
		"connected":   a.isConnected,
		"monitoring":  a.monitoringTemp.Load(),
		"currentData": a.deviceManager.GetCurrentFanData(),
		"temperature": a.currentTemp,
		"productId":   productIDHex,
		"model":       model,
	}
}

// UpdateConfig 更新配置
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	oldCfg := a.configManager.Get()
	if len(cfg.FanCurveProfiles) == 0 && len(oldCfg.FanCurveProfiles) > 0 {
		cfg.FanCurveProfiles = cloneFanCurveProfiles(oldCfg.FanCurveProfiles)
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
	normalizeCurveProfilesConfig(&cfg)
	if idx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = cloneFanCurve(cfg.FanCurve)
	}
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	cfg.LegionFnQ = types.NormalizeLegionFnQConfig(cfg.LegionFnQ)
	if a.legionFnQSupportChecked.Load() && !a.legionFnQSupported.Load() && (cfg.LegionFnQ.Enabled || cfg.LegionFnQ.TakeOverFan) {
		return fmt.Errorf("Lenovo Legion Fn+Q 仅支持拯救者设备")
	}
	normalizeHotkeyConfig(&cfg)
	normalizeManualGearMemoryConfig(&cfg)

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
	normalizeCurveProfilesConfig(&cfg)
	cfg.FanCurve = cloneFanCurve(curve)
	idx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = cloneFanCurve(cfg.FanCurve)
	}
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	return a.configManager.Update(cfg)
}

// ResetLearnedOffsets 清空学习到的曲线偏移。
func (a *CoreApp) ResetLearnedOffsets() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.SmartControl = smartcontrol.ResetLearnedState(cfg.SmartControl, cfg.FanCurve)
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

	a.logInfo("应用当前挡位设置: %s %s", setGear, level)
	a.deviceManager.SetManualGear(setGear, level)
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
	a.configManager.Update(cfg)
	a.rememberManualGearLevel(gear, level)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return a.deviceManager.SetManualGear(gear, level)
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

func normalizeHotkeyConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.ManualGearToggleHotkey); err != nil {
			cfg.ManualGearToggleHotkey = types.GetDefaultConfig(false).ManualGearToggleHotkey
			changed = true
		}
	}
	if cfg.AutoControlToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.AutoControlToggleHotkey); err != nil {
			cfg.AutoControlToggleHotkey = types.GetDefaultConfig(false).AutoControlToggleHotkey
			changed = true
		}
	}
	if cfg.CurveProfileToggleHotkey != "" {
		if _, _, err := hotkeysvc.ParseShortcut(cfg.CurveProfileToggleHotkey); err != nil {
			cfg.CurveProfileToggleHotkey = types.GetDefaultConfig(false).CurveProfileToggleHotkey
			changed = true
		}
	}

	return changed
}

func normalizeManualGearMemoryConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
		changed = true
	}

	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if level, ok := cfg.ManualGearLevels[gear]; !ok {
			cfg.ManualGearLevels[gear] = "中"
			changed = true
		} else {
			normalized := normalizeManualLevel(level)
			if normalized != level {
				cfg.ManualGearLevels[gear] = normalized
				changed = true
			}
		}
	}

	normalizedCurrent := normalizeManualLevel(cfg.ManualLevel)
	if normalizedCurrent != cfg.ManualLevel {
		cfg.ManualLevel = normalizedCurrent
		changed = true
	}

	if cfg.ManualGear != "" {
		if remembered, ok := cfg.ManualGearLevels[cfg.ManualGear]; !ok || remembered != normalizedCurrent {
			cfg.ManualGearLevels[cfg.ManualGear] = normalizedCurrent
			changed = true
		}
	}

	return changed
}

func (a *CoreApp) applyHotkeyBindings(cfg types.AppConfig) {
	if a.hotkeyManager == nil {
		return
	}
	if err := a.hotkeyManager.UpdateBindings(cfg.ManualGearToggleHotkey, cfg.AutoControlToggleHotkey, cfg.CurveProfileToggleHotkey); err != nil {
		a.logError("更新全局快捷键失败: %v", err)
	}
}

func (a *CoreApp) handleHotkeyAction(action hotkeysvc.Action, shortcut string) {
	a.safeGo("handleHotkeyAction", func() {
		var message string
		success := true

		switch action {
		case hotkeysvc.ActionToggleManualGear:
			msg, err := a.toggleManualGearByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleAutoMode:
			msg, err := a.toggleAutoControlByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleCurveProfile:
			msg, err := a.toggleCurveProfileByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		default:
			success = false
			message = "未知快捷键动作"
		}

		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventHotkeyTriggered, map[string]any{
				"action":   string(action),
				"shortcut": shortcut,
				"success":  success,
				"message":  message,
			})
		}

		title := appmeta.AppName + " 快捷键"
		if !success {
			title = appmeta.AppName + " 快捷键失败"
		}
		if a.notifier != nil {
			a.notifier.Notify(title, message)
		}
	})
}

func (a *CoreApp) toggleCurveProfileByHotkey() (string, error) {
	profile, err := a.CycleFanCurveProfile()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("温控曲线已切换: %s", profile.Name), nil
}

func (a *CoreApp) toggleAutoControlByHotkey() (string, error) {
	cfg := a.configManager.Get()
	target := !cfg.AutoControl
	if err := a.SetAutoControl(target); err != nil {
		return "", err
	}
	if target {
		return "智能变频已开启", nil
	}
	return "智能变频已关闭", nil
}

func (a *CoreApp) toggleManualGearByHotkey() (string, error) {
	cfg := a.configManager.Get()

	if cfg.AutoControl {
		if err := a.SetAutoControl(false); err != nil {
			return "", fmt.Errorf("切换到手动模式失败: %w", err)
		}
	}

	nextGear, nextLevel := a.getNextManualGearWithMemory(cfg.ManualGear, cfg.ManualLevel)
	if ok := a.SetManualGear(nextGear, nextLevel); !ok {
		return "", fmt.Errorf("应用手动挡位失败")
	}

	rpm := getManualGearRPM(nextGear, nextLevel)
	if rpm > 0 {
		return fmt.Sprintf("手动挡位: %s %s (%d RPM)", nextGear, nextLevel, rpm), nil
	}
	return fmt.Sprintf("手动挡位: %s %s", nextGear, nextLevel), nil
}

func (a *CoreApp) getNextManualGearWithMemory(currentGear, currentLevel string) (string, string) {
	sequence := []string{"静音", "标准", "强劲", "超频"}
	nextIndex := 0

	for i, gear := range sequence {
		if gear == currentGear {
			nextIndex = (i + 1) % len(sequence)
			break
		}
	}

	a.rememberManualGearLevel(currentGear, currentLevel)
	fallbackLevel := normalizeManualLevel(currentLevel)
	level := a.getRememberedManualLevel(sequence[nextIndex], fallbackLevel)

	return sequence[nextIndex], level
}

func normalizeManualLevel(level string) string {
	if level == "低" || level == "中" || level == "高" {
		return level
	}
	return "中"
}

func cloneManualGearLevels(source map[string]string) map[string]string {
	cloned := map[string]string{}
	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if source == nil {
			cloned[gear] = "中"
			continue
		}
		cloned[gear] = normalizeManualLevel(source[gear])
	}
	return cloned
}

func (a *CoreApp) syncManualGearLevelMemory(cfg types.AppConfig) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.syncManualGearLevelMemoryLocked(cfg)
}

func (a *CoreApp) syncManualGearLevelMemoryLocked(cfg types.AppConfig) {

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}

	defaultLevel := normalizeManualLevel(cfg.ManualLevel)
	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if fromCfg, ok := cfg.ManualGearLevels[gear]; ok {
			a.manualGearLevelMemory[gear] = normalizeManualLevel(fromCfg)
			continue
		}
		a.manualGearLevelMemory[gear] = defaultLevel
	}

	a.manualGearLevelMemory[cfg.ManualGear] = normalizeManualLevel(cfg.ManualLevel)
}

func (a *CoreApp) rememberManualGearLevel(gear, level string) {
	if gear != "静音" && gear != "标准" && gear != "强劲" && gear != "超频" {
		return
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}
	a.manualGearLevelMemory[gear] = normalizeManualLevel(level)
}

func (a *CoreApp) getRememberedManualLevel(gear, fallback string) string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.manualGearLevelMemory == nil {
		return normalizeManualLevel(fallback)
	}
	if level, ok := a.manualGearLevelMemory[gear]; ok {
		return normalizeManualLevel(level)
	}
	return normalizeManualLevel(fallback)
}

func getManualGearRPM(gear, level string) int {
	commands, ok := types.GearCommands[gear]
	if !ok {
		return 0
	}

	for _, cmd := range commands {
		if (level == "低" && containsLevel(cmd.Name, "低")) ||
			(level == "中" && containsLevel(cmd.Name, "中")) ||
			(level == "高" && containsLevel(cmd.Name, "高")) {
			return cmd.RPM
		}
	}

	return 0
}

func containsLevel(name, level string) bool {
	return strings.Contains(name, level)
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
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, enabled)
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

func (a *CoreApp) stopTemperatureMonitoring() {
	if !a.monitoringTemp.Load() {
		return
	}

	select {
	case a.stopMonitoring <- true:
	default:
	}
}

// startTemperatureMonitoring 开始温度监控
func (a *CoreApp) startTemperatureMonitoring() {
	// CAS：原子地从 false 翻到 true，确保 Start/ConnectDevice 并发调用时只有一条循环启动。
	if !a.monitoringTemp.CompareAndSwap(false, true) {
		return
	}

	// 清理可能残留的停止信号，避免新监控循环被立即中断。
	select {
	case <-a.stopMonitoring:
	default:
	}

	// 注意：不在此处立即调用 EnterAutoMode，因为在启动时温度数据（桥接程序）可能尚未就绪。
	// 如果在温度读取成功之前切换到软件控制模式，设备将不会收到转速指令，导致风扇停转。
	// EnterAutoMode 和转速设置会在首次成功读取温度后，由 SetFanSpeed 内部统一完成。

	cfg, cfgRevision := a.configManager.GetWithRevision()
	updateInterval := temperatureMonitorInterval(cfg.TempUpdateRate)

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
		GpuSensor:  cfg.GpuSensor,
	}
	initialTemp := a.tempReader.Read(initialSelection)
	if initialTemp.ControlTemp > 0 {
		rawTempHistory = append(rawTempHistory, initialTemp.ControlTemp)
	}
	lastTargetRPM := -1
	learningDirty := false
	lastLearningSave := time.Now()
	lastMonitorTick := time.Now()
	var smartCfg types.SmartControlConfig
	smartCfgRevision := cfgRevision - 1

	// 每个曲线点对应一个稳态采样桶。
	steadyObserver := smartcontrol.NewStableObserver(len(cfg.FanCurve))
	timer := time.NewTimer(updateInterval)
	defer timer.Stop()

	for a.monitoringTemp.Load() {
		select {
		case <-a.stopMonitoring:
			a.monitoringTemp.Store(false)
			return
		case <-timer.C:
			now := time.Now()
			gap := now.Sub(lastMonitorTick)
			lastMonitorTick = now
			if a.maybeRecoverFromSystemResume("temperature-monitor", gap, updateInterval) {
				timer.Reset(updateInterval)
				continue
			}

			cfg, cfgRevision = a.configManager.GetWithRevision()
			updateInterval = temperatureMonitorInterval(cfg.TempUpdateRate)
			selection := types.TemperatureSelection{
				TempSource: cfg.TempSource,
				GpuDevice:  cfg.GpuDevice,
				CpuSensor:  cfg.CpuSensor,
				GpuSensor:  cfg.GpuSensor,
			}
			temp := a.tempReader.Read(selection)

			a.mutex.Lock()
			a.currentTemp = temp
			a.mutex.Unlock()

			historyPoint, recorded := a.tempHistory.Add(temp, a.deviceManager.GetCurrentFanData())

			// 广播温度更新
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, temp)
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

				if prevTargetRPM >= 0 {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
					if targetRPM > 0 {
						targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
					}
				}

				fanData := a.deviceManager.GetCurrentFanData()
				if shouldSendTargetRPM(targetRPM, prevTargetRPM, smartCfg.MinRPMChange, fanData) {
					if a.deviceManager.SetFanSpeed(targetRPM) {
						lastTargetRPM = targetRPM
					} else {
						lastTargetRPM = -1
						a.logError("智能控温转速下发失败，将在下个周期重试: %d RPM", targetRPM)
					}
				}

				if smartCfg.Learning && !spikeSuppressed {
					steady := steadyObserver.Observe(controlTemp, targetRPM, cfg.FanCurve)
					if steady.Ready && steady.BucketIdx >= 0 {
						newOffsets, changed := smartcontrol.LearnSteadyOffset(
							steady.BucketIdx,
							steady.MeanTemp,
							steady.LocalEff,
							steady.HaveEff,
							cfg.FanCurve,
							smartCfg.LearnedOffsets,
							smartCfg,
						)
						if changed {
							smartCfg.LearnedOffsets = newOffsets
							cfg.SmartControl = smartCfg
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
			}

			if !cfg.AutoControl {
				lastTargetRPM = -1
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

func shouldSendTargetRPM(targetRPM, prevTargetRPM, minRPMChange int, fanData *types.FanData) bool {
	if targetRPM <= 0 {
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
	return deviceTargetRPM == 0 || absRPMDelta(targetRPM, deviceTargetRPM) >= minRPMChange
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
	a.logInfo("启动健康监控系统")

	a.healthCheckTicker = time.NewTicker(30 * time.Second)

	a.safeGo("healthMonitoringLoop", func() {
		defer a.healthCheckTicker.Stop()
		lastHealthCheck := time.Now()

		for {
			select {
			case <-a.healthCheckTicker.C:
				now := time.Now()
				gap := now.Sub(lastHealthCheck)
				lastHealthCheck = now
				if a.maybeRecoverFromSystemResume("health-monitor", gap, 30*time.Second) {
					continue
				}

				a.performHealthCheck()
			case <-a.cleanupChan:
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

// performHealthCheck 执行健康检查
func (a *CoreApp) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			a.logError("健康检查中发生panic: %v", r)
		}
	}()

	a.trayManager.CheckHealth()
	a.checkDeviceHealth()

	a.logDebug("健康检查完成 - 托盘:%v 设备连接:%v",
		a.trayManager.IsInitialized(), a.isConnected)
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
				capturePanic(a, "goroutine:"+name, r)
			}
		}()

		fn()
	}()
}

// launchGUI 启动 GUI 程序
func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	guiCandidates := append(appmeta.GUIExecutableCandidates(exeDir), appmeta.GUIExecutableCandidates(filepath.Join(exeDir, ".."))...)
	guiPath := appmeta.FirstExistingPath(guiCandidates)
	if guiPath == "" {
		return fmt.Errorf("GUI 程序不存在: %v", guiCandidates)
	}

	cmd := exec.Command(guiPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 GUI 程序失败: %v", err)
	}

	// 使用 fmt 而非日志系统，避免循环依赖
	fmt.Printf("GUI 程序已启动，PID: %d\n", cmd.Process.Pid)

	go func() {
		cmd.Wait()
	}()

	return nil
}

package coreapp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	hotkeysvc "github.com/TIANLI0/THRM/internal/hotkey"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/notifier"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/tray"
	"github.com/TIANLI0/THRM/internal/types"
)

// CoreApp 核心应用结构
type CoreApp struct {
	ctx context.Context

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

	isConnected             bool
	monitoringTemp          atomic.Bool
	currentTemp             types.TemperatureData
	deviceSettings          *types.DeviceSettings
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

	powerNotifyStop func()

	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	cleanupChan       chan bool
	quitChan          chan bool

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

	// restartGUIDelay 重启 GUI 前等待 GUI 客户端退出的时间
	restartGUIDelay = 500 * time.Millisecond
)

func systemResumeDetectionThreshold(expectedInterval time.Duration) time.Duration {
	threshold := min(max(expectedInterval*6, systemResumeDetectionFloor), systemResumeDetectionCeiling)
	return threshold
}

func shouldRecoverFromSystemResumeGap(gap, expectedInterval time.Duration) bool {
	return gap >= systemResumeDetectionThreshold(expectedInterval)
}

// NewCoreApp 创建核心应用实例。
func NewCoreApp(debugMode, isAutoStart bool, iconData []byte) *CoreApp {
	installDir := config.GetInstallDir()
	customLogger, err := logger.NewCustomLogger(debugMode, installDir)
	if err != nil {
		panic(fmt.Sprintf("初始化日志系统失败: %v", err))
	} else {
		customLogger.Info("核心服务启动")
		customLogger.Info("安装目录: %s", installDir)
		customLogger.Info("调试模式: %v", debugMode)
		customLogger.Info("自启动模式: %v", isAutoStart)
		customLogger.CleanOldLogs()
	}

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

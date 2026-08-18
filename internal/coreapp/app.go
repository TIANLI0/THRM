package coreapp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/device"
	hotkeysvc "github.com/TIANLI0/THRM/internal/hotkey"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/laptopfan"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/notifier"
	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/rtss"
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
	laptopFanReader  *laptopfan.Reader
	configManager    *config.Manager
	trayManager      *tray.Manager
	hotkeyManager    *hotkeysvc.Manager
	notifier         *notifier.Manager
	autostartManager *autostart.Manager
	pluginManager    *plugins.Manager
	rtssPublisher    *rtss.Publisher
	logger           *logger.CustomLogger
	ipcServer        *ipc.Server

	isConnected    bool
	monitoringTemp atomic.Bool
	// extendedSensorsUntil 是"临时全开扩展传感器"的到期时刻（UnixMilli，0=关闭）。
	// 内存、硬盘、主板/EC 这些温度只有散热收益测试需要，而打开它们要额外建立
	// SMART/SPD/Super I/O 通道，对 7×24 挂托盘的核心不是零成本。所以既不做成
	// 持久化设置，也不常开——测试期间临时打开，结束即关。
	//
	// 存到期时刻而不是布尔，是因为它必须能自己关掉：GUI 崩了或卡死时没人来发关闭
	// 指令，一个永远开着的扩展轮询正是这里要避免的东西。
	extendedSensorsUntil    atomic.Int64
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
	resumeReconnectWanted   atomic.Bool
	suspendGeneration       atomic.Uint64
	connectGeneration       atomic.Uint64
	stopping                atomic.Bool
	lastResumeRecoveryUnix  int64

	powerNotifyStop      func()
	hidArrivalNotifyStop func()

	guiLastResponse   int64
	guiMonitorEnabled bool
	quitChan          chan bool

	mutex                 sync.RWMutex
	monitorMutex          sync.Mutex
	monitorStop           chan struct{}
	monitorDone           chan struct{}
	monitorStopping       bool
	healthMutex           sync.Mutex
	healthStop            chan struct{}
	healthDone            chan struct{}
	healthStopping        bool
	connectMutex          sync.Mutex
	connectDone           chan struct{}
	connectResult         bool
	deviceControlMutex    sync.Mutex
	reconnectMutex        sync.Mutex
	reconnectCancel       chan struct{}
	reconnectDone         chan struct{}
	reconnectWake         chan struct{}
	manualGearLevelMemory map[string]string
}

const (
	systemResumeDetectionFloor   = 20 * time.Second                                             // 系统恢复检测阈值下限
	systemResumeDetectionCeiling = 45 * time.Second                                             // 系统恢复检测阈值上限
	systemResumeRecoveryCooldown = 5 * time.Second                                              // 系统恢复后自动重连的冷却时间
	suspendCleanupGrace          = 2 * time.Second                                              // 挂起清理宽限时间
	pawnIOInstallerTimeout       = 90 * time.Second                                             // PawnIO 安装程序超时时间
	pawnIOAlreadyExistsExitCode  = 183                                                          // PawnIO 安装程序退出码，表示已存在安装
	pawnIORegistryPath           = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO` // PawnIO 注册表路径
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
	historyPath := temperatureHistoryPath(installDir)
	migrateLegacyTemperatureHistory(historyPath, installDir, customLogger)
	historyRetentionHours := types.NormalizeTemperatureHistoryRetentionHours(configMgr.Get().TemperatureHistoryRetentionHours)
	historyPointsPerHour := int(time.Hour / temperature.DefaultHistorySampleInterval)
	tempHistory := temperature.NewHistoryRecorder(historyPath, historyPointsPerHour*historyRetentionHours, temperature.DefaultHistorySampleInterval, customLogger)
	trayMgr := tray.NewManager(customLogger, iconData)
	autostartMgr := autostart.NewManager(customLogger)
	pluginMgr := plugins.NewManager(customLogger)

	app := &CoreApp{
		ctx:                context.Background(),
		deviceManager:      deviceMgr,
		bridgeManager:      bridgeMgr,
		tempReader:         tempReader,
		tempHistory:        tempHistory,
		laptopFanReader:    laptopfan.NewReader(customLogger),
		currentTemp:        types.TemperatureData{BridgeOk: true},
		configManager:      configMgr,
		trayManager:        trayMgr,
		autostartManager:   autostartMgr,
		pluginManager:      pluginMgr,
		rtssPublisher:      rtss.New(),
		logger:             customLogger,
		isConnected:        false,
		lastDeviceMode:     "",
		userSetAutoControl: false,
		isAutoStartLaunch:  isAutoStart,
		debugMode:          debugMode,
		guiLastResponse:    time.Now().Unix(),
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

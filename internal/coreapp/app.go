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
	extendedSensorsUntil atomic.Int64
	currentTemp          types.TemperatureData
	deviceSettings       *types.DeviceSettings
	// lastSmartLightStatus 用于去重智能温控灯效状态广播：温度每秒都在采样，
	// 但只有真正换了温度区间才值得推送一次事件。
	lastSmartLightStatus types.SmartTempLightStatus
	// deviceModeMu 保护 lastDeviceMode 与 userSetAutoControl。它们在 HID 读协程的回调里
	// 读写，不能用 a.mutex：那把锁可能被持有者拿着去等设备锁，进而卡住读协程。
	deviceModeMu            sync.Mutex
	lastDeviceMode          string
	userSetAutoControl      bool
	isAutoStartLaunch       bool
	debugMode               bool
	legionFnQSupported      atomic.Bool
	legionFnQSupportChecked atomic.Bool
	legionFnQRegistered     atomic.Bool
	reconnectInProgress     atomic.Bool
	// startupConfigApplied 标记"本进程已经把 App 配置整体下发过一次"。
	startupConfigApplied    atomic.Bool
	autoReconnectSuppressed atomic.Bool
	// suppressedBeforeSuspend 记录挂起前的 autoReconnectSuppressed，唤醒时据此恢复。
	suppressedBeforeSuspend atomic.Bool
	resumeRecoveryRunning   atomic.Bool
	systemSuspended         atomic.Bool
	resumeReconnectWanted   atomic.Bool
	suspendGeneration       atomic.Uint64
	// automaticResumeGeneration 让"自动唤醒后等待用户"的观察协程失效：再次挂起或
	// 用户在场的唤醒通知到达时递增。
	automaticResumeGeneration atomic.Uint64
	// activeAutomaticWake 记录正处于"自动唤醒、等待用户"阶段的那一轮 generation。
	// 普通睡眠期间它与 automaticResumeGeneration 不相等，压风扇的逻辑据此不动作。
	activeAutomaticWake atomic.Uint64
	// suspendInputTick 是收到挂起通知时的 GetTickCount，hasSuspendInputTick 表示它有效。
	// 之后若 GetLastInputInfo 晚于它，说明用户回到了电脑前。
	suspendInputTick    atomic.Uint32
	hasSuspendInputTick atomic.Bool
	// quietHoldMutex 串行化自动唤醒期间的"连上设备把风扇压到 0"，HID 到达通知与
	// 等待协程可能同时触发。
	quietHoldMutex    sync.Mutex
	connectGeneration atomic.Uint64
	stopping          atomic.Bool
	// shutdownInProgress 在系统广播"即将关机"后置位，用来拦住控温循环：
	// 关机前已经下发目标 0，若循环随后又把曲线目标写回去，风扇就会一直转。
	shutdownInProgress     atomic.Bool
	lastResumeRecoveryUnix int64

	powerNotifyStop      func()
	hidArrivalNotifyStop func()
	// hidArrivalNotifyActive 表示 HID 接口到达通知已注册成功，健康检查据此放慢重连兜底。
	hidArrivalNotifyActive atomic.Bool
	// healthReconnectStreak 是健康检查连续发现设备未连接并触发重连的次数，
	// lastHealthReconnectUnixNano 是最近一次由健康检查触发重连的时刻。
	healthReconnectStreak       atomic.Int32
	lastHealthReconnectUnixNano atomic.Int64
	// learnedOffsetsUnsaved 表示控温循环已合并进内存配置、但尚未落盘的学习偏移。
	learnedOffsetsUnsaved atomic.Bool

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
	automaticResumeGrace         = 10 * time.Minute                                             // 自动唤醒后无人操作时，等这么久仍醒着才恢复设备控制
	automaticResumePollInterval  = 2 * time.Second                                              // 自动唤醒后检查用户输入的间隔
	quietHoldRetryInterval       = 10 * time.Second                                             // 自动唤醒期间压风扇失败后的重试间隔
	manualConnectWaitTimeout     = 20 * time.Second                                             // 手动连接等待进行中连接尝试的上限
	slowConnectAttemptWarning    = 20 * time.Second                                             // 单次连接尝试超过该时长仍未返回时记录告警
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

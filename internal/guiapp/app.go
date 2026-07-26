package guiapp

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/theme"
	"github.com/TIANLI0/THRM/internal/types"
	"go.uber.org/zap"
)

// App struct - GUI 应用程序结构
type App struct {
	ctx       context.Context
	ipcClient *ipc.Client
	mutex     sync.RWMutex

	ipcHealthMutex sync.Mutex
	ipcTimeouts    uint32
	lastIPCTimeout time.Time

	// shuttingDown 让 IPC 看护协程在退出流程中停手。
	// 否则 QuitAll 让核心退出后，看护协程会把它当成"核心掉线"重新拉起来。
	shuttingDown atomic.Bool

	// 缓存的状态
	isConnected bool
	currentTemp types.TemperatureData

	// 自定义主题管理器（发现/播种/读取安装目录与用户目录下的主题）
	themeManager *theme.Manager
}

// 为了与前端 API 兼容，重新导出类型
type (
	FanCurvePoint             = types.FanCurvePoint
	FanCurveProfile           = types.FanCurveProfile
	FanCurveProfilesPayload   = types.FanCurveProfilesPayload
	FanData                   = types.FanData
	GearCommand               = types.GearCommand
	TemperatureData           = types.TemperatureData
	TemperatureHistoryPoint   = types.TemperatureHistoryPoint
	TemperatureHistoryPayload = types.TemperatureHistoryPayload
	BridgeTemperatureData     = types.BridgeTemperatureData
	DeviceDebugCommandResult  = types.DeviceDebugCommandResult
	DeviceDebugFrame          = types.DeviceDebugFrame
	DeviceDebugCommandPreset  = types.DeviceDebugCommandPreset
	DeviceSettings            = types.DeviceSettings
	DeviceGearRPM             = types.DeviceGearRPM
	DeviceStatusRead          = types.DeviceStatusRead
	AppConfig                 = types.AppConfig
)

var guiLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	guiLogger = logger.Sugar()
}

// Package device 提供 HID 设备通信功能
package device

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/sstallion/go-hid"
)

const (
	// VendorID 设备厂商ID
	VendorID = 0x37D7
	// ProductIDBS2PRO BS2PRO 产品ID
	ProductIDBS2PRO = 0x1002
	// ProductIDBS2 BS2 产品ID
	ProductIDBS2 = 0x1001
	// ProductIDBS3 BS3 产品ID
	ProductIDBS3 = 0x1003
	// ProductIDBS3PRO BS3PRO 产品ID
	ProductIDBS3PRO = 0x1004
)

var supportedHIDProductIDs = []uint16{ProductIDBS2PRO, ProductIDBS3, ProductIDBS3PRO, ProductIDBS2}

// idleBLEScanCooldown 限制“本会话从未连接成功”时自动重连的 BLE 扫描频率。
//
// Windows 上 BLE 扫描经由 WinRT 广播监听，每条收到的广播都会产生原生层分配，
// 在广播密集的环境（办公室）里每 30 秒一次的健康检查扫描会让原生内存无限增长
// （Go 堆不可见）。BS1 用户开机首连与手动连接仍走全量发现，不受此冷却限制。
const idleBLEScanCooldown = 15 * time.Minute

func shouldSkipBLEFallback(preferLastTransport bool, lastDeviceType string, sinceLastBLEScan time.Duration) bool {
	if !preferLastTransport {
		return false
	}
	switch lastDeviceType {
	case types.DeviceTypeHID:
		// 上次是 HID：等待 Windows 重新枚举 HID 接口即可，BLE 扫描只会白白泄漏。
		return true
	case types.DeviceTypeBLE:
		// 本会话确实在用 BS1，设备大概率就在附近，保持每次重连都扫描。
		return false
	default:
		// 本会话从未连接成功：设备很可能根本不在（未携带/未通电），
		// 只保留低频兜底扫描，避免健康检查每 30 秒触发一次原生内存泄漏。
		// sinceLastBLEScan < 0 表示本会话尚未扫描过，放行首次扫描。
		return sinceLastBLEScan >= 0 && sinceLastBLEScan < idleBLEScanCooldown
	}
}

// realtimeWriteErrorDecay 是实时转速写入失败计数的有效期。
//
// 该计数原先只在写入成功时归零，而稳态智能控温下 shouldSendTargetRPM 会把相邻两次
// 写入拉开到几分钟乃至更久（转速不变就不下发）。于是三次相隔数小时的蓝牙瞬时抖动会被
// 当成"连续失败"，触发一次毫无必要的主动断开重连——用户看到的就是"偶尔莫名断连"。
// 超过该间隔的旧失败不再计入连续性判定。
const realtimeWriteErrorDecay = 2 * time.Minute

// realtimeWriteErrorsExpired 判断距上次写入失败是否已超过有效期。
// 零值 lastErrorAt 表示本次连接尚无失败记录。
func realtimeWriteErrorsExpired(lastErrorAt, now time.Time) bool {
	if lastErrorAt.IsZero() {
		return false
	}
	return now.Sub(lastErrorAt) > realtimeWriteErrorDecay
}

const (
	maxConsecutiveReadErrors          = 20
	maxConsecutiveRealtimeWriteErrors = 3
	realtimeWakeKickRPM               = 1700
	realtimeWakeKickDuration          = 450 * time.Millisecond
	realtimeWakeVerifyDelay           = 3 * time.Second
	gearSelectionVerifyTimeout        = 1400 * time.Millisecond
	hidReadPollInterval               = 100 * time.Millisecond
	// hidIdleReadPollInterval 是后台空闲（无 GUI 连接且未开启智能控温）时的轮询间隔。
	// HID 句柄设为非阻塞后，读循环靠"空转即休眠"来避免占满 CPU，这意味着常驻
	// 10 次/秒的唤醒。空闲时没人消费实时转速（托盘 5 秒才刷新一次），
	// 放慢到 2 次/秒可显著降低后台唤醒频率与笔电待机功耗。
	hidIdleReadPollInterval = 500 * time.Millisecond
	// hidBusyReadPollInterval 是多命令事务进行中的空转休眠间隔。灯效上传要连发
	// 35 条命令并逐条等待 ACK；按 100ms/500ms 轮询，一次上传要占用设备锁 3.5s
	// 乃至 17s，期间所有 0xEF 状态帧都会被丢弃，空闲模式下更是逼近 900ms 的
	// 响应超时。事务期间改用毫秒级轮询把整个上传压回几百毫秒。
	hidBusyReadPollInterval = 2 * time.Millisecond
	hidReadErrorRetryDelay  = 500 * time.Millisecond

	// gearRPMSlotCount 是固件挡位转速表的槽位数（0x26 的挡位索引 0..3）。
	gearRPMSlotCount = 4

	// flashWriteSpacing 是两条"会让固件擦写数据闪存"的命令之间强制拉开的最小间隔。
	//
	// 固件的闪存例程（RAM 0x200006ae）开头会把 PFIC->IRER[0]/[1] 全写 1，
	// 连蓝牙链路层自己的 LLE/BB 中断一起关掉，直到一整页 256 字节擦除并回写完成。
	// 连着下发多条落盘命令等于连续制造多个中断黑窗；HID over USB 只是被延迟，
	// 但蓝牙连接下足以错过连续几个连接事件，攒够就是 supervision timeout 掉线，
	// 用户看到的就是"散热器偶发重启"。留出几个连接间隔让链路自己恢复。
	flashWriteSpacing = 150 * time.Millisecond
)

func modelNameForProductID(productID uint16) string {
	switch productID {
	case ProductIDBS2:
		return "BS2"
	case ProductIDBS2PRO:
		return "BS2PRO"
	case ProductIDBS3:
		return "BS3"
	case ProductIDBS3PRO:
		return "BS3PRO"
	default:
		return "Unknown"
	}
}

// Manager 设备管理器
type Manager struct {
	device              *hid.Device
	isConnected         bool
	productID           uint16 // 当前连接的产品ID
	deviceType          string // "hid" 或 "ble"
	lastDeviceType      string // last successful transport, retained across disconnects
	mutex               sync.RWMutex
	logger              types.Logger
	currentFanData      atomic.Pointer[types.FanData]
	connectionGen       atomic.Uint64
	debugCapture        atomic.Bool
	idlePolling         atomic.Bool
	lastCommandedRPM    int
	hasCommandedRPM     bool
	realtimeMode        bool
	realtimeWakeGen     uint64
	lightConfig         types.LightStripConfig
	hasLightConfig      bool
	smartLightPreset    byte
	hasSmartLightPreset bool
	// smartLightBandIndex 是当前生效的温度区间下标，回差判定与界面展示都依赖它。
	smartLightBandIndex int
	// smartLightTemperature 是最近一次参与判定的温度，仅用于展示。
	smartLightTemperature int

	// Firmware commands 0x46 (RGB enable) and 0x48 (gear light) erase and
	// reprogram a 256-byte data-flash page on every call, and unlike 0x0C/0x0D
	// they do not compare against the value already stored. Caching the last
	// acknowledged value lets the host drop the redundant writes that a
	// reconnect replay would otherwise issue, which is the main source of
	// back-to-back flash cycles during a reconnect.
	rgbEnabled    bool
	hasRGBEnabled bool

	// deviceGearRPM 缓存设备侧四个挡位槽当前存的转速。
	//
	// 0x26 同样每次调用都擦写一页数据闪存且不比较新旧值，但它比 0x46/0x48 麻烦：
	// 除了写转速，它还负责换挡（清实时标志 + 写当前挡位）。所以转速没变时不能
	// 整条跳过，只能改用"只换挡、不落盘"的 0x08。缓存由 0x27 读回播种。
	deviceGearRPM    [gearRPMSlotCount]int
	hasDeviceGearRPM [gearRPMSlotCount]bool

	// lastFlashWriteAt 是最近一次下发落盘命令的时间，用于 flashWriteSpacing 限频。
	lastFlashWriteAt time.Time
	// controllerTier 是 0x07 读回的控制器能力档位，决定哪些挡位真的能选中。
	controllerTier    int
	hasControllerTier bool
	gearLightEnabled  bool
	hasGearLight      bool

	consecutiveRealtimeWriteErrors int
	realtimeWriteRecoveryScheduled bool
	lastRealtimeWriteErrorAt       time.Time

	// activeTransactions 统计正在等待 ACK 的多命令事务（灯效上传、挡位写入等）。
	// 大于 0 时读循环改用 hidBusyReadPollInterval，使 ACK 在毫秒级返回，
	// 而不是被空闲轮询间隔拖到接近 deviceResponseTimeout。
	activeTransactions atomic.Int32
	// txWake 用于立刻打断读循环当前的空转休眠，让事务的第一条命令也能快速拿到 ACK。
	txWake chan struct{}

	// HID 监控协程生命周期（监控协程是 HID 句柄的唯一拥有者，负责最终关闭）。
	monitorStop        chan struct{}
	monitorDone        chan struct{}
	explicitDisconnect bool // 是否为显式断开（区别于读错误导致的意外断开）
	disconnectNotify   bool // 显式断开时是否触发断连回调

	// BLE 管理器 (BS1)
	bleManager *BLEManager
	// lastBLEScanAt 记录最近一次自动重连触发 BLE 扫描的时间，用于空闲冷却限频。
	lastBLEScanAt time.Time

	// 回调函数
	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	debugMutex  sync.Mutex
	debugSeq    uint64
	debugFrames []types.DeviceDebugFrame
	queryMutex  sync.Mutex
	responses   *responseBroker
}

// NewManager 创建新的设备管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger:     logger,
		bleManager: NewBLEManager(logger),
		responses:  newResponseBroker(),
		txWake:     make(chan struct{}, 1),
		// -1 表示"尚未判定过任何温度区间"，与合法下标 0 区分开。
		smartLightBandIndex: -1,
	}
}

// SeedLastTransport 用持久化的“上次成功连接方式”初始化自动重连偏好，
// 使 BLE 扫描的跳过/限频策略在进程重启后依然成立。
// 仅在本会话尚未建立过连接时生效，且只接受已知的传输类型。
func (m *Manager) SeedLastTransport(transport string) {
	if transport != types.DeviceTypeHID && transport != types.DeviceTypeBLE {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.lastDeviceType == "" {
		m.lastDeviceType = transport
	}
}

// SetCallbacks 设置回调函数
func (m *Manager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	m.onFanDataUpdate = onFanDataUpdate
	m.onDisconnect = onDisconnect
	m.bleManager.SetCallbacks(onFanDataUpdate, onDisconnect)
}

// SetDebugCapture controls expensive raw HID frame capture. Normal background
// control does not need it; device-setting queries and explicit debug commands
// temporarily enable capture for their own duration.
func (m *Manager) SetDebugCapture(enabled bool) {
	m.debugCapture.Store(enabled)
	m.bleManager.SetDebugCapture(enabled)
}

// Init 初始化 HID 库
func (m *Manager) Init() error {
	return hid.Init()
}

// Exit 清理 HID 库
func (m *Manager) Exit() error {
	return hid.Exit()
}

// Connect performs full discovery and is used for startup and explicit user
// requests where the attached model may have changed.
func (m *Manager) Connect() (bool, map[string]string) {
	return m.connect(false)
}

// Reconnect prefers the last successful transport. A Bluetooth-connected
// BS2PRO is exposed by Windows as HID, so falling through to the BS1 BLE scan
// while that HID interface is still re-enumerating only delays the next retry.
func (m *Manager) Reconnect() (bool, map[string]string) {
	return m.connect(true)
}

func (m *Manager) connect(preferLastTransport bool) (bool, map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isConnected {
		return true, nil
	}

	// 先尝试 HID 连接 (BS2/BS2PRO/BS3/BS3PRO)
	var device *hid.Device
	var err error

	var connectedProductID uint16
	for _, productID := range supportedHIDProductIDs {
		m.logInfo("正在连接设备 - 厂商ID: 0x%04X, 产品ID: 0x%04X", VendorID, productID)

		device, err = hid.OpenFirst(VendorID, productID)
		if err == nil {
			m.logInfo("成功连接到产品ID: 0x%04X", productID)
			connectedProductID = productID
			break
		} else {
			m.logError("产品ID 0x%04X 连接失败: %v", productID, err)
		}
	}

	if err == nil && device != nil {
		// HID 连接成功 (BS2/BS2PRO/BS3/BS3PRO)
		m.device = device
		m.isConnected = true
		m.productID = connectedProductID
		m.deviceType = types.DeviceTypeHID
		m.lastDeviceType = types.DeviceTypeHID
		m.currentFanData.Store(nil)
		m.resetRealtimeControlStateLocked()
		m.connectionGen.Add(1)

		// 为本次连接创建独立的监控生命周期信号。
		m.monitorStop = make(chan struct{})
		m.monitorDone = make(chan struct{})
		m.explicitDisconnect = false
		m.disconnectNotify = false

		modelName := modelNameForProductID(connectedProductID)

		// 获取设备信息
		deviceInfo, infoErr := device.GetDeviceInfo()
		var info map[string]string
		if infoErr == nil {
			m.logInfo("设备连接成功: %s %s (型号: %s)", deviceInfo.MfrStr, deviceInfo.ProductStr, modelName)
			info = map[string]string{
				"manufacturer": deviceInfo.MfrStr,
				"product":      deviceInfo.ProductStr,
				"serial":       deviceInfo.SerialNbr,
				"model":        modelName,
				"productId":    fmt.Sprintf("0x%04X", connectedProductID),
			}
		} else {
			m.logError("设备连接成功,但获取设备信息失败: %v", infoErr)
			info = map[string]string{
				"manufacturer": "Unknown",
				"product":      modelName,
				"serial":       "Unknown",
				"model":        modelName,
				"productId":    fmt.Sprintf("0x%04X", connectedProductID),
			}
		}

		// 开始监控设备数据（显式传入本次连接的句柄与信号，避免与后续重连串扰）
		go m.monitorDeviceData(device, m.monitorStop, m.monitorDone)

		return true, info
	}

	var sinceLastBLEScan time.Duration = -1
	if !m.lastBLEScanAt.IsZero() {
		sinceLastBLEScan = time.Since(m.lastBLEScanAt)
	}
	if shouldSkipBLEFallback(preferLastTransport, m.lastDeviceType, sinceLastBLEScan) {
		if m.lastDeviceType == types.DeviceTypeHID {
			m.logInfo("上次连接设备为 HID，等待 Windows 重新枚举 HID 接口，跳过仅用于 BS1 的 BLE 扫描")
		} else {
			m.logDebug("本会话尚未连接过设备，BLE 扫描处于冷却期（距上次 %v），跳过本轮扫描", sinceLastBLEScan.Round(time.Second))
		}
		return false, nil
	}

	// HID 连接失败，尝试 BLE 连接 (BS1)
	m.logInfo("HID 设备未找到，尝试 BLE 扫描 BS1 设备...")
	m.lastBLEScanAt = time.Now()
	m.mutex.Unlock() // 释放锁，BLE 扫描可能耗时较长
	success, bleInfo := m.bleManager.Connect()
	m.mutex.Lock() // 重新获取锁

	// A second caller may have completed a connection while this caller was
	// scanning. Do not emit another logical connection or advance the generation.
	if m.isConnected {
		return true, nil
	}
	if success {
		m.isConnected = true
		m.deviceType = types.DeviceTypeBLE
		m.lastDeviceType = types.DeviceTypeBLE
		m.productID = 0
		m.currentFanData.Store(nil)
		m.resetRealtimeControlStateLocked()
		m.connectionGen.Add(1)
		m.logInfo("BS1 BLE 设备连接成功")
		return true, bleInfo
	}

	m.logError("所有设备连接尝试都失败（HID 和 BLE）")
	return false, nil
}

// Disconnect 断开设备连接，并触发断连回调。
func (m *Manager) Disconnect() {
	m.disconnect(true, false)
}

// DisconnectSilently 断开设备连接，但不触发断连回调。
func (m *Manager) DisconnectSilently() {
	m.disconnect(false, false)
}

// DisconnectForRecovery 断开设备以便执行恢复重连。
//
// 休眠/唤醒后 HID 句柄可能失效。恢复时不能直接 Close 仍由监控协程拥有的句柄
// （会导致 cgo use-after-free），但也不能继续把它标记为已连接，否则后续 Connect
// 会错误地直接返回成功而不重新打开设备。等待超时后本方法会安全地脱离旧句柄，
// 允许恢复流程建立新连接；旧监控协程返回时仍会自行关闭旧句柄。
func (m *Manager) DisconnectForRecovery() {
	m.disconnect(false, true)
}

func (m *Manager) disconnect(notify, detachOnTimeout bool) {
	m.mutex.Lock()
	if !m.isConnected {
		m.mutex.Unlock()
		return
	}

	if m.deviceType == types.DeviceTypeBLE {
		m.bleManager.Disconnect()
		m.isConnected = false
		m.deviceType = ""
		m.currentFanData.Store(nil)
		m.resetRealtimeControlStateLocked()
		onDisconnect := notify && m.onDisconnect != nil
		m.mutex.Unlock()

		m.logInfo("设备连接已断开")
		if onDisconnect {
			m.onDisconnect()
		}
		return
	}

	// HID：不在此处直接 Close（监控协程可能正阻塞在读操作上，直接 Close 会触发
	// hidapi 的 use-after-free 崩溃）。改为标记显式断开意图并通知监控协程停止，
	// 由监控协程退出读循环后统一关闭句柄并按需触发断连回调（见 finalizeMonitor）。
	m.explicitDisconnect = true
	m.disconnectNotify = notify
	stop := m.monitorStop
	done := m.monitorDone
	dev := m.device
	m.mutex.Unlock()

	if stop != nil && done != nil {
		// 通知监控协程停止读取，并等待其退出后完成关闭与回调。
		select {
		case <-stop:
		default:
			close(stop)
		}

		// 非阻塞监控通常会在一个轮询周期内退出；等待超时后仍不强行关闭由监控协程
		// 拥有的句柄，避免触发崩溃，交由监控协程稍后自行收尾。
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			m.logError("等待设备监控协程退出超时，延后由监控协程自行收尾")
			if detachOnTimeout {
				m.detachStalledHID(dev)
			}
		}
		return
	}

	// 没有监控协程（异常情况）时，安全关闭并清理。
	m.closeDeviceLocked(dev)
	m.mutex.Lock()
	if m.device == dev {
		m.device = nil
		m.isConnected = false
		m.deviceType = ""
		m.productID = 0
		m.currentFanData.Store(nil)
		m.resetRealtimeControlStateLocked()
	}
	onDisconnect := notify && m.onDisconnect != nil
	m.mutex.Unlock()

	m.logInfo("设备连接已断开")
	if onDisconnect {
		m.onDisconnect()
	}
}

// detachStalledHID 从管理器状态中脱离仍卡在读取中的旧 HID 句柄。
// 不在这里 Close：只有旧监控协程已经退出时，finalizeMonitor 才能安全关闭它。
func (m *Manager) detachStalledHID(dev *hid.Device) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if dev == nil || m.device != dev {
		return
	}

	m.device = nil
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.monitorStop = nil
	m.monitorDone = nil
	m.explicitDisconnect = false
	m.disconnectNotify = false
	m.currentFanData.Store(nil)
	m.resetRealtimeControlStateLocked()
	m.logError("HID 监控协程在断开超时后仍未退出，已脱离失效句柄并允许恢复重连")
}

// resetLightStateCacheLocked 丢弃"设备侧灯光开关值"的缓存。断开连接、以及会重建
// 固件 LED 状态的维护命令（0x03/0x05/0x06）之后必须调用，否则缓存会让 App 误以为
// 设备已经处于目标状态而跳过必要的写入。
func (m *Manager) resetLightStateCacheLocked() {
	m.hasRGBEnabled = false
	m.hasGearLight = false
	m.hasLightConfig = false
	m.hasSmartLightPreset = false
	m.smartLightBandIndex = -1
	m.smartLightTemperature = 0
}

// resetGearRPMCacheLocked 丢弃挡位转速缓存。断开连接、以及会重建固件挡位转速表的
// 维护命令之后，缓存都不再可信——此时必须退回用 0x26 下发，宁可多擦一次闪存，
// 也不能用 0x08 换到一个转速其实对不上的挡位。
func (m *Manager) resetGearRPMCacheLocked() {
	m.deviceGearRPM = [gearRPMSlotCount]int{}
	m.hasDeviceGearRPM = [gearRPMSlotCount]bool{}
}

// awaitFlashWriteWindowLocked 在下发落盘命令前，把它与上一条落盘命令拉开
// flashWriteSpacing。调用方必须持有 m.mutex：这里的休眠是有意持锁的，
// 就是要防止别的控制路径在这个间隔里插进另一条落盘命令。
//
// 持锁休眠不会挡住 ACK：读循环是在 responses.deliver 之后才去取锁的，
// 取不到只会丢掉一帧 0xEF 状态通知，不影响命令应答。
func (m *Manager) awaitFlashWriteWindowLocked() {
	if m.lastFlashWriteAt.IsZero() {
		return
	}
	elapsed := time.Since(m.lastFlashWriteAt)
	wait := flashWriteSpacing - elapsed
	if wait <= 0 {
		return
	}
	m.logDebug("距上次固件闪存写入仅 %v，等待 %v 后再下发下一条落盘命令",
		elapsed.Round(time.Millisecond), wait.Round(time.Millisecond))
	time.Sleep(wait)
}

// noteFlashWriteLocked 记录一次已下发的落盘命令。
//
// 0x43 的落盘是固件回完 ACK 之后再由 TMOS 事件 0x20 异步做的，这里统一按
// "命令返回时"记时；flashWriteSpacing 的余量覆盖了那点延迟。
func (m *Manager) noteFlashWriteLocked() {
	m.lastFlashWriteAt = time.Now()
}

func (m *Manager) resetRealtimeControlStateLocked() {
	m.realtimeWakeGen++
	m.lastCommandedRPM = 0
	m.hasCommandedRPM = false
	m.realtimeMode = false
	m.consecutiveRealtimeWriteErrors = 0
	m.realtimeWriteRecoveryScheduled = false
	m.lastRealtimeWriteErrorAt = time.Time{}
}

// closeDeviceLocked 在持有锁的情况下安全关闭 HID 句柄。
func (m *Manager) closeDeviceLocked(dev *hid.Device) {
	if dev == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logError("关闭设备时发生错误: %v", r)
		}
	}()
	dev.Close()
}

// IsConnected 检查设备是否已连接
func (m *Manager) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected
}

// ConnectionGeneration returns a monotonically increasing identifier for each
// successful physical connection. Consumers use it to discard control state
// derived from a previous HID handle after reconnecting.
func (m *Manager) ConnectionGeneration() uint64 {
	return m.connectionGen.Load()
}

// GetProductID 获取当前连接设备的产品ID
func (m *Manager) GetProductID() uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.productID
}

// GetModelName 获取当前连接设备的型号名称
func (m *Manager) GetModelName() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.deviceType == types.DeviceTypeBLE {
		return "BS1"
	}
	return modelNameForProductID(m.productID)
}

// GetDeviceType 获取当前连接设备的类型
func (m *Manager) GetDeviceType() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.deviceType
}

// IsBS1 检查当前连接设备是否为 BS1
func (m *Manager) IsBS1() bool {
	return m.GetDeviceType() == types.DeviceTypeBLE
}

// GetCurrentFanData 获取当前风扇数据
func (m *Manager) GetCurrentFanData() *types.FanData {
	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.GetCurrentFanData()
	}
	return m.currentFanData.Load()
}

// SetIdlePolling 切换 HID 读循环的空闲轮询模式。
// 由核心在"无 GUI 连接且未开启智能控温"时开启，此时没人消费实时转速数据。
func (m *Manager) SetIdlePolling(idle bool) {
	m.idlePolling.Store(idle)
}

// readPollInterval 返回当前的空转休眠间隔。
func (m *Manager) readPollInterval() time.Duration {
	if m.activeTransactions.Load() > 0 {
		return hidBusyReadPollInterval
	}
	if m.idlePolling.Load() {
		return hidIdleReadPollInterval
	}
	return hidReadPollInterval
}

// beginTransaction 标记一段"连发多条命令并逐条等 ACK"的事务开始，并立刻唤醒
// 读循环。返回的函数必须在事务结束时调用。
func (m *Manager) beginTransaction() func() {
	m.activeTransactions.Add(1)
	select {
	case m.txWake <- struct{}{}:
	default:
	}
	return func() { m.activeTransactions.Add(-1) }
}

// waitPollInterval 在读到空数据后休眠一个轮询间隔。返回 false 表示收到停止信号。
// 用 select 而非 time.Sleep：休眠期间也必须能立即响应断开，否则空闲模式下
// 拉长的间隔会让断连/重连流程多等半秒。
func (m *Manager) waitPollInterval(stop <-chan struct{}) bool {
	timer := time.NewTimer(m.readPollInterval())
	defer timer.Stop()
	select {
	case <-stop:
		return false
	case <-m.txWake:
		// 事务刚开始：立即结束本次休眠，后续休眠会自动改用事务间隔。
		return true
	case <-timer.C:
		return true
	}
}

// monitorDeviceData 监控设备数据
//
// 该协程是 HID 句柄的唯一拥有者：无论因停止信号还是读错误退出，都由它负责关闭句柄
// （见 finalizeMonitor），从而避免“读操作进行中被其他协程 Close”导致的 cgo 崩溃
// （典型触发场景：睡眠唤醒后句柄失效，读卡住时执行断开）。
func (m *Manager) monitorDeviceData(device *hid.Device, stop <-chan struct{}, done chan struct{}) {
	// 退出时统一收尾：关闭句柄、清理状态、按需触发断连回调。
	defer m.finalizeMonitor(device, done)

	// 解析或回调中的任何 panic 都不能让整个进程崩溃，这里兜底恢复。
	defer func() {
		if r := recover(); r != nil {
			m.logError("设备数据监控协程发生panic，已恢复: %v", r)
		}
	}()

	if device == nil {
		return
	}

	// SetNonblock 只影响 Read；失败时不能退回 ReadWithTimeout，因为失效的 Windows
	// HID 句柄可能让原生超时读取永久不返回，进而阻塞断联回调与自动重连。
	if err := device.SetNonblock(true); err != nil {
		m.logError("设置非阻塞模式失败: %v", err)
		return
	}

	buffer := make([]byte, 64)
	consecutiveErrors := 0

	for {
		// 优先响应停止信号，确保断开时尽快退出读循环，再由 finalizeMonitor 安全关闭句柄。
		select {
		case <-stop:
			m.logInfo("收到停止信号，停止设备数据监控")
			return
		default:
		}

		// Read 受上面的非阻塞设置控制，不会把协程永久困在失效的原生句柄中。
		// 空闲时短暂休眠，避免无数据轮询占用 CPU，且能快速响应停止/重连请求。
		n, err := device.Read(buffer)
		if err != nil {
			if err == hid.ErrTimeout {
				consecutiveErrors = 0
				if !m.waitPollInterval(stop) {
					return
				}
				continue
			}

			consecutiveErrors++
			if consecutiveErrors == 1 || consecutiveErrors%5 == 0 {
				m.logError("读取设备数据失败 (%d/%d): %v", consecutiveErrors, maxConsecutiveReadErrors, err)
			}

			if consecutiveErrors >= maxConsecutiveReadErrors {
				m.logError("连续读取失败次数过多，设备可能已断开")
				return
			}

			select {
			case <-stop:
				return
			case <-time.After(hidReadErrorRetryDelay):
			}
			continue
		}

		consecutiveErrors = 0
		if n == 0 {
			if !m.waitPollInterval(stop) {
				return
			}
			continue
		}

		raw := buffer[:n]
		m.recordDebugFrame("rx", types.DeviceTypeHID, raw)
		if frame, ok := deviceproto.ParseFrame(raw); ok && frame.ChecksumOK {
			m.responses.deliver(frame)
		}
		fanData := m.parseFanData(buffer, n)
		if fanData != nil {
			// A monitor from a detached pre-suspend handle can unblock after a
			// replacement connection has been established. Do not let that old
			// handle overwrite the fresh connection's status cache.
			//
			// Several firmware setters publish 0xEF before their command ACK. The
			// sender holds m.mutex while waiting for that ACK, so blocking here
			// would prevent the monitor from reading the very next HID report and
			// deadlock the transaction. Dropping this intermediate notification is
			// safe: the ACK remains authoritative and normal periodic status reports
			// refresh the cache immediately afterward.
			if !m.mutex.TryRLock() {
				continue
			}
			active := m.isConnected && m.device == device
			m.mutex.RUnlock()
			if !active {
				return
			}

			if fanData.CurrentMode&0x01 == 0 {
				// A physical gear change or a reconnect places the device back in
				// gear mode. The next software target must send a fresh realtime
				// mode-entry command rather than assuming the old session remains.
				if m.mutex.TryLock() {
					if m.device == device {
						m.realtimeMode = false
						m.hasCommandedRPM = false
					}
					m.mutex.Unlock()
				}
			}

			// 无锁原子写
			m.currentFanData.Store(fanData)

			if m.onFanDataUpdate != nil {
				m.onFanDataUpdate(fanData)
			}
		}
	}
}

// finalizeMonitor 监控协程退出时的收尾：关闭句柄、清理状态并按需触发断连回调。
//
// 关闭句柄只在读循环已退出后进行，因此不会与读操作并发，杜绝 use-after-free 崩溃。
func (m *Manager) finalizeMonitor(device *hid.Device, done chan struct{}) {
	if done != nil {
		defer close(done)
	}

	m.mutex.Lock()
	// 若当前活动句柄已不是本协程的句柄，说明它已在恢复流程中被脱离或被新连接替换。
	// 当前协程已经退出读循环，因此现在关闭它自己的旧句柄是安全的，且不会影响新连接。
	if m.device != device {
		m.closeDeviceLocked(device)
		m.mutex.Unlock()
		m.logDebug("已关闭过期 HID 监控协程持有的旧句柄")
		return
	}

	wasConnected := m.isConnected
	explicit := m.explicitDisconnect
	notifyOnExplicit := m.disconnectNotify

	m.closeDeviceLocked(device)
	m.device = nil
	m.isConnected = false
	m.deviceType = ""
	m.productID = 0
	m.currentFanData.Store(nil)
	m.monitorStop = nil
	m.monitorDone = nil
	m.explicitDisconnect = false
	m.disconnectNotify = false
	m.resetRealtimeControlStateLocked()
	m.resetLightStateCacheLocked()
	m.resetGearRPMCacheLocked()
	m.mutex.Unlock()

	// 触发回调：显式断开按调用方意图，意外断开（读错误）则始终通知。
	shouldNotify := wasConnected
	if explicit {
		shouldNotify = notifyOnExplicit
	}

	m.logInfo("设备连接已断开")
	if shouldNotify && m.onDisconnect != nil {
		m.onDisconnect()
	}
}

// parseFanData 解析风扇数据
func (m *Manager) parseFanData(data []byte, length int) *types.FanData {
	if length <= 0 || length > len(data) {
		return nil
	}
	frame, ok := deviceproto.ParseFrame(data[:length])
	if !ok || !frame.ChecksumOK || frame.Command != deviceproto.CmdStatusNotify || len(frame.Payload) < 7 {
		return nil
	}
	payload := frame.Payload
	fanData := &types.FanData{
		ReportID:     frame.ReportID,
		MagicSync:    0x5AA5,
		Command:      frame.Command,
		FrameLength:  frame.Length,
		GearSettings: payload[0],
		CurrentMode:  payload[1],
		Reserved1:    payload[2],
		CurrentRPM:   binary.LittleEndian.Uint16(payload[3:5]),
		TargetRPM:    binary.LittleEndian.Uint16(payload[5:7]),
	}

	// 解析挡位设置
	maxGear, setGear := m.parseGearSettings(fanData.GearSettings)
	fanData.MaxGear = maxGear
	fanData.SetGear = setGear

	fanData.WorkMode = m.parseWorkMode(fanData.CurrentMode)

	return fanData
}

// parseGearSettings 解析挡位设置
func (m *Manager) parseGearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	switch maxGearCode {
	case 0x2:
		maxGear = "标准"
	case 0x4:
		maxGear = "强劲"
	case 0x6:
		maxGear = "超频"
	default:
		maxGear = fmt.Sprintf("未知(0x%X)", maxGearCode)
	}

	switch setGearCode {
	case 0x8:
		setGear = "静音"
	case 0xA:
		setGear = "标准"
	case 0xC:
		setGear = "强劲"
	case 0xE:
		setGear = "超频"
	default:
		setGear = fmt.Sprintf("未知(0x%X)", setGearCode)
	}

	return
}

// parseWorkMode 解析工作模式
func (m *Manager) parseWorkMode(mode uint8) string {
	switch mode {
	case 0x04, 0x02, 0x06, 0x0A, 0x08, 0x00:
		return "挡位工作模式"
	case 0x05, 0x03, 0x07, 0x0B, 0x09, 0x01:
		return "自动模式(实时转速)"
	default:
		return fmt.Sprintf("未知模式(0x%02X)", mode)
	}
}

// SetFanSpeed 设置风扇转速
func (m *Manager) SetFanSpeed(rpm int) bool {
	if m.IsBS1() {
		if err := m.bleManager.SetFanSpeed(rpm); err != nil {
			m.logError("BS1 设置转速失败: %v", err)
			return false
		}
		return true
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	if rpm < 0 || rpm > 5000 {
		return false
	}

	return m.setRealtimeFanSpeedLocked(rpm, "风扇转速")
}

// SetCustomFanSpeed 设置自定义风扇转速。协议字段是 uint16，App 开放 0..5000 RPM。
func (m *Manager) SetCustomFanSpeed(rpm int) bool {
	if rpm < 0 || rpm > 5000 {
		m.logError("自定义转速超出有效范围: %d RPM", rpm)
		return false
	}
	if m.IsBS1() {
		if err := m.bleManager.SetFanSpeed(rpm); err != nil {
			m.logError("BS1 设置自定义转速失败: %v", err)
			return false
		}
		return true
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	return m.setRealtimeFanSpeedLocked(rpm, "自定义风扇转速")
}

// setRealtimeFanSpeedLocked applies a target RPM without repeatedly re-entering
// realtime mode. BS2PRO treats mode entry as a state transition; issuing it on
// every temperature tick can interrupt the active control session and has been
// observed to make some HID stacks unstable while the app is in the background.
func (m *Manager) setRealtimeFanSpeedLocked(rpm int, name string) bool {
	if m.hasCommandedRPM && m.lastCommandedRPM == rpm && m.realtimeMode {
		return true
	}
	wakingFromZero := m.hasCommandedRPM && m.lastCommandedRPM == 0 && rpm > 0

	if !m.realtimeMode {
		if err := m.sendHIDAckLocked(deviceproto.CmdEnterRealtimeRPM, nil, 1, 3); err != nil {
			m.noteRealtimeWriteResultLocked(false)
			m.logError("进入实时转速模式失败: %v", err)
			return false
		}
		m.realtimeMode = true
	}

	// A stopped rotor can fail to start when the curve asks for a low target such
	// as 1000 RPM. Keep the realtime session alive at zero, then use the factory
	// quiet-gear RPM as a short start pulse before settling at the exact curve
	// target. This preserves true 0 RPM and does not rewrite any gear/RGB setting.
	if wakingFromZero && rpm < realtimeWakeKickRPM {
		if err := m.sendRealtimeTargetLocked(realtimeWakeKickRPM); err != nil {
			m.realtimeMode = false
			m.noteRealtimeWriteResultLocked(false)
			m.logError("发送风扇唤醒脉冲失败: %v", err)
			return false
		}
		time.Sleep(realtimeWakeKickDuration)
	}
	if err := m.sendRealtimeTargetLocked(rpm); err != nil {
		// The target write leaves the hardware state unknown. Force a full mode
		// handshake for the next retry instead of assuming the first command stuck.
		m.realtimeMode = false
		m.noteRealtimeWriteResultLocked(false)
		m.logError("设置%s失败: %v", name, err)
		return false
	}

	m.lastCommandedRPM = rpm
	m.hasCommandedRPM = true
	m.realtimeWakeGen++
	wakeGeneration := m.realtimeWakeGen
	m.noteRealtimeWriteResultLocked(true)
	if rpm == 0 {
		// Do not send 0x24 here. Leaving realtime mode after a zero target was the
		// sequence associated with occasional non-recoverable wake failures.
		m.logDebug("已设置真正的 0 RPM，并保持实时控制会话以便后续唤醒")
	} else {
		m.logDebug("已设置%s: %d RPM", name, rpm)
	}
	if wakingFromZero {
		m.scheduleRealtimeWakeVerificationLocked(rpm, wakeGeneration)
	}
	return true
}

func (m *Manager) sendRealtimeTargetLocked(rpm int) error {
	payload := []byte{byte(rpm), byte(rpm >> 8)}
	return m.sendHIDAckLocked(deviceproto.CmdSetRealtimeRPM, payload, 1)
}

// scheduleRealtimeWakeVerificationLocked checks the actual tachometer after a
// zero-to-positive transition. The fallback deliberately uses only mode/gear
// commands; it never invokes 0x06, so user presets and lighting stay intact.
func (m *Manager) scheduleRealtimeWakeVerificationLocked(targetRPM int, generation uint64) {
	go func() {
		time.Sleep(realtimeWakeVerifyDelay)

		m.mutex.Lock()
		defer m.mutex.Unlock()
		if !m.isConnected || m.device == nil || !m.realtimeMode ||
			m.realtimeWakeGen != generation || m.lastCommandedRPM != targetRPM {
			return
		}
		fanData := m.currentFanData.Load()
		if fanData == nil || fanData.CurrentRPM > 0 {
			return
		}

		m.logWarn("目标 %d RPM 但风扇仍为 0 RPM，执行无损软唤醒", targetRPM)
		if err := m.recoverRealtimeWakeLocked(targetRPM); err != nil {
			m.noteRealtimeWriteResultLocked(false)
			m.logError("风扇软唤醒失败: %v", err)
			return
		}
		m.noteRealtimeWriteResultLocked(true)
		m.logInfo("风扇软唤醒序列已完成，目标 %d RPM", targetRPM)
	}()
}

func (m *Manager) recoverRealtimeWakeLocked(targetRPM int) error {
	// 这段序列要连发四条命令并逐条等 ACK，中间还有两次电机起转延时。整段都持有
	// 设备写锁，按空闲轮询间隔取 ACK 会把锁多占好几秒，控温环期间完全写不进转速。
	defer m.beginTransaction()()

	if err := m.sendHIDAckLocked(deviceproto.CmdExitRealtimeRPM, nil, 1, 2); err != nil {
		return fmt.Errorf("退出实时模式: %w", err)
	}
	m.realtimeMode = false

	// Re-selecting the currently active fixed gear asks the firmware to run its
	// normal motor start path without changing the user's gear RPM table or their
	// preferred gear. We immediately return to realtime mode and exact target.
	wakeGear := fixedGearForWake(m.currentFanData.Load())
	if err := m.sendHIDAckLocked(deviceproto.CmdSetFixedGear, []byte{wakeGear}, 1); err != nil {
		return fmt.Errorf("触发固定挡位启动: %w", err)
	}
	time.Sleep(realtimeWakeKickDuration)
	if err := m.sendHIDAckLocked(deviceproto.CmdEnterRealtimeRPM, nil, 1, 3); err != nil {
		return fmt.Errorf("重新进入实时模式: %w", err)
	}
	m.realtimeMode = true

	kickRPM := max(targetRPM, realtimeWakeKickRPM)
	if err := m.sendRealtimeTargetLocked(kickRPM); err != nil {
		return fmt.Errorf("发送恢复启动脉冲: %w", err)
	}
	if kickRPM != targetRPM {
		time.Sleep(realtimeWakeKickDuration)
		if err := m.sendRealtimeTargetLocked(targetRPM); err != nil {
			return fmt.Errorf("恢复曲线目标: %w", err)
		}
	}
	return nil
}

func fixedGearForWake(data *types.FanData) byte {
	if data == nil {
		return 1
	}
	switch data.GearSettings & 0x0f {
	case 0x08:
		return 1
	case 0x0a:
		return 2
	case 0x0c:
		return 3
	case 0x0e:
		return 4
	default:
		return 1
	}
}

func (m *Manager) noteRealtimeWriteResultLocked(success bool) {
	if success {
		m.consecutiveRealtimeWriteErrors = 0
		m.realtimeWriteRecoveryScheduled = false
		m.lastRealtimeWriteErrorAt = time.Time{}
		return
	}

	// 距上次失败已久说明中间那段时间设备是好的（只是没有需要下发的新转速），
	// 这次失败应当重新起算，而不是接着一个陈旧的计数继续攒。
	now := time.Now()
	if realtimeWriteErrorsExpired(m.lastRealtimeWriteErrorAt, now) {
		m.logDebug("距上次实时转速写入失败已超过 %v，重新起算连续失败计数", realtimeWriteErrorDecay)
		m.consecutiveRealtimeWriteErrors = 0
		m.realtimeWriteRecoveryScheduled = false
	}
	m.lastRealtimeWriteErrorAt = now

	m.consecutiveRealtimeWriteErrors++
	if m.consecutiveRealtimeWriteErrors < maxConsecutiveRealtimeWriteErrors || m.realtimeWriteRecoveryScheduled {
		return
	}
	m.realtimeWriteRecoveryScheduled = true
	m.logError("实时转速连续写入失败 %d 次，主动断开并重连设备", m.consecutiveRealtimeWriteErrors)

	go func() {
		m.mutex.Lock()
		shouldRecover := m.realtimeWriteRecoveryScheduled &&
			m.consecutiveRealtimeWriteErrors >= maxConsecutiveRealtimeWriteErrors
		m.mutex.Unlock()
		if shouldRecover {
			m.Disconnect()
		}
	}()
}

// EnterAutoMode 进入自动模式
func (m *Manager) EnterAutoMode() error {
	if m.IsBS1() {
		return m.bleManager.WriteCommand(types.BS1CmdEnterDynamic)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}

	// 发送进入实时转速模式的命令
	if err := m.sendHIDAckLocked(deviceproto.CmdEnterRealtimeRPM, nil, 1, 3); err != nil {
		return fmt.Errorf("进入自动模式失败: %v", err)
	}
	m.realtimeMode = true

	m.logInfo("已切换到自动模式，开始智能变频")
	return nil
}

// SetManualGear 设置手动挡位
func (m *Manager) SetManualGear(gear, level string) bool {
	if m.IsBS1() {
		// BS1 只有4个固定挡位，无子级别
		if err := m.bleManager.SetManualGear(gear); err != nil {
			m.logError("BS1 设置挡位失败: %v", err)
			return false
		}
		return true
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	commands, exists := types.GearCommands[gear]
	if !exists {
		m.logError("未找到挡位 %s 的命令", gear)
		return false
	}

	var selectedCommand *types.GearCommand
	for i := range commands {
		cmd := &commands[i]
		switch level {
		case "低":
			if strings.Contains(cmd.Name, "低") {
				selectedCommand = cmd
			}
		case "中":
			if strings.Contains(cmd.Name, "中") {
				selectedCommand = cmd
			}
		case "高":
			if strings.Contains(cmd.Name, "高") {
				selectedCommand = cmd
			}
		}
		if selectedCommand != nil {
			break
		}
	}

	if selectedCommand == nil {
		m.logError("未找到挡位 %s %s 的命令", gear, level)
		return false
	}

	frame, ok := deviceproto.ParseFrame(selectedCommand.Command)
	if !ok {
		m.logError("挡位 %s %s 的预设命令无效", gear, level)
		return false
	}
	// 固件的 0x08 分支不校验 payload：任何字节都会被直接写进当前挡位字段。
	// 预设表出错时不能把非法挡位号送进设备。
	if frame.Command == deviceproto.CmdSetFixedGear {
		if len(frame.Payload) != 1 || frame.Payload[0] < 1 || frame.Payload[0] > 4 {
			m.logError("挡位 %s %s 的预设命令携带了非法挡位号: %v", gear, level, frame.Payload)
			return false
		}
		if tier, known := m.controllerTierLocked(); known {
			if maxGear := MaxGearForFixedGearCommand(tier); int(frame.Payload[0]) > maxGear {
				m.logError("控制器能力档位 %d 最高只能选到挡位 %d，设备会忽略挡位 %d 但仍回 ACK", tier, maxGear, frame.Payload[0])
				return false
			}
		}
	}
	if err := m.sendHIDAckLocked(frame.Command, frame.Payload, 1); err != nil {
		m.logError("设置挡位 %s %s 失败: %v", gear, level, err)
		return false
	}

	m.logInfo("设置挡位成功: %s %s (目标转速: %d RPM)", gear, level, selectedCommand.RPM)
	m.resetRealtimeControlStateLocked()
	return true
}

// SetManualGearRPM 按自定义转速设置手动挡位(HID 通过 0x26 下发指定转速; BS1 回退固定挡位)
func (m *Manager) SetManualGearRPM(gear, level string, rpm int) bool {
	if m.IsBS1() {
		if err := m.bleManager.SetManualGear(gear); err != nil {
			m.logError("BS1 设置挡位失败: %v", err)
			return false
		}
		return true
	}

	idx, ok := types.GearIndex(gear)
	if !ok {
		m.logError("未知挡位 %s", gear)
		return false
	}
	// 固件的挡位槽只有四个（0x26 的挡位索引 0..3），挡位转速缓存也按这个长度开。
	// 越界的索引既会写坏缓存数组，也会被固件当成非法参数拒掉。
	if idx < 0 || idx >= gearRPMSlotCount {
		m.logError("挡位 %s 的索引 %d 超出固件挡位槽范围 0..%d", gear, idx, gearRPMSlotCount-1)
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}
	if rpm < types.ManualGearMinRPM || rpm > types.ManualGearMaxRPM {
		m.logError("手动挡位转速超出有效范围: %d RPM", rpm)
		return false
	}
	// 0x26 会照常回 ACK=1 并写入转速，但在能力档位不允许时静默跳过换挡。
	// 提前拦住能给出确切原因，而不是等 0xEF 确认超时后报一句"无法确认已切换"。
	if tier, known := m.controllerTierLocked(); known {
		if maxGear := MaxGearForGearRPMCommand(tier); idx+1 > maxGear {
			m.logError("控制器能力档位 %d 最高只能选到挡位 %d，设备会写入 %s 的转速但不会切换过去", tier, maxGear, gear)
			return false
		}
	}
	defer m.beginTransaction()()

	// 0x26 会擦写一整页数据闪存，而且固件不比较新旧值（0x0C/0x0D 会比较，
	// 0x26/0x46/0x48 都漏了）。设备侧转速已经一致时改用只换挡、不落盘的 0x08：
	// 0x08 放行的挡位集合在每个能力档位上都是 0x26 的超集（档位 1 时 0x08 允许
	// 1..3、0x26 只允许 1..2），上面那道门控已经按 0x26 的规则挡过一次了。
	command := deviceproto.CmdSetGearRPM
	payload := []byte{byte(idx), byte(rpm), byte(rpm >> 8)}
	writesFlash := true
	if m.hasDeviceGearRPM[idx] && m.deviceGearRPM[idx] == rpm {
		command = deviceproto.CmdSetFixedGear
		payload = []byte{byte(idx + 1)}
		writesFlash = false
		m.logDebug("挡位 %s 的设备侧转速已是 %d RPM，改用 0x08 换挡，跳过一次固件闪存写入", gear, rpm)
	}
	if writesFlash {
		m.awaitFlashWriteWindowLocked()
	}

	// 两条命令都会照常回 ACK=1，却可能因为能力档位而静默不换挡。Register for 0xEF
	// before the write because the firmware publishes status before its command ACK.
	statusWaiter := m.responses.register(deviceproto.CmdStatusNotify)
	ack, err := m.sendHIDCommandAndWaitLocked(command, payload, hidControlReportLen, deviceResponseTimeout)
	if writesFlash {
		m.noteFlashWriteLocked()
	}
	if err == nil {
		err = validateACK(ack, 1)
	}
	if err != nil {
		m.responses.cancel(statusWaiter)
		if writesFlash {
			// 写入结果未知，缓存不再可信。
			m.hasDeviceGearRPM[idx] = false
		}
		m.logError("设置挡位 %s %s (%d RPM) 失败: %v", gear, level, rpm, err)
		return false
	}
	if writesFlash {
		m.deviceGearRPM[idx] = rpm
		m.hasDeviceGearRPM[idx] = true
	}
	if actualGear, err := m.waitForSelectedGearLocked(statusWaiter, idx+1, gearSelectionVerifyTimeout); err != nil {
		if actualGear > 0 {
			m.logError("挡位 %s 的 RPM 已写入，但设备实际仍在挡位 %d: %v", gear, actualGear, err)
		} else {
			m.logError("挡位 %s 的 RPM 已写入，但无法确认设备已切换: %v", gear, err)
		}
		return false
	}

	m.logInfo("设置挡位成功: %s %s (自定义转速: %d RPM)", gear, level, rpm)
	m.resetRealtimeControlStateLocked()
	return true
}

func statusSelectedGear(frame deviceproto.Frame) (gear int, manual bool, ok bool) {
	if frame.Command != deviceproto.CmdStatusNotify || !frame.ChecksumOK || len(frame.Payload) < 2 {
		return 0, false, false
	}
	return deviceproto.DecodeSelectedGear(frame.Payload[0]), frame.Payload[1]&0x01 == 0, true
}

func (m *Manager) waitForSelectedGearLocked(first *responseWaiter, expectedGear int, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	waiter := first
	lastGear := 0
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return lastGear, fmt.Errorf("等待挡位 %d 状态确认超时", expectedGear)
		}
		frame, err := waitForResponse(m.responses, waiter, remaining)
		if err != nil {
			return lastGear, err
		}
		gear, manual, ok := statusSelectedGear(frame)
		if ok {
			lastGear = gear
			if manual && gear == expectedGear {
				return gear, nil
			}
		}
		// A periodic frame may have raced just before the command. Keep waiting
		// until the firmware confirms the requested manual gear or the deadline.
		waiter = m.responses.register(deviceproto.CmdStatusNotify)
	}
}

// NoteGearRPMTableFromDevice 用 0x27 读回的挡位转速表播种缓存。
//
// 播种之后，重连重放在设备侧转速本来就一致时可以用 0x08 换挡，
// 省掉一次固件数据闪存擦写——这是重连路径上仅剩的一次无条件擦写。
func (m *Manager) NoteGearRPMTableFromDevice(table []types.DeviceGearRPM) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, item := range table {
		idx := item.Gear - 1
		if idx < 0 || idx >= gearRPMSlotCount || item.RPM <= 0 {
			continue
		}
		m.deviceGearRPM[idx] = item.RPM
		m.hasDeviceGearRPM[idx] = true
	}
}

// SetGearLight 设置挡位灯
func (m *Manager) SetGearLight(enabled bool) bool {
	if m.IsBS1() {
		m.logInfo("BS1 不支持挡位灯设置")
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	// 0x48 每次都会擦写一页数据闪存，且固件不比较新旧值。已知设备侧就是这个值时
	// 直接跳过，避免重连重放白白消耗一次闪存擦写。
	if m.hasGearLight && m.gearLightEnabled == enabled {
		m.logDebug("挡位灯已是 %t，跳过一次固件闪存写入", enabled)
		return true
	}

	payload := byte(0x00)
	if enabled {
		payload = 0x01
	}
	m.awaitFlashWriteWindowLocked()
	err := m.sendHIDAckLocked(deviceproto.CmdGearLight, []byte{payload}, 1)
	m.noteFlashWriteLocked()
	if err != nil {
		m.hasGearLight = false
		m.logError("设置挡位灯失败: %v", err)
		return false
	}

	m.gearLightEnabled = enabled
	m.hasGearLight = true
	return true
}

// SetPowerOnStart 设置通电自启动
func (m *Manager) SetPowerOnStart(enabled bool) bool {
	if m.IsBS1() {
		if err := m.bleManager.SetPowerOnStart(enabled); err != nil {
			m.logError("BS1 设置通电自启动失败: %v", err)
			return false
		}
		return true
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	var payload byte
	if enabled {
		payload = 0x01
	} else {
		payload = 0x02
	}

	// 固件的 0x0C 分支会先比较新旧值，值没变就不落盘；但值真变了仍是一次整页擦写，
	// 所以照样纳入落盘限频。
	m.awaitFlashWriteWindowLocked()
	err := m.sendHIDAckLocked(deviceproto.CmdSetPowerOnStart, []byte{payload}, 1)
	m.noteFlashWriteLocked()
	if err != nil {
		m.logError("设置通电自启动失败: %v", err)
		return false
	}

	return true
}

// SetSmartStartStop 设置智能启停
func (m *Manager) SetSmartStartStop(mode string) bool {
	if m.IsBS1() {
		m.logInfo("BS1 不支持智能启停设置")
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	var payload byte
	switch mode {
	case "off":
		payload = 0x00
	case "immediate":
		payload = 0x01
	case "delayed":
		payload = 0x02
	default:
		return false
	}

	// 同 0x0C：固件会比较，但值变了就是一次整页擦写。
	m.awaitFlashWriteWindowLocked()
	err := m.sendHIDAckLocked(deviceproto.CmdSetSmartStartStop, []byte{payload}, 1)
	m.noteFlashWriteLocked()
	if err != nil {
		m.logError("设置智能启停失败: %v", err)
		return false
	}

	return true
}

// SetBrightness 设置亮度
func (m *Manager) SetBrightness(percentage int) bool {
	if m.IsBS1() {
		m.logInfo("BS1 不支持亮度设置")
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	if percentage < 0 || percentage > 100 {
		return false
	}
	cfg := m.lightConfig
	if !m.hasLightConfig {
		cfg = types.GetDefaultLightStripConfig()
	}
	cfg.Brightness = percentage
	if err := m.setLightStripLocked(cfg); err != nil {
		m.logError("设置亮度失败: %v", err)
		return false
	}
	m.rememberLightConfigLocked(cfg)
	return true
}

// 日志辅助方法
func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}

func (m *Manager) logWarn(format string, v ...any) {
	if m.logger != nil {
		m.logger.Warn(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}

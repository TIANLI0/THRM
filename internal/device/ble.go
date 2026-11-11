package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/types"
	"tinygo.org/x/bluetooth"
)

// BLEManager BS1 蓝牙低功耗设备管理器
type BLEManager struct {
	adapter bleAdapter
	device  bluetooth.Device
	// deviceOpen 表示 device 持有的 WinRT 设备与 GattSession 还没释放。
	//
	// tinygo bluetooth 0.17 起，Windows 上链路断开时库不再替我们 Disconnect
	// （0.16 及以前会，见 #43），释放完全由本类型负责，而且只能释放一次：
	// 二次 Release 是 use-after-free，recover 拦不住。所有释放都经过
	// takeBLEDeviceLocked，由它清掉这个标记。
	deviceOpen bool
	writeChar  bluetooth.DeviceCharacteristic
	notifyChar bluetooth.DeviceCharacteristic

	isConnected    bool
	deviceAddress  string
	mutex          sync.RWMutex
	logger         types.Logger
	currentFanData *types.FanData

	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	stopChan chan struct{}

	debugSeq     uint64
	debugFrames  []types.DeviceDebugFrame
	debugCapture atomic.Bool
	queryMutex   sync.Mutex
	responses    *responseBroker
	scanTimeout  time.Duration

	// realtimeMode 记录固件当前是否处于实时转速模式（0x23）。保活包要据此决定
	// 能不能发 0x23——挡位模式下发它会把设备踢回实时模式，见 heartbeatLoop。
	realtimeMode bool

	// writeFrame 下发一帧原始数据，nil 时走 GATT 写入特征值。
	// 模式切换的正确性没有硬件就无法验证，测试注入替身来观察实际发出的命令序列。
	writeFrame func(cmd []byte) error

	// closeDevice 释放一个 BLE 设备，nil 时调用 bluetooth.Device.Disconnect。
	// 释放次数没有硬件观察不到，测试注入替身来计数。
	closeDevice func(dev bluetooth.Device) error

	// hidTransport 非 nil 时表示当前连接走 USB HID 而不是 BLE。
	//
	// BS1 插 USB 线时优先走 HID：插线即用、不用配对、不受蓝牙适配器影响，
	// 官方 App 在插线时走的也是这条路（其连接方式选择逻辑先判 HID、再退 BLE）。
	// 两种传输共用本类型之上的全部 BS1 功能逻辑——命令集、应答等待、
	// 0xEF 状态解析、调试记录——只有"怎么收发一帧"这一层不同。
	hidTransport *bs1HIDTransport

	// hidUnresponsive 记录最近一次 HID 连接失败是否属于"设备在枚举列表里但不响应"。
	//
	// 这种情况和"设备没插"要分开处理：前者需要用户拔插 USB 线，光靠软件重连
	// 永远不会成功（已实测：新句柄、旧句柄、0x05 握手全部无应答）。CoreApp 据此
	// 给用户发一次系统通知，而不是让他反复点「重新连接」。
	hidUnresponsive bool
}

const defaultBLEScanTimeout = 10 * time.Second

// bs1ModeSwitchDelay 是 0x23 与随后的 0x21 之间的间隔。固件切模式需要一点时间，
// 紧挨着发目标转速会被丢弃。只在真正发生模式切换时付出这次等待。
const bs1ModeSwitchDelay = 50 * time.Millisecond

// bs1KeepAliveCommand 选择本轮保活要发的帧。
//
// 实时模式下交替发 0x23 与 0x45，让固件的实时会话保持续期；挡位模式下只能发
// 0x45——0x23 会把设备切出挡位模式，参见 heartbeatLoop 的说明。
func bs1KeepAliveCommand(realtime bool, index int) []byte {
	if realtime && index%2 == 0 {
		return types.BS1CmdEnterRealtime
	}
	return types.BS1CmdKeepAlive
}

// RoInitialize/CoInitializeEx 表示“本线程此前已初始化，可继续使用”的两个 HRESULT。
const (
	hresultSFalse         = 0x00000001 // S_FALSE
	hresultRPCChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
)

// isAdapterEnabled 判断 Adapter.Enable() 的返回值是否其实代表成功。
//
// tinygo 在 Windows 上的 Enable() 只是一句 ole.RoInitialize(1)，而 go-ole 把任何
// 非零 HRESULT 一律包成 error 返回。RoInitialize 在当前 OS 线程已经初始化过 COM
// 时返回的是 S_FALSE(1)——这是成功语义，格式化出来却成了 errno 1 的“函数不正确”。
// Go 的协程不绑定 OS 线程，于是重连恰好落在哪个线程决定了这里是成功还是“失败”，
// 表现就是休眠唤醒若干次之后 BS1 随机连不上（issue #43）。
// internal/laptopfan 的 coInitialize 早就按同一规则放行了这两个 HRESULT。
func isAdapterEnabled(err error) bool {
	if err == nil {
		return true
	}
	// 用结构化断言取 HRESULT，避免在跨平台文件里引入只在 Windows 构建的 go-ole。
	var coded interface{ Code() uintptr }
	if !errors.As(err, &coded) {
		return false
	}
	switch coded.Code() {
	case hresultSFalse, hresultRPCChangedMode:
		return true
	default:
		return false
	}
}

// tinygo's Scan blocks until StopScan is called. Keeping this small interface
// makes that lifecycle testable without requiring Bluetooth hardware.
type bleAdapter interface {
	Enable() error
	Scan(callback func(*bluetooth.Adapter, bluetooth.ScanResult)) error
	StopScan() error
	Connect(address bluetooth.Address, params bluetooth.ConnectionParams) (bluetooth.Device, error)
}

// NewBLEManager 创建 BLE 设备管理器
func NewBLEManager(logger types.Logger) *BLEManager {
	return &BLEManager{
		adapter:     bluetooth.DefaultAdapter,
		logger:      logger,
		stopChan:    make(chan struct{}),
		scanTimeout: defaultBLEScanTimeout,
		responses:   newResponseBroker(),
	}
}

// SetCallbacks 设置回调函数
func (b *BLEManager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	b.onFanDataUpdate = onFanDataUpdate
	b.onDisconnect = onDisconnect
}

func (b *BLEManager) SetDebugCapture(enabled bool) {
	b.debugCapture.Store(enabled)
}

// Connect 扫描并连接 BS1 BLE 设备
func (b *BLEManager) Connect() (bool, map[string]string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.isConnected {
		return true, nil
	}

	b.logInfo("初始化 BLE 适配器...")
	if err := b.adapter.Enable(); !isAdapterEnabled(err) {
		b.logError("启用 BLE 适配器失败: %v", err)
		return false, nil
	}

	// 扫描 BS1 设备
	b.logInfo("开始扫描 BS1 BLE 设备...")
	matcher := newBS1ScanMatcher(b.deviceAddress, time.Now())
	found := make(chan bool, 1)
	timedOut := atomic.Bool{}
	scanTimeout := b.scanTimeout
	if scanTimeout <= 0 {
		scanTimeout = defaultBLEScanTimeout
	}
	timeout := time.AfterFunc(scanTimeout, func() {
		timedOut.Store(true)
		if err := b.adapter.StopScan(); err != nil {
			b.logError("failed to stop timed-out BLE scan: %v", err)
		}
	})

	err := b.adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
		name := result.LocalName()
		hasService := result.HasServiceUUID(bs1ServiceUUID)
		if !strings.Contains(name, "BS1") && !strings.Contains(name, "Flydigi") && !(name == "" && hasService) {
			return
		}
		match, firstSeen := matcher.offer(result.Address, name, hasService, time.Now())
		if firstSeen {
			displayName := name
			if displayName == "" {
				displayName = "(无名称, 服务 0xFFF0)"
			}
			b.logInfo("发现 BLE 设备: %s (%s)", displayName, result.Address.String())
		}
		if match {
			select {
			case found <- true:
			default:
			}
			if stopErr := b.adapter.StopScan(); stopErr != nil {
				b.logError("找到 BS1 后停止 BLE 扫描失败: %v", stopErr)
			}
		}
	})
	timeout.Stop()
	if err != nil && !timedOut.Load() {
		b.logError("BLE 扫描失败: %v", err)
		return false, nil
	}

	// Scan only returns after StopScan. The callback or timeout has therefore
	// already decided whether a target was found; never wait a second time here.
	select {
	case <-found:
	default:
		// 超时前只见过无名称的 0xFFF0 广播，也按它连接，好过整轮扫描白跑。
		if !matcher.acceptFallback() {
			b.logError("扫描超时，未找到 BS1 设备")
			return false, nil
		}
	}
	targetAddress, targetName, byService := matcher.target()
	if byService {
		b.logInfo("广播未带设备名，按服务 UUID 0xFFF0 匹配到 BS1: %s", targetAddress.String())
	}
	b.logInfo("已找到 BS1 设备: %s (%s)", targetName, targetAddress.String())

	// 连接设备
	b.logInfo("正在连接 BS1 设备...")
	device, err := b.adapter.Connect(targetAddress, bluetooth.ConnectionParams{})
	if err != nil {
		b.logError("连接 BS1 设备失败: %v", err)
		return false, nil
	}
	b.device = device
	b.deviceOpen = true
	b.deviceAddress = targetAddress.String()

	// 发现服务和特征值
	if !b.discoverCharacteristics() {
		if dev, ok := b.takeBLEDeviceLocked(); ok {
			b.releaseBLEDevice(dev)
		}
		return false, nil
	}

	b.isConnected = true
	b.stopChan = make(chan struct{})
	// 新连接建立时固件处于挡位模式，实时模式标记必须跟着复位。
	b.realtimeMode = false

	// 启用通知
	b.enableNotifications()

	// 启动心跳
	go b.heartbeatLoop()

	info := map[string]string{
		"manufacturer": "Flydigi",
		"product":      targetName,
		"serial":       b.deviceAddress,
		"model":        "BS1",
		"productId":    "",
	}

	b.logInfo("BS1 设备连接成功: %s", targetName)
	return true, info
}

// ConnectHID 尝试通过 USB HID 连接 BS1。只试 HID，不做 BLE 扫描。
//
// 与 Connect 分开是有意的：Manager.connect 在 BS2/BS3 系列 HID 尝试失败后、
// BLE 扫描之前调用本方法。必须排在那之前，因为 shouldSkipBLEFallback 在
// "上次连的是 HID 设备"时会直接跳过整个 BLE 分支——先连过 BS2 的用户插上
// BS1 就永远连不上了。
func (b *BLEManager) ConnectHID() (bool, map[string]string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// 每次尝试都重新判定"设备是否已枚举但不响应"。
	//
	// 这个结论只对当前这一刻的 USB 状态有效：设备一旦被拔下改接充电头，
	// 就应该正常回退到 BLE。若不在入口重置，上一次的 true 会一直残留，
	// connect() 就会一直跳过 BLE 扫描，蓝牙再也连不上。
	b.hidUnresponsive = false

	if b.isConnected {
		return true, nil
	}

	path, productName, ok := enumerateBS1HID()
	if !ok {
		return false, nil
	}

	transport, err := newBS1HIDTransport(path)
	if err != nil {
		b.logError("打开 BS1 USB HID 接口失败: %v", err)
		return false, nil
	}

	b.hidTransport = transport
	b.isConnected = true
	b.realtimeMode = false
	b.stopChan = make(chan struct{})

	// 读循环独占句柄；它非正常退出即代表设备掉线，走统一的断连处理。
	go func() {
		if runErr := transport.Run(b.handleHIDFrame); runErr != nil {
			b.logError("BS1 USB HID 读循环结束: %v", runErr)
			b.handleHIDTransportLost(transport)
		}
	}()

	// 句柄能打开不代表设备在响应。系统睡眠唤醒后，设备会留在枚举列表里却对
	// 任何命令都不应答（实测新句柄、旧句柄、0x05 握手全部无应答），只有重新
	// 枚举 USB 才能恢复。不等第一帧就宣告连接成功，Core 会进入「已连接但转速
	// 空白」的假状态，而且每次自动重连都会重演一遍。
	if !transport.WaitFirstFrame(bs1HIDProbeTimeout) {
		// Run 收到 stop 会返回 nil，因此不会触发 handleDisconnect，可以安全复位。
		if !transport.Close(bs1HIDCloseTimeout) {
			b.logError("BS1 HID 读循环未在超时内退出，放弃本次连接")
		}
		b.hidTransport = nil
		b.isConnected = false
		// 不能把 stopChan 置 nil：detachHIDTransport / Disconnect 里都是
		// "select 默认分支 close(b.stopChan)"，close(nil) 会直接 panic。
		// 保持这个已创建的通道开着即可，后续正常路径会关它。
		b.hidUnresponsive = true
		b.logError("BS1 USB HID 设备无响应（%v 内未收到任何数据）: 设备仍在系统里但不再通信，"+
			"请拔下 USB-C 线重新插入（或重启电脑）。软件重连无法恢复。", bs1HIDProbeTimeout)
		return false, nil
	}

	name := productName
	if name == "" {
		name = types.BS1DeviceName
	}
	b.logInfo("BS1 已通过 USB HID 连接: %s", name)

	return true, map[string]string{
		"manufacturer": "Flydigi",
		"product":      name,
		"serial":       "",
		"model":        "BS1",
		"productId":    fmt.Sprintf("0x%04X", BS1HIDProductID),
	}
}

// handleHIDFrame 处理来自 BS1 HID 读循环的一帧原始报文。
//
// 与 enableNotifications 里 BLE 通知回调做的事完全相同：先记调试帧，再把
// 应答派发给等待该命令的调用方，最后尝试按 0xEF 状态帧解析。两个传输因此
// 共用同一套应答与状态逻辑。
func (b *BLEManager) handleHIDFrame(raw []byte) {
	// 传输标签沿用 DeviceTypeBLE：BS1 的 deviceType 就是它，调试面板里
	// 两边保持一致，避免同一台设备在不同传输下显示成两种类型。
	b.recordDebugFrame("rx", types.DeviceTypeBLE, raw)

	if frame, ok := deviceproto.ParseFrame(raw); ok && frame.ChecksumOK {
		b.responses.deliver(frame)
	}

	fanData := b.parseBS1Notification(raw, bs1GearEncodingHIDASCII)
	if fanData == nil {
		return
	}

	b.mutex.Lock()
	b.currentFanData = fanData
	b.mutex.Unlock()

	if b.onFanDataUpdate != nil {
		b.onFanDataUpdate(fanData)
	}
}

// detachHIDTransport 原子地取出并清空 HID 传输，同时复位连接状态。
// 返回 nil 表示当前连接不是 HID（应走原来的 BLE 断开路径）。
func (b *BLEManager) detachHIDTransport() *bs1HIDTransport {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.hidTransport == nil {
		return nil
	}
	_, transport := b.resetConnectionLocked()
	return transport
}

// bs1ServiceFallbackDelay 是只凭服务 UUID 认设备前，先等带名字广播的时间。
const bs1ServiceFallbackDelay = 3 * time.Second

// bs1ScanMatcher 从扫描结果里挑出 BS1（issue #46）。
//
// BS1 只在刚上电、刚断开的短窗口里在广播中带 "Flydigi BS1" 的名字，其余时间只广播
// 服务 UUID 0xFFF0——官方软件就是按这个 UUID 认设备的。只认名字的话，散热器先开、
// 电脑后开时 THRM 一整天都扫不到它。
//
// 但 0xFFF0 是很多廉价 BLE 设备共用的自定义服务，所以 UUID 只作后备：
//   - 名字含 "BS1" 的广播立即采用；
//   - 带了别的名字的广播一律不算，那显然不是 BS1；
//   - 无名称的 0xFFF0 广播先记为候选。地址是本次运行连过的 BS1 就立即采用，
//     否则等满 bs1ServiceFallbackDelay 仍没有带名字的广播，或扫描超时，再用它。
type bs1ScanMatcher struct {
	mu           sync.Mutex
	knownAddress string
	start        time.Time
	seen         map[string]bool

	address   bluetooth.Address
	name      string
	byService bool
	matched   bool

	candidate    bluetooth.Address
	hasCandidate bool
}

func newBS1ScanMatcher(knownAddress string, start time.Time) *bs1ScanMatcher {
	return &bs1ScanMatcher{knownAddress: knownAddress, start: start, seen: map[string]bool{}}
}

// offer 评估一条扫描结果。match 表示已认定目标、可以停止扫描；firstSeen 表示这是
// 本轮第一次见到该地址，供调用方决定是否记日志（广播每秒会来好几次）。
func (m *bs1ScanMatcher) offer(address bluetooth.Address, name string, hasService bool, now time.Time) (match, firstSeen bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := address.String()
	firstSeen = !m.seen[key]
	m.seen[key] = true

	if m.matched {
		return false, firstSeen
	}
	if strings.Contains(name, "BS1") {
		m.address, m.name, m.byService, m.matched = address, name, false, true
		return true, firstSeen
	}
	if name != "" || !hasService {
		return false, firstSeen
	}
	if !m.hasCandidate {
		m.candidate, m.hasCandidate = address, true
	}
	if key == m.knownAddress || now.Sub(m.start) >= bs1ServiceFallbackDelay {
		m.address, m.name, m.byService, m.matched = address, "Flydigi BS1", true, true
		return true, firstSeen
	}
	return false, firstSeen
}

// acceptFallback 在扫描超时后采用最早记下的无名称 0xFFF0 候选。
func (m *bs1ScanMatcher) acceptFallback() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.matched {
		return true
	}
	if !m.hasCandidate {
		return false
	}
	m.address, m.name, m.byService, m.matched = m.candidate, "Flydigi BS1", true, true
	return true
}

func (m *bs1ScanMatcher) target() (bluetooth.Address, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.address, m.name, m.byService
}

// BS1 BLE 服务和特征值 UUID
var (
	bs1ServiceUUID    = bluetooth.NewUUID([16]byte{0x00, 0x00, 0xff, 0xf0, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0x80, 0x5f, 0x9b, 0x34, 0xfb}) // 0000fff0-...
	bs1WriteCharUUID  = bluetooth.NewUUID([16]byte{0x00, 0x00, 0xff, 0xf2, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0x80, 0x5f, 0x9b, 0x34, 0xfb}) // 0000fff2-...
	bs1NotifyCharUUID = bluetooth.NewUUID([16]byte{0x00, 0x00, 0xff, 0xf1, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0x80, 0x5f, 0x9b, 0x34, 0xfb}) // 0000fff1-...
)

// discoverCharacteristics 发现 GATT 服务和特征值
func (b *BLEManager) discoverCharacteristics() bool {
	b.logInfo("正在发现 GATT 服务...")

	// 只发现目标服务 0xFFF0
	services, err := b.device.DiscoverServices([]bluetooth.UUID{bs1ServiceUUID})
	if err != nil {
		b.logError("发现 GATT 服务失败: %v", err)
		// 回退：发现所有服务
		services, err = b.device.DiscoverServices(nil)
		if err != nil {
			b.logError("发现所有 GATT 服务也失败: %v", err)
			return false
		}
	}

	var foundWrite, foundNotify bool

	for _, service := range services {
		svcUUID := service.UUID().String()
		b.logInfo("发现服务: %s", svcUUID)

		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			b.logError("发现特征值失败: %v", err)
			continue
		}

		for _, char := range chars {
			charUUID := char.UUID()
			b.logInfo("  特征值: %s", charUUID.String())

			if charUUID == bs1WriteCharUUID {
				b.writeChar = char
				foundWrite = true
				b.logInfo("  -> 匹配为写入特征值 (FFF2)")
			}

			if charUUID == bs1NotifyCharUUID {
				b.notifyChar = char
				foundNotify = true
				b.logInfo("  -> 匹配为通知特征值 (FFF1)")
			}
		}
	}

	if !foundWrite {
		b.logError("未找到写入特征值 (FFF2)")
		return false
	}

	if !foundNotify {
		b.logInfo("未找到通知特征值 (FFF1)，将无法接收设备状态")
	}

	b.logInfo("GATT 特征值发现完成 (写入: %v, 通知: %v)", foundWrite, foundNotify)
	return true
}

// enableNotifications 启用通知接收
func (b *BLEManager) enableNotifications() {
	err := b.notifyChar.EnableNotifications(func(buf []byte) {
		// 通知回调运行在蓝牙栈线程上，唤醒后可能收到异常数据，需兜底防止进程崩溃。
		defer func() {
			if r := recover(); r != nil {
				b.logError("BLE 通知回调发生panic，已恢复: %v", r)
			}
		}()

		b.recordDebugFrame("rx", types.DeviceTypeBLE, buf)
		if frame, ok := deviceproto.ParseFrame(buf); ok && frame.ChecksumOK {
			b.responses.deliver(frame)
		}
		fanData := b.parseBS1Notification(buf, bs1GearEncodingBitfield)
		if fanData != nil {
			b.mutex.Lock()
			b.currentFanData = fanData
			b.mutex.Unlock()

			if b.onFanDataUpdate != nil {
				b.onFanDataUpdate(fanData)
			}
		}
	})

	if err != nil {
		b.logError("启用 BLE 通知失败: %v", err)
	} else {
		b.logInfo("BLE 通知已启用")
	}
}

// bs1GearEncoding 描述 0xEF 首字节（挡位字节）的编码方式。
//
// 同一个字节在两条传输上语义完全不同，实测数据：
//
//	BLE: 0x6A → 高 4 位 = 最大挡位、低 4 位 = 当前挡位（8/A/C/E = 静音…超频）
//	HID: 0x30/0x32/0x34 → '0'/'2'/'4'，值 = 0x30 + 2×(挡位-1)（真机抓包 1/2/3 挡）
//
// 必须由调用方按传输显式传入，不能靠 b.hidTransport 反推：断开竞态下它可能已被
// 清空，而这时仍可能有帧正在解析。
type bs1GearEncoding uint8

const (
	bs1GearEncodingBitfield bs1GearEncoding = iota // BLE：位域
	bs1GearEncodingHIDASCII                        // USB HID：ASCII 挡位码
)

// parseBS1Notification 解析 BS1 状态通知数据
// BS1 格式: [5A A5] [EF] [0B] [gearSettings] [mode] [reserved] [currentRPM_LE] [targetRPM_LE] ...
// 与 BS2/BS2PRO 类似，但没有 ReportID 前缀
//
// 挡位字节的编码随传输不同，由 enc 指定（见 bs1GearEncoding）。
func (b *BLEManager) parseBS1Notification(data []byte, enc bs1GearEncoding) *types.FanData {
	frame, ok := deviceproto.ParseFrame(data)
	if !ok || !frame.ChecksumOK || frame.Command != deviceproto.CmdStatusNotify || len(frame.Payload) < 7 {
		return nil
	}
	payload := frame.Payload
	fanData := &types.FanData{
		ReportID:     0, // BS1 没有 ReportID
		MagicSync:    0x5AA5,
		Command:      frame.Command,
		FrameLength:  frame.Length,
		GearSettings: payload[0],
		CurrentMode:  payload[1],
		Reserved1:    payload[2],
		CurrentRPM:   binary.LittleEndian.Uint16(payload[3:5]),
		TargetRPM:    binary.LittleEndian.Uint16(payload[5:7]),
	}

	switch enc {
	case bs1GearEncodingHIDASCII:
		// HID 的单字节只带当前挡位，没有最大挡位信息，MaxGear 留空表示未知。
		fanData.SetGear = bs1HIDGearName(fanData.GearSettings)
	default:
		// 解析挡位设置（与 BS2/BS2PRO 相同的编码）
		maxGear, setGear := parseBS1GearSettings(fanData.GearSettings)
		fanData.MaxGear = maxGear
		fanData.SetGear = setGear
	}
	fanData.SelectedGear = decodeBS1SelectedGear(fanData.GearSettings, enc)
	fanData.WorkMode = parseBS1WorkMode(fanData.CurrentMode)

	return fanData
}

// bs1HIDGearName 把 HID 的挡位字节换成挡位名。
//
// 真机实测（2026-11-05）：静音 0x30、标准 0x32、强劲 0x34；超频 0x36 是同一公式
// 0x30+2×(挡位-1) 的延伸，未实测（设超频要 PD 供电，否则设备静默降级回强劲）。
// 无法识别的值返回空串——界面显示 "--"，而不是过去那种"未知(0x0)"。
func bs1HIDGearName(gearByte uint8) string {
	switch gearByte {
	case 0x30:
		return "静音"
	case 0x32:
		return "标准"
	case 0x34:
		return "强劲"
	case 0x36:
		return "超频"
	default:
		return ""
	}
}

// decodeBS1SelectedGear 把挡位字节换算成 1..4，无法识别返回 0。
func decodeBS1SelectedGear(gearByte uint8, enc bs1GearEncoding) int {
	if enc == bs1GearEncodingHIDASCII {
		if gearByte < 0x30 || gearByte > 0x36 || gearByte%2 != 0 {
			return 0
		}
		return int((gearByte-0x30)/2) + 1
	}
	switch gearByte & 0x0F {
	case 0x8:
		return 1
	case 0xA:
		return 2
	case 0xC:
		return 3
	case 0xE:
		return 4
	default:
		return 0
	}
}

// parseBS1GearSettings 解析 BS1 挡位设置
func parseBS1GearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	maxGearMap := map[uint8]string{
		0x2: "标准",
		0x4: "强劲",
		0x6: "超频",
	}

	setGearMap := map[uint8]string{
		0x8: "静音",
		0xA: "标准",
		0xC: "强劲",
		0xE: "超频",
	}

	if val, ok := maxGearMap[maxGearCode]; ok {
		maxGear = val
	} else {
		maxGear = fmt.Sprintf("未知(0x%X)", maxGearCode)
	}

	if val, ok := setGearMap[setGearCode]; ok {
		setGear = val
	} else {
		setGear = fmt.Sprintf("未知(0x%X)", setGearCode)
	}

	return
}

// parseBS1WorkMode 解析 BS1 工作模式
func parseBS1WorkMode(mode uint8) string {
	switch mode {
	case 0x02, 0x04, 0x06, 0x08, 0x0A, 0x00:
		return "挡位工作模式"
	case 0x01, 0x03, 0x05, 0x07, 0x09, 0x0B:
		return "自动模式(实时转速)"
	default:
		return fmt.Sprintf("未知模式(0x%02X)", mode)
	}
}

// heartbeatLoop 定时发送保活包维持 BLE 连接。
//
// 0x23 不是无副作用的保活包，它的语义是“进入实时转速模式”。挡位模式下发出去会
// 把固件踢回实时模式，而此时实时目标还是 0，风扇随即停转——用户看到的就是刚切好
// 的挡位几秒后掉到 0 转（issue #44）。因此 0x23 只在确实处于实时模式时用来续期，
// 挡位模式一律只发 0x45（纯状态查询，对风扇模式没有副作用）。
func (b *BLEManager) heartbeatLoop() {
	// 系统休眠/唤醒后蓝牙栈状态可能异常，写入操作的 panic 不应导致进程崩溃。
	defer func() {
		if r := recover(); r != nil {
			b.logError("BLE 心跳协程发生panic，已恢复: %v", r)
		}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	heartbeatIndex := 0
	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			b.mutex.RLock()
			connected := b.isConnected
			realtime := b.realtimeMode
			b.mutex.RUnlock()

			if !connected {
				return
			}

			cmd := bs1KeepAliveCommand(realtime, heartbeatIndex)
			heartbeatIndex++

			if err := b.WriteCommand(cmd); err != nil {
				b.logError("发送保活包失败: %v", err)
				b.handleDisconnect()
				return
			}
		}
	}
}

// WriteCommand 下发一帧命令。
//
// BLE 与 USB HID 共用这一个出口：具体走哪条传输由 hidTransport 是否为空决定。
// 上层的全部 BS1 命令都经过这里，所以换传输不需要改任何命令逻辑。
func (b *BLEManager) WriteCommand(cmd []byte) error {
	b.mutex.RLock()
	connected := b.isConnected
	write := b.writeFrame
	hidT := b.hidTransport
	b.mutex.RUnlock()

	if !connected {
		return fmt.Errorf("BS1 设备未连接")
	}

	switch {
	case write != nil:
		// 测试注入的替身优先，便于观察实际发出的命令序列
	case hidT != nil:
		write = hidT.Write
	default:
		write = b.writeGATT
	}

	if err := write(cmd); err != nil {
		return err
	}
	b.recordDebugFrame("tx", types.DeviceTypeBLE, cmd)
	return nil
}

// writeGATT 通过写入特征值下发一帧。
func (b *BLEManager) writeGATT(cmd []byte) error {
	// 优先使用 WriteWithoutResponse（抓包显示 BS1 使用 Write Command 0x52）
	_, err := b.writeChar.WriteWithoutResponse(cmd)
	if err != nil {
		// 回退到 Write with Response
		_, err2 := b.writeChar.Write(cmd)
		if err2 != nil {
			return fmt.Errorf("BLE 写入失败: WriteWithoutResponse=%v, Write=%v", err, err2)
		}
	}
	return nil
}

// Disconnect 断开 BS1 连接（BLE 或 USB HID）
func (b *BLEManager) Disconnect() {
	// HID 传输的断开路径与 BLE 不同：句柄由读循环拥有，只能请求停止并等它
	// 自己关闭；在这里直接 Close 会触发 hidapi 的 use-after-free。因此先单独
	// 处理，不进入下面的 BLE 逻辑。
	if transport := b.detachHIDTransport(); transport != nil {
		if !transport.Close(bs1HIDCloseTimeout) {
			b.logError("BS1 HID 读循环未在超时内退出，句柄将由读循环自行关闭")
		}
		b.logInfo("BS1 USB HID 连接已断开")
		return
	}

	b.mutex.Lock()
	if !b.isConnected {
		b.mutex.Unlock()
		return
	}

	b.isConnected = false
	b.realtimeMode = false

	// 防止与 handleDisconnect 竞争导致 stopChan 被重复关闭。
	select {
	case <-b.stopChan:
	default:
		close(b.stopChan)
	}

	dev, ok := b.takeBLEDeviceLocked()
	b.mutex.Unlock()

	if ok {
		b.releaseBLEDevice(dev)
	}
	b.logInfo("BS1 BLE 连接已断开")
}

// handleDisconnect 处理意外断开
func (b *BLEManager) handleDisconnect() {
	b.mutex.Lock()
	wasConnected, transport := b.resetConnectionLocked()
	dev, devOpen := b.takeBLEDeviceLocked()
	b.mutex.Unlock()

	// 链路已断但设备对象和 GattSession 仍开着（MaintainConnection 还是 true），
	// 必须在这里释放，否则旧会话一直占着设备，下次 Connect 又新开一个。
	if devOpen {
		b.releaseBLEDevice(dev)
	}

	if transport != nil {
		// 目前只有 BLE 心跳会走到这里，HID 传输不会出现；万一出现，读循环还在跑，
		// 只能请求它停下，句柄仍由它自己关闭，不能在这里 Close。
		transport.requestStop()
	}
	b.reportDisconnect(wasConnected, transport != nil)
}

// handleHIDTransportLost 处理 HID 读循环的异常退出。
//
// 只有退出的仍是当前传输时才走断开流程：Close 超时后旧读循环可能晚些才退出，
// 那时也许已经重连上了新的传输，不能拿旧循环的退出去拆新连接。
func (b *BLEManager) handleHIDTransportLost(transport *bs1HIDTransport) {
	b.mutex.Lock()
	if b.hidTransport != transport {
		b.mutex.Unlock()
		return
	}
	wasConnected, _ := b.resetConnectionLocked()
	b.mutex.Unlock()

	b.reportDisconnect(wasConnected, true)
}

// resetConnectionLocked 复位连接状态，返回复位前是否已连接以及被摘下的 HID 传输
// （BLE 连接时为 nil）。调用方必须持有 b.mutex 写锁。
//
// stopChan 在锁内关闭：放到锁外的话，期间建立的新连接会换上新的 stopChan，
// 随后被这里误关，新连接的心跳协程跟着退出。
func (b *BLEManager) resetConnectionLocked() (wasConnected bool, transport *bs1HIDTransport) {
	wasConnected = b.isConnected
	transport = b.hidTransport
	b.isConnected = false
	b.realtimeMode = false
	b.hidTransport = nil

	select {
	case <-b.stopChan:
	default:
		close(b.stopChan)
	}
	return wasConnected, transport
}

// takeBLEDeviceLocked 摘下尚未释放的 BLE 设备，交给调用方在锁外释放。
// 同一个设备只会被摘下一次，第二次调用返回 false。调用方必须持有 b.mutex 写锁。
func (b *BLEManager) takeBLEDeviceLocked() (bluetooth.Device, bool) {
	if !b.deviceOpen {
		return bluetooth.Device{}, false
	}
	b.deviceOpen = false
	dev := b.device
	b.device = bluetooth.Device{}
	return dev, true
}

// releaseBLEDevice 关闭并释放 BLE 设备对象与 GattSession。
// 唤醒后蓝牙句柄可能已失效，底层断开调用的异常不应导致进程崩溃。
func (b *BLEManager) releaseBLEDevice(dev bluetooth.Device) {
	defer func() {
		if r := recover(); r != nil {
			b.logError("断开 BLE 设备时发生错误，已恢复: %v", r)
		}
	}()
	closeDevice := b.closeDevice
	if closeDevice == nil {
		closeDevice = bluetooth.Device.Disconnect
	}
	if err := closeDevice(dev); err != nil {
		b.logError("关闭 BLE 会话失败: %v", err)
	}
}

// reportDisconnect 在锁外记录断开并通知上层。
func (b *BLEManager) reportDisconnect(wasConnected, wasHID bool) {
	if !wasConnected {
		return
	}
	if wasHID {
		b.logInfo("BS1 USB HID 连接已断开")
	} else {
		b.logInfo("BS1 BLE 连接已断开")
	}
	if b.onDisconnect != nil {
		b.onDisconnect()
	}
}

// IsConnected 检查 BLE 是否已连接
func (b *BLEManager) IsConnected() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.isConnected
}

// HIDUnresponsive 报告最近一次 HID 连接失败是否因为"设备在枚举列表里但不响应"。
//
// 这种状态下软件重连永远不会成功（实测新句柄、旧句柄、0x05 握手全部无应答），
// 只能靠用户拔插 USB 线或重启系统，所以要和普通的"设备不在"区分开。
func (b *BLEManager) HIDUnresponsive() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.hidUnresponsive
}

// GetCurrentFanData 获取当前风扇数据
func (b *BLEManager) GetCurrentFanData() *types.FanData {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.currentFanData
}

// SetFanSpeed 设置 BS1 实时目标转速。
//
// 前置命令是 0x23（进入实时转速模式）。此前这里发的是 0x46 0x01——那是 RGB 使能，
// 与转速无关；真正的 0x23 反倒被当成保活包周期性发送，两者位置刚好写反了。
func (b *BLEManager) SetFanSpeed(rpm int) error {
	// BS1 走的是与 HID 相同的 uint16 协议字段，App 只开放 0..5000 RPM。
	// Manager.SetFanSpeed 的范围校验写在 BS1 分支之后，BS1 得自己拦一道。
	if rpm < 0 || rpm > types.RealtimeRPMMax {
		return fmt.Errorf("转速超出有效范围: %d RPM", rpm)
	}

	b.mutex.RLock()
	inRealtime := b.realtimeMode
	b.mutex.RUnlock()

	if !inRealtime {
		if err := b.WriteCommand(types.BS1CmdEnterRealtime); err != nil {
			return fmt.Errorf("进入实时转速模式失败: %v", err)
		}
		time.Sleep(bs1ModeSwitchDelay)
		b.mutex.Lock()
		b.realtimeMode = true
		b.mutex.Unlock()
	}

	if err := b.WriteCommand(types.BuildBS1RPMCommand(rpm)); err != nil {
		// 目标写入失败后固件模式未知，下次重新走一遍模式握手而不是假定 0x23 已生效。
		b.mutex.Lock()
		b.realtimeMode = false
		b.mutex.Unlock()
		return fmt.Errorf("设置转速失败: %v", err)
	}

	// 自动调速每隔几秒就写一次，Info 级会把唤醒/重连这些真正有用的事件淹掉。
	b.logDebug("BS1 已设置转速: %d RPM", rpm)
	return nil
}

// EnterRealtimeMode 让 BS1 进入实时转速控制模式。
func (b *BLEManager) EnterRealtimeMode() error {
	if err := b.WriteCommand(types.BS1CmdEnterRealtime); err != nil {
		return err
	}
	b.mutex.Lock()
	b.realtimeMode = true
	b.mutex.Unlock()
	return nil
}

// SetManualGear 设置 BS1 手动挡位（无子级别）
func (b *BLEManager) SetManualGear(gear string) error {
	cmd, ok := types.BS1GearCommands[gear]
	if !ok {
		return fmt.Errorf("未知挡位: %s", gear)
	}

	if err := b.WriteCommand(cmd.Command); err != nil {
		return fmt.Errorf("设置挡位 %s 失败: %v", gear, err)
	}

	// 固件已回到挡位模式，保活包必须随之停发 0x23，否则几秒后就被踢回实时模式。
	b.mutex.Lock()
	b.realtimeMode = false
	b.mutex.Unlock()

	b.logInfo("BS1 设置挡位成功: %s (目标转速: %d RPM)", gear, cmd.RPM)
	return nil
}

// SetPowerOnStart 设置 BS1 通电自启动
func (b *BLEManager) SetPowerOnStart(enabled bool) error {
	var cmd []byte
	if enabled {
		cmd = types.BS1CmdPowerOnStartEnable
	} else {
		cmd = types.BS1CmdPowerOnStartDisable
	}

	if err := b.WriteCommand(cmd); err != nil {
		return fmt.Errorf("设置通电自启动失败: %v", err)
	}

	b.logInfo("BS1 通电自启动: %v", enabled)
	return nil
}

// 日志辅助方法
func (b *BLEManager) logInfo(format string, v ...any) {
	if b.logger != nil {
		logger.Wrapped(b.logger).Info(format, v...)
	}
}

func (b *BLEManager) logError(format string, v ...any) {
	if b.logger != nil {
		logger.Wrapped(b.logger).Error(format, v...)
	}
}

func (b *BLEManager) logDebug(format string, v ...any) {
	if b.logger != nil {
		logger.Wrapped(b.logger).Debug(format, v...)
	}
}

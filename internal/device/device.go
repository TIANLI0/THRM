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

const (
	maxConsecutiveReadErrors          = 20
	maxConsecutiveRealtimeWriteErrors = 3
	hidReadTimeout                    = time.Second
	hidReadErrorRetryDelay            = 500 * time.Millisecond
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
	device           *hid.Device
	isConnected      bool
	productID        uint16 // 当前连接的产品ID
	deviceType       string // "hid" 或 "ble"
	mutex            sync.RWMutex
	logger           types.Logger
	currentFanData   atomic.Pointer[types.FanData]
	connectionGen    atomic.Uint64
	debugCapture     atomic.Bool
	lastCommandedRPM int
	hasCommandedRPM  bool
	realtimeMode     bool

	consecutiveRealtimeWriteErrors int
	realtimeWriteRecoveryScheduled bool

	// HID 监控协程生命周期（监控协程是 HID 句柄的唯一拥有者，负责最终关闭）。
	monitorStop        chan struct{}
	monitorDone        chan struct{}
	explicitDisconnect bool // 是否为显式断开（区别于读错误导致的意外断开）
	disconnectNotify   bool // 显式断开时是否触发断连回调

	// BLE 管理器 (BS1)
	bleManager *BLEManager

	// 回调函数
	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()

	// lightCmdBuf 是发送灯效命令时复用的 65 字节缓冲。
	// Why: 一次设灯效要发 30+ 帧，旧实现每帧 append + make，~35 次堆分配。
	// 该缓冲只在持有 m.mutex 的灯效命令路径上使用，是线程安全的。
	lightCmdBuf [65]byte

	debugMutex  sync.Mutex
	debugSeq    uint64
	debugFrames []types.DeviceDebugFrame
	queryMutex  sync.Mutex
}

// NewManager 创建新的设备管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger:     logger,
		bleManager: NewBLEManager(logger),
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

// Connect 连接设备（先尝试 HID BS2/BS2PRO/BS3/BS3PRO，再尝试 BLE BS1）
func (m *Manager) Connect() (bool, map[string]string) {
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

	// HID 连接失败，尝试 BLE 连接 (BS1)
	m.logInfo("HID 设备未找到，尝试 BLE 扫描 BS1 设备...")
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
// 休眠/唤醒后，hidapi 的 ReadWithTimeout 在极少数机器上可能永远不返回。此时不能
// 直接 Close 仍在读取的句柄（会导致 cgo use-after-free），但也不能继续把它标记为
// 已连接，否则后续 Connect 会错误地直接返回成功而不重新打开设备。超时后本方法会
// 安全地脱离旧句柄，允许恢复流程建立新连接；旧监控协程返回时仍会自行关闭旧句柄。
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

		// ReadWithTimeout 最多等待 1 秒，正常会很快退出；超时则不强行关闭仍可能在读的
		// 句柄，避免触发崩溃，交由监控协程稍后自行收尾。
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

func (m *Manager) resetRealtimeControlStateLocked() {
	m.lastCommandedRPM = 0
	m.hasCommandedRPM = false
	m.realtimeMode = false
	m.consecutiveRealtimeWriteErrors = 0
	m.realtimeWriteRecoveryScheduled = false
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

	// 设置非阻塞模式
	if err := device.SetNonblock(true); err != nil {
		m.logError("设置非阻塞模式失败: %v", err)
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

		// 空闲时设备不会持续上报。使用 1 秒超时可将 HID 空读唤醒频率减半，
		// 同时仍能在约 1 秒内响应停止/重连请求。
		n, err := device.ReadWithTimeout(buffer, hidReadTimeout)
		if err != nil {
			if err == hid.ErrTimeout {
				consecutiveErrors = 0
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

		if n > 0 {
			m.recordDebugFrame("rx", types.DeviceTypeHID, buffer[:n])
			fanData := m.parseFanData(buffer, n)
			if fanData != nil {
				// A monitor from a detached pre-suspend handle can unblock after a
				// replacement connection has been established. Do not let that old
				// handle overwrite the fresh connection's status cache.
				m.mutex.RLock()
				active := m.isConnected && m.device == device
				m.mutex.RUnlock()
				if !active {
					return
				}

				if fanData.CurrentMode&0x01 == 0 {
					// A physical gear change or a reconnect places the device back in
					// gear mode. The next software target must send a fresh realtime
					// mode-entry command rather than assuming the old session remains.
					m.mutex.Lock()
					if m.device == device {
						m.realtimeMode = false
						m.hasCommandedRPM = false
					}
					m.mutex.Unlock()
				}

				// 无锁原子写
				m.currentFanData.Store(fanData)

				if m.onFanDataUpdate != nil {
					m.onFanDataUpdate(fanData)
				}
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
	if length < 11 {
		return nil
	}

	// 检查同步头
	magic := binary.BigEndian.Uint16(data[1:3])
	if magic != 0x5AA5 {
		return nil
	}

	if data[3] != 0xEF {
		return nil
	}

	fanData := &types.FanData{
		ReportID:     data[0],
		MagicSync:    magic,
		Command:      data[3],
		Status:       data[4],
		GearSettings: data[5],
		CurrentMode:  data[6],
		Reserved1:    data[7],
	}

	// 解析转速 (小端序)
	if length >= 10 {
		fanData.CurrentRPM = binary.LittleEndian.Uint16(data[8:10])
	}
	if length >= 12 {
		fanData.TargetRPM = binary.LittleEndian.Uint16(data[10:12])
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

	if rpm < 0 || rpm > 4000 {
		return false
	}

	return m.setRealtimeFanSpeedLocked(rpm, "风扇转速")
}

// SetCustomFanSpeed 设置自定义风扇转速（无限制）
func (m *Manager) SetCustomFanSpeed(rpm int) bool {
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

	m.logWarn("警告：设置自定义转速 %d RPM（无上下限限制）", rpm)

	return m.setRealtimeFanSpeedLocked(rpm, "自定义风扇转速")
}

// setRealtimeFanSpeedLocked applies a target RPM without repeatedly re-entering
// realtime mode. BS2PRO treats mode entry as a state transition; issuing it on
// every temperature tick can interrupt the active control session and has been
// observed to make some HID stacks unstable while the app is in the background.
func (m *Manager) setRealtimeFanSpeedLocked(rpm int, name string) bool {
	if rpm == 0 && m.hasCommandedRPM && m.lastCommandedRPM == 0 && !m.realtimeMode {
		return true
	}

	if !m.realtimeMode {
		if err := m.writeHIDFrameLocked(deviceproto.CmdEnterRealtimeRPM, nil, hidControlReportLen); err != nil {
			m.noteRealtimeWriteResultLocked(false)
			m.logError("进入实时转速模式失败: %v", err)
			return false
		}
		time.Sleep(50 * time.Millisecond)
		m.realtimeMode = true
	}

	speedBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(speedBytes, uint16(rpm))
	if err := m.writeHIDFrameLocked(deviceproto.CmdSetRealtimeRPM, speedBytes, hidControlReportLen); err != nil {
		// The target write leaves the hardware state unknown. Force a full mode
		// handshake for the next retry instead of assuming the first command stuck.
		m.realtimeMode = false
		m.noteRealtimeWriteResultLocked(false)
		m.logError("设置%s失败: %v", name, err)
		return false
	}

	if rpm == 0 {
		time.Sleep(50 * time.Millisecond)
		if err := m.writeHIDFrameLocked(deviceproto.CmdExitRealtimeRPM, nil, hidControlReportLen); err != nil {
			m.realtimeMode = false
			m.noteRealtimeWriteResultLocked(false)
			m.logError("退出实时转速模式失败: %v", err)
			return false
		}
		m.realtimeMode = false
	}

	m.lastCommandedRPM = rpm
	m.hasCommandedRPM = true
	m.noteRealtimeWriteResultLocked(true)
	if rpm == 0 {
		m.logDebug("已关闭风扇（RPM=0 + 退出实时模式）")
	} else {
		m.logDebug("已设置%s: %d RPM", name, rpm)
	}
	return true
}

func (m *Manager) noteRealtimeWriteResultLocked(success bool) {
	if success {
		m.consecutiveRealtimeWriteErrors = 0
		m.realtimeWriteRecoveryScheduled = false
		return
	}

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
	if err := m.writeHIDFrameLocked(deviceproto.CmdEnterRealtimeRPM, nil, hidControlReportLen); err != nil {
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

	// 发送命令，确保第一个字节是ReportID
	cmdWithReportID := append([]byte{0x02}, selectedCommand.Command...)

	m.recordDebugFrame("tx", types.DeviceTypeHID, cmdWithReportID)
	_, err := m.device.Write(cmdWithReportID)
	if err != nil {
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

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	cmd := types.BuildGearRPMCommand(idx, rpm)
	cmdWithReportID := append([]byte{0x02}, cmd...)

	m.recordDebugFrame("tx", types.DeviceTypeHID, cmdWithReportID)
	if _, err := m.device.Write(cmdWithReportID); err != nil {
		m.logError("设置挡位 %s %s (%d RPM) 失败: %v", gear, level, rpm, err)
		return false
	}

	m.logInfo("设置挡位成功: %s %s (自定义转速: %d RPM)", gear, level, rpm)
	m.resetRealtimeControlStateLocked()
	return true
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

	payload := byte(0x00)
	if enabled {
		payload = 0x01
	}
	if err := m.writeHIDFrameLocked(deviceproto.CmdGearLight, []byte{payload}, hidControlReportLen); err != nil {
		m.logError("设置挡位灯失败: %v", err)
		return false
	}

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

	if err := m.writeHIDFrameLocked(deviceproto.CmdSetPowerOnStart, []byte{payload}, hidControlReportLen); err != nil {
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

	if err := m.writeHIDFrameLocked(deviceproto.CmdSetSmartStartStop, []byte{payload}, hidControlReportLen); err != nil {
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

	switch percentage {
	case 0:
		payload := []byte{0x1C, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := m.writeHIDFrameLocked(0x47, payload, hidControlReportLen); err != nil {
			m.logError("设置亮度失败: %v", err)
			return false
		}
	case 100:
		if err := m.writeHIDFrameLocked(0x43, nil, hidControlReportLen); err != nil {
			m.logError("设置亮度失败: %v", err)
			return false
		}
	default:
		return false
	}

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

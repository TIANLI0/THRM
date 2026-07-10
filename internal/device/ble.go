package device

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"tinygo.org/x/bluetooth"
)

// BLEManager BS1 蓝牙低功耗设备管理器
type BLEManager struct {
	adapter    *bluetooth.Adapter
	device     bluetooth.Device
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
}

// NewBLEManager 创建 BLE 设备管理器
func NewBLEManager(logger types.Logger) *BLEManager {
	return &BLEManager{
		adapter:  bluetooth.DefaultAdapter,
		logger:   logger,
		stopChan: make(chan struct{}),
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
	if err := b.adapter.Enable(); err != nil {
		b.logError("启用 BLE 适配器失败: %v", err)
		return false, nil
	}

	// 扫描 BS1 设备
	b.logInfo("开始扫描 BS1 BLE 设备...")
	var targetAddress bluetooth.Address
	var targetName string
	found := make(chan bool, 1)

	err := b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		name := result.LocalName()
		if strings.Contains(name, "BS1") || strings.Contains(name, "Flydigi") {
			b.logInfo("发现 BLE 设备: %s (%s)", name, result.Address.String())
			if strings.Contains(name, "BS1") {
				targetAddress = result.Address
				targetName = name
				adapter.StopScan()
				select {
				case found <- true:
				default:
				}
			}
		}
	})
	if err != nil {
		b.logError("BLE 扫描失败: %v", err)
		return false, nil
	}

	// 等待扫描结果（最多 10 秒）
	select {
	case <-found:
		b.logInfo("已找到 BS1 设备: %s (%s)", targetName, targetAddress.String())
	case <-time.After(10 * time.Second):
		b.adapter.StopScan()
		b.logError("扫描超时，未找到 BS1 设备")
		return false, nil
	}

	// 连接设备
	b.logInfo("正在连接 BS1 设备...")
	device, err := b.adapter.Connect(targetAddress, bluetooth.ConnectionParams{})
	if err != nil {
		b.logError("连接 BS1 设备失败: %v", err)
		return false, nil
	}
	b.device = device
	b.deviceAddress = targetAddress.String()

	// 发现服务和特征值
	if !b.discoverCharacteristics() {
		device.Disconnect()
		return false, nil
	}

	b.isConnected = true
	b.stopChan = make(chan struct{})

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
		fanData := b.parseBS1Notification(buf)
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

// parseBS1Notification 解析 BS1 蓝牙通知数据
// BS1 格式: [5A A5] [EF] [0B] [gearSettings] [mode] [reserved] [currentRPM_LE] [targetRPM_LE] ...
// 与 BS2/BS2PRO 类似，但没有 ReportID 前缀
func (b *BLEManager) parseBS1Notification(data []byte) *types.FanData {
	if len(data) < 9 {
		return nil
	}

	// 检查同步头
	if data[0] != 0x5A || data[1] != 0xA5 {
		return nil
	}

	if data[2] != 0xEF {
		return nil
	}

	fanData := &types.FanData{
		ReportID:     0, // BS1 没有 ReportID
		MagicSync:    0x5AA5,
		Command:      data[2],
		Status:       data[3],
		GearSettings: data[4],
		CurrentMode:  data[5],
		Reserved1:    data[6],
	}

	// 解析转速 (小端序)
	if len(data) >= 9 {
		fanData.CurrentRPM = binary.LittleEndian.Uint16(data[7:9])
	}
	if len(data) >= 11 {
		fanData.TargetRPM = binary.LittleEndian.Uint16(data[9:11])
	}

	// 解析挡位设置（与 BS2/BS2PRO 相同的编码）
	maxGear, setGear := parseBS1GearSettings(fanData.GearSettings)
	fanData.MaxGear = maxGear
	fanData.SetGear = setGear
	fanData.WorkMode = parseBS1WorkMode(fanData.CurrentMode)

	return fanData
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

// heartbeatLoop 定时发送心跳包保持 BLE 连接
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
			b.mutex.RUnlock()

			if !connected {
				return
			}

			// 交替发送两种心跳包
			var cmd []byte
			if heartbeatIndex%2 == 0 {
				cmd = types.BS1CmdHeartbeat1
			} else {
				cmd = types.BS1CmdHeartbeat2
			}
			heartbeatIndex++

			if err := b.WriteCommand(cmd); err != nil {
				b.logError("发送心跳包失败: %v", err)
				b.handleDisconnect()
				return
			}
		}
	}
}

// WriteCommand 通过 BLE 发送命令
func (b *BLEManager) WriteCommand(cmd []byte) error {
	b.mutex.RLock()
	connected := b.isConnected
	b.mutex.RUnlock()

	if !connected {
		return fmt.Errorf("BLE 设备未连接")
	}

	// 优先使用 WriteWithoutResponse（抓包显示 BS1 使用 Write Command 0x52）
	_, err := b.writeChar.WriteWithoutResponse(cmd)
	if err != nil {
		// 回退到 Write with Response
		_, err2 := b.writeChar.Write(cmd)
		if err2 != nil {
			return fmt.Errorf("BLE 写入失败: WriteWithoutResponse=%v, Write=%v", err, err2)
		}
	}
	b.recordDebugFrame("tx", types.DeviceTypeBLE, cmd)
	return nil
}

// Disconnect 断开 BLE 连接
func (b *BLEManager) Disconnect() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.isConnected {
		return
	}

	b.isConnected = false

	// 防止与 handleDisconnect 竞争导致 stopChan 被重复关闭。
	select {
	case <-b.stopChan:
	default:
		close(b.stopChan)
	}

	// 唤醒后蓝牙句柄可能已失效，底层断开调用的异常不应导致进程崩溃。
	func() {
		defer func() {
			if r := recover(); r != nil {
				b.logError("断开 BLE 设备时发生错误，已恢复: %v", r)
			}
		}()
		b.device.Disconnect()
	}()
	b.logInfo("BS1 BLE 连接已断开")
}

// handleDisconnect 处理意外断开
func (b *BLEManager) handleDisconnect() {
	b.mutex.Lock()
	wasConnected := b.isConnected
	b.isConnected = false
	b.mutex.Unlock()

	if wasConnected {
		select {
		case <-b.stopChan:
		default:
			close(b.stopChan)
		}
		b.logInfo("BS1 BLE 连接已断开")
		if b.onDisconnect != nil {
			b.onDisconnect()
		}
	}
}

// IsConnected 检查 BLE 是否已连接
func (b *BLEManager) IsConnected() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.isConnected
}

// GetCurrentFanData 获取当前风扇数据
func (b *BLEManager) GetCurrentFanData() *types.FanData {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.currentFanData
}

// SetFanSpeed 设置 BS1 风扇转速
func (b *BLEManager) SetFanSpeed(rpm int) error {
	// 先进入动态模式
	if err := b.WriteCommand(types.BS1CmdEnterDynamic); err != nil {
		return fmt.Errorf("进入动态模式失败: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// 发送转速命令
	cmd := types.BuildBS1RPMCommand(rpm)
	if err := b.WriteCommand(cmd); err != nil {
		return fmt.Errorf("设置转速失败: %v", err)
	}

	b.logInfo("BS1 已设置转速: %d RPM", rpm)
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
		b.logger.Info(format, v...)
	}
}

func (b *BLEManager) logError(format string, v ...any) {
	if b.logger != nil {
		b.logger.Error(format, v...)
	}
}

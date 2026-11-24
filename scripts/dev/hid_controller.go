//go:build ignore

// 独立调试脚本，不属于 THRM 构建：ignore 约束让 go build/vet/test ./... 跳过它。
// 运行方式：go run scripts/dev/hid_controller.go
package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

// 风扇数据结构
type FanData struct {
	ReportID     uint8  // 报告ID
	MagicSync    uint16 // 同步头 0x5AA5
	Command      uint8  // 命令码
	Status       uint8  // 状态字节
	GearSettings uint8  // 最高挡位和设置挡位
	CurrentMode  uint8  // 当前模式
	Reserved1    uint8  // 预留字节
	CurrentRPM   uint16 // 风扇实时转速
	TargetRPM    uint16 // 风扇目标转速
	MaxGear      uint8  // 最高挡位 (从GearSettings解析)
	SetGear      uint8  // 设置挡位 (从GearSettings解析)
}

// 解析挡位设置
func parseGearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	// 模式映射: 2=标准, 4=强劲, 6=超频
	maxGearMap := map[uint8]string{
		0x2: "标准",
		0x4: "强劲",
		0x6: "超频",
	}

	// 挡位映射: 8=静音, A=标准, C=强劲, E=超频
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

// 解析工作模式
func parseWorkMode(mode uint8) string {
	switch mode {
	case 0x04:
		return "挡位工作模式"
	case 0x05:
		return "自动模式(实时转速)"
	default:
		return fmt.Sprintf("未知模式(0x%02X)", mode)
	}
}

// 解析HID数据包
func parseFanData(data []byte, length int) *FanData {
	if length < 11 {
		fmt.Printf("数据包长度不足，需要至少11字节，实际: %d\n", length)
		return nil
	}

	// 检查同步头
	magic := binary.BigEndian.Uint16(data[1:3])
	if magic != 0x5AA5 {
		fmt.Printf("同步头不匹配，期望: 0x5AA5, 实际: 0x%04X\n", magic)
		return nil
	}

	fanData := &FanData{
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

	return fanData
}

// 显示风扇数据
func displayFanData(fanData *FanData) {
	fmt.Println("\n=== 风扇数据解析 ===")
	fmt.Printf("报告ID: 0x%02X\n", fanData.ReportID)
	fmt.Printf("同步头: 0x%04X\n", fanData.MagicSync)
	fmt.Printf("命令码: 0x%02X\n", fanData.Command)
	fmt.Printf("状态字节: 0x%02X\n", fanData.Status)

	maxGear, setGear := parseGearSettings(fanData.GearSettings)
	fmt.Printf("挡位设置: 0x%02X (最高挡位: %s, 设置挡位: %s)\n",
		fanData.GearSettings, maxGear, setGear)

	fmt.Printf("当前模式: %s (0x%02X)\n", parseWorkMode(fanData.CurrentMode), fanData.CurrentMode)
	fmt.Printf("预留字节: 0x%02X\n", fanData.Reserved1)
	fmt.Printf("风扇实时转速: %d RPM\n", fanData.CurrentRPM)
	fmt.Printf("风扇目标转速: %d RPM\n", fanData.TargetRPM)
	fmt.Println("==================")
}

func main() {
	fmt.Println("HID连接测试")

	// 初始化HID库
	err := hid.Init()
	if err != nil {
		fmt.Printf("初始化HID库失败: %v\n", err)
		return
	}
	defer func() {
		// 清理HID库资源
		if err := hid.Exit(); err != nil {
			fmt.Printf("清理HID库失败: %v\n", err)
		}
	}()

	// 目标设备的厂商ID和候选产品ID
	vendorID := uint16(0x37D7)
	productIDs := []uint16{0x1004, 0x1003, 0x1002}
	var device *hid.Device
	var connectedProductID uint16

	for _, productID := range productIDs {
		fmt.Printf("正在连接设备 - 厂商ID: 0x%04X, 产品ID: 0x%04X\n", vendorID, productID)
		device, err = hid.OpenFirst(vendorID, productID)
		if err == nil {
			connectedProductID = productID
			break
		}
		fmt.Printf("打开设备失败: %v\n", err)
	}

	if device == nil {
		fmt.Println("未找到可连接的 BS2PRO/BS3/BS3PRO 设备")
		return
	}
	defer func() {
		if err := device.Close(); err != nil {
			fmt.Printf("关闭设备失败: %v\n", err)
		}
	}()

	fmt.Printf("设备连接成功！产品ID: 0x%04X\n", connectedProductID)

	// 获取设备信息
	deviceInfo, err := device.GetDeviceInfo()
	if err != nil {
		fmt.Printf("获取设备信息失败: %v\n", err)
	} else {
		fmt.Printf("设备详细信息:\n")
		fmt.Printf("  制造商字符串: %s\n", deviceInfo.MfrStr)
		fmt.Printf("  产品字符串: %s\n", deviceInfo.ProductStr)
		fmt.Printf("  序列号: %s\n", deviceInfo.SerialNbr)
		fmt.Printf("  版本号: 0x%04X\n", deviceInfo.ReleaseNbr)
	}

	// 尝试读取数据（非阻塞模式）
	err = device.SetNonblock(true)
	if err != nil {
		fmt.Printf("设置非阻塞模式失败: %v\n", err)
	}

	// 读取示例
	buffer := make([]byte, 64)
	fmt.Println("尝试读取数据（超时5秒）...")

	n, err := device.ReadWithTimeout(buffer, 5*time.Second)
	if err != nil {
		if err == hid.ErrTimeout {
			fmt.Println("读取超时，设备可能没有发送数据")
		} else {
			fmt.Printf("读取数据失败: %v\n", err)
		}
	} else {
		fmt.Printf("读取到 %d 字节数据: ", n)
		for i := range n {
			fmt.Printf("%02X ", buffer[i])
		}
		fmt.Println()

		// 解析风扇数据
		if fanData := parseFanData(buffer, n); fanData != nil {
			displayFanData(fanData)
		}
	}

	// 发送数据示例（如果需要）
	// 第一个字节通常是报告ID，对于只支持单一报告的设备应为0
	// outputData := []byte{0x00, 0x01, 0x02, 0x03} // 示例数据
	// n, err = device.Write(outputData)
	// if err != nil {
	//     fmt.Printf("发送数据失败: %v\n", err)
	// } else {
	//     fmt.Printf("成功发送 %d 字节数据\n", n)
	// }

	fmt.Println("HID设备操作完成")
}

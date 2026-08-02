//go:build windows

package laptopfan

import "fmt"

const (
	// ATK WMI DSTS 设备 ID，与 Linux asus-wmi 驱动一致。
	asusDevIDCPUFan = 0x00110013
	asusDevIDGPUFan = 0x00110014

	// DSTS 返回值的存在位：置位表示固件支持该设备。
	asusDstsPresenceBit = 0x00010000
)

// readAsusFanSpeeds 通过 root\WMI 的 AsusAtkWmi_WMNB.DSTS 读取华硕机型
// （ROG/TUF 等）的风扇转速。返回值低 16 位为转速（单位：百 RPM），
// 与 asus-wmi 驱动及 G-Helper 的换算一致。
func readAsusFanSpeeds(session *wmiSession) (FanSpeeds, error) {
	caller, err := session.caller("AsusAtkWmi_WMNB", "DSTS")
	if err != nil {
		return FanSpeeds{}, err
	}

	cpuRPM, err := readAsusFanRPM(caller, asusDevIDCPUFan)
	if err != nil {
		return FanSpeeds{}, err
	}
	// 部分机型（如无独显或单风扇）不提供 GPU 风扇，容忍缺失。
	gpuRPM, err := readAsusFanRPM(caller, asusDevIDGPUFan)
	if err != nil {
		gpuRPM = 0
	}

	return validateSpeeds(FanSpeeds{CPUFanRPM: cpuRPM, GPUFanRPM: gpuRPM})
}

func readAsusFanRPM(caller *wmiMethodCaller, devID uint32) (int, error) {
	status, err := caller.call("Device_ID", int32(devID), "device_status")
	if err != nil {
		return 0, fmt.Errorf("DSTS(0x%08x): %w", devID, err)
	}
	if status&asusDstsPresenceBit == 0 {
		return 0, fmt.Errorf("DSTS(0x%08x) 设备不存在（status=0x%08x）", devID, status)
	}
	value := status & 0xffff
	if value == 0xffff {
		return 0, fmt.Errorf("DSTS(0x%08x) 返回无效转速", devID)
	}
	return int(value) * 100, nil
}

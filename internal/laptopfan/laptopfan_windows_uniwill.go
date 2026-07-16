//go:build windows

package laptopfan

import (
	"fmt"
	"strconv"

	"github.com/go-ole/go-ole"
)

const (
	// Uniwill EC RAM 风扇转速寄存器（16 位大端），与 qc71_laptop 的
	// FAN_RPM_1_ADDR / FAN_RPM_2_ADDR 一致。
	ecRegFan1RPM = 0x0464
	ecRegFan2RPM = 0x046C

	// GetSetULong 输入 u64 的第 5 字节为功能码：1=读 EC。
	uniwillFunctionRead = 1

	// WMI 读失败时固件返回的哨兵值。
	uniwillReadErrorValue = 0xfefefefe
)

// readUniwillFanSpeeds 通过 root\WMI 的 AcpiTest_MULong.GetSetULong（ExecMethod）
// 读取 Uniwill/同方准系统（机械革命等品牌）的 EC RAM。
func readUniwillFanSpeeds() (FanSpeeds, error) {
	var speeds FanSpeeds
	err := withWMIService(func(service *ole.IDispatch) error {
		caller, err := newWMIMethodCaller(service, "AcpiTest_MULong", "GetSetULong")
		if err != nil {
			return err
		}
		defer caller.release()

		cpuRPM, err := readUniwillEC16(caller, ecRegFan1RPM)
		if err != nil {
			return err
		}
		gpuRPM, err := readUniwillEC16(caller, ecRegFan2RPM)
		if err != nil {
			return err
		}
		speeds = FanSpeeds{CPUFanRPM: cpuRPM, GPUFanRPM: gpuRPM}
		return nil
	})
	if err != nil {
		return FanSpeeds{}, err
	}
	return validateSpeeds(speeds)
}

// readUniwillEC16 读取 16 位大端 EC 寄存器。单次 GetSetULong 返回 addr（低字节）与
// addr+1（高字节）两个连续字节，因此一次调用即可拼出完整数值。
func readUniwillEC16(caller *wmiMethodCaller, addr uint16) (int, error) {
	// 输入 u64：byte0=addr_low, byte1=addr_high, byte5=功能码(1=读)。
	// CIM uint64 经 IDispatch 自动化传输时以十进制字符串表示。
	data := uint64(addr) | uint64(uniwillFunctionRead)<<40
	value, err := caller.call(map[string]interface{}{
		"Data": strconv.FormatUint(data, 10),
	}, "Return")
	if err != nil {
		return 0, fmt.Errorf("GetSetULong(0x%04x): %w", addr, err)
	}
	if value == uniwillReadErrorValue {
		return 0, fmt.Errorf("EC 读取错误（0x%04x 返回哨兵值）", addr)
	}

	dataLow := int(value & 0xff)         // EC[addr]
	dataHigh := int((value >> 8) & 0xff) // EC[addr+1]
	return dataLow<<8 | dataHigh, nil
}

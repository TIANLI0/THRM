//go:build windows

package laptopfan

import (
	"fmt"

	"github.com/go-ole/go-ole"
)

const (
	// LENOVO_FAN_METHOD 的 FanID，与 LenovoLegionToolkit 一致。
	lenovoFanIDCPU = 0
	lenovoFanIDGPU = 1
)

// readLenovoFanSpeeds 通过 root\WMI 的 LENOVO_FAN_METHOD.Fan_GetCurrentFanSpeed
// 读取联想拯救者（Legion）系列的风扇转速，输出即为 RPM。
func readLenovoFanSpeeds() (FanSpeeds, error) {
	var speeds FanSpeeds
	err := withWMIService(func(service *ole.IDispatch) error {
		caller, err := newWMIMethodCaller(service, "LENOVO_FAN_METHOD", "Fan_GetCurrentFanSpeed")
		if err != nil {
			return err
		}
		defer caller.release()

		cpuRPM, err := readLenovoFanRPM(caller, lenovoFanIDCPU)
		if err != nil {
			return err
		}
		// 无独显/单风扇机型可能没有第二个风扇，容忍缺失。
		gpuRPM, err := readLenovoFanRPM(caller, lenovoFanIDGPU)
		if err != nil {
			gpuRPM = 0
		}
		speeds = FanSpeeds{CPUFanRPM: cpuRPM, GPUFanRPM: gpuRPM}
		return nil
	})
	if err != nil {
		return FanSpeeds{}, err
	}
	return validateSpeeds(speeds)
}

func readLenovoFanRPM(caller *wmiMethodCaller, fanID int) (int, error) {
	value, err := caller.call(map[string]interface{}{
		"FanID": int32(fanID),
	}, "CurrentFanSpeed")
	if err != nil {
		return 0, fmt.Errorf("Fan_GetCurrentFanSpeed(%d): %w", fanID, err)
	}
	return int(value), nil
}

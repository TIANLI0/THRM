//go:build windows

package laptopfan

import (
	"fmt"

	"github.com/go-ole/go-ole"
)

const (
	// LENOVO_FAN_METHOD 的 FanID，与 LenovoLegionToolkit 一致。
	lenovoFanIDCPU    = 0
	lenovoFanIDGPU    = 1
	lenovoCpuFanSpeed = 0x04030001
	lenovoGpuFanSpeed = 0x04030002
	lenovoPchFanSpeed = 0x04030004
)

// readLenovoFanSpeeds 优先尝试通过 LENOVO_FAN_METHOD.Fan_GetCurrentFanSpeed
// 读取风扇转速；若失败则回退到 modern 方式 LENOVO_OTHER_METHOD.GetFeatureValue。
func readLenovoFanSpeeds() (FanSpeeds, error) {
	// 先尝试传统方式
	speeds, err := readLenovoFanSpeedsLegacy()
	if err == nil {
		return validateSpeeds(speeds)
	}
	// 回退到 Modern 方式
	return readLenovoFanSpeedsModern()
}

// readLenovoFanSpeedsLegacy 通过 LENOVO_FAN_METHOD 获取风扇转速。
func readLenovoFanSpeedsLegacy() (FanSpeeds, error) {
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
	return speeds, nil
}

// readLenovoFanSpeedsModern 通过 LENOVO_OTHER_METHOD.GetFeatureValue 获取风扇转速。
func readLenovoFanSpeedsModern() (FanSpeeds, error) {
	var speeds FanSpeeds
	err := withWMIService(func(service *ole.IDispatch) error {
		caller, err := newWMIMethodCaller(service, "LENOVO_OTHER_METHOD", "GetFeatureValue")
		if err != nil {
			return err
		}
		defer caller.release()

		cpuRPM, err := readLenovoFanRPMModern(caller, lenovoCpuFanSpeed)
		if err != nil {
			return err
		}
		// GPU 风扇可能不存在
		gpuRPM, err := readLenovoFanRPMModern(caller, lenovoGpuFanSpeed)
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

func readLenovoFanRPMModern(caller *wmiMethodCaller, idRaw uint32) (int, error) {
	value, err := caller.call(map[string]interface{}{
		"IDs": idRaw,
	}, "Value")
	if err != nil {
		return 0, fmt.Errorf("GetFeatureValue(0x%x): %w", idRaw, err)
	}
	return int(value), nil
}

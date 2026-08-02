//go:build windows

package laptopfan

import "fmt"

const (
	// LENOVO_FAN_METHOD 的 FanID，与 LenovoLegionToolkit 一致。
	lenovoFanIDCPU    = 0
	lenovoFanIDGPU    = 1
	lenovoCpuFanSpeed = 0x04030001
	lenovoGpuFanSpeed = 0x04030002
	lenovoPchFanSpeed = 0x04030004
)

// readLenovoFanSpeedsLegacy 通过 LENOVO_FAN_METHOD.Fan_GetCurrentFanSpeed
// 获取风扇转速。较新的机型没有这个类，会由后端探测自动落到 modern 实现。
func readLenovoFanSpeedsLegacy(session *wmiSession) (FanSpeeds, error) {
	caller, err := session.caller("LENOVO_FAN_METHOD", "Fan_GetCurrentFanSpeed")
	if err != nil {
		return FanSpeeds{}, err
	}

	cpuRPM, err := readLenovoFanRPM(caller, lenovoFanIDCPU)
	if err != nil {
		return FanSpeeds{}, err
	}
	// 无独显/单风扇机型可能没有第二个风扇，容忍缺失。
	gpuRPM, err := readLenovoFanRPM(caller, lenovoFanIDGPU)
	if err != nil {
		gpuRPM = 0
	}

	return validateSpeeds(FanSpeeds{CPUFanRPM: cpuRPM, GPUFanRPM: gpuRPM})
}

// readLenovoFanSpeedsModern 通过 LENOVO_OTHER_METHOD.GetFeatureValue 获取风扇转速。
func readLenovoFanSpeedsModern(session *wmiSession) (FanSpeeds, error) {
	caller, err := session.caller("LENOVO_OTHER_METHOD", "GetFeatureValue")
	if err != nil {
		return FanSpeeds{}, err
	}

	cpuRPM, err := readLenovoFanRPMModern(caller, lenovoCpuFanSpeed)
	if err != nil {
		return FanSpeeds{}, err
	}
	// GPU 风扇可能不存在
	gpuRPM, err := readLenovoFanRPMModern(caller, lenovoGpuFanSpeed)
	if err != nil {
		gpuRPM = 0
	}

	return validateSpeeds(FanSpeeds{CPUFanRPM: cpuRPM, GPUFanRPM: gpuRPM})
}

func readLenovoFanRPM(caller *wmiMethodCaller, fanID int) (int, error) {
	value, err := caller.call("FanID", int32(fanID), "CurrentFanSpeed")
	if err != nil {
		return 0, fmt.Errorf("Fan_GetCurrentFanSpeed(%d): %w", fanID, err)
	}
	return int(value), nil
}

func readLenovoFanRPMModern(caller *wmiMethodCaller, idRaw uint32) (int, error) {
	value, err := caller.call("IDs", idRaw, "Value")
	if err != nil {
		return 0, fmt.Errorf("GetFeatureValue(0x%x): %w", idRaw, err)
	}
	return int(value), nil
}

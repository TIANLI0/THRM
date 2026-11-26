//go:build darwin

package temperature

import (
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

// readSensorCPUTemp 读取 macOS 的 CPU 温度。
//
// gopsutil 在 Apple Silicon 上经 IOHIDEventSystem 枚举传感器，名称形如
// "PMU tdie1"（CPU 核心 die）、"pACC MTR Temp Sensor0"（性能核簇）；在 Intel
// Mac 上读 SMC，键名形如 TC0P/TC0D。两者都不含其它平台匹配的 cpu/core 字样，
// 所以单独挑选，并取同组最高值作为封装温度。
func (r *Reader) readSensorCPUTemp() int {
	stats, err := sensors.SensorsTemperatures()
	if err != nil && len(stats) == 0 {
		return 0
	}
	return pickDarwinCPUTemp(stats)
}

func pickDarwinCPUTemp(stats []sensors.TemperatureStat) int {
	groups := []func(key string) bool{
		// Apple Silicon：CPU die。
		func(key string) bool { return strings.Contains(key, "tdie") },
		// Apple Silicon：CPU 簇（性能核/能效核）。
		func(key string) bool {
			return strings.Contains(key, "acc mtr temp") || strings.Contains(key, "cpu")
		},
		// Intel SMC：CPU die / PECI / proximity。
		func(key string) bool {
			switch key {
			case "tc0e", "tc0f", "tc0d", "tc0p", "tc0c", "tcxc", "tc0h":
				return true
			}
			return false
		},
	}
	for _, match := range groups {
		best := 0.0
		for _, stat := range stats {
			key := strings.ToLower(strings.TrimSpace(stat.SensorKey))
			if !match(key) {
				continue
			}
			if stat.Temperature > 0 && stat.Temperature < 130 && stat.Temperature > best {
				best = stat.Temperature
			}
		}
		if best > 0 {
			return int(best + 0.5)
		}
	}
	return 0
}

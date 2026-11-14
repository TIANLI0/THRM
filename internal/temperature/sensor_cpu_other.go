//go:build !linux

package temperature

import (
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

// readSensorCPUTemp 通过 gopsutil 读取 CPU 温度。非 Linux 平台保留原行为。
func (r *Reader) readSensorCPUTemp() int {
	sensorTemps, err := sensors.SensorsTemperatures()
	if err != nil {
		return 0
	}
	for _, sensor := range sensorTemps {
		if strings.Contains(strings.ToLower(sensor.SensorKey), "tz00") ||
			strings.Contains(strings.ToLower(sensor.SensorKey), "cpu") ||
			strings.Contains(strings.ToLower(sensor.SensorKey), "core") {
			return int(sensor.Temperature)
		}
	}
	return 0
}

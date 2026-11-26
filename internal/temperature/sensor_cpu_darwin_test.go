//go:build darwin

package temperature

import (
	"testing"

	"github.com/shirou/gopsutil/v4/sensors"
)

func TestPickDarwinCPUTemp(t *testing.T) {
	cases := []struct {
		name  string
		stats []sensors.TemperatureStat
		want  int
	}{
		{"apple silicon die wins", []sensors.TemperatureStat{
			{SensorKey: "pACC MTR Temp Sensor0", Temperature: 70},
			{SensorKey: "PMU tdie1", Temperature: 52.4},
			{SensorKey: "PMU tdie2", Temperature: 55.6},
			{SensorKey: "NAND CH0 temp", Temperature: 90},
		}, 56},
		{"apple silicon cluster fallback", []sensors.TemperatureStat{
			{SensorKey: "eACC MTR Temp Sensor0", Temperature: 41},
			{SensorKey: "pACC MTR Temp Sensor2", Temperature: 48},
		}, 48},
		{"intel smc", []sensors.TemperatureStat{
			{SensorKey: "TA0P", Temperature: 30},
			{SensorKey: "TC0P", Temperature: 61},
			{SensorKey: "TC0D", Temperature: 0},
		}, 61},
		{"ignores bogus readings", []sensors.TemperatureStat{
			{SensorKey: "PMU tdie1", Temperature: 200},
		}, 0},
	}
	for _, tc := range cases {
		if got := pickDarwinCPUTemp(tc.stats); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

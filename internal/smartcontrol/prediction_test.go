package smartcontrol

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestThermalPredictorAnticipatesRisingCPULoad(t *testing.T) {
	predictor := NewThermalPredictor()
	start := time.Unix(1_717_000_000, 0)
	samples := []types.TemperatureData{
		{CPUTemp: 60, GPUTemp: 45, CPUPower: 20},
		{CPUTemp: 62, GPUTemp: 45, CPUPower: 22},
		{CPUTemp: 65, GPUTemp: 46, CPUPower: 70},
	}

	var prediction ThermalPrediction
	for i, sample := range samples {
		prediction = predictor.Observe(sample, start.Add(time.Duration(i)*2*time.Second), types.TempSourceCPU, 6)
	}
	if prediction.CPUTemp <= samples[len(samples)-1].CPUTemp {
		t.Fatalf("predicted CPU temperature = %d, want above measured %d", prediction.CPUTemp, samples[len(samples)-1].CPUTemp)
	}
	if prediction.ControlTemp != prediction.CPUTemp {
		t.Fatalf("CPU control temperature = %d, want %d", prediction.ControlTemp, prediction.CPUTemp)
	}
	if prediction.CPURise <= 0 {
		t.Fatalf("CPU anticipated rise = %.2f, want positive", prediction.CPURise)
	}
}

func TestThermalPredictorUsesSelectedGPU(t *testing.T) {
	predictor := NewThermalPredictor()
	start := time.Unix(1_717_000_000, 0)
	for i, gpuTemp := range []int{55, 58, 62} {
		prediction := predictor.Observe(types.TemperatureData{
			CPUTemp:  50,
			GPUTemp:  gpuTemp,
			GPUPower: float64(30 + i*40),
		}, start.Add(time.Duration(i)*2*time.Second), types.TempSourceGPU, 6)
		if i == 2 {
			if prediction.ControlTemp != prediction.GPUTemp {
				t.Fatalf("GPU control temperature = %d, want %d", prediction.ControlTemp, prediction.GPUTemp)
			}
			if prediction.GPUTemp <= gpuTemp {
				t.Fatalf("predicted GPU temperature = %d, want above measured %d", prediction.GPUTemp, gpuTemp)
			}
		}
	}
}

func TestThermalPredictorDoesNotPredictCoolingBelowMeasurement(t *testing.T) {
	predictor := NewThermalPredictor()
	start := time.Unix(1_717_000_000, 0)
	var prediction ThermalPrediction
	for i, cpuTemp := range []int{75, 72, 69} {
		prediction = predictor.Observe(types.TemperatureData{CPUTemp: cpuTemp, GPUTemp: 50, CPUPower: 50}, start.Add(time.Duration(i)*2*time.Second), types.TempSourceCPU, 12)
	}
	if prediction.CPUTemp != 69 || prediction.ControlTemp != 69 {
		t.Fatalf("cooling prediction = %+v, want measured 69°C", prediction)
	}
	if prediction.CPURise != 0 {
		t.Fatalf("cooling sample anticipated rise = %.2f, want 0", prediction.CPURise)
	}
}

func TestThermalPredictorBoundsPowerLead(t *testing.T) {
	predictor := NewThermalPredictor()
	start := time.Unix(1_717_000_000, 0)
	predictor.Observe(types.TemperatureData{CPUTemp: 65, CPUPower: 10}, start, types.TempSourceCPU, 12)
	predictor.Observe(types.TemperatureData{CPUTemp: 65, CPUPower: 10}, start.Add(2*time.Second), types.TempSourceCPU, 12)
	prediction := predictor.Observe(types.TemperatureData{CPUTemp: 65, CPUPower: 1000}, start.Add(4*time.Second), types.TempSourceCPU, 12)
	if prediction.CPURise > maxPowerLead+0.01 {
		t.Fatalf("anticipated rise = %.2f, exceeded power lead cap %.2f", prediction.CPURise, maxPowerLead)
	}
}

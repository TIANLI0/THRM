package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestCompactTemperatureEventPayload(t *testing.T) {
	sharedCPUSensors := []types.TemperatureSensor{{Key: "cpu-package", Name: "CPU Package", Value: 71}}
	sharedGPUSensors := []types.TemperatureSensor{{Key: "gpu-core", Name: "GPU Core", Value: 66}}
	sharedCPUPowerSensors := []types.PowerSensor{{Key: "cpu/power/package", Name: "CPU Package Power", Value: 45.5}}
	sharedGPUPowerSensors := []types.PowerSensor{{Key: "gpu/power/board", Name: "GPU Board Power", Value: 80.2}}
	sharedGPUDevices := []types.TemperatureGPUDevice{{
		Key:    "gpu0",
		Name:   "GPU 0",
		Vendor: "nvidia",
		Sensors: []types.TemperatureSensor{{
			Key:   "gpu-core",
			Name:  "GPU Core",
			Value: 66,
		}},
		PowerSensors: sharedGPUPowerSensors,
	}}

	previous := types.TemperatureData{
		CPUTemp:         70,
		CpuSensors:      sharedCPUSensors,
		GpuSensors:      sharedGPUSensors,
		CpuPowerSensors: sharedCPUPowerSensors,
		GpuPowerSensors: sharedGPUPowerSensors,
		GpuDevices:      sharedGPUDevices,
	}
	current := previous
	current.CPUTemp = 72

	compact := compactTemperatureEventPayload(current, previous)
	if compact.CpuSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged cpuSensors")
	}
	if compact.GpuSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged gpuSensors")
	}
	if compact.CpuPowerSensors != nil || compact.GpuPowerSensors != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged power sensors")
	}
	if compact.GpuDevices != nil {
		t.Fatal("compactTemperatureEventPayload() should strip unchanged gpuDevices")
	}

	changed := current
	changed.GpuSensors = []types.TemperatureSensor{{Key: "gpu-hotspot", Name: "GPU Hotspot", Value: 77}}
	compactChanged := compactTemperatureEventPayload(changed, previous)
	if len(compactChanged.GpuSensors) != 1 || compactChanged.GpuSensors[0].Key != "gpu-hotspot" {
		t.Fatal("compactTemperatureEventPayload() should keep changed gpuSensors")
	}

	cleared := current
	cleared.CpuSensors = []types.TemperatureSensor{}
	compactCleared := compactTemperatureEventPayload(cleared, previous)
	if compactCleared.CpuSensors == nil {
		t.Fatal("compactTemperatureEventPayload() should keep explicit empty cpuSensors to clear stale metadata")
	}
	if len(compactCleared.CpuSensors) != 0 {
		t.Fatalf("compactTemperatureEventPayload() kept unexpected cpuSensors length: %d", len(compactCleared.CpuSensors))
	}
}

func TestTrackBridgeTemperatureStaleness(t *testing.T) {
	tests := []struct {
		name           string
		temp           types.TemperatureData
		lastUpdate     int64
		staleCount     int
		wantUpdate     int64
		wantStaleCount int
		wantRestartNow bool
	}{
		{
			name:           "reset when bridge is not ok",
			temp:           types.TemperatureData{BridgeOk: false, UpdateTime: 1000},
			lastUpdate:     1000,
			staleCount:     2,
			wantUpdate:     0,
			wantStaleCount: 0,
		},
		{
			name:           "accept fresh update time",
			temp:           types.TemperatureData{BridgeOk: true, UpdateTime: 2000},
			lastUpdate:     1000,
			staleCount:     2,
			wantUpdate:     2000,
			wantStaleCount: 0,
		},
		{
			name:           "trigger restart after repeated stale update",
			temp:           types.TemperatureData{BridgeOk: true, UpdateTime: 2000},
			lastUpdate:     2000,
			staleCount:     staleBridgeUpdateThreshold - 1,
			wantUpdate:     2000,
			wantStaleCount: staleBridgeUpdateThreshold,
			wantRestartNow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdate, gotStaleCount, gotRestartNow := trackBridgeTemperatureStaleness(tt.temp, tt.lastUpdate, tt.staleCount)
			if gotUpdate != tt.wantUpdate {
				t.Fatalf("trackBridgeTemperatureStaleness() update = %d, want %d", gotUpdate, tt.wantUpdate)
			}
			if gotStaleCount != tt.wantStaleCount {
				t.Fatalf("trackBridgeTemperatureStaleness() staleCount = %d, want %d", gotStaleCount, tt.wantStaleCount)
			}
			if gotRestartNow != tt.wantRestartNow {
				t.Fatalf("trackBridgeTemperatureStaleness() restart = %v, want %v", gotRestartNow, tt.wantRestartNow)
			}
		})
	}
}

package temperature

import (
	"context"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/types"
)

// TestBridgeFallbackPath verifies temperature reading falls back to native
// when the bridge is not supported (standard Linux path). On these platforms
// the native path is the normal path, so no bridge warning must be raised.
func TestBridgeFallbackPath(t *testing.T) {
	oldExec := execHelperCommand
	defer func() { execHelperCommand = oldExec }()

	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		// Return timeout to skip nvidia-smi (no real GPU needed for this test)
		return nil, context.DeadlineExceeded
	}

	bridgeMgr := bridge.NewManager(testLogger{})
	if bridgeMgr.IsSupported() {
		t.Skip("Bridge unexpectedly supported on this platform")
	}

	r := NewReader(bridgeMgr, testLogger{})
	sel := types.TemperatureSelection{TempSource: types.TempSourceMax}
	result := r.Read(sel)

	if !result.BridgeOk {
		t.Error("BridgeOk should stay true when the bridge is not supported (native path is normal)")
	}
	if result.BridgeMsg != "" {
		t.Errorf("BridgeMsg should be empty when the bridge is not supported, got: %s", result.BridgeMsg)
	}
	t.Logf("CPU temp: %d, GPU temp: %d", result.CPUTemp, result.GPUTemp)
}

// TestTempSourceResolve verifies control temperature resolution
func TestTempSourceResolve(t *testing.T) {
	tests := []struct {
		name     string
		cpuTemp  int
		gpuTemp  int
		source   string
		wantCtrl int
	}{
		{"max with CPU higher", 80, 60, "max", 80},
		{"max with GPU higher", 50, 75, "max", 75},
		{"max normalized from empty", 80, 60, "", 80},
		{"cpu only", 80, 60, "cpu", 80},
		{"gpu only", 50, 75, "gpu", 75},
		{"both zero", 0, 0, "max", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveControlTemp(tt.cpuTemp, tt.gpuTemp, tt.source)
			if got != tt.wantCtrl {
				t.Errorf("resolveControlTemp(%d, %d, %q) = %d, want %d",
					tt.cpuTemp, tt.gpuTemp, tt.source, got, tt.wantCtrl)
			}
		})
	}
}

// TestMultiSensorAverage verifies multi-sensor CPU temperature averaging
func TestMultiSensorAverage(t *testing.T) {
	tests := []struct {
		name    string
		sensors []types.TemperatureSensor
		keys    []string
		want    int
		wantOk  bool
	}{
		{
			name: "average three sensors",
			sensors: []types.TemperatureSensor{
				{Key: "core0", Value: 40},
				{Key: "core1", Value: 50},
				{Key: "core2", Value: 60},
			},
			keys:   []string{"core0", "core1"},
			want:   (40 + 50) / 2,
			wantOk: true,
		},
		{
			name: "single sensor match",
			sensors: []types.TemperatureSensor{
				{Key: "core0", Value: 40},
			},
			keys:   []string{"core0"},
			want:   40,
			wantOk: true,
		},
		{
			name: "no match",
			sensors: []types.TemperatureSensor{
				{Key: "core0", Value: 40},
			},
			keys:   []string{"package"},
			want:   0,
			wantOk: false,
		},
		{
			name:    "empty keys",
			sensors: []types.TemperatureSensor{{Key: "core0", Value: 40}},
			keys:    nil,
			want:    0,
			wantOk:  false,
		},
		{
			name: "case-insensitive match",
			sensors: []types.TemperatureSensor{
				{Key: "Core0", Value: 55},
			},
			keys:   []string{"core0"},
			want:   55,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := averageSelectedCpuTemp(tt.sensors, tt.keys)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("average = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestApplyMultiSensorCpuAverage verifies the full apply function
func TestApplyMultiSensorCpuAverage(t *testing.T) {
	temp := &types.TemperatureData{
		CPUTemp:       70,
		GPUTemp:       60,
		MaxTemp:       70,
		ControlTemp:   70,
		ControlSource: types.TempSourceMax,
		CpuSensors: []types.TemperatureSensor{
			{Key: "core0", Value: 40},
			{Key: "core1", Value: 50},
			{Key: "core2", Value: 60},
		},
	}

	// Apply multi-sensor average for core0 + core1
	applyMultiSensorCpuAverage(temp, []string{"core0", "core1"})

	expectedAvg := (40 + 50) / 2
	if temp.CPUTemp != expectedAvg {
		t.Errorf("CPUTemp = %d, want %d", temp.CPUTemp, expectedAvg)
	}
	// MaxTemp = max(CPU=45, GPU=60) = 60
	if temp.MaxTemp != 60 {
		t.Errorf("MaxTemp = %d, want %d (max of CPU=%d, GPU=%d)", temp.MaxTemp, 60, expectedAvg, temp.GPUTemp)
	}
	if temp.ControlTemp != 60 {
		t.Errorf("ControlTemp = %d, want %d (max source)", temp.ControlTemp, 60)
	}
}

// TestCalculateTargetRPM_EdgeCases covers boundary conditions
func TestCalculateTargetRPM_EdgeCases(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 50, RPM: 1500},
		{Temperature: 70, RPM: 3000},
		{Temperature: 90, RPM: 4000},
	}

	tests := []struct {
		name string
		temp int
		want int
	}{
		{"below min", 20, 800},
		{"at min", 30, 800},
		{"mid first segment", 40, 1150},
		{"at mid", 50, 1500},
		{"mid second segment", 60, 2250},
		{"at max", 90, 4000},
		{"above max", 110, 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTargetRPM(tt.temp, curve)
			if got != tt.want {
				t.Errorf("CalculateTargetRPM(%d) = %d, want %d", tt.temp, got, tt.want)
			}
		})
	}
}

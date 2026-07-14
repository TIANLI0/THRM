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

// TestNvidiaTempParse verifies nvidia-smi output parsing
func TestNvidiaTempParse(t *testing.T) {
	oldExec := execHelperCommand
	defer func() { execHelperCommand = oldExec }()

	tests := []struct {
		name     string
		output   string
		hasError bool
		wantTemp int
	}{
		{
			name:     "single GPU",
			output:   "71\n",
			hasError: false,
			wantTemp: 71,
		},
		{
			name:     "with trailing newline",
			output:   "65\n\n",
			hasError: false,
			wantTemp: 65,
		},
		{
			name:     "multiple GPUs - first only",
			output:   "65\n72\n",
			hasError: false,
			wantTemp: 65,
		},
		{
			name:     "garbage output",
			output:   "N/A\n",
			hasError: false,
			wantTemp: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
				if tt.hasError {
					return nil, context.DeadlineExceeded
				}
				return []byte(tt.output), nil
			}

			r := NewReader(nil, testLogger{})
			got := r.readNvidiaGPUTemp()
			if got != tt.wantTemp {
				t.Errorf("readNvidiaGPUTemp() = %d, want %d", got, tt.wantTemp)
			}
		})
	}
}

// TestNvidiaTempTimeout verifies timeout returns 0
func TestNvidiaTempTimeout(t *testing.T) {
	oldExec := execHelperCommand
	defer func() { execHelperCommand = oldExec }()

	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %v", timeout)
		}
		return nil, context.DeadlineExceeded
	}

	r := NewReader(nil, testLogger{})
	got := r.readNvidiaGPUTemp()
	if got != 0 {
		t.Errorf("readNvidiaGPUTemp() timeout = %d, want 0", got)
	}
}

func TestReadNvidiaGPUPower(t *testing.T) {
	oldExec := execHelperCommand
	defer func() { execHelperCommand = oldExec }()

	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %v", timeout)
		}
		if name != "nvidia-smi" {
			t.Fatalf("unexpected command: %s", name)
		}
		return []byte("123.45\n"), nil
	}

	if got := NewReader(nil, testLogger{}).readNvidiaGPUPower(); got != 123.45 {
		t.Fatalf("readNvidiaGPUPower() = %.2f, want 123.45", got)
	}
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

// TestDetectGPUVendorCacheExpiry verifies cached GPU vendor expires after TTL
func TestDetectGPUVendorCacheExpiry(t *testing.T) {
	oldExec := execHelperCommand
	oldNow := readTimeNow
	defer func() {
		execHelperCommand = oldExec
		readTimeNow = oldNow
	}()

	now := time.Unix(1_717_000_000, 0)
	readTimeNow = func() time.Time { return now }

	calls := 0
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		calls++
		return []byte("NVIDIA-SMI 555.00"), nil
	}

	r := NewReader(nil, testLogger{})

	// First call - should invoke exec
	v1 := r.detectGPUVendor()
	if v1 != "nvidia" {
		t.Fatalf("detectGPUVendor() = %q, want nvidia", v1)
	}
	if calls != 1 {
		t.Fatalf("First call should invoke command, calls=%d", calls)
	}

	// Second call within TTL - should use cache
	v2 := r.detectGPUVendor()
	if v2 != "nvidia" {
		t.Errorf("Cached detectGPUVendor() = %q", v2)
	}
	if calls != 1 {
		t.Fatalf("Cache should avoid re-invocation, calls=%d", calls)
	}

	// After TTL expiry - should invoke again
	now = now.Add(gpuVendorCacheTTL + time.Second)
	v3 := r.detectGPUVendor()
	if v3 != "nvidia" {
		t.Errorf("Post-TTL detectGPUVendor() = %q", v3)
	}
	if calls != 2 {
		t.Fatalf("Post-TTL should invoke command again, calls=%d", calls)
	}
}

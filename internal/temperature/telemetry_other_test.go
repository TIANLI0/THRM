//go:build !linux

package temperature

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDetectGPUVendorCachesResult(t *testing.T) {
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
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %s", timeout)
		}
		if name != "nvidia-smi" {
			t.Fatalf("unexpected command: %s", name)
		}
		return []byte("NVIDIA-SMI 555.00"), nil
	}

	r := NewReader(nil, testLogger{})
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() = %q, want nvidia", got)
	}
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() cached = %q, want nvidia", got)
	}
	if calls != 1 {
		t.Fatalf("detectGPUVendor() calls = %d, want 1 with cache hit", calls)
	}

	now = now.Add(gpuVendorCacheTTL + time.Second)
	if got := r.detectGPUVendor(); got != "nvidia" {
		t.Fatalf("detectGPUVendor() after ttl = %q, want nvidia", got)
	}
	if calls != 2 {
		t.Fatalf("detectGPUVendor() calls after ttl = %d, want 2", calls)
	}
}

// TestNvidiaTempAndPowerParse 验证合并查询的输出解析。
// GPU 温度与功耗现在由一次 nvidia-smi --query-gpu=temperature.gpu,power.draw 取回，
// 降级路径因此从两个进程降到一个。
func TestNvidiaTempAndPowerParse(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantTemp  int
		wantPower float64
	}{
		{name: "temp and power", output: "71, 123.45\n", wantTemp: 71, wantPower: 123.45},
		{name: "trailing newlines", output: "65, 30.0\n\n", wantTemp: 65, wantPower: 30},
		{name: "multiple GPUs - first only", output: "65, 30.0\n72, 40.0\n", wantTemp: 65, wantPower: 30},
		{name: "power not available", output: "65, [N/A]\n", wantTemp: 65, wantPower: 0},
		{name: "garbage output", output: "N/A\n", wantTemp: 0, wantPower: 0},
		{name: "empty output", output: "\n", wantTemp: 0, wantPower: 0},
		{name: "out of range temp", output: "999, 10.0\n", wantTemp: 0, wantPower: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTemp, gotPower := parseNvidiaTempAndPower(tt.output)
			if gotTemp != tt.wantTemp {
				t.Errorf("temp = %d, want %d", gotTemp, tt.wantTemp)
			}
			if gotPower != tt.wantPower {
				t.Errorf("power = %.2f, want %.2f", gotPower, tt.wantPower)
			}
		})
	}
}

// TestGPUTempAndPowerSingleCommand 验证合并后只发一次 nvidia-smi 查询命令。
func TestGPUTempAndPowerSingleCommand(t *testing.T) {
	oldExec := execHelperCommand
	defer func() { execHelperCommand = oldExec }()

	queries := 0
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %v", timeout)
		}
		if name != "nvidia-smi" {
			t.Fatalf("unexpected command: %s", name)
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "--query-gpu=") {
				queries++
				if arg != "--query-gpu=temperature.gpu,power.draw" {
					t.Fatalf("unexpected query: %s", arg)
				}
			}
		}
		return []byte("71, 123.45\n"), nil
	}

	r := NewReader(nil, testLogger{})
	temp, power := r.readGPUTempAndPower()
	if temp != 71 || power != 123.45 {
		t.Fatalf("readGPUTempAndPower() = (%d, %.2f), want (71, 123.45)", temp, power)
	}
	if queries != 1 {
		t.Fatalf("nvidia-smi 查询次数 = %d, want 1", queries)
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
	temp, power := r.readGPUTempAndPower()
	if temp != 0 || power != 0 {
		t.Errorf("readGPUTempAndPower() timeout = (%d, %.2f), want (0, 0)", temp, power)
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

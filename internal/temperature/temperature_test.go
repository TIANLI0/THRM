package temperature

import (
	"context"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestResolveControlTempFallsBackToAvailableSensor(t *testing.T) {
	if got := resolveControlTemp(0, 67, "cpu"); got != 67 {
		t.Fatalf("CPU source fallback = %d, want 67", got)
	}
	if got := resolveControlTemp(58, 0, "gpu"); got != 58 {
		t.Fatalf("GPU source fallback = %d, want 58", got)
	}
	if got := resolveControlTemp(0, 0, "max"); got != 0 {
		t.Fatalf("empty fallback = %d, want 0", got)
	}
}

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Debug(string, ...any) {}
func (testLogger) Close()               {}
func (testLogger) CleanOldLogs()        {}
func (testLogger) SetDebugMode(bool)    {}
func (testLogger) GetLogDir() string    { return "" }

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

// 没有可信 CPU 温度来源的平台（Windows）必须彻底放弃兜底：ACPI 温区不是 CPU 核心
// 温度，拿它填补空缺会给出错误的控温依据，并把 PawnIO 故障伪装成一次正常读数。
// 这里断言降级路径既不返回伪造的 CPU 温度，也不再为此创建 wmic 进程。
func TestCPUFallbackDisabledWithoutTrustedSource(t *testing.T) {
	if platformHasCPUTempFallback {
		t.Skip("本平台存在可信的进程内 CPU 温度来源")
	}

	oldExec := execHelperCommand
	defer func() {
		execHelperCommand = oldExec
	}()

	var spawned []string
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		spawned = append(spawned, name)
		return nil, context.DeadlineExceeded
	}

	r := NewReader(nil, testLogger{})
	// disableGpu=true 把 GPU 链路排除在外，只观察 CPU 链路的行为。
	if reading := r.readFallback(true); reading.cpuTemp != 0 {
		t.Fatalf("readFallback().cpuTemp = %d, want 0", reading.cpuTemp)
	}
	if len(spawned) != 0 {
		t.Fatalf("CPU 降级读取不应创建任何外部进程，实际调用: %v", spawned)
	}
}

// CPU 专属故障说明必须原样透传给上层，界面才能给出重装 PawnIO 的指引。
func TestCopyBridgeTemperatureMetadataCarriesCPUTempError(t *testing.T) {
	const wantErr = "未读取到 CPU 温度；请重装 PawnIO"

	var temp types.TemperatureData
	copyBridgeTemperatureMetadata(&temp, types.BridgeTemperatureData{
		GpuTemp:      55,
		CpuTempError: wantErr,
	}, types.TemperatureSelection{TempSource: types.TempSourceMax})

	if temp.CPUTempError != wantErr {
		t.Fatalf("CPUTempError = %q, want %q", temp.CPUTempError, wantErr)
	}
}

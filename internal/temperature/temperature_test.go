package temperature

import (
	"context"
	"testing"
	"time"
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

func TestReadWindowsCPUTempUsesTimeout(t *testing.T) {
	oldExec := execHelperCommand
	defer func() {
		execHelperCommand = oldExec
	}()

	called := false
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		called = true
		if timeout != helperCommandTimeout {
			t.Fatalf("unexpected timeout: %s", timeout)
		}
		if name != "wmic" {
			t.Fatalf("unexpected command: %s", name)
		}
		return nil, context.DeadlineExceeded
	}

	r := NewReader(nil, testLogger{})
	if got := r.readWindowsCPUTemp(); got != 0 {
		t.Fatalf("readWindowsCPUTemp() = %d, want 0 on timeout", got)
	}
	if !called {
		t.Fatal("readWindowsCPUTemp() did not invoke helper command")
	}
}

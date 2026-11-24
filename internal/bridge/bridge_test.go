package bridge

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (l testLogger) Info(format string, v ...any)  {}
func (l testLogger) Error(format string, v ...any) {}
func (l testLogger) Warn(format string, v ...any)  {}
func (l testLogger) Debug(format string, v ...any) {}
func (l testLogger) Close()                        {}
func (l testLogger) CleanOldLogs()                 {}
func (l testLogger) SetDebugMode(enabled bool)     {}
func (l testLogger) GetLogDir() string             { return "" }

func TestNewManager(t *testing.T) {
	m := NewManager(testLogger{})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestInitialState(t *testing.T) {
	m := NewManager(testLogger{})
	status := m.GetStatus()
	if state, ok := status["state"]; !ok {
		t.Error("GetStatus should include 'state'")
	} else if state != BridgeStateNotStarted {
		t.Errorf("initial state = %q, want %q", state, BridgeStateNotStarted)
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager(testLogger{})
	status := m.GetStatus()
	if _, ok := status["state"]; !ok {
		t.Error("GetStatus should include 'state'")
	}
}

func TestStop_NoOp(t *testing.T) {
	m := NewManager(testLogger{})
	m.Stop()
	m.Stop()
}

func TestGetTemperature_NoBridge(t *testing.T) {
	m := NewManager(testLogger{})
	temp := m.GetTemperature(types.TemperatureSelection{})
	_ = temp
}

func TestBridgeStateConstants(t *testing.T) {
	states := []string{
		BridgeStateNotStarted, BridgeStateStarting, BridgeStateRunning,
		BridgeStateAttached, BridgeStateDegraded, BridgeStateStopping,
		BridgeStateStopped, BridgeStateFailed,
	}
	seen := make(map[string]bool)
	for _, s := range states {
		if s == "" {
			t.Error("BridgeState constant should not be empty")
		}
		if seen[s] {
			t.Errorf("Duplicate bridge state: %q", s)
		}
		seen[s] = true
	}
}

// 用户诊断包里的真实崩溃输出：异常消息是 GBK 乱码，但堆栈帧名是 ASCII。
func TestForwardBridgeStderr_DetectsIntelGclCrash(t *testing.T) {
	m := NewManager(testLogger{})
	crash := strings.Join([]string{
		"[init] LibreHardwareMonitor 初始化尝试 1/3",
		"���:  System.AccessViolationException: ��",
		"   �� LibreHardwareMonitor.Interop.IntelGcl.ctlEnumerateDevices(ctl_api_handle_t hAPIHandle, UInt32& pCount, ctl_device_adapter_handle_t[] phDevices)",
		"   �� LibreHardwareMonitor.Hardware.Gpu.IntelGpuGroup..ctor(List`1 intelCpus, ISettings settings)",
	}, "\n")

	var saw atomic.Bool
	m.forwardBridgeStderr(strings.NewReader(crash), &saw)
	if !saw.Load() {
		t.Fatal("Intel GCL 崩溃堆栈未被识别")
	}

	saw.Store(false)
	m.forwardBridgeStderr(strings.NewReader("[init] 已按启动参数跳过 Intel GCL（ControlLib.dll）\nsome other error"), &saw)
	if saw.Load() {
		t.Fatal("非崩溃输出被误判为 Intel GCL 崩溃")
	}
}

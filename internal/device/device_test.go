package device

import (
	"testing"
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
	if m.IsConnected() {
		t.Error("New manager should not be connected")
	}
}

func TestVendorIDAndProductIDs(t *testing.T) {
	if VendorID != 0x37D7 {
		t.Errorf("VendorID = 0x%04X, want 0x37D7", VendorID)
	}
	productIDs := map[uint16]string{
		ProductIDBS2:    "BS2",
		ProductIDBS2PRO: "BS2PRO",
		ProductIDBS3:    "BS3",
		ProductIDBS3PRO: "BS3PRO",
	}
	for pid, name := range productIDs {
		if pid == 0 {
			t.Errorf("%s ProductID is 0", name)
		}
	}
}

func TestSetCallbacks(t *testing.T) {
	m := NewManager(testLogger{})
	m.SetCallbacks(nil, nil)
}

func TestGetCurrentFanData(t *testing.T) {
	m := NewManager(testLogger{})
	data := m.GetCurrentFanData()
	_ = data
}

func TestIsBS1(t *testing.T) {
	m := NewManager(testLogger{})
	if m.IsBS1() {
		t.Log("Warning: IsBS1 returned true on uninitialized manager")
	}
}

func TestGetModelName(t *testing.T) {
	m := NewManager(testLogger{})
	name := m.GetModelName()
	if name != "" {
		t.Logf("Model name on uninitialized manager = %q", name)
	}
}

func TestGetDeviceType(t *testing.T) {
	m := NewManager(testLogger{})
	dt := m.GetDeviceType()
	if dt != "" {
		t.Logf("Device type on uninitialized manager = %q", dt)
	}
}

func TestDebugCommandPresets(t *testing.T) {
	presets := DebugCommandPresets()
	if len(presets) == 0 {
		t.Error("DebugCommandPresets should return non-empty list")
	}
}

func TestResetRealtimeControlStateLocked(t *testing.T) {
	m := NewManager(testLogger{})
	m.lastCommandedRPM = 1800
	m.hasCommandedRPM = true
	m.realtimeMode = true
	m.consecutiveRealtimeWriteErrors = maxConsecutiveRealtimeWriteErrors
	m.realtimeWriteRecoveryScheduled = true

	m.resetRealtimeControlStateLocked()

	if m.lastCommandedRPM != 0 || m.hasCommandedRPM || m.realtimeMode ||
		m.consecutiveRealtimeWriteErrors != 0 || m.realtimeWriteRecoveryScheduled {
		t.Fatal("resetRealtimeControlStateLocked did not clear the complete realtime control state")
	}
}

func TestDebugCaptureSkipsBackgroundFrameWork(t *testing.T) {
	m := NewManager(testLogger{})
	if id := m.recordDebugFrame("rx", "hid", []byte{0x00}); id != 0 {
		t.Fatalf("recordDebugFrame() = %d with capture disabled, want 0", id)
	}
	if frames := m.GetDebugFrames(); len(frames) != 0 {
		t.Fatalf("captured %d frames with capture disabled", len(frames))
	}

	m.SetDebugCapture(true)
	if id := m.recordDebugFrame("rx", "hid", []byte{0x00}); id == 0 {
		t.Fatal("recordDebugFrame did not capture a frame after enabling capture")
	}
	if frames := m.GetDebugFrames(); len(frames) != 1 {
		t.Fatalf("captured %d frames with capture enabled, want 1", len(frames))
	}
}

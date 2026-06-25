package bridge

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (l testLogger) Info(format string, v ...any)    {}
func (l testLogger) Error(format string, v ...any)   {}
func (l testLogger) Warn(format string, v ...any)    {}
func (l testLogger) Debug(format string, v ...any)   {}
func (l testLogger) Close()                          {}
func (l testLogger) CleanOldLogs()                   {}
func (l testLogger) SetDebugMode(enabled bool)       {}
func (l testLogger) GetLogDir() string               { return "" }

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

package tray

import (
	"testing"
	"time"
)

type testLogger struct{}

func (l testLogger) Info(string, ...any)  {}
func (l testLogger) Error(string, ...any) {}
func (l testLogger) Warn(string, ...any)  {}
func (l testLogger) Debug(string, ...any) {}
func (l testLogger) Close()               {}
func (l testLogger) CleanOldLogs()        {}
func (l testLogger) SetDebugMode(bool)    {}
func (l testLogger) GetLogDir() string    { return "" }

func TestWaitForShellReady_ReturnsTrue(t *testing.T) {
	if !waitForShellReady(nil, time.Second) {
		t.Fatal("waitForShellReady should always return true on non-Windows")
	}
}

func TestWaitForTraySettle_NoPanic(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("waitForTraySettle panicked: %v", r)
			}
		}()
		waitForTraySettle(make(chan struct{}), time.Millisecond, time.Second)
	}()
}

func TestNewManager_CreatesInstance(t *testing.T) {
	iconData := []byte{0x89, 0x50, 0x4e, 0x47}
	m := NewManager(testLogger{}, iconData)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.logger == nil {
		t.Fatal("logger should not be nil")
	}
	if m.done == nil {
		t.Fatal("done channel should not be nil")
	}
	if m.uiQueue == nil {
		t.Fatal("uiQueue should not be nil")
	}
	if len(m.iconData) != len(iconData) {
		t.Fatal("iconData length mismatch")
	}
	if m.curveMenuItems == nil {
		t.Fatal("curveMenuItems should not be nil")
	}
}

func TestManager_IsReady_NotInitially(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if m.IsReady() {
		t.Fatal("IsReady should return false before Init")
	}
}

func TestManager_IsInitialized_NotInitially(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if m.IsInitialized() {
		t.Fatal("IsInitialized should return false before Init")
	}
}

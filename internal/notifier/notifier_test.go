package notifier

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

type testLogger2 struct{}

func (l testLogger2) Info(format string, v ...any)    {}
func (l testLogger2) Error(format string, v ...any)   {}
func (l testLogger2) Warn(format string, v ...any)    {}
func (l testLogger2) Debug(format string, v ...any)   {}
func (l testLogger2) Close()                          {}
func (l testLogger2) CleanOldLogs()                   {}
func (l testLogger2) SetDebugMode(enabled bool)       {}
func (l testLogger2) GetLogDir() string               { return "" }

func TestNewManager(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNotify_NoCrash(t *testing.T) {
	m := NewManager(testLogger2{}, nil)
	m.Notify("Test Title", "Test Message")
}

func TestNewManager_WithIcon(t *testing.T) {
	m := NewManager(testLogger{}, []byte{0x00, 0x01, 0x02})
	if m == nil {
		t.Fatal("NewManager with icon returned nil")
	}
}

var _ types.Logger = testLogger{}

package types

import (
	"testing"
)

type testLogger struct {
	infoCalled    int
	errorCalled   int
	warnCalled    int
	debugCalled   int
	closeCalled   int
	cleanCalled   int
	debugMode     bool
	infoFormat    string
	infoArgs      []any
	errorFormat   string
	errorArgs     []any
	warnFormat    string
	warnArgs      []any
	debugFormat   string
	debugArgs     []any
}

func (l *testLogger) Info(format string, v ...any)  { l.infoCalled++; l.infoFormat = format; l.infoArgs = v }
func (l *testLogger) Error(format string, v ...any) { l.errorCalled++; l.errorFormat = format; l.errorArgs = v }
func (l *testLogger) Warn(format string, v ...any)  { l.warnCalled++; l.warnFormat = format; l.warnArgs = v }
func (l *testLogger) Debug(format string, v ...any) { l.debugCalled++; l.debugFormat = format; l.debugArgs = v }
func (l *testLogger) Close()                         { l.closeCalled++ }
func (l *testLogger) CleanOldLogs()                  { l.cleanCalled++ }
func (l *testLogger) SetDebugMode(enabled bool)      { l.debugMode = enabled }
func (l *testLogger) GetLogDir() string              { return "/tmp/test-logs" }

func TestLoggerInterfaceSatisfied(t *testing.T) {
	var _ Logger = &testLogger{}
}

func TestLoggerMethodExistence(t *testing.T) {
	l := &testLogger{}

	l.Info("info %d", 1)
	if l.infoCalled != 1 {
		t.Fatalf("Info should be called once, got %d", l.infoCalled)
	}
	if l.infoFormat != "info %d" {
		t.Fatalf("Info format mismatch: %q", l.infoFormat)
	}

	l.Error("error %s", "msg")
	if l.errorCalled != 1 {
		t.Fatalf("Error should be called once, got %d", l.errorCalled)
	}

	l.Warn("warn %v", true)
	if l.warnCalled != 1 {
		t.Fatalf("Warn should be called once, got %d", l.warnCalled)
	}

	l.Debug("debug")
	if l.debugCalled != 1 {
		t.Fatalf("Debug should be called once, got %d", l.debugCalled)
	}
}

func TestLoggerClose(t *testing.T) {
	l := &testLogger{}
	l.Close()
	if l.closeCalled != 1 {
		t.Fatalf("Close should be called once, got %d", l.closeCalled)
	}
}

func TestLoggerCleanOldLogs(t *testing.T) {
	l := &testLogger{}
	l.CleanOldLogs()
	if l.cleanCalled != 1 {
		t.Fatalf("CleanOldLogs should be called once, got %d", l.cleanCalled)
	}
}

func TestLoggerSetDebugMode(t *testing.T) {
	l := &testLogger{}
	l.SetDebugMode(true)
	if !l.debugMode {
		t.Fatal("SetDebugMode(true) should set debugMode to true")
	}
	l.SetDebugMode(false)
	if l.debugMode {
		t.Fatal("SetDebugMode(false) should set debugMode to false")
	}
}

func TestLoggerGetLogDir(t *testing.T) {
	l := &testLogger{}
	if got := l.GetLogDir(); got != "/tmp/test-logs" {
		t.Fatalf("GetLogDir() = %q, want /tmp/test-logs", got)
	}
}

func TestLoggerInfoFormatPassthrough(t *testing.T) {
	l := &testLogger{}
	l.Info("hello %s %d", "world", 42)
	if l.infoFormat != "hello %s %d" {
		t.Fatalf("format mismatch: %q", l.infoFormat)
	}
	if len(l.infoArgs) != 2 {
		t.Fatalf("args count mismatch: %d", len(l.infoArgs))
	}
}

func TestLoggerMultipleCalls(t *testing.T) {
	l := &testLogger{}
	l.Info("first")
	l.Info("second")
	l.Error("third")
	if l.infoCalled != 2 {
		t.Fatalf("Info should be called 2 times, got %d", l.infoCalled)
	}
	if l.errorCalled != 1 {
		t.Fatalf("Error should be called 1 time, got %d", l.errorCalled)
	}
}

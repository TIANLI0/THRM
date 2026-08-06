package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestSetDebugModeUpdatesSharedLevel(t *testing.T) {
	logger := &CustomLogger{atom: newAtomicLevel(false)}
	if logger.atom.Enabled(zapcore.DebugLevel) {
		t.Fatal("debug level enabled by default")
	}

	logger.SetDebugMode(true)
	if !logger.GetDebugMode() || !logger.atom.Enabled(zapcore.DebugLevel) {
		t.Fatal("SetDebugMode(true) did not enable debug logging")
	}

	logger.SetDebugMode(false)
	if logger.GetDebugMode() || logger.atom.Enabled(zapcore.DebugLevel) {
		t.Fatal("SetDebugMode(false) did not restore info logging")
	}
}

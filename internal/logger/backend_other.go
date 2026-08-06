//go:build !linux && !windows

package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newPlatformBackend(atom zap.AtomicLevel, _, _ string, _ bool) (platformBackend, error) {
	return platformBackend{
		cores: []zapcore.Core{consoleCore(atom, zapcore.AddSync(os.Stderr), false)},
	}, nil
}

func defaultLogDir(_ string) string {
	return ""
}

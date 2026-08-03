//go:build windows

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func newPlatformBackend(atom zap.AtomicLevel, installDir, _ string, persistent bool) (platformBackend, error) {
	if !persistent {
		return platformBackend{
			cores: []zapcore.Core{consoleCore(atom, zapcore.AddSync(os.Stderr), false)},
		}, nil
	}

	logDir := defaultLogDir(installDir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return platformBackend{}, backendError("创建日志目录失败", err)
	}

	appLogRotate := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02"))),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	}
	debugLogRotate := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, fmt.Sprintf("debug_%s.log", time.Now().Format("2006-01-02"))),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	}

	fileEncoder := zapcore.NewJSONEncoder(logEncoderConfig())
	appCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(appLogRotate),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapcore.InfoLevel
		}),
	)
	// Debug 文件只收 Debug 级日志，避免 Info 及以上日志重复落盘。
	debugCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(debugLogRotate),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return atom.Enabled(level) && level < zapcore.InfoLevel
		}),
	)
	console := consoleCore(atom, zapcore.AddSync(os.Stdout), true)

	return platformBackend{
		cores:   []zapcore.Core{appCore, debugCore, console},
		logDir:  logDir,
		closers: []io.Closer{appLogRotate, debugLogRotate},
	}, nil
}

func defaultLogDir(installDir string) string {
	return filepath.Join(installDir, "logs")
}

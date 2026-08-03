// Package logger 提供基于 zap 的跨平台日志记录功能。
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// GUIIdentifier 与 CoreIdentifier 会成为 Linux journal 中的
	// SYSLOG_IDENTIFIER，便于用 journalctl 精确筛选两个进程的日志。
	GUIIdentifier  = "thrm"
	CoreIdentifier = "thrm-core"
)

type platformBackend struct {
	cores   []zapcore.Core
	logDir  string
	closers []io.Closer
}

// CustomLogger 是业务层使用的日志记录器封装。
type CustomLogger struct {
	logger    *zap.Logger
	sugar     *zap.SugaredLogger
	debugMode bool
	logDir    string
	atom      zap.AtomicLevel
	closers   []io.Closer
}

// NewCustomLogger 创建核心服务日志记录器。
//
// Windows 继续写入安装目录下的轮转日志；Linux 使用 systemd journal，
// journal 不可用时自动回退到标准错误，不会尝试写入需要 root 的 /var/log。
func NewCustomLogger(debugMode bool, installDir string) (*CustomLogger, error) {
	atom := newAtomicLevel(debugMode)
	backend, err := newPlatformBackend(atom, installDir, CoreIdentifier, true)
	if err != nil {
		return nil, err
	}

	logger := zap.New(zapcore.NewTee(backend.cores...), zap.AddCaller(), zap.AddCallerSkip(1))
	return &CustomLogger{
		logger:    logger,
		sugar:     logger.Sugar(),
		debugMode: debugMode,
		logDir:    backend.logDir,
		atom:      atom,
		closers:   backend.closers,
	}, nil
}

// NewProcessLogger 创建不带业务封装的进程日志记录器，供 GUI 等入口使用。
// Linux 与核心服务共用 journal 策略，其他平台使用本机的控制台后端。
func NewProcessLogger(identifier string) *zap.Logger {
	atom := newAtomicLevel(false)
	backend, err := newPlatformBackend(atom, "", identifier, false)
	if err != nil || len(backend.cores) == 0 {
		fallback, fallbackErr := zap.NewProduction()
		if fallbackErr == nil {
			return fallback
		}
		return zap.NewNop()
	}
	return zap.New(zapcore.NewTee(backend.cores...), zap.AddCaller())
}

func newAtomicLevel(debugMode bool) zap.AtomicLevel {
	atom := zap.NewAtomicLevel()
	if debugMode {
		atom.SetLevel(zapcore.DebugLevel)
	} else {
		atom.SetLevel(zapcore.InfoLevel)
	}
	return atom
}

func logEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func consoleCore(atom zap.AtomicLevel, output zapcore.WriteSyncer, colored bool) zapcore.Core {
	config := logEncoderConfig()
	if colored {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return zapcore.NewCore(zapcore.NewConsoleEncoder(config), output, atom)
}

// Info 记录信息日志。
func (l *CustomLogger) Info(format string, v ...any) {
	l.sugar.Infof(format, v...)
}

// Error 记录错误日志。
func (l *CustomLogger) Error(format string, v ...any) {
	l.sugar.Errorf(format, v...)
}

// Debug 记录调试日志。
func (l *CustomLogger) Debug(format string, v ...any) {
	l.sugar.Debugf(format, v...)
}

// Warn 记录警告日志。
func (l *CustomLogger) Warn(format string, v ...any) {
	l.sugar.Warnf(format, v...)
}

// Fatal 记录致命错误日志并退出。
func (l *CustomLogger) Fatal(format string, v ...any) {
	l.sugar.Fatalf(format, v...)
}

// Close 刷新日志并关闭后端连接或轮转文件。
func (l *CustomLogger) Close() {
	if l.logger != nil {
		_ = l.logger.Sync()
	}
	for _, closer := range l.closers {
		if closer != nil {
			_ = closer.Close()
		}
	}
}

// CleanOldLogs 清理旧日志文件（保留 7 天）。journal 的保留策略由 journald 管理，
// Linux 上 logDir 为空，因此这里自然成为空操作。
func (l *CustomLogger) CleanOldLogs() {
	if l.logDir == "" {
		return
	}
	files, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -7)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".log") || strings.HasSuffix(file.Name(), ".log.gz") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				_ = os.Remove(filepath.Join(l.logDir, file.Name()))
			}
		}
	}
}

// SetDebugMode 设置调试模式。
func (l *CustomLogger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
	if enabled {
		l.atom.SetLevel(zapcore.DebugLevel)
	} else {
		l.atom.SetLevel(zapcore.InfoLevel)
	}
}

// GetLogDir 获取文件日志目录。使用 journal 的平台返回空字符串。
func (l *CustomLogger) GetLogDir() string {
	return l.logDir
}

// GetDebugMode 获取调试模式状态。
func (l *CustomLogger) GetDebugMode() bool {
	return l.debugMode
}

// GetZapLogger 获取底层 zap logger。
func (l *CustomLogger) GetZapLogger() *zap.Logger {
	return l.logger
}

// GetSugar 获取 sugar logger。
func (l *CustomLogger) GetSugar() *zap.SugaredLogger {
	return l.sugar
}

func backendError(action string, err error) error {
	return fmt.Errorf("%s: %w", action, err)
}

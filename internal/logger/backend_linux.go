//go:build linux

package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func newPlatformBackend(atom zap.AtomicLevel, _, identifier string, _ bool) (platformBackend, error) {
	sink, err := openJournalSink(identifier)
	if err != nil {
		// 桌面入口通常没有可见 stderr；journal 不可用时再落到 XDG state。
		logDir := defaultLogDir("")
		if logDir != "" && os.MkdirAll(logDir, 0o755) == nil {
			file := &lumberjack.Logger{
				Filename:   filepath.Join(logDir, sanitizeJournalIdentifier(identifier)+".log"),
				MaxSize:    10,
				MaxBackups: 7,
				MaxAge:     7,
				Compress:   true,
			}
			return platformBackend{
				cores: []zapcore.Core{
					zapcore.NewCore(zapcore.NewJSONEncoder(logEncoderConfig()), zapcore.AddSync(file), atom),
					consoleCore(atom, zapcore.AddSync(os.Stderr), false),
				},
				logDir:  logDir,
				closers: []io.Closer{file},
			}, nil
		}

		return platformBackend{
			cores: []zapcore.Core{consoleCore(atom, zapcore.AddSync(os.Stderr), false)},
		}, nil
	}

	return platformBackend{
		cores:   []zapcore.Core{newJournalCore(atom, sink)},
		closers: []io.Closer{sink},
	}, nil
}

func defaultLogDir(_ string) string {
	homeDir, _ := os.UserHomeDir()
	if stateDir := appmeta.UserStateDir(homeDir); stateDir != "" {
		return filepath.Join(stateDir, "logs")
	}
	return ""
}

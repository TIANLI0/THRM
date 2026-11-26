//go:build darwin

package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// newPlatformBackend 在 macOS 上把日志写进 ~/Library/Logs/THRM/<identifier>.log，
// “控制台”应用会自动列出这个目录。双击 .app 或被 launchd 拉起时没有可见的
// stderr，所以只有 stderr 是终端时才额外输出到控制台。
func newPlatformBackend(atom zap.AtomicLevel, _, identifier string, _ bool) (platformBackend, error) {
	var cores []zapcore.Core
	var closers []io.Closer

	logDir := defaultLogDir("")
	if logDir != "" && os.MkdirAll(logDir, 0o755) == nil {
		file := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, logFileBaseName(identifier)+".log"),
			MaxSize:    10,
			MaxBackups: 7,
			MaxAge:     7,
			Compress:   true,
		}
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(logEncoderConfig()), zapcore.AddSync(file), atom))
		closers = append(closers, file)
	} else {
		logDir = ""
	}

	if len(cores) == 0 || isTerminal(os.Stderr) {
		cores = append(cores, consoleCore(atom, zapcore.AddSync(os.Stderr), false))
	}

	return platformBackend{cores: cores, logDir: logDir, closers: closers}, nil
}

func defaultLogDir(_ string) string {
	homeDir, _ := os.UserHomeDir()
	return appmeta.UserLogDir(homeDir)
}

func logFileBaseName(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return GUIIdentifier
	}
	return strings.Map(func(r rune) rune {
		if r == '/' || r == 0x5C || r == ':' {
			return '-'
		}
		return r
	}, identifier)
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

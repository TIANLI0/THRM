//go:build darwin

package coreapp

import (
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

// macOS 崩溃详情与普通日志放在一起：~/Library/Logs/THRM/crash。
func platformCrashLogDir() string {
	homeDir, _ := os.UserHomeDir()
	logDir := appmeta.UserLogDir(homeDir)
	if logDir == "" {
		return ""
	}
	return filepath.Join(logDir, "crash")
}

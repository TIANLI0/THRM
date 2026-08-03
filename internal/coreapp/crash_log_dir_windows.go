//go:build windows

package coreapp

import (
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/config"
)

func platformCrashLogDir() string {
	installDir := config.GetInstallDir()
	if installDir == "" {
		return "logs"
	}
	return filepath.Join(installDir, "logs")
}

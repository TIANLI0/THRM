//go:build windows

package config

import "path/filepath"

func platformInstallConfigWriteDir(installDir string) string {
	if installDir == "" {
		return ""
	}
	return filepath.Join(installDir, "config")
}

func shouldMigrateInstallConfig() bool {
	return false
}

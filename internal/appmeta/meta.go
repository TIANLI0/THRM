// Package appmeta provides shared application metadata constants and helpers.
package appmeta

import (
	"os"
	"path/filepath"
)

const (
	AppName              = "THRM"
	LegacyAppName        = "BS2PRO Controller"
	CoreName             = "THRM Core"
	IPCPipeName          = "THRM-IPC"
	LegacyIPCPipeName    = "BS2PRO-Controller-IPC"
	BridgePipeName       = "THRM_TempBridge"
	LegacyBridgePipeName = "BS2PRO_TempBridge"
	ConfigDirName        = ".thrm"
	LegacyConfigDirName  = ".bs2pro-controller"
	XDGDirName           = "thrm"
	NotificationCacheDir = "THRM"
	LegacyNotifyCacheDir = "BS2PRO-Controller"
	ProtocolVersion      = "3.0"
	RepositoryURL        = "https://github.com/TIANLI0/THRM"
	LatestReleaseURL     = RepositoryURL + "/releases/latest"
)

func IPCPipeCandidates() []string {
	return []string{IPCPipeName, LegacyIPCPipeName}
}

func BridgePipeCandidates() []string {
	return []string{BridgePipeName, LegacyBridgePipeName}
}

func FirstExistingPath(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func LegacyUserConfigDir(homeDir string) string {
	return filepath.Join(homeDir, LegacyConfigDirName)
}

// LegacyUserConfigDirs returns old per-user configuration directories in
// migration order. The current platform configuration directory is excluded.
func LegacyUserConfigDirs(homeDir string) []string {
	if homeDir == "" {
		return nil
	}

	currentDir := filepath.Clean(UserConfigDir(homeDir))
	candidates := []string{
		filepath.Join(homeDir, ConfigDirName),
		LegacyUserConfigDir(homeDir),
	}
	legacyDirs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == currentDir {
			continue
		}
		duplicate := false
		for _, existing := range legacyDirs {
			if existing == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			legacyDirs = append(legacyDirs, candidate)
		}
	}
	return legacyDirs
}

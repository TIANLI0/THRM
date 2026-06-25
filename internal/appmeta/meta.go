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

func UserConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ConfigDirName)
}

func LegacyUserConfigDir(homeDir string) string {
	return filepath.Join(homeDir, LegacyConfigDirName)
}

package appmeta

import (
	"os"
	"path/filepath"
)

const (
	AppName                = "THRM"
	LegacyAppName          = "BS2PRO Controller"
	ExecutableName         = "THRM.exe"
	LegacyExecutableName   = "BS2PRO-Controller.exe"
	CoreName               = "THRM Core"
	CoreExecutableName     = "THRM Core.exe"
	LegacyCoreExecutable   = "BS2PRO-Core.exe"
	BridgeName             = "THRM TempBridge"
	BridgeExecutableName   = "THRM TempBridge.exe"
	LegacyBridgeExecutable = "TempBridge.exe"
	IPCPipeName            = "THRM-IPC"
	LegacyIPCPipeName      = "BS2PRO-Controller-IPC"
	BridgePipeName         = "THRM_TempBridge"
	LegacyBridgePipeName   = "BS2PRO_TempBridge"
	BridgeMutexName        = `Global\THRM_TempBridge_Singleton`
	LegacyBridgeMutexName  = `Global\BS2PRO_TempBridge_Singleton`
	PawnIOInstallerName    = "PawnIO_setup.exe"
	ConfigDirName          = ".thrm"
	LegacyConfigDirName    = ".bs2pro-controller"
	NotificationCacheDir   = "THRM"
	LegacyNotifyCacheDir   = "BS2PRO-Controller"
	ProtocolVersion        = "3.0"
	RepositoryURL          = "https://github.com/TIANLI0/BS2PRO-Controller"
	LatestReleaseURL       = RepositoryURL + "/releases/latest"
)

func CoreExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, CoreExecutableName),
		filepath.Join(baseDir, LegacyCoreExecutable),
	}
}

func GUIExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, ExecutableName),
		filepath.Join(baseDir, LegacyExecutableName),
	}
}

func BridgeExecutableCandidates(baseDir string) []string {
	return []string{
		filepath.Join(baseDir, "bridge", BridgeExecutableName),
		filepath.Join(baseDir, "..", "bridge", BridgeExecutableName),
		filepath.Join(baseDir, BridgeExecutableName),
		filepath.Join(baseDir, "bridge", LegacyBridgeExecutable),
		filepath.Join(baseDir, "..", "bridge", LegacyBridgeExecutable),
		filepath.Join(baseDir, LegacyBridgeExecutable),
	}
}

func PawnIOInstallerPath(baseDir string) string {
	return filepath.Join(baseDir, "drivers", "PawnIO", PawnIOInstallerName)
}

func PawnIOInstallerCandidates(baseDir string) []string {
	return []string{
		PawnIOInstallerPath(baseDir),
		filepath.Join(baseDir, PawnIOInstallerName),
	}
}

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

//go:build windows

package appmeta

import (
	"path/filepath"
)

const (
	ExecutableName         = "THRM.exe"
	LegacyExecutableName   = "BS2PRO-Controller.exe"
	CoreExecutableName     = "THRM Core.exe"
	LegacyCoreExecutable   = "BS2PRO-Core.exe"
	BridgeName             = "THRM TempBridge"
	BridgeExecutableName   = "THRM TempBridge.exe"
	LegacyBridgeExecutable = "TempBridge.exe"
	BridgeMutexName        = `Global\THRM_TempBridge_Singleton`
	LegacyBridgeMutexName  = `Global\BS2PRO_TempBridge_Singleton`
	PawnIOInstallerName    = "PawnIO_setup.exe"
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

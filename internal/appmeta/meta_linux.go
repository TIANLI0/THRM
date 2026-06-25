//go:build linux

package appmeta

import (
	"os"
	"path/filepath"
)

const (
	ExecutableName       = "thrm"
	LegacyExecutableName = "bs2pro-controller"
	CoreExecutableName   = "thrm-core"
	LegacyCoreExecutable = "bs2pro-core"
	BridgeName           = ""
	BridgeExecutableName = ""
	PawnIOInstallerName  = ""
)

func CoreExecutableCandidates(baseDir string) []string {
	candidates := []string{
		filepath.Join(baseDir, CoreExecutableName),
		filepath.Join(baseDir, LegacyCoreExecutable),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, CoreExecutableName),
			filepath.Join(exeDir, LegacyCoreExecutable),
			filepath.Join(exeDir, "..", "core", CoreExecutableName),
			filepath.Join(exeDir, "..", "core", LegacyCoreExecutable),
		)
	}
	return candidates
}

func GUIExecutableCandidates(baseDir string) []string {
	candidates := []string{
		filepath.Join(baseDir, ExecutableName),
		filepath.Join(baseDir, LegacyExecutableName),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, ExecutableName),
			filepath.Join(exeDir, LegacyExecutableName),
			filepath.Join(exeDir, "..", ExecutableName),
			filepath.Join(exeDir, "..", LegacyExecutableName),
		)
	}
	return candidates
}

func BridgeExecutableCandidates(baseDir string) []string {
	return nil
}

func PawnIOInstallerPath(baseDir string) string {
	return ""
}

func PawnIOInstallerCandidates(baseDir string) []string {
	return nil
}

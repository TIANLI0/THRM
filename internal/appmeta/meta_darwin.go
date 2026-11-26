//go:build darwin

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

// UserConfigDir follows the macOS convention: ~/Library/Application Support/<App>.
func UserConfigDir(homeDir string) string {
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, "Library", "Application Support", ConfigDirName)
}

// UserStateDir is for persistent mutable data which is not configuration,
// such as temperature history.
func UserStateDir(homeDir string) string {
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, "Library", "Application Support", ConfigDirName, "state")
}

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

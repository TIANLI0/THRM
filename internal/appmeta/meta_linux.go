//go:build linux

package appmeta

import (
	"os"
	"path/filepath"
	"strings"
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

// UserConfigDir follows the XDG Base Directory specification. Relative XDG
// paths are invalid and therefore ignored.
func UserConfigDir(homeDir string) string {
	return xdgUserDir("XDG_CONFIG_HOME", homeDir, ".config")
}

// UserStateDir is for persistent mutable data which is not configuration,
// such as temperature history.
func UserStateDir(homeDir string) string {
	return xdgUserDir("XDG_STATE_HOME", homeDir, ".local", "state")
}

func xdgUserDir(environmentName, homeDir string, fallbackParts ...string) string {
	if configured := strings.TrimSpace(os.Getenv(environmentName)); filepath.IsAbs(configured) {
		return filepath.Join(configured, XDGDirName)
	}
	if homeDir == "" {
		return ""
	}
	parts := append([]string{homeDir}, fallbackParts...)
	parts = append(parts, XDGDirName)
	return filepath.Join(parts...)
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

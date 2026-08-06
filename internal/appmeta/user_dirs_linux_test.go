//go:build linux

package appmeta

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestLinuxUserDirsHonorAbsoluteXDGPaths(t *testing.T) {
	configHome := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)

	if got, want := UserConfigDir("/ignored"), filepath.Join(configHome, XDGDirName); got != want {
		t.Fatalf("UserConfigDir = %q, want %q", got, want)
	}
	if got, want := UserStateDir("/ignored"), filepath.Join(stateHome, XDGDirName); got != want {
		t.Fatalf("UserStateDir = %q, want %q", got, want)
	}
}

func TestLinuxUserDirsIgnoreRelativeXDGPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative-config")
	t.Setenv("XDG_STATE_HOME", "relative-state")
	home := filepath.Join(string(filepath.Separator), "home", "testuser")

	if got, want := UserConfigDir(home), filepath.Join(home, ".config", XDGDirName); got != want {
		t.Fatalf("UserConfigDir = %q, want %q", got, want)
	}
	if got, want := UserStateDir(home), filepath.Join(home, ".local", "state", XDGDirName); got != want {
		t.Fatalf("UserStateDir = %q, want %q", got, want)
	}
}

func TestLinuxLegacyConfigDirsIncludePreXDGLocations(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := filepath.Join(string(filepath.Separator), "home", "testuser")
	dirs := LegacyUserConfigDirs(home)

	for _, want := range []string{
		filepath.Join(home, ConfigDirName),
		filepath.Join(home, LegacyConfigDirName),
	} {
		if !slices.Contains(dirs, want) {
			t.Fatalf("LegacyUserConfigDirs() = %v, missing %q", dirs, want)
		}
	}
}

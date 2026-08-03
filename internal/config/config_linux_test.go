//go:build linux

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestLinuxSaveDoesNotFallBackToInstallDir(t *testing.T) {
	tmpDir := t.TempDir()
	installDir := filepath.Join(tmpDir, "install")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatal(err)
	}

	blockedConfigHome := filepath.Join(tmpDir, "not-a-directory")
	if err := os.WriteFile(blockedConfigHome, []byte("blocked"), 0644); err != nil {
		t.Fatal(err)
	}
	isolateUserDirs(t, tmpDir)
	t.Setenv("XDG_CONFIG_HOME", blockedConfigHome)

	m := NewManager(installDir, testLogger{})
	m.Set(types.GetDefaultConfig(false))
	if err := m.Save(); err == nil {
		t.Fatal("Save() succeeded even though the user config directory is unavailable")
	}
	if _, err := os.Stat(filepath.Join(installDir, "config")); !os.IsNotExist(err) {
		t.Fatalf("Linux config fallback wrote beside the executable: %v", err)
	}
}

func TestLinuxMigratesDotThrmConfigToXDG(t *testing.T) {
	tmpDir := t.TempDir()
	isolateUserDirs(t, tmpDir)
	legacyDir := filepath.Join(tmpDir, ".thrm")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyConfig := []byte(`{"autoControl":true,"fanCurve":[{"temperature":30,"rpm":800},{"temperature":70,"rpm":3000}]}`)
	if err := os.WriteFile(filepath.Join(legacyDir, "config.json"), legacyConfig, 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(filepath.Join(tmpDir, "install"), testLogger{})
	loaded := m.Load(false)
	if !loaded.AutoControl {
		t.Fatal("did not load the pre-XDG .thrm configuration")
	}
	if _, err := os.Stat(filepath.Join(m.GetDefaultConfigDir(), "config.json")); err != nil {
		t.Fatalf("legacy configuration was not migrated to XDG_CONFIG_HOME: %v", err)
	}
}

//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsPortableConfigStaysInInstallDir(t *testing.T) {
	tmpDir := t.TempDir()
	isolateUserDirs(t, filepath.Join(tmpDir, "home"))
	installDir := filepath.Join(tmpDir, "portable")
	installConfigDir := filepath.Join(installDir, "config")
	if err := os.MkdirAll(installConfigDir, 0755); err != nil {
		t.Fatal(err)
	}
	installConfigPath := filepath.Join(installConfigDir, "config.json")
	configData := []byte(`{"autoControl":true,"fanCurve":[{"temperature":30,"rpm":800},{"temperature":70,"rpm":3000}]}`)
	if err := os.WriteFile(installConfigPath, configData, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(installDir, testLogger{})
	loaded := manager.Load(false)
	if !loaded.AutoControl {
		t.Fatal("did not load the portable configuration")
	}
	if loaded.ConfigPath != installConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", loaded.ConfigPath, installConfigPath)
	}
	if _, err := os.Stat(filepath.Join(manager.GetDefaultConfigDir(), "config.json")); !os.IsNotExist(err) {
		t.Fatalf("portable configuration was copied to the user directory: %v", err)
	}
}

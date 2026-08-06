//go:build linux

package coreapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/temperature"
)

func TestTemperatureHistoryPathUsesXDGStateHome(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", stateHome)
	installDir := filepath.Join(t.TempDir(), "usr", "bin")

	want := filepath.Join(stateHome, "thrm", temperature.DefaultHistoryRelativePath)
	if got := temperatureHistoryPath(installDir); got != want {
		t.Fatalf("temperatureHistoryPath() = %q, want %q", got, want)
	}
}

func TestMigrateLegacyTemperatureHistory(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "portable")
	legacyPath := filepath.Join(installDir, temperature.DefaultHistoryRelativePath)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatal(err)
	}
	want := []byte("legacy-history")
	if err := os.WriteFile(legacyPath, want, 0644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "state", temperature.DefaultHistoryRelativePath)

	migrateLegacyTemperatureHistory(destination, installDir, nil)
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("migrated history is missing: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("migrated history = %q, want %q", got, want)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("migration removed the legacy history: %v", err)
	}
}

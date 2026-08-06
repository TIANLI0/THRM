//go:build linux

package guiapp

import (
	"path/filepath"
	"testing"
)

func TestWindowStatePathUsesXDGConfigHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", configHome)

	want := filepath.Join(configHome, "thrm", windowStateFileName)
	if got := windowStatePath(); got != want {
		t.Fatalf("windowStatePath() = %q, want %q", got, want)
	}
}

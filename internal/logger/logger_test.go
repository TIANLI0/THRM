//go:build windows

package logger

import (
	"path/filepath"
	"testing"
)

func TestDefaultLogDirPrefersInstallDirWhenWritable(t *testing.T) {
	installDir := t.TempDir()

	got := defaultLogDir(installDir)
	want := filepath.Join(installDir, "logs")

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}

func TestDefaultLogDirUsesRelativeLogsWhenInstallDirEmpty(t *testing.T) {
	got := defaultLogDir("")
	want := "logs"

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}

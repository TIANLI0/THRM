//go:build linux

package coreapp

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	original := os.Stderr
	os.Stderr = writer
	run()
	os.Stderr = original
	_ = writer.Close()
	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	return string(data)
}

func TestCapturePanicLinuxUsesStandardErrorBeforeLoggerIsReady(t *testing.T) {
	var path string
	output := captureStderr(t, func() {
		path = CapturePanic(nil, "startup", "boom")
	})

	if path != "" {
		t.Fatalf("CapturePanic() path = %q, want empty for journal logging", path)
	}
	for _, marker := range []string{"THRM Core Crash Report", "source: startup", "panic: boom", "--- stack ---"} {
		if !strings.Contains(output, marker) {
			t.Errorf("stderr does not contain %q: %s", marker, output)
		}
	}
}

func TestResolveCrashLogDirLinuxDoesNotUseInstallDirOrVarLog(t *testing.T) {
	if got := resolveCrashLogDir(nil); got != "" {
		t.Fatalf("resolveCrashLogDir(nil) = %q, want empty", got)
	}
	if got := resolveCrashLogDir(&CoreApp{}); got != "" {
		t.Fatalf("resolveCrashLogDir(empty app) = %q, want empty", got)
	}
}

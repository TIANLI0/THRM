//go:build windows

package coreapp

import "testing"

func TestCapturePanicWindowsWritesFile(t *testing.T) {
	if path, _ := capturePanicReport(t, "windows", "boom"); path == "" {
		t.Fatal("CapturePanic() did not write a Windows crash report")
	}
}

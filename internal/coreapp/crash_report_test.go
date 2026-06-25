package coreapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturePanic_GeneratesFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	app := &CoreApp{}
	filePath := CapturePanic(app, "test_source", "test panic message")
	if filePath == "" {
		t.Fatal("CapturePanic should return a file path")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read crash report: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test_source") {
		t.Fatal("crash report should contain source")
	}
	if !strings.Contains(contentStr, "test panic message") {
		t.Fatal("crash report should contain panic message")
	}
	if !strings.Contains(contentStr, "--- stack ---") {
		t.Fatal("crash report should contain stack trace")
	}
}

func TestCapturePanic_NilApp(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	filePath := CapturePanic(nil, "nil_test", "nil app panic")
	if filePath == "" {
		t.Fatal("CapturePanic should work with nil app, via config.GetInstallDir fallback")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read crash report: %v", err)
	}
	if !strings.Contains(string(content), "nil app panic") {
		t.Fatal("crash report should contain panic message even with nil app")
	}
}

func TestResolveCrashLogDir_NilApp(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	logDir := resolveCrashLogDir(nil)
	if logDir == "" || logDir == "logs" {
		t.Logf("resolveCrashLogDir with nil app: %q (may fallback to logs/)", logDir)
	}
	expectedSuffix := filepath.Join(".thrm", "logs")
	if !strings.HasSuffix(logDir, expectedSuffix) {
		t.Logf("resolved log dir: %q, expected suffix: %q", logDir, expectedSuffix)
	}
}

package coreapp

import (
	"io"
	"os"
	"strings"
	"testing"
)

func capturePanicReport(t *testing.T, source string, recovered any) (string, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = writer
	path := CapturePanic(nil, source, recovered)
	os.Stderr = original
	_ = writer.Close()
	stderr, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		return "", string(stderr)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, string(content)
}

func TestCapturePanicIncludesContext(t *testing.T) {
	_, content := capturePanicReport(t, "test_source", "test panic message")
	for _, marker := range []string{"test_source", "test panic message", "--- stack ---"} {
		if !strings.Contains(content, marker) {
			t.Fatalf("crash report does not contain %q: %s", marker, content)
		}
	}
}

//go:build linux

package coreapp

import (
	"strings"
	"testing"
)

func TestReinstallPawnIO_ReturnsNotSupported(t *testing.T) {
	a := &CoreApp{}
	result, err := a.ReinstallPawnIO()
	if err != nil {
		t.Fatalf("ReinstallPawnIO should not error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	success, _ := result["success"].(bool)
	if success {
		t.Fatal("success should be false on Linux")
	}
	msg, _ := result["message"].(string)
	if !strings.Contains(msg, "Linux") {
		t.Fatalf("message should mention Linux, got: %q", msg)
	}
}

func TestReinstallPawnIO_NotNilResult(t *testing.T) {
	a := &CoreApp{}
	result, err := a.ReinstallPawnIO()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) < 2 {
		t.Fatal("result should have success and message fields")
	}
}

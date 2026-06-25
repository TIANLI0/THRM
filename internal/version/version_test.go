package version

import (
	"testing"
)

func TestGet(t *testing.T) {
	v := Get()
	if v == "" {
		t.Error("Get() should not return empty")
	}
}

func TestBuildVersion(t *testing.T) {
	if BuildVersion == "" {
		t.Error("BuildVersion should default to 'dev'")
	}
}

package appmeta

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Shared constants (meta.go) ---

func TestSharedConstants(t *testing.T) {
	if AppName != "THRM" {
		t.Fatalf("AppName = %q, want THRM", AppName)
	}
	if IPCPipeName != "THRM-IPC" {
		t.Fatalf("IPCPipeName = %q", IPCPipeName)
	}
	if ConfigDirName != ".thrm" {
		t.Fatalf("ConfigDirName = %q", ConfigDirName)
	}
	if ProtocolVersion != "3.0" {
		t.Fatalf("ProtocolVersion = %q", ProtocolVersion)
	}
	if RepositoryURL != "https://github.com/TIANLI0/THRM" {
		t.Fatalf("RepositoryURL = %q", RepositoryURL)
	}
	if LegacyAppName != "BS2PRO Controller" {
		t.Fatalf("LegacyAppName = %q", LegacyAppName)
	}
	if CoreName != "THRM Core" {
		t.Fatalf("CoreName = %q", CoreName)
	}
}

func TestLatestReleaseURL(t *testing.T) {
	want := "https://github.com/TIANLI0/THRM/releases/latest"
	if LatestReleaseURL != want {
		t.Fatalf("LatestReleaseURL = %q, want %q", LatestReleaseURL, want)
	}
}

func TestIPCPipeCandidates(t *testing.T) {
	c := IPCPipeCandidates()
	if len(c) != 2 {
		t.Fatalf("IPCPipeCandidates length = %d, want 2", len(c))
	}
	if c[0] != IPCPipeName {
		t.Fatalf("IPCPipeCandidates[0] = %q", c[0])
	}
	if c[1] != LegacyIPCPipeName {
		t.Fatalf("IPCPipeCandidates[1] = %q", c[1])
	}
}

func TestBridgePipeCandidates(t *testing.T) {
	c := BridgePipeCandidates()
	if len(c) != 2 {
		t.Fatalf("BridgePipeCandidates length = %d, want 2", len(c))
	}
	if c[0] != BridgePipeName {
		t.Fatalf("BridgePipeCandidates[0] = %q", c[0])
	}
}

func TestFirstExistingPath_Hit(t *testing.T) {
	got := FirstExistingPath([]string{"/tmp", "/nonexistent"})
	if got != "/tmp" {
		t.Fatalf("FirstExistingPath = %q, want /tmp", got)
	}
}

func TestFirstExistingPath_Miss(t *testing.T) {
	got := FirstExistingPath([]string{"/nonexistent1", "/nonexistent2"})
	if got != "" {
		t.Fatalf("FirstExistingPath = %q, want empty", got)
	}
}

func TestFirstExistingPath_Empty(t *testing.T) {
	got := FirstExistingPath([]string{})
	if got != "" {
		t.Fatalf("FirstExistingPath = %q, want empty", got)
	}
}

func TestFirstExistingPath_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	f, err := os.Create(filepath.Join(tmp, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	got := FirstExistingPath([]string{filepath.Join(tmp, "test.txt")})
	if got != filepath.Join(tmp, "test.txt") {
		t.Fatalf("FirstExistingPath = %q", got)
	}
}

func TestUserConfigDir(t *testing.T) {
	got := UserConfigDir("/home/testuser")
	// 期望值同样用 filepath.Join 构造：路径分隔符随平台变化。
	want := filepath.Join("/home/testuser", ConfigDirName)
	if got != want {
		t.Fatalf("UserConfigDir = %q, want %q", got, want)
	}
}

func TestLegacyUserConfigDir(t *testing.T) {
	got := LegacyUserConfigDir("/home/testuser")
	want := filepath.Join("/home/testuser", LegacyConfigDirName)
	if got != want {
		t.Fatalf("LegacyUserConfigDir = %q, want %q", got, want)
	}
}

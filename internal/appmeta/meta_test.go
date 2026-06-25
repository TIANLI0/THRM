package appmeta

import (
	"os"
	"path/filepath"
	"strings"
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
	want := "/home/testuser/.thrm"
	if got != want {
		t.Fatalf("UserConfigDir = %q, want %q", got, want)
	}
}

func TestLegacyUserConfigDir(t *testing.T) {
	got := LegacyUserConfigDir("/home/testuser")
	want := "/home/testuser/.bs2pro-controller"
	if got != want {
		t.Fatalf("LegacyUserConfigDir = %q, want %q", got, want)
	}
}

// --- Linux-specific constants (meta_linux.go) ---

func TestLinuxExecutableNameNoExe(t *testing.T) {
	if ExecutableName != "thrm" {
		t.Fatalf("ExecutableName = %q, want thrm", ExecutableName)
	}
	if strings.Contains(ExecutableName, ".exe") {
		t.Fatal("ExecutableName must not contain .exe")
	}
	if LegacyExecutableName != "bs2pro-controller" {
		t.Fatalf("LegacyExecutableName = %q", LegacyExecutableName)
	}
}

func TestLinuxCoreExecutableName(t *testing.T) {
	if CoreExecutableName != "thrm-core" {
		t.Fatalf("CoreExecutableName = %q, want thrm-core", CoreExecutableName)
	}
	if strings.Contains(CoreExecutableName, ".exe") {
		t.Fatal("CoreExecutableName must not contain .exe")
	}
	if LegacyCoreExecutable != "bs2pro-core" {
		t.Fatalf("LegacyCoreExecutable = %q", LegacyCoreExecutable)
	}
}

func TestLinuxBridgeConstantsEmpty(t *testing.T) {
	if BridgeName != "" {
		t.Fatalf("BridgeName = %q, want empty", BridgeName)
	}
	if BridgeExecutableName != "" {
		t.Fatalf("BridgeExecutableName = %q, want empty", BridgeExecutableName)
	}
	if PawnIOInstallerName != "" {
		t.Fatalf("PawnIOInstallerName = %q, want empty", PawnIOInstallerName)
	}
}

func TestLinuxBridgeExecutableCandidatesNil(t *testing.T) {
	if BridgeExecutableCandidates("/tmp") != nil {
		t.Fatal("BridgeExecutableCandidates should return nil")
	}
}

func TestLinuxPawnIOInstallerPathEmpty(t *testing.T) {
	if PawnIOInstallerPath("/tmp") != "" {
		t.Fatal("PawnIOInstallerPath should return empty")
	}
}

func TestLinuxPawnIOInstallerCandidatesNil(t *testing.T) {
	if PawnIOInstallerCandidates("/tmp") != nil {
		t.Fatal("PawnIOInstallerCandidates should return nil")
	}
}

func TestLinuxCoreExecutableCandidatesNoExe(t *testing.T) {
	candidates := CoreExecutableCandidates("/usr/bin")
	for _, c := range candidates {
		if strings.Contains(c, ".exe") {
			t.Fatalf("CoreExecutableCandidates contains .exe: %q", c)
		}
	}
	if len(candidates) < 2 {
		t.Fatalf("CoreExecutableCandidates too few: %v", candidates)
	}
	hasThrmCore := false
	for _, c := range candidates {
		if strings.HasSuffix(c, "thrm-core") {
			hasThrmCore = true
			break
		}
	}
	if !hasThrmCore {
		t.Fatal("CoreExecutableCandidates should contain thrm-core")
	}
}

func TestLinuxGUIExecutableCandidatesNoExe(t *testing.T) {
	candidates := GUIExecutableCandidates("/usr/bin")
	for _, c := range candidates {
		if strings.Contains(c, ".exe") {
			t.Fatalf("GUIExecutableCandidates contains .exe: %q", c)
		}
	}
	if len(candidates) < 2 {
		t.Fatalf("GUIExecutableCandidates too few: %v", candidates)
	}
	hasThrm := false
	for _, c := range candidates {
		if strings.HasSuffix(c, "thrm") {
			hasThrm = true
			break
		}
	}
	if !hasThrm {
		t.Fatal("GUIExecutableCandidates should contain thrm")
	}
}

func TestLinuxCoreExecutableCandidatesIncludesExeDir(t *testing.T) {
	candidates := CoreExecutableCandidates("/usr/bin")
	expected := filepath.Join("/usr/bin", "thrm-core")
	found := false
	for _, c := range candidates {
		if c == expected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CoreExecutableCandidates missing %q: %v", expected, candidates)
	}
}

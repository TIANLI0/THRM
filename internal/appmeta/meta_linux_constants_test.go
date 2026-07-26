//go:build linux

package appmeta

import (
	"path/filepath"
	"strings"
	"testing"
)

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

//go:build windows

package coreapp

import (
	"strings"
	"testing"
)

func TestBuildPawnIOReinstallScript(t *testing.T) {
	script := buildPawnIOReinstallScript(`C:\THRM\drivers\PawnIO\PawnIO_setup.exe`, `C:\THRM\THRM.exe`, 4242)
	for _, want := range []string{
		`PID eq 4242`,
		`reg query "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" /reg:64`,
		`if "!PAWNIO_PRESENT!"=="1" (`,
		`-uninstall -silent`,
		`-install -silent`,
		`start "" "%GUI_FILE%"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("reinstall script missing %q:\n%s", want, script)
		}
	}
	if strings.Index(script, `-install -silent`) > strings.Index(script, `start "" "%GUI_FILE%"`) {
		t.Fatalf("THRM restart was emitted before the PawnIO install step:\n%s", script)
	}
}

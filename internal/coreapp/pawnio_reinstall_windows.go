//go:build windows

package coreapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func buildPawnIOReinstallScript(installerPath, guiPath string, corePID int) string {
	escape := func(value string) string {
		value = strings.NewReplacer("^", "^^", "&", "^&", "<", "^<", ">", "^>", "|", "^|").Replace(value)
		value = strings.ReplaceAll(value, "%", "%%")
		return strings.ReplaceAll(value, "!", "")
	}

	var script strings.Builder
	writeLine := func(line string) { script.WriteString(line); script.WriteString("\r\n") }
	writeLine("@echo off")
	writeLine("setlocal enableextensions enabledelayedexpansion")
	writeLine(`set "PATH=%SystemRoot%\System32;%PATH%"`)
	writeLine("title THRM PawnIO reinstall")
	writeLine(fmt.Sprintf(`set "INSTALLER_FILE=%s"`, escape(installerPath)))
	writeLine(fmt.Sprintf(`set "GUI_FILE=%s"`, escape(guiPath)))
	writeLine(":waitcore")
	writeLine(fmt.Sprintf(`tasklist /FI "PID eq %d" /NH 2>nul | find "%d" >nul`, corePID, corePID))
	writeLine("if not errorlevel 1 (")
	writeLine("  timeout /t 1 /nobreak >nul")
	writeLine("  goto waitcore")
	writeLine(")")
	writeLine(`set "PAWNIO_PRESENT=0"`)
	writeLine(`reg query "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" /reg:64 >nul 2>&1 && set "PAWNIO_PRESENT=1"`)
	writeLine(`reg query "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" /reg:32 >nul 2>&1 && set "PAWNIO_PRESENT=1"`)
	writeLine(`if "!PAWNIO_PRESENT!"=="1" (`)
	writeLine(`  start "" /wait "%INSTALLER_FILE%" -uninstall -silent`)
	writeLine(`  if not "!ERRORLEVEL!"=="0" echo PawnIO uninstall returned !ERRORLEVEL!; continuing with installation.`)
	writeLine(")")
	writeLine(`start "" /wait "%INSTALLER_FILE%" -install -silent`)
	writeLine(`set "INSTALL_EXIT=!ERRORLEVEL!"`)
	writeLine(`if not "!INSTALL_EXIT!"=="0" timeout /t 6 /nobreak >nul`)
	writeLine(`if exist "%GUI_FILE%" start "" "%GUI_FILE%"`)
	writeLine(`del "%~f0" >nul 2>&1`)
	writeLine(`rmdir "%~dp0" >nul 2>&1`)
	return script.String()
}

func launchPawnIOReinstallScript(installerPath, guiPath string, corePID int) error {
	scriptDir, err := os.MkdirTemp("", "THRM-pawnio-reinstall-")
	if err != nil {
		return fmt.Errorf("create PawnIO reinstall directory: %w", err)
	}
	scriptPath := filepath.Join(scriptDir, "run-pawnio-reinstall.bat")
	if err := os.WriteFile(scriptPath, []byte(buildPawnIOReinstallScript(installerPath, guiPath, corePID)), 0o644); err != nil {
		_ = os.RemoveAll(scriptDir)
		return fmt.Errorf("write PawnIO reinstall script: %w", err)
	}

	cmd := exec.Command("cmd", "/d", "/c", "start", "", "cmd", "/d", "/c", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(scriptDir)
		return fmt.Errorf("launch PawnIO reinstall script: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

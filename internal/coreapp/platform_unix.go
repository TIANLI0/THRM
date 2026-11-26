//go:build linux || darwin

package coreapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	return map[string]any{
		"success": false,
		"message": "PawnIO is not supported on " + platformDisplayName(),
	}, nil
}

// platformDisplayName 返回面向用户的系统名称（runtime.GOOS 是小写的 linux/darwin）。
func platformDisplayName() string {
	if runtime.GOOS == "darwin" {
		return "macOS"
	}
	return "Linux"
}

func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	candidates := append(
		appmeta.GUIExecutableCandidates(exeDir),
		appmeta.GUIExecutableCandidates(filepath.Join(exeDir, ".."))...,
	)
	guiPath := appmeta.FirstExistingPath(candidates)
	if guiPath == "" {
		if path, err := exec.LookPath(appmeta.ExecutableName); err == nil {
			guiPath = path
		}
	}
	if guiPath == "" {
		return fmt.Errorf("GUI executable not found: %v", candidates)
	}

	cmd := guiLaunchCommand(guiPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GUI: %w", err)
	}

	go cmd.Wait()
	return nil
}

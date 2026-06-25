//go:build linux

package coreapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	return map[string]any{
		"success": false,
		"message": "PawnIO is not supported on Linux",
	}, nil
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

	cmd := exec.Command(guiPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GUI: %w", err)
	}

	go cmd.Wait()
	return nil
}

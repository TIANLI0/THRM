//go:build linux

package coreapp

import "os/exec"

func guiLaunchCommand(guiPath string) *exec.Cmd {
	return exec.Command(guiPath)
}

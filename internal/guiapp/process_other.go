//go:build !linux && !windows

package guiapp

import "os/exec"

func configureCoreCommand(cmd *exec.Cmd) {}

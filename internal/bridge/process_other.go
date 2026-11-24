//go:build !windows

package bridge

import (
	"os/exec"
	"syscall"
)

func isProcessRunning(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return false
	}

	return cmd.Process.Signal(syscall.Signal(0)) == nil
}

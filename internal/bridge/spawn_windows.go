//go:build windows

package bridge

import (
	"os/exec"
	"syscall"
)

func configureCmdSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

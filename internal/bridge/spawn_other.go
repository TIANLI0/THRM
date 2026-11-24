//go:build !windows

package bridge

import "os/exec"

func configureCmdSysProcAttr(cmd *exec.Cmd) {
}

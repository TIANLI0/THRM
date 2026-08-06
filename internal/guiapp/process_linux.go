//go:build linux

package guiapp

import (
	"os"
	"os/exec"
	"strings"
)

func configureCoreCommand(cmd *exec.Cmd) {
	// This WebKitGTK workaround belongs to the GUI and must not leak into core
	// or processes launched by core after a restart.
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "__NV_DISABLE_EXPLICIT_SYNC=") {
			cmd.Env = append(cmd.Env, variable)
		}
	}

	// Go 的 os/exec 会在字段为 nil 时把子进程输出接到 /dev/null。显式继承标准
	// 输出后，Go runtime 在 logger 初始化前产生的致命信息仍能被启动 GUI 的
	// systemd scope、终端或其他服务管理器收集。
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}

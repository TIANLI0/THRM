//go:build darwin

package coreapp

import (
	"os/exec"
	"strings"
)

// guiLaunchCommand 在 GUI 位于 .app 包内时经 LaunchServices（open）启动：
// 直接 exec 包内可执行文件时窗口不会被激活到前台，Dock 图标也可能重复。
// 开发时的裸可执行文件仍直接运行。
func guiLaunchCommand(guiPath string) *exec.Cmd {
	if bundle, _, ok := strings.Cut(guiPath, ".app/Contents/MacOS/"); ok {
		return exec.Command("/usr/bin/open", bundle+".app")
	}
	return exec.Command(guiPath)
}

//go:build darwin

package coreapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
)

// ReinstallPawnIO 是 Windows 专用的驱动安装流程（PawnIO 是 NT 驱动），macOS 上不存在。
//
// 这里保留方法而不是从 IPC 协议里删掉，原因有二：
//  1. 上游把 ReinstallPawnIO 定义在跨平台的 types / ipc 层，删了会与后续
//     git merge upstream 产生冲突；
//  2. 前端各语言版本仍可能发出该请求，返回「不支持」比「未知方法」更友好。
func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	return map[string]any{
		"success": false,
		"message": "PawnIO is not supported on macOS",
	}, nil
}

// launchGUI 优先按 macOS 的惯例启动 .app：Wails 有单实例锁，
// 直接起裸可执行文件会和已安装的 THRM.app 互相抢实例。
func launchGUI() error {
	home, _ := os.UserHomeDir()
	exePath, _ := os.Executable()
	exeDir := ""
	if exePath != "" {
		exeDir = filepath.Dir(exePath)
	}

	apps := []string{
		filepath.Join(home, "Applications", "THRM.app"),
		"/Applications/THRM.app",
	}
	if exeDir != "" {
		apps = append(apps,
			filepath.Join(exeDir, "THRM.app"),
			filepath.Join(exeDir, "..", "THRM.app"),
		)
	}
	for _, app := range apps {
		if _, err := os.Stat(app); err == nil {
			if out, err := exec.Command("/usr/bin/open", "-a", app).CombinedOutput(); err != nil {
				return fmt.Errorf("open %s: %v (%s)", app, err, strings.TrimSpace(string(out)))
			}
			return nil
		}
	}

	// 没有 .app 时回退到可执行文件（开发场景）。
	candidates := appmeta.GUIExecutableCandidates(exeDir)
	guiPath := appmeta.FirstExistingPath(candidates)
	if guiPath == "" {
		if path, err := exec.LookPath(appmeta.ExecutableName); err == nil {
			guiPath = path
		}
	}
	if guiPath == "" {
		return fmt.Errorf("THRM.app 和 GUI 可执行文件都没找到")
	}
	return exec.Command(guiPath).Start()
}

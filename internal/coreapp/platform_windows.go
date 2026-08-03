//go:build windows

package coreapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"golang.org/x/sys/windows/registry"
)

// ReinstallPawnIO schedules the bundled PawnIO installer after THRM exits.
func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	installDir := config.GetInstallDir()
	installerPath := appmeta.FirstExistingPath(appmeta.PawnIOInstallerCandidates(installDir))
	if installerPath == "" {
		return nil, fmt.Errorf("未找到 PawnIO 安装包，已尝试路径: %v", appmeta.PawnIOInstallerCandidates(installDir))
	}
	guiPath := appmeta.FirstExistingPath(appmeta.GUIExecutableCandidates(installDir))
	if guiPath == "" {
		return nil, fmt.Errorf("未找到 THRM 主程序，已尝试路径: %v", appmeta.GUIExecutableCandidates(installDir))
	}

	result := map[string]any{
		"success":     false,
		"path":        installerPath,
		"restartPath": guiPath,
	}
	installedVersionBefore := readInstalledPawnIOVersion()
	if installedVersionBefore != "" {
		result["installedVersionBefore"] = installedVersionBefore
	}

	if err := launchPawnIOReinstallScript(installerPath, guiPath, os.Getpid()); err != nil {
		result["error"] = err.Error()
		return result, err
	}

	a.logInfo("已安排重新安装 PawnIO，正在退出 THRM: %s", installerPath)
	result["success"] = true
	result["scheduled"] = true
	a.safeGo("pawnio-reinstall-quit", func() {
		time.Sleep(800 * time.Millisecond)
		a.onQuitRequest()
	})

	return result, nil
}
func readInstalledPawnIOVersion() string {
	for _, access := range []uint32{registry.QUERY_VALUE | registry.WOW64_64KEY, registry.QUERY_VALUE | registry.WOW64_32KEY} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, pawnIORegistryPath, access)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("DisplayVersion")
		_ = key.Close()
		if err == nil && strings.TrimSpace(version) != "" {
			return strings.TrimSpace(version)
		}
	}
	return ""
}

// launchGUI 启动 GUI 程序
func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	guiCandidates := append(appmeta.GUIExecutableCandidates(exeDir), appmeta.GUIExecutableCandidates(filepath.Join(exeDir, ".."))...)
	guiPath := appmeta.FirstExistingPath(guiCandidates)
	if guiPath == "" {
		return fmt.Errorf("GUI 程序不存在: %v", guiCandidates)
	}

	cmd := exec.Command(guiPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 GUI 程序失败: %v", err)
	}

	// 使用 fmt 而非日志系统，避免循环依赖
	fmt.Printf("GUI 程序已启动，PID: %d\n", cmd.Process.Pid)

	go func() {
		cmd.Wait()
	}()

	return nil
}

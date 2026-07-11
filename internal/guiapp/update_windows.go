//go:build windows

package guiapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting string) error {
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("安装包不存在: %w", err)
	}

	batEsc := func(s string) string {
		s = strings.NewReplacer("^", "^^", "&", "^&", "<", "^<", ">", "^>", "|", "^|").Replace(s)
		s = strings.ReplaceAll(s, "%", "%%")
		s = strings.ReplaceAll(s, "!", "")
		return s
	}

	pid := os.Getpid()

	var b strings.Builder
	w := func(line string) { b.WriteString(line); b.WriteString("\r\n") }

	w("@echo off")
	w("setlocal enableextensions enabledelayedexpansion")
	// System32 置前，确保 tasklist/find/timeout 用系统自带程序（防 PATH 污染）。
	w(`set "PATH=%SystemRoot%\System32;%PATH%"`)
	w("chcp 65001>nul")
	w(fmt.Sprintf("title %s", batEsc(windowTitle)))
	w("mode con: cols=64 lines=16>nul")
	// 取回车符(CR)，用于单行原地刷新（<nul set /p 不换行 + 行尾 CR 回到行首）。
	w(`for /f %%a in ('copy /Z "%~f0" nul') do set "CR=%%a"`)
	w("echo.")
	w("echo    ================================================")
	w(fmt.Sprintf("echo      %s", batEsc(windowTitle)))
	w("echo    ================================================")
	w("echo.")
	w(fmt.Sprintf("echo      %s", batEsc(windowBody)))
	w("echo.")
	// 1) 等待 GUI 退出。
	w(":waitgui")
	w(fmt.Sprintf(`tasklist /FI "PID eq %d" 2>nul | find "%d" >nul`, pid, pid))
	w("if not errorlevel 1 (")
	w("  timeout /t 1 /nobreak >nul")
	w("  goto waitgui")
	w(")")
	// 2) 静默安装（UAC 由系统按安装器清单自动触发）。
	w(fmt.Sprintf(`start "" "%s" /S`, installerPath))
	// 3) 动态等待安装进程结束。
	w(`set "frames=|/-\"`)
	w("set /a sec=0")
	w(`set "started="`)
	w(":waitinstall")
	w("set /a sec+=1")
	w("set /a idx=sec %% 4")
	w(`for %%j in (!idx!) do set "spin=!frames:~%%j,1!"`)
	w(fmt.Sprintf(`<nul set /p "=      %s [!spin!] !sec!s   !CR!"`, batEsc(windowBody)))
	w(`set "running="`)
	w(fmt.Sprintf(`tasklist /FI "IMAGENAME eq %s" 2>nul | find /I "%s" >nul && set "running=1"`, updateInstallerName, updateInstallerName))
	w(`if defined running set "started=1"`)
	w("if not defined running if defined started goto done")
	w("if not defined running if !sec! geq 90 goto done")
	w("timeout /t 1 /nobreak >nul")
	w("goto waitinstall")
	// 4) 完成并重启。
	w(":done")
	w("echo.")
	w("echo.")
	w(fmt.Sprintf("echo      %s", batEsc(windowRestarting)))
	w("timeout /t 2 /nobreak >nul")
	if exePath != "" {
		w(fmt.Sprintf(`start "" "%s"`, exePath))
	}
	w("exit")

	scriptPath := filepath.Join(filepath.Dir(installerPath), "run-update.bat")
	// 清理上一轮可能残留的脚本。
	_ = os.Remove(scriptPath)
	if err := os.WriteFile(scriptPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("写入更新脚本失败: %w", err)
	}
	cmd := exec.Command("cmd", "/d", "/c", "start", "", "cmd", "/d", "/c", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP：引导进程本身无窗口并与 GUI 分离；
		// 可见的是 start 打开的新控制台窗口。
		CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新安装程序失败: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

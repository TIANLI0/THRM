//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	darwinLaunchAgentLabel = "com.tianli0.thrm.core"
	autoStartMethodDarwin  = "launchagent"
)

// Manager 在 macOS 上用 LaunchAgent 实现开机自启。
type Manager struct {
	logger types.Logger
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{logger: logger}
}

func darwinLaunchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", darwinLaunchAgentLabel+".plist")
}

func (m *Manager) IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}

// SetWindowsAutoStart 名称沿用跨平台接口，macOS 下写 LaunchAgent。
func (m *Manager) SetWindowsAutoStart(enable bool) error {
	return m.SetAutoStartWithMethod(enable, autoStartMethodDarwin)
}

func (m *Manager) GetAutoStartMethod() string {
	if m.CheckWindowsAutoStart() {
		return autoStartMethodDarwin
	}
	return "none"
}

func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	plistPath := darwinLaunchAgentPath()
	if plistPath == "" {
		return fmt.Errorf("cannot resolve LaunchAgents directory")
	}

	if !enable {
		_ = exec.Command("/bin/launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), darwinLaunchAgentLabel)).Run()
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key>
	<array><string>%s</string></array>
	<key>RunAtLoad</key><true/>
	<key>KeepAlive</key><false/>
	<key>ProcessType</key><string>Interactive</string>
</dict>
</plist>
`, darwinLaunchAgentLabel, exePath)

	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return err
	}

	_ = exec.Command("/bin/launchctl", "bootout",
		fmt.Sprintf("gui/%d/%s", os.Getuid(), darwinLaunchAgentLabel)).Run()
	if out, err := exec.Command("/bin/launchctl", "bootstrap",
		fmt.Sprintf("gui/%d", os.Getuid()), plistPath).CombinedOutput(); err != nil {
		// 已经加载过时不视为失败
		if !strings.Contains(string(out), "already") {
			return fmt.Errorf("launchctl bootstrap: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (m *Manager) CheckWindowsAutoStart() bool {
	plistPath := darwinLaunchAgentPath()
	if plistPath == "" {
		return false
	}
	_, err := os.Stat(plistPath)
	return err == nil
}

func (m *Manager) EnsureAutoStartTaskHealthy() (bool, error) {
	return m.CheckWindowsAutoStart(), nil
}

// DetectAutoStartLaunch 判断本次启动是否由 LaunchAgent 触发。
func DetectAutoStartLaunch(args []string) bool {
	for _, a := range args {
		if a == "--autostart" || a == "-autostart" {
			return true
		}
	}
	return false
}

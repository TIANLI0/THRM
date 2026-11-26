//go:build darwin

package autostart

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// MethodLaunchAgent 是 macOS 上唯一的自启动方式：~/Library/LaunchAgents 下的
// 用户级 launchd 条目。方法名只为与其它平台共用同一套接口，传什么都按它处理。
const MethodLaunchAgent = "launchagent"

// launchAgentLabel 同时是 plist 文件名，“系统设置 → 通用 → 登录项”里按
// AssociatedBundleIdentifiers 归到 THRM 名下。
const launchAgentLabel = appmeta.BundleIdentifier + ".core"

type Manager struct {
	logger types.Logger
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{logger: logger}
}

func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func (m *Manager) IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}

func (m *Manager) SetWindowsAutoStart(enable bool) error {
	return m.SetAutoStartWithMethod(enable, MethodLaunchAgent)
}

func (m *Manager) GetAutoStartMethod() string {
	if launchAgentEnabled(launchAgentPath()) {
		return MethodLaunchAgent
	}
	return "none"
}

// SetAutoStartWithMethod 写入/删除 LaunchAgent。只落文件、不调用 launchctl
// bootstrap：条目在下次登录时由 launchd 加载；现在就加载会立刻再拉起一个核心。
func (m *Manager) SetAutoStartWithMethod(enable bool, _ string) error {
	plistPath := launchAgentPath()
	if !enable {
		// 已加载的条目要先卸载，否则本次登录会话里 launchd 仍记着它。
		// 未加载时 bootout 会报错，忽略即可。
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), launchAgentLabel)).Run()
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove launch agent: %w", err)
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(plistPath, buildLaunchAgentPlist(exePath), 0644); err != nil {
		return fmt.Errorf("write launch agent: %w", err)
	}
	return nil
}

func buildLaunchAgentPlist(exePath string) []byte {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(exePath))
	// KeepAlive 不开：用户从托盘退出后不该被 launchd 立刻拉起来。
	// ProcessType=Interactive 让 App Nap 不去节流控温循环。
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--autostart</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>LimitLoadToSessionType</key>
	<string>Aqua</string>
	<key>AssociatedBundleIdentifiers</key>
	<string>%s</string>
</dict>
</plist>
`, launchAgentLabel, escaped.String(), appmeta.BundleIdentifier)
}

func (m *Manager) CheckWindowsAutoStart() bool {
	return launchAgentEnabled(launchAgentPath())
}

// EnsureAutoStartTaskHealthy 与 Windows 侧同名方法配平。条目指向的可执行文件
// 不存在了（.app 被挪了位置）时按当前路径重写，否则登录后什么也不会启动。
func (m *Manager) EnsureAutoStartTaskHealthy() (bool, error) {
	plistPath := launchAgentPath()
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return false, nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return false, nil
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	if bytes.Equal(data, buildLaunchAgentPlist(exePath)) {
		return false, nil
	}
	if err := os.WriteFile(plistPath, buildLaunchAgentPlist(exePath), 0644); err != nil {
		return false, fmt.Errorf("rewrite launch agent: %w", err)
	}
	return true, nil
}

// launchAgentEnabled 只看 plist 是否存在且没被写成 Disabled。用户在“登录项”
// 里关掉后台项目时 launchd 自己记录禁用状态、不改文件，这种情况读不到。
func launchAgentEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	compact := strings.Join(strings.Fields(string(data)), "")
	return !strings.Contains(compact, "<key>Disabled</key><true/>")
}

func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}
	return false
}

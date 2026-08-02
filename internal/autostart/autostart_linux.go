//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// MethodDesktop 是 Linux 上唯一的自启动方式：XDG autostart 条目。
// 方法名保留在 API 里（Windows 有注册表/计划任务两种），前端按平台传入。
const MethodDesktop = "desktop"

type Manager struct {
	logger types.Logger
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{logger: logger}
}

func autostartDesktopPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	// 遵循 XDG：$XDG_CONFIG_HOME 未设置时才回落到 ~/.config。
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" || !filepath.IsAbs(configHome) {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", "thrm.desktop")
}

func (m *Manager) IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}

func (m *Manager) SetWindowsAutoStart(enable bool) error {
	return m.SetAutoStartWithMethod(enable, MethodDesktop)
}

func (m *Manager) GetAutoStartMethod() string {
	if autostartEnabled(autostartDesktopPath()) {
		return MethodDesktop
	}
	return "none"
}

// SetAutoStartWithMethod 写入/删除 XDG autostart 条目。method 参数只为与 Windows
// 侧保持同一套接口，Linux 上只有 desktop 一种方式，传什么都按它处理。
func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	desktopPath := autostartDesktopPath()
	if !enable {
		if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove autostart entry: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	exePath, err := os.Executable()
	if err != nil {
		exePath = appmeta.ExecutableName
	}
	// Hidden=false 必须显式写：桌面环境关掉自启动项时会把 Hidden=true 写回同一个
	// 文件，重新开启时要盖回去。Icon 与安装脚本一致，否则条目没有图标。
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=Flydigi BS Series Fan Controller
Exec="%s" --autostart
Icon=thrm
Terminal=false
Hidden=false
StartupWMClass=thrm
X-GNOME-Autostart-enabled=true
`, appmeta.AppName, exePath)
	if err := os.WriteFile(desktopPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	return nil
}

func (m *Manager) CheckWindowsAutoStart() bool {
	return autostartEnabled(autostartDesktopPath())
}

// EnsureAutoStartTaskHealthy 与 Windows 侧同名方法配平。XDG autostart 条目没有
// "电池供电时不启动""限制运行时长"之类的设置，无需升级既有条目。
func (m *Manager) EnsureAutoStartTaskHealthy() (bool, error) {
	return false, nil
}

// autostartEnabled 判断 autostart 条目是否真的会被执行。
//
// 只看文件存在不够：GNOME/KDE 关掉某项时不删文件，而是写 Hidden=true
// （GNOME 另有 X-GNOME-Autostart-enabled=false），此时界面不该再显示已开启。
func autostartEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	for line := range strings.Lines(string(data)) {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "hidden":
			if value == "true" {
				return false
			}
		case "x-gnome-autostart-enabled":
			if value == "false" {
				return false
			}
		}
	}
	return true
}

func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}
	return false
}

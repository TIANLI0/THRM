//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

type Manager struct {
	logger types.Logger
}

func NewManager(logger types.Logger) *Manager {
	return &Manager{logger: logger}
}

func autostartDesktopPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "thrm.desktop")
}

func (m *Manager) IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}

func (m *Manager) SetWindowsAutoStart(enable bool) error {
	return m.SetAutoStartWithMethod(enable, "desktop")
}

func (m *Manager) GetAutoStartMethod() string {
	if _, err := os.Stat(autostartDesktopPath()); err == nil {
		return "desktop"
	}
	return "none"
}

func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	desktopPath := autostartDesktopPath()
	if enable {
		if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
			return fmt.Errorf("create autostart dir: %w", err)
		}
		exePath, err := os.Executable()
		if err != nil {
			exePath = appmeta.ExecutableName
		}
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=Flydigi BS Series Fan Controller
Exec=%s --autostart
Terminal=false
Hidden=false
X-GNOME-Autostart-enabled=true
`, appmeta.AppName, exePath)
		return os.WriteFile(desktopPath, []byte(content), 0644)
	}
	os.Remove(desktopPath)
	return nil
}

func (m *Manager) CheckWindowsAutoStart() bool {
	_, err := os.Stat(autostartDesktopPath())
	return err == nil
}

func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}
	return false
}

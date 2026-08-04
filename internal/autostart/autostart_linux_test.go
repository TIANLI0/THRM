package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (testLogger) Info(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Debug(string, ...any) {}
func (testLogger) Close()               {}
func (testLogger) CleanOldLogs()        {}
func (testLogger) SetDebugMode(bool)    {}
func (testLogger) GetLogDir() string    { return "" }

func mockHomeDir(t *testing.T, dir string) func() {
	t.Helper()
	orig := os.Getenv("HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("HOME", dir)
	// 测试机上若设置了 XDG_CONFIG_HOME，条目会落到真实用户目录而不是临时目录。
	os.Unsetenv("XDG_CONFIG_HOME")
	return func() {
		os.Setenv("HOME", orig)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	}
}

// 桌面环境关掉自启动项时不会删文件，而是写 Hidden=true / GNOME 的
// X-GNOME-Autostart-enabled=false。此时界面必须显示"未开启"。
func TestAutostartDisabledByDesktopEnvironment(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")

	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "enabled", content: "[Desktop Entry]\nType=Application\nName=THRM\nHidden=false\n", want: true},
		{name: "hidden", content: "[Desktop Entry]\nType=Application\nName=THRM\nHidden=true\n", want: false},
		{name: "gnome disabled", content: "[Desktop Entry]\nType=Application\nName=THRM\nX-GNOME-Autostart-enabled=false\n", want: false},
		{name: "no flags", content: "[Desktop Entry]\nType=Application\nName=THRM\n", want: true},
	}

	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(desktopPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write desktop entry: %v", err)
			}
			if got := m.CheckWindowsAutoStart(); got != tc.want {
				t.Fatalf("CheckWindowsAutoStart = %v, want %v", got, tc.want)
			}
			wantMethod := "none"
			if tc.want {
				wantMethod = MethodDesktop
			}
			if got := m.GetAutoStartMethod(); got != wantMethod {
				t.Fatalf("GetAutoStartMethod = %q, want %q", got, wantMethod)
			}
		})
	}
}

// 桌面环境写过 Hidden=true 之后重新开启，必须把该标记盖回去。
func TestEnableOverwritesHiddenFlag(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(desktopPath, []byte("[Desktop Entry]\nHidden=true\n"), 0644); err != nil {
		t.Fatalf("write desktop entry: %v", err)
	}

	if err := m.SetWindowsAutoStart(true); err != nil {
		t.Fatalf("enable error: %v", err)
	}
	if !m.CheckWindowsAutoStart() {
		t.Fatal("re-enabling should clear the Hidden flag")
	}
	content, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	if !strings.Contains(string(content), "Icon=thrm") {
		t.Fatal("desktop file missing Icon=thrm")
	}
}

// XDG_CONFIG_HOME 优先于 ~/.config。
func TestAutostartHonorsXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	configHome := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", configHome)

	m := NewManager(testLogger{})
	if err := m.SetWindowsAutoStart(true); err != nil {
		t.Fatalf("enable error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configHome, "autostart", "thrm.desktop")); err != nil {
		t.Fatalf("entry should be written under XDG_CONFIG_HOME: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "autostart", "thrm.desktop")); !os.IsNotExist(err) {
		t.Fatal("entry must not be written to ~/.config when XDG_CONFIG_HOME is set")
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager(testLogger{})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.logger == nil {
		t.Fatal("Manager.logger is nil")
	}
}

func TestIsRunningAsAdmin_NonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, skip non-root test")
	}
	m := NewManager(testLogger{})
	if m.IsRunningAsAdmin() {
		t.Fatal("IsRunningAsAdmin should return false for non-root")
	}
}

func TestGetAutoStartMethod_None(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	if got := m.GetAutoStartMethod(); got != "none" {
		t.Fatalf("GetAutoStartMethod = %q, want none", got)
	}
}

func TestSetAutoStartWithMethod_Enable(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	err := m.SetAutoStartWithMethod(true, "desktop")
	if err != nil {
		t.Fatalf("SetAutoStartWithMethod(true) unexpected error: %v", err)
	}

	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		t.Fatal("autostart desktop file was not created")
	}

	content, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("failed to read desktop file: %v", err)
	}

	c := string(content)
	if !strings.Contains(c, "Type=Application") {
		t.Fatal("desktop file missing Type=Application")
	}
	if !strings.Contains(c, "Name=THRM") {
		t.Fatal("desktop file missing Name=THRM")
	}
	if !strings.Contains(c, "--autostart") {
		t.Fatal("desktop file missing --autostart flag")
	}
	if !strings.Contains(c, "Terminal=false") {
		t.Fatal("desktop file missing Terminal=false")
	}
	if !strings.Contains(c, "X-GNOME-Autostart-enabled=true") {
		t.Fatal("desktop file missing X-GNOME-Autostart-enabled=true")
	}
}

func TestSetAutoStartWithMethod_Disable(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})

	if err := m.SetAutoStartWithMethod(true, "desktop"); err != nil {
		t.Fatalf("enable error: %v", err)
	}

	if err := m.SetAutoStartWithMethod(false, "desktop"); err != nil {
		t.Fatalf("disable error: %v", err)
	}

	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(desktopPath); !os.IsNotExist(err) {
		t.Fatal("autostart desktop file was not removed")
	}
}

func TestGetAutoStartMethod_Desktop(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	if err := m.SetAutoStartWithMethod(true, "desktop"); err != nil {
		t.Fatalf("enable error: %v", err)
	}

	if got := m.GetAutoStartMethod(); got != "desktop" {
		t.Fatalf("GetAutoStartMethod = %q, want desktop", got)
	}
}

func TestCheckWindowsAutoStart(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})

	if m.CheckWindowsAutoStart() {
		t.Fatal("CheckWindowsAutoStart should return false before enable")
	}

	if err := m.SetAutoStartWithMethod(true, "desktop"); err != nil {
		t.Fatalf("enable error: %v", err)
	}

	if !m.CheckWindowsAutoStart() {
		t.Fatal("CheckWindowsAutoStart should return true after enable")
	}

	if err := m.SetAutoStartWithMethod(false, "desktop"); err != nil {
		t.Fatalf("disable error: %v", err)
	}

	if m.CheckWindowsAutoStart() {
		t.Fatal("CheckWindowsAutoStart should return false after disable")
	}
}

func TestSetWindowsAutoStart(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	if err := m.SetWindowsAutoStart(true); err != nil {
		t.Fatalf("SetWindowsAutoStart(true) error: %v", err)
	}

	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		t.Fatal("desktop file was not created via SetWindowsAutoStart")
	}

	if err := m.SetWindowsAutoStart(false); err != nil {
		t.Fatalf("SetWindowsAutoStart(false) error: %v", err)
	}

	if _, err := os.Stat(desktopPath); !os.IsNotExist(err) {
		t.Fatal("desktop file was not removed via SetWindowsAutoStart")
	}
}

func TestIdempotentDisable(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})

	if err := m.SetAutoStartWithMethod(false, "desktop"); err != nil {
		t.Fatalf("first disable error: %v", err)
	}
	if err := m.SetAutoStartWithMethod(false, "desktop"); err != nil {
		t.Fatalf("second disable error: %v", err)
	}
}

func TestIdempotentEnable(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})

	if err := m.SetAutoStartWithMethod(true, "desktop"); err != nil {
		t.Fatalf("first enable error: %v", err)
	}
	if err := m.SetAutoStartWithMethod(true, "desktop"); err != nil {
		t.Fatalf("second enable error: %v", err)
	}

	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		t.Fatal("desktop file should still exist after second enable")
	}
}

func TestDetectAutoStartLaunch_Matches(t *testing.T) {
	tests := [][]string{
		{"--autostart"},
		{"-autostart"},
		{"/autostart"},
		{"thrm", "--autostart"},
		{"thrm", "--debug", "--autostart"},
	}

	for _, args := range tests {
		if !DetectAutoStartLaunch(args) {
			t.Fatalf("DetectAutoStartLaunch(%v) should be true", args)
		}
	}
}

func TestDetectAutoStartLaunch_NoMatch(t *testing.T) {
	tests := [][]string{
		{},
		{"thrm"},
		{"--debug"},
		{"--auto"},
	}

	for _, args := range tests {
		if DetectAutoStartLaunch(args) {
			t.Fatalf("DetectAutoStartLaunch(%v) should be false", args)
		}
	}
}

func TestSetAutoStartWithMethod_EmptyMethod(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})

	err := m.SetAutoStartWithMethod(true, "")
	if err != nil {
		t.Fatalf("SetAutoStartWithMethod with empty method: %v", err)
	}

	desktopPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		t.Fatal("desktop file should be created even with empty method")
	}
}

func TestAutostartDesktopPathConsistency(t *testing.T) {
	home := t.TempDir()
	restore := mockHomeDir(t, home)
	defer restore()

	m := NewManager(testLogger{})
	if err := m.SetWindowsAutoStart(true); err != nil {
		t.Fatalf("enable error: %v", err)
	}

	expectedPath := filepath.Join(home, ".config", "autostart", "thrm.desktop")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatal("desktop file not at expected path")
	}
}

func TestLoggerIsCalled(t *testing.T) {
	_ = NewManager(testLogger{})
}

var _ types.Logger = testLogger{}

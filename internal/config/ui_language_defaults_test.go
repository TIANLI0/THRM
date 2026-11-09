package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func loadRawConfig(t *testing.T, raw string) types.AppConfig {
	t.Helper()
	tmpDir := t.TempDir()
	isolateUserDirs(t, tmpDir)
	m := NewManager("", testLogger{})
	configDir := m.GetDefaultConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return m.Load(false)
}

// 升级前的配置没有 uiLanguage：保持旧版的中文托盘，直到 GUI 上报真实语言。
func TestLoadLegacyConfigKeepsChineseUILanguage(t *testing.T) {
	if got := loadRawConfig(t, `{"autoControl": true}`).UILanguage; got != "zh-CN" {
		t.Fatalf("旧配置的界面语言应补为 zh-CN，得到 %q", got)
	}
}

func TestLoadConfigNormalizesUILanguage(t *testing.T) {
	cases := map[string]string{
		`{"uiLanguage": "en-US"}`: "en-US",
		`{"uiLanguage": "ja"}`:    "ja-JP",
		`{"uiLanguage": "ko"}`:    "ko-KR",
		`{"uiLanguage": ""}`:      "",
		`{"uiLanguage": "xx-YY"}`: "",
	}
	for raw, want := range cases {
		if got := loadRawConfig(t, raw).UILanguage; got != want {
			t.Errorf("%s: UILanguage = %q, want %q", raw, got, want)
		}
	}
}

// 全新安装不写死语言，交给核心按系统界面语言推断。
func TestDefaultConfigLeavesUILanguageUnset(t *testing.T) {
	if got := types.GetDefaultConfig(false).UILanguage; got != "" {
		t.Fatalf("默认配置不应预设界面语言，得到 %q", got)
	}
}

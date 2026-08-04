package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/theme"
)

// embeddedThemes 内置默认主题（含官方 THRM 参考主题）。
//
// 作用：1) 首次运行时把这些主题播种到用户配置目录，方便用户直接编辑；
//  2. 当磁盘上的主题文件缺失时作为安全兜底，保证 THRM 始终可选。
//
//go:embed all:themes
var embeddedThemes embed.FS

// newThemeManager 基于当前可执行文件位置与用户配置目录构造主题管理器。
func newThemeManager() *theme.Manager {
	// 内置主题：把 embed 根从 "themes" 下沉，使路径形如 "thrm/theme.json"。
	var builtin fs.FS
	if sub, err := fs.Sub(embeddedThemes, "themes"); err == nil {
		builtin = sub
	}

	// 安装目录下的 themes（与可执行文件同级）仅作为只读来源。
	installThemesDir := ""
	if exePath, err := os.Executable(); err == nil {
		installThemesDir = filepath.Join(filepath.Dir(exePath), "themes")
	}

	// 用户配置目录下的 themes 是默认的播种/编辑目标。
	userThemesDir := ""
	home, _ := os.UserHomeDir()
	if userConfigDir := appmeta.UserConfigDir(home); userConfigDir != "" {
		userThemesDir = filepath.Join(userConfigDir, "themes")
	}

	return theme.NewManager(installThemesDir, userThemesDir, builtin)
}

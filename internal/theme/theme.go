// Package theme 负责「自定义界面主题」的发现、播种与读取。
package theme

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const (
	manifestName = "theme.json"
	styleName    = "theme.css"

	// SourceUser/SourceInstall/SourceBuiltin 标记主题来源，便于前端提示与排序。
	SourceUser    = "user"
	SourceInstall = "install"
	SourceBuiltin = "builtin"
)

// Meta 是主题清单（theme.json）解析后的元数据，同时作为返回给前端的结构。
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Base        string `json:"base"` // light | dark
	Author      string `json:"author,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"` // user | install | builtin
}

// Manager 统一管理主题的发现与读取。
type Manager struct {
	installDir string // 安装目录下的 themes 目录（只读来源）
	userDir    string // 用户配置目录下的 themes 目录（可写）
	builtin    fs.FS  // 程序内置的默认主题（已 Sub 到 themes 根，路径形如 thrm/theme.json）
}

// NewManager 创建主题管理器。
//   - installThemesDir：安装目录下的 themes 目录（一般与可执行文件同级）。
//   - userThemesDir：用户配置目录下的 themes 目录，用于保存可编辑副本。
//   - builtin：内置默认主题文件系统，根目录下应直接是各主题文件夹（thrm/...）。可为 nil。
func NewManager(installThemesDir, userThemesDir string, builtin fs.FS) *Manager {
	return &Manager{
		installDir: installThemesDir,
		userDir:    userThemesDir,
		builtin:    builtin,
	}
}

// validID 校验主题 id：仅允许小写字母、数字、连字符、下划线，避免被用作路径穿越或非法选择器。
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func normalizeBase(base string) string {
	if base == "dark" {
		return "dark"
	}
	return "light"
}

// EnsureSeeded 在首次运行时把内置主题播种到磁盘，方便用户直接编辑。
//
// 对每个内置主题：若安装目录与用户目录都不存在该主题，则写入用户目录。
// 安装目录始终被视为只读；失败不影响后续从内置读取。
func (m *Manager) EnsureSeeded() {
	if m.builtin == nil {
		return
	}
	entries, err := fs.ReadDir(m.builtin, ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if !validID(id) {
			continue
		}
		if m.themeExistsOnDisk(m.installDir, id) || m.themeExistsOnDisk(m.userDir, id) {
			continue
		}
		_ = m.copyBuiltin(id, m.userDir)
	}
}

func (m *Manager) themeExistsOnDisk(baseDir, id string) bool {
	if baseDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(baseDir, id, manifestName))
	return err == nil
}

// copyBuiltin 把内置主题 id 复制到 baseDir/id。
func (m *Manager) copyBuiltin(id, baseDir string) error {
	if baseDir == "" {
		return fmt.Errorf("目标目录为空")
	}
	srcRoot := id
	entries, err := fs.ReadDir(m.builtin, srcRoot)
	if err != nil {
		return err
	}
	dstDir := filepath.Join(baseDir, id)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue // 内置主题为扁平结构，忽略子目录
		}
		data, err := fs.ReadFile(m.builtin, srcRoot+"/"+entry.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, entry.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// List 返回所有可用主题，按来源合并去重（用户 > 安装 > 内置），并按名称排序。
func (m *Manager) List() []Meta {
	merged := map[string]Meta{}

	// 内置（最低优先级）
	if m.builtin != nil {
		if entries, err := fs.ReadDir(m.builtin, "."); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				if meta, ok := m.readBuiltinMeta(entry.Name()); ok {
					merged[meta.ID] = meta
				}
			}
		}
	}

	// 安装目录覆盖内置
	for _, meta := range m.scanDir(m.installDir, SourceInstall) {
		merged[meta.ID] = meta
	}
	// 用户目录优先级最高
	for _, meta := range m.scanDir(m.userDir, SourceUser) {
		merged[meta.ID] = meta
	}

	out := make([]Meta, 0, len(merged))
	for _, meta := range merged {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (m *Manager) scanDir(baseDir, source string) []Meta {
	if baseDir == "" {
		return nil
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}
	var out []Meta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(baseDir, entry.Name(), manifestName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		meta, ok := parseMeta(data, entry.Name())
		if !ok {
			continue
		}
		meta.Source = source
		out = append(out, meta)
	}
	return out
}

func (m *Manager) readBuiltinMeta(id string) (Meta, bool) {
	data, err := fs.ReadFile(m.builtin, id+"/"+manifestName)
	if err != nil {
		return Meta{}, false
	}
	meta, ok := parseMeta(data, id)
	if !ok {
		return Meta{}, false
	}
	meta.Source = SourceBuiltin
	return meta, true
}

// parseMeta 解析 theme.json；folderName 用于在清单缺少 id 时兜底，并校验 id 合法性。
func parseMeta(data []byte, folderName string) (Meta, bool) {
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, false
	}
	if meta.ID == "" {
		meta.ID = folderName
	}
	if !validID(meta.ID) {
		return Meta{}, false
	}
	if meta.Name == "" {
		meta.Name = meta.ID
	}
	meta.Base = normalizeBase(meta.Base)
	return meta, true
}

// ReadCSS 读取指定主题的 theme.css 内容。
// 查找顺序：用户目录 > 安装目录 > 内置，返回首个命中的内容。
func (m *Manager) ReadCSS(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("非法主题 id: %q", id)
	}

	for _, baseDir := range []string{m.userDir, m.installDir} {
		if baseDir == "" {
			continue
		}
		path := filepath.Join(baseDir, id, styleName)
		if data, err := os.ReadFile(path); err == nil {
			return string(data), nil
		}
	}

	if m.builtin != nil {
		if data, err := fs.ReadFile(m.builtin, id+"/"+styleName); err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("未找到主题 %q 的样式文件", id)
}

// ResolveDir 返回应优先暴露给用户编辑的 themes 目录（用于「打开主题文件夹」）。
func (m *Manager) ResolveDir() string {
	if m.userDir != "" {
		_ = os.MkdirAll(m.userDir, 0o755)
		return m.userDir
	}
	if m.installDir != "" {
		if _, err := os.Stat(m.installDir); err == nil {
			return m.installDir
		}
	}
	return m.installDir
}

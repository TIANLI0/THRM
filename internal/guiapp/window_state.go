package guiapp

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const windowStateFileName = "window.json"

const (
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
	minWindowWidth      = 800
	minWindowHeight     = 600
	windowPositionUnset = -1
)

// WindowState 记录窗口尺寸、位置与最大化状态。
type WindowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
}

// DefaultWindowState 返回默认窗口状态（居中、默认尺寸）。
func DefaultWindowState() WindowState {
	return WindowState{
		Width:     defaultWindowWidth,
		Height:    defaultWindowHeight,
		X:         windowPositionUnset,
		Y:         windowPositionUnset,
		Maximised: false,
	}
}

// windowStatePath 返回窗口状态文件路径，优先用户配置目录，失败时退回安装目录。
func windowStatePath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(appmeta.UserConfigDir(homeDir), windowStateFileName)
	}
	if exePath, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exePath), "config", windowStateFileName)
	}
	return windowStateFileName
}

// LoadWindowState 读取持久化的窗口状态；不存在或损坏时返回默认值。
func LoadWindowState() WindowState {
	state := DefaultWindowState()

	data, err := os.ReadFile(windowStatePath())
	if err != nil {
		return state
	}

	var loaded WindowState
	if err := json.Unmarshal(data, &loaded); err != nil {
		mainLogger.Warnf("窗口状态文件解析失败，使用默认值: %v", err)
		return state
	}

	if loaded.Width >= minWindowWidth {
		state.Width = loaded.Width
	}
	if loaded.Height >= minWindowHeight {
		state.Height = loaded.Height
	}
	state.X = loaded.X
	state.Y = loaded.Y
	state.Maximised = loaded.Maximised
	return state
}

// saveWindowState 将窗口状态写入磁盘（原子写入，避免半截文件）。
func saveWindowState(state WindowState) {
	path := windowStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		mainLogger.Errorf("创建窗口状态目录失败: %v", err)
		return
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		mainLogger.Errorf("序列化窗口状态失败: %v", err)
		return
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		mainLogger.Errorf("写入窗口状态失败: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		mainLogger.Errorf("替换窗口状态文件失败: %v", err)
		_ = os.Remove(tmp)
	}
}

// RestoreWindowPosition 供 main 在窗口启动后恢复上次记忆的位置。
func (a *App) RestoreWindowPosition(state WindowState) {
	a.applyWindowPosition(state)
}

// PersistWindowState 供外部（如前端或 main）主动触发窗口状态保存。
func (a *App) PersistWindowState() {
	a.captureWindowState()
}

// applyWindowPosition 在窗口启动后恢复位置：仅当记录了有效且落在屏幕范围内的坐标时才定位，
// 否则保持系统默认（居中），避免窗口被恢复到已断开的显示器之外。
func (a *App) applyWindowPosition(state WindowState) {
	if a.ctx == nil || state.Maximised {
		return
	}
	if state.X == windowPositionUnset || state.Y == windowPositionUnset {
		return
	}
	if !a.positionOnScreen(state.X, state.Y, state.Width, state.Height) {
		mainLogger.Warnf("记录的窗口位置 (%d,%d) 不在任何屏幕内，忽略并居中", state.X, state.Y)
		return
	}
	wailsruntime.WindowSetPosition(a.ctx, state.X, state.Y)
}

// positionOnScreen 粗粒度判断窗口左上角是否可接受。
func (a *App) positionOnScreen(x, y, width, height int) bool {
	screens, err := wailsruntime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return true
	}

	totalWidth, maxHeight := 0, 0
	for _, s := range screens {
		totalWidth += s.Size.Width
		if s.Size.Height > maxHeight {
			maxHeight = s.Size.Height
		}
	}
	if totalWidth <= 0 || maxHeight <= 0 {
		return true
	}

	// 允许标题栏部分越界，但至少要留出一段可点击区域用于拖回。
	const margin = 120
	return x >= -margin && x <= totalWidth-margin &&
		y >= -margin && y <= maxHeight-margin
}

// captureWindowState 从当前运行时读取窗口几何信息并持久化。
func (a *App) captureWindowState() {
	if a.ctx == nil {
		return
	}

	maximised := wailsruntime.WindowIsMaximised(a.ctx)

	if maximised {
		prev := LoadWindowState()
		prev.Maximised = true
		saveWindowState(prev)
		return
	}

	w, h := wailsruntime.WindowGetSize(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)

	state := WindowState{
		Width:     w,
		Height:    h,
		X:         x,
		Y:         y,
		Maximised: false,
	}
	if w < minWindowWidth || h < minWindowHeight {
		prev := LoadWindowState()
		state.Width = prev.Width
		state.Height = prev.Height
	}
	saveWindowState(state)
}

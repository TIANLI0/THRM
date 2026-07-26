package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ShowWindow 显示主窗口
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}

// HideWindow 隐藏主窗口到托盘
func (a *App) HideWindow() {
	if a.ctx != nil {
		a.captureWindowState()
		runtime.WindowHide(a.ctx)
	}
}

// QuitApp 完全退出应用
func (a *App) QuitApp() {
	guiLogger.Info("GUI 请求退出")

	// 先让看护协程停手，否则它会把随后关闭的连接当成"核心掉线"而尝试重连。
	a.shuttingDown.Store(true)
	a.captureWindowState()

	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// QuitAll 完全退出应用（包括核心服务）
func (a *App) QuitAll() {
	guiLogger.Info("GUI 请求完全退出（包括核心服务）")

	// 必须在通知核心退出之前置位：否则核心退出与本进程退出之间的窗口里，
	// 看护协程会把核心当成掉线并把它重新拉起来。
	a.shuttingDown.Store(true)
	a.captureWindowState()

	a.sendRequest(ipc.ReqQuitApp, nil)

	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// InitSystemTray 初始化系统托盘（保持API兼容，实际由核心服务处理）
func (a *App) InitSystemTray() {
	// 托盘由核心服务管理，GUI 不需要处理
}

// UpdateGuiResponseTime 更新GUI响应时间（供前端调用）
func (a *App) UpdateGuiResponseTime() error {
	_, err := a.sendRequest(ipc.ReqUpdateGuiResponseTime, nil)
	return err
}

// GetDebugInfo 获取调试信息
func (a *App) GetDebugInfo() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDebugInfo, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var info map[string]any
	json.Unmarshal(resp.Data, &info)
	return info
}

// SetDebugMode 设置调试模式
func (a *App) SetDebugMode(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetDebugMode, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (a *App) SendDeviceDebugCommand(hexCommand string, waitMs int) (DeviceDebugCommandResult, error) {
	resp, err := a.sendRequest(ipc.ReqSendDeviceDebugCommand, ipc.DeviceDebugCommandParams{Hex: hexCommand, WaitMs: waitMs})
	if err != nil {
		return DeviceDebugCommandResult{}, err
	}
	if !resp.Success {
		return DeviceDebugCommandResult{}, fmt.Errorf("%s", resp.Error)
	}
	var result DeviceDebugCommandResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return DeviceDebugCommandResult{}, err
	}
	return result, nil
}

func (a *App) GetDeviceDebugFrames() []DeviceDebugFrame {
	resp, err := a.sendRequest(ipc.ReqGetDeviceDebugFrames, nil)
	if err != nil || !resp.Success {
		return nil
	}
	var frames []DeviceDebugFrame
	json.Unmarshal(resp.Data, &frames)
	return frames
}

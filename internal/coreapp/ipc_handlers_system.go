package coreapp

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// 自启动相关

func (a *CoreApp) handleIPCSetWindowsAutoStart(req ipc.Request) ipc.Response {
	var params ipc.SetBoolParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetWindowsAutoStart(params.Enabled); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCCheckWindowsAutoStart(_ ipc.Request) ipc.Response {
	enabled := a.autostartManager.CheckWindowsAutoStart()
	return a.dataResponse(enabled)
}

func (a *CoreApp) handleIPCIsRunningAsAdmin(_ ipc.Request) ipc.Response {
	isAdmin := a.autostartManager.IsRunningAsAdmin()
	return a.dataResponse(isAdmin)
}

func (a *CoreApp) handleIPCGetAutoStartMethod(_ ipc.Request) ipc.Response {
	method := a.autostartManager.GetAutoStartMethod()
	return a.dataResponse(method)
}

func (a *CoreApp) handleIPCSetAutoStartWithMethod(req ipc.Request) ipc.Response {
	var params ipc.SetAutoStartWithMethodParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.autostartManager.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
		return a.errorResponse(err.Error())
	}
	// 只改系统自启动项而不落盘的话，下一次配置广播会把 GUI 上的开关打回去。
	a.persistWindowsAutoStart(params.Enable)
	return a.successResponse(true)
}

// 窗口相关

func (a *CoreApp) handleIPCShowWindow(_ ipc.Request) ipc.Response {
	a.onShowWindowRequest()
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCHideWindow(_ ipc.Request) ipc.Response {
	// GUI 自己处理隐藏
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCQuitApp(_ ipc.Request) ipc.Response {
	a.safeGo("onQuitRequest", func() {
		a.onQuitRequest()
	})
	return a.successResponse(true)
}

// 调试相关

func (a *CoreApp) handleIPCGetDebugInfo(_ ipc.Request) ipc.Response {
	info := a.GetDebugInfo()
	return a.dataResponse(info)
}

func (a *CoreApp) handleIPCSetDebugMode(req ipc.Request) ipc.Response {
	var params ipc.SetBoolParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetDebugMode(params.Enabled); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCSendDeviceDebugCommand(req ipc.Request) ipc.Response {
	var params ipc.DeviceDebugCommandParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	result, err := a.SendDeviceDebugCommand(params.Hex, params.WaitMs)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(result)
}

func (a *CoreApp) handleIPCGetDeviceDebugFrames(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetDeviceDebugFrames())
}

func (a *CoreApp) handleIPCUpdateGuiResponseTime(_ ipc.Request) ipc.Response {
	atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
	return a.successResponse(true)
}

// 系统相关

func (a *CoreApp) handleIPCPing(_ ipc.Request) ipc.Response {
	return a.dataResponse("pong")
}

func (a *CoreApp) handleIPCIsAutoStartLaunch(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.isAutoStartLaunch)
}

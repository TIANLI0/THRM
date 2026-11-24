package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// 设备相关

func (a *CoreApp) handleIPCConnect(_ ipc.Request) ipc.Response {
	success := a.ConnectDevice()
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCDisconnect(_ ipc.Request) ipc.Response {
	a.DisconnectDevice()
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCGetDeviceStatus(_ ipc.Request) ipc.Response {
	status := a.GetDeviceStatus()
	return a.dataResponse(status)
}

func (a *CoreApp) handleIPCGetCurrentFanData(_ ipc.Request) ipc.Response {
	data := a.deviceManager.GetCurrentFanData()
	return a.dataResponse(data)
}

func (a *CoreApp) handleIPCRefreshDeviceSettings(_ ipc.Request) ipc.Response {
	settings, err := a.RefreshDeviceSettings()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(settings)
}

func (a *CoreApp) handleIPCReinitializeFirmware(req ipc.Request) ipc.Response {
	var factoryReset bool
	if len(req.Data) > 0 && string(req.Data) != "null" {
		if err := json.Unmarshal(req.Data, &factoryReset); err != nil {
			return a.errorResponse("解析固件重新初始化参数失败: " + err.Error())
		}
	}
	settings, err := a.ReinitializeDeviceFirmware(factoryReset)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(settings)
}

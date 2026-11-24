package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// 固件更新

func (a *CoreApp) handleIPCGetFirmwareUpdateStatus(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetFirmwareUpdateStatus())
}

func (a *CoreApp) handleIPCListFirmwareUpdates(_ ipc.Request) ipc.Response {
	list, err := a.ListFirmwareUpdates()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(list)
}

func (a *CoreApp) handleIPCInstallFirmwareDriver(_ ipc.Request) ipc.Response {
	if err := a.InstallFirmwareDriver(); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCUninstallFirmwareDriver(_ ipc.Request) ipc.Response {
	if err := a.UninstallFirmwareDriver(); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCFlashFirmware(req ipc.Request) ipc.Response {
	var params ipc.FlashFirmwareParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析固件刷写参数失败: " + err.Error())
	}
	if err := a.FlashFirmware(params); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

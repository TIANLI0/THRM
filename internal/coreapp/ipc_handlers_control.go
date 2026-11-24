package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// 控制相关

func (a *CoreApp) handleIPCSetAutoControl(req ipc.Request) ipc.Response {
	var params ipc.SetAutoControlParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetAutoControl(params.Enabled); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCSetManualGear(req ipc.Request) ipc.Response {
	var params ipc.SetManualGearParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	success := a.SetManualGear(params.Gear, params.Level)
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCGetAvailableGears(_ ipc.Request) ipc.Response {
	gears := types.GearCommands
	return a.dataResponse(gears)
}

func (a *CoreApp) handleIPCSetCustomSpeed(req ipc.Request) ipc.Response {
	var params ipc.SetCustomSpeedParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCSetGearLight(req ipc.Request) ipc.Response {
	var params ipc.SetBoolParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	success := a.SetGearLight(params.Enabled)
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCSetPowerOnStart(req ipc.Request) ipc.Response {
	var params ipc.SetBoolParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	success := a.SetPowerOnStart(params.Enabled)
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCSetSmartStartStop(req ipc.Request) ipc.Response {
	var params ipc.SetStringParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	success := a.SetSmartStartStop(params.Value)
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCSetBrightness(req ipc.Request) ipc.Response {
	var params ipc.SetIntParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	success := a.SetBrightness(params.Value)
	return a.successResponse(success)
}

func (a *CoreApp) handleIPCSetLightStrip(req ipc.Request) ipc.Response {
	var params ipc.SetLightStripParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetLightStrip(params.Config); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCGetSmartLightStatus(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetSmartLightStatus())
}

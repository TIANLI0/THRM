package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// 温度相关

func (a *CoreApp) handleIPCGetTemperature(_ ipc.Request) ipc.Response {
	a.mutex.RLock()
	temp := a.currentTemp
	a.mutex.RUnlock()
	return a.dataResponse(temp)
}

func (a *CoreApp) handleIPCGetTemperatureHistory(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.tempHistory.Snapshot())
}

func (a *CoreApp) handleIPCSetTemperatureHistoryEnabled(req ipc.Request) ipc.Response {
	var params ipc.SetBoolParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetTemperatureHistoryEnabled(params.Enabled); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCSetTemperatureHistoryRetentionHours(req ipc.Request) ipc.Response {
	var params ipc.SetIntParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.SetTemperatureHistoryRetentionHours(params.Value); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCTestTemperatureReading(_ ipc.Request) ipc.Response {
	temp := a.tempReader.Read(a.temperatureSelection(a.configManager.Get()))
	return a.dataResponse(temp)
}

func (a *CoreApp) handleIPCTestBridgeProgram(_ ipc.Request) ipc.Response {
	data := a.bridgeManager.GetTemperature(a.temperatureSelection(a.configManager.Get()))
	return a.dataResponse(data)
}

func (a *CoreApp) handleIPCGetBridgeProgramStatus(_ ipc.Request) ipc.Response {
	status := a.bridgeManager.GetStatus()
	return a.dataResponse(status)
}

func (a *CoreApp) handleIPCRestartPawnIO(_ ipc.Request) ipc.Response {
	result, err := a.bridgeManager.RestartPawnIO()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(result)
}

func (a *CoreApp) handleIPCReinstallPawnIO(_ ipc.Request) ipc.Response {
	result, err := a.ReinstallPawnIO()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(result)
}

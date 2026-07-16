package coreapp

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	a.logDebug("处理 IPC 请求[%s] type=%s", req.RequestID, req.Type)

	switch req.Type {
	// 设备相关
	case ipc.ReqConnect:
		success := a.ConnectDevice()
		return a.successResponse(success)

	case ipc.ReqDisconnect:
		a.DisconnectDevice()
		return a.successResponse(true)

	case ipc.ReqGetDeviceStatus:
		status := a.GetDeviceStatus()
		return a.dataResponse(status)

	case ipc.ReqGetCurrentFanData:
		data := a.deviceManager.GetCurrentFanData()
		return a.dataResponse(data)

	case ipc.ReqRefreshDeviceSettings:
		settings, err := a.RefreshDeviceSettings()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(settings)

	// 配置相关
	case ipc.ReqGetConfig:
		cfg := a.configManager.Get()
		return a.dataResponse(cfg)

	case ipc.ReqUpdateConfig:
		var cfg types.AppConfig
		if err := json.Unmarshal(req.Data, &cfg); err != nil {
			return a.errorResponse("解析配置失败: " + err.Error())
		}
		if err := a.UpdateConfig(cfg); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetFanCurve:
		var curve []types.FanCurvePoint
		if err := json.Unmarshal(req.Data, &curve); err != nil {
			return a.errorResponse("解析风扇曲线失败: " + err.Error())
		}
		if err := a.SetFanCurve(curve); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqGetFanCurve:
		curve := a.configManager.Get().FanCurve
		return a.dataResponse(curve)

	case ipc.ReqResetLearnedOffsets:
		if err := a.ResetLearnedOffsets(); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(map[string]bool{"ok": true})

	case ipc.ReqGetFanCurveProfiles:
		return a.dataResponse(a.GetFanCurveProfiles())

	case ipc.ReqSetActiveFanCurveProfile:
		var params ipc.SetActiveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		profile, err := a.SetActiveFanCurveProfile(params.ID)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqSaveFanCurveProfile:
		var params ipc.SaveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		profile, err := a.SaveFanCurveProfile(params)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqDeleteFanCurveProfile:
		var params ipc.DeleteFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.DeleteFanCurveProfile(params.ID); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqExportFanCurveProfiles:
		code, err := a.ExportFanCurveProfiles()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(code)

	case ipc.ReqImportFanCurveProfiles:
		var params ipc.ImportFanCurveProfilesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.ImportFanCurveProfiles(params.Code); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 控制相关
	case ipc.ReqSetAutoControl:
		var params ipc.SetAutoControlParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetAutoControl(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetManualGear:
		var params ipc.SetManualGearParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetManualGear(params.Gear, params.Level)
		return a.successResponse(success)

	case ipc.ReqGetAvailableGears:
		gears := types.GearCommands
		return a.dataResponse(gears)

	case ipc.ReqSetCustomSpeed:
		var params ipc.SetCustomSpeedParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetGearLight:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetGearLight(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetPowerOnStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetPowerOnStart(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetSmartStartStop:
		var params ipc.SetStringParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetSmartStartStop(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetBrightness:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetBrightness(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetLightStrip:
		var params ipc.SetLightStripParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetLightStrip(params.Config); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 温度相关
	case ipc.ReqGetTemperature:
		a.mutex.RLock()
		temp := a.currentTemp
		a.mutex.RUnlock()
		return a.dataResponse(temp)

	case ipc.ReqGetTemperatureHistory:
		return a.dataResponse(a.tempHistory.Snapshot())

	case ipc.ReqSetTemperatureHistoryEnabled:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetTemperatureHistoryEnabled(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqTestTemperatureReading:
		cfg := a.configManager.Get()
		temp := a.tempReader.Read(types.TemperatureSelection{
			TempSource: cfg.TempSource,
			GpuDevice:  cfg.GpuDevice,
			CpuSensor:  cfg.CpuSensor,
			GpuSensor:  cfg.GpuSensor,
			DisableGpu: cfg.DisableGpuMonitoring,
		})
		return a.dataResponse(temp)

	case ipc.ReqTestBridgeProgram:
		cfg := a.configManager.Get()
		data := a.bridgeManager.GetTemperature(types.TemperatureSelection{
			TempSource: cfg.TempSource,
			GpuDevice:  cfg.GpuDevice,
			CpuSensor:  cfg.CpuSensor,
			GpuSensor:  cfg.GpuSensor,
			DisableGpu: cfg.DisableGpuMonitoring,
		})
		return a.dataResponse(data)

	case ipc.ReqGetBridgeProgramStatus:
		status := a.bridgeManager.GetStatus()
		return a.dataResponse(status)

	case ipc.ReqRestartPawnIO:
		result, err := a.bridgeManager.RestartPawnIO()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(result)

	case ipc.ReqReinstallPawnIO:
		result, err := a.ReinstallPawnIO()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(result)

	// 自启动相关
	case ipc.ReqSetWindowsAutoStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetWindowsAutoStart(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqCheckWindowsAutoStart:
		enabled := a.autostartManager.CheckWindowsAutoStart()
		return a.dataResponse(enabled)

	case ipc.ReqIsRunningAsAdmin:
		isAdmin := a.autostartManager.IsRunningAsAdmin()
		return a.dataResponse(isAdmin)

	case ipc.ReqGetAutoStartMethod:
		method := a.autostartManager.GetAutoStartMethod()
		return a.dataResponse(method)

	case ipc.ReqSetAutoStartWithMethod:
		var params ipc.SetAutoStartWithMethodParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.autostartManager.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 窗口相关
	case ipc.ReqShowWindow:
		a.onShowWindowRequest()
		return a.successResponse(true)

	case ipc.ReqHideWindow:
		// GUI 自己处理隐藏
		return a.successResponse(true)

	case ipc.ReqQuitApp:
		a.safeGo("onQuitRequest", func() {
			a.onQuitRequest()
		})
		return a.successResponse(true)

	// 调试相关
	case ipc.ReqGetDebugInfo:
		info := a.GetDebugInfo()
		return a.dataResponse(info)

	case ipc.ReqSetDebugMode:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetDebugMode(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSendDeviceDebugCommand:
		var params ipc.DeviceDebugCommandParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		result, err := a.SendDeviceDebugCommand(params.Hex, params.WaitMs)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(result)

	case ipc.ReqGetDeviceDebugFrames:
		return a.dataResponse(a.GetDeviceDebugFrames())

	case ipc.ReqUpdateGuiResponseTime:
		atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
		return a.successResponse(true)

	// 系统相关
	case ipc.ReqPing:
		return a.dataResponse("pong")

	case ipc.ReqIsAutoStartLaunch:
		return a.dataResponse(a.isAutoStartLaunch)

	default:
		return a.errorResponse(fmt.Sprintf("未知的请求类型: %s", req.Type))
	}
}

// 响应辅助方法
func (a *CoreApp) successResponse(success bool) ipc.Response {
	data, _ := json.Marshal(success)
	return ipc.Response{Success: true, Data: data}
}

func (a *CoreApp) errorResponse(errMsg string) ipc.Response {
	return ipc.Response{Success: false, Error: errMsg}
}

func (a *CoreApp) dataResponse(data any) ipc.Response {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return a.errorResponse("序列化数据失败: " + err.Error())
	}
	return ipc.Response{Success: true, Data: dataBytes}
}

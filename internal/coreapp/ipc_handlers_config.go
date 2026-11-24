package coreapp

import (
	"encoding/json"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// 配置相关

func (a *CoreApp) handleIPCGetConfig(_ ipc.Request) ipc.Response {
	cfg := a.configManager.Get()
	return a.dataResponse(cfg)
}

func (a *CoreApp) handleIPCUpdateConfig(req ipc.Request) ipc.Response {
	var cfg types.AppConfig
	if err := json.Unmarshal(req.Data, &cfg); err != nil {
		return a.errorResponse("解析配置失败: " + err.Error())
	}
	if err := a.UpdateConfig(cfg); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCGetFlydigiCompatStatus(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetFlydigiCompatStatus())
}

func (a *CoreApp) handleIPCSetFlydigiCompat(req ipc.Request) ipc.Response {
	var enabled bool
	if err := json.Unmarshal(req.Data, &enabled); err != nil {
		return a.errorResponse("解析飞智兼容开关失败: " + err.Error())
	}
	status, err := a.SetFlydigiCompat(enabled)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(status)
}

func (a *CoreApp) handleIPCPreviewRTSSPosition(req ipc.Request) ipc.Response {
	var params ipc.PreviewRTSSPositionParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析 RTSS 位置失败: " + err.Error())
	}
	a.PreviewRTSSPosition(params.Mode, params.X, params.Y)
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCSetUILanguage(req ipc.Request) ipc.Response {
	var params ipc.SetStringParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析界面语言失败: " + err.Error())
	}
	lang, err := a.SetUILanguage(params.Value)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(lang)
}

func (a *CoreApp) handleIPCSetFanCurve(req ipc.Request) ipc.Response {
	var curve []types.FanCurvePoint
	if err := json.Unmarshal(req.Data, &curve); err != nil {
		return a.errorResponse("解析风扇曲线失败: " + err.Error())
	}
	if err := a.SetFanCurve(curve); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCGetFanCurve(_ ipc.Request) ipc.Response {
	curve := a.configManager.Get().FanCurve
	return a.dataResponse(curve)
}

func (a *CoreApp) handleIPCResetLearnedOffsets(_ ipc.Request) ipc.Response {
	if err := a.ResetLearnedOffsets(); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(map[string]bool{"ok": true})
}

func (a *CoreApp) handleIPCSetExtendedSensors(req ipc.Request) ipc.Response {
	var enabled bool
	if err := json.Unmarshal(req.Data, &enabled); err != nil {
		return a.errorResponse("解析扩展传感器开关失败: " + err.Error())
	}
	return a.dataResponse(map[string]bool{"enabled": a.SetExtendedSensors(enabled)})
}

func (a *CoreApp) handleIPCGetCoolingBenefit(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetCoolingBenefit())
}

func (a *CoreApp) handleIPCSaveCoolingBenefitReport(req ipc.Request) ipc.Response {
	var params ipc.SaveCoolingBenefitReportParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析散热收益测试结果失败: " + err.Error())
	}
	payload, err := a.SaveCoolingBenefitReport(params)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(payload)
}

func (a *CoreApp) handleIPCExportCoolingBenefitText(_ ipc.Request) ipc.Response {
	text, err := a.ExportCoolingBenefitText()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(text)
}

func (a *CoreApp) handleIPCClearCoolingBenefit(_ ipc.Request) ipc.Response {
	payload, err := a.ClearCoolingBenefit()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(payload)
}

func (a *CoreApp) handleIPCGetFanCurveProfiles(_ ipc.Request) ipc.Response {
	return a.dataResponse(a.GetFanCurveProfiles())
}

func (a *CoreApp) handleIPCSetActiveFanCurveProfile(req ipc.Request) ipc.Response {
	var params ipc.SetActiveFanCurveProfileParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	profile, err := a.SetActiveFanCurveProfile(params.ID)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(profile)
}

func (a *CoreApp) handleIPCSaveFanCurveProfile(req ipc.Request) ipc.Response {
	var params ipc.SaveFanCurveProfileParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	profile, err := a.SaveFanCurveProfile(params)
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(profile)
}

func (a *CoreApp) handleIPCDeleteFanCurveProfile(req ipc.Request) ipc.Response {
	var params ipc.DeleteFanCurveProfileParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.DeleteFanCurveProfile(params.ID); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

func (a *CoreApp) handleIPCExportFanCurveProfiles(_ ipc.Request) ipc.Response {
	code, err := a.ExportFanCurveProfiles()
	if err != nil {
		return a.errorResponse(err.Error())
	}
	return a.dataResponse(code)
}

func (a *CoreApp) handleIPCImportFanCurveProfiles(req ipc.Request) ipc.Response {
	var params ipc.ImportFanCurveProfilesParams
	if err := json.Unmarshal(req.Data, &params); err != nil {
		return a.errorResponse("解析参数失败: " + err.Error())
	}
	if err := a.ImportFanCurveProfiles(params.Code); err != nil {
		return a.errorResponse(err.Error())
	}
	return a.successResponse(true)
}

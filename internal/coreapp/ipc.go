package coreapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// ipcRequestHandler 是单个 IPC 请求类型的处理函数。
type ipcRequestHandler func(a *CoreApp, req ipc.Request) ipc.Response

// ipcRequestHandlers 是请求类型到处理函数的分发表，各处理函数按领域放在 ipc_handlers_*.go。
// ipc.ReqSubscribeEvents / ipc.ReqUnsubscribeEvents 由 ipc.Server 直接处理，不经过这里。
var ipcRequestHandlers = map[ipc.RequestType]ipcRequestHandler{
	// 设备相关
	ipc.ReqConnect:               (*CoreApp).handleIPCConnect,
	ipc.ReqDisconnect:            (*CoreApp).handleIPCDisconnect,
	ipc.ReqGetDeviceStatus:       (*CoreApp).handleIPCGetDeviceStatus,
	ipc.ReqGetCurrentFanData:     (*CoreApp).handleIPCGetCurrentFanData,
	ipc.ReqRefreshDeviceSettings: (*CoreApp).handleIPCRefreshDeviceSettings,
	ipc.ReqReinitializeFirmware:  (*CoreApp).handleIPCReinitializeFirmware,

	// 配置相关
	ipc.ReqGetConfig:                (*CoreApp).handleIPCGetConfig,
	ipc.ReqUpdateConfig:             (*CoreApp).handleIPCUpdateConfig,
	ipc.ReqGetFlydigiCompatStatus:   (*CoreApp).handleIPCGetFlydigiCompatStatus,
	ipc.ReqSetFlydigiCompat:         (*CoreApp).handleIPCSetFlydigiCompat,
	ipc.ReqPreviewRTSSPosition:      (*CoreApp).handleIPCPreviewRTSSPosition,
	ipc.ReqSetUILanguage:            (*CoreApp).handleIPCSetUILanguage,
	ipc.ReqSetFanCurve:              (*CoreApp).handleIPCSetFanCurve,
	ipc.ReqGetFanCurve:              (*CoreApp).handleIPCGetFanCurve,
	ipc.ReqResetLearnedOffsets:      (*CoreApp).handleIPCResetLearnedOffsets,
	ipc.ReqSetExtendedSensors:       (*CoreApp).handleIPCSetExtendedSensors,
	ipc.ReqGetCoolingBenefit:        (*CoreApp).handleIPCGetCoolingBenefit,
	ipc.ReqSaveCoolingBenefitReport: (*CoreApp).handleIPCSaveCoolingBenefitReport,
	ipc.ReqExportCoolingBenefitText: (*CoreApp).handleIPCExportCoolingBenefitText,
	ipc.ReqClearCoolingBenefit:      (*CoreApp).handleIPCClearCoolingBenefit,
	ipc.ReqGetFanCurveProfiles:      (*CoreApp).handleIPCGetFanCurveProfiles,
	ipc.ReqSetActiveFanCurveProfile: (*CoreApp).handleIPCSetActiveFanCurveProfile,
	ipc.ReqSaveFanCurveProfile:      (*CoreApp).handleIPCSaveFanCurveProfile,
	ipc.ReqDeleteFanCurveProfile:    (*CoreApp).handleIPCDeleteFanCurveProfile,
	ipc.ReqExportFanCurveProfiles:   (*CoreApp).handleIPCExportFanCurveProfiles,
	ipc.ReqImportFanCurveProfiles:   (*CoreApp).handleIPCImportFanCurveProfiles,

	// 控制相关
	ipc.ReqSetAutoControl:      (*CoreApp).handleIPCSetAutoControl,
	ipc.ReqSetManualGear:       (*CoreApp).handleIPCSetManualGear,
	ipc.ReqGetAvailableGears:   (*CoreApp).handleIPCGetAvailableGears,
	ipc.ReqSetCustomSpeed:      (*CoreApp).handleIPCSetCustomSpeed,
	ipc.ReqSetGearLight:        (*CoreApp).handleIPCSetGearLight,
	ipc.ReqSetPowerOnStart:     (*CoreApp).handleIPCSetPowerOnStart,
	ipc.ReqSetSmartStartStop:   (*CoreApp).handleIPCSetSmartStartStop,
	ipc.ReqSetBrightness:       (*CoreApp).handleIPCSetBrightness,
	ipc.ReqSetLightStrip:       (*CoreApp).handleIPCSetLightStrip,
	ipc.ReqGetSmartLightStatus: (*CoreApp).handleIPCGetSmartLightStatus,

	// 固件更新
	ipc.ReqGetFirmwareUpdateStatus: (*CoreApp).handleIPCGetFirmwareUpdateStatus,
	ipc.ReqListFirmwareUpdates:     (*CoreApp).handleIPCListFirmwareUpdates,
	ipc.ReqInstallFirmwareDriver:   (*CoreApp).handleIPCInstallFirmwareDriver,
	ipc.ReqUninstallFirmwareDriver: (*CoreApp).handleIPCUninstallFirmwareDriver,
	ipc.ReqFlashFirmware:           (*CoreApp).handleIPCFlashFirmware,

	// 温度相关
	ipc.ReqGetTemperature:                      (*CoreApp).handleIPCGetTemperature,
	ipc.ReqGetTemperatureHistory:               (*CoreApp).handleIPCGetTemperatureHistory,
	ipc.ReqSetTemperatureHistoryEnabled:        (*CoreApp).handleIPCSetTemperatureHistoryEnabled,
	ipc.ReqSetTemperatureHistoryRetentionHours: (*CoreApp).handleIPCSetTemperatureHistoryRetentionHours,
	ipc.ReqTestTemperatureReading:              (*CoreApp).handleIPCTestTemperatureReading,
	ipc.ReqTestBridgeProgram:                   (*CoreApp).handleIPCTestBridgeProgram,
	ipc.ReqGetBridgeProgramStatus:              (*CoreApp).handleIPCGetBridgeProgramStatus,
	ipc.ReqRestartPawnIO:                       (*CoreApp).handleIPCRestartPawnIO,
	ipc.ReqReinstallPawnIO:                     (*CoreApp).handleIPCReinstallPawnIO,

	// 自启动相关
	ipc.ReqSetWindowsAutoStart:    (*CoreApp).handleIPCSetWindowsAutoStart,
	ipc.ReqCheckWindowsAutoStart:  (*CoreApp).handleIPCCheckWindowsAutoStart,
	ipc.ReqIsRunningAsAdmin:       (*CoreApp).handleIPCIsRunningAsAdmin,
	ipc.ReqGetAutoStartMethod:     (*CoreApp).handleIPCGetAutoStartMethod,
	ipc.ReqSetAutoStartWithMethod: (*CoreApp).handleIPCSetAutoStartWithMethod,

	// 窗口相关
	ipc.ReqShowWindow: (*CoreApp).handleIPCShowWindow,
	ipc.ReqHideWindow: (*CoreApp).handleIPCHideWindow,
	ipc.ReqQuitApp:    (*CoreApp).handleIPCQuitApp,

	// 调试相关
	ipc.ReqGetDebugInfo:           (*CoreApp).handleIPCGetDebugInfo,
	ipc.ReqSetDebugMode:           (*CoreApp).handleIPCSetDebugMode,
	ipc.ReqSendDeviceDebugCommand: (*CoreApp).handleIPCSendDeviceDebugCommand,
	ipc.ReqGetDeviceDebugFrames:   (*CoreApp).handleIPCGetDeviceDebugFrames,
	ipc.ReqUpdateGuiResponseTime:  (*CoreApp).handleIPCUpdateGuiResponseTime,

	// 系统相关
	ipc.ReqPing:              (*CoreApp).handleIPCPing,
	ipc.ReqIsAutoStartLaunch: (*CoreApp).handleIPCIsAutoStartLaunch,
}

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	a.logDebug("处理 IPC 请求[%s] type=%s", req.RequestID, req.Type)

	if handler, ok := ipcRequestHandlers[req.Type]; ok {
		return handler(a, req)
	}
	return a.errorResponse(fmt.Sprintf("未知的请求类型: %s", req.Type))
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

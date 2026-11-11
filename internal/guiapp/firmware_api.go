package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/fwupdate"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SelectFirmwareFile 弹出原生文件对话框，让用户选本地 .hex/.bin 固件，返回路径。
func (a *App) SelectFirmwareFile() (string, error) {
	if a.GetDebugInfo()["debugMode"] != true {
		return "", fmt.Errorf("固件更新仅在调试模式下可用，请先开启调试模式")
	}
	return a.promptOpenFile(application.OpenFileDialogOptions{
		Title: "选择固件文件",
		Filters: []application.FileFilter{
			{DisplayName: "固件 (*.hex;*.bin)", Pattern: "*.hex;*.bin"},
		},
	})
}

// FirmwareUpdateStatus 是固件更新面板所需状态（与 coreapp 同构，经 IPC 传输）。
type FirmwareUpdateStatus struct {
	Supported        bool   `json:"supported"`
	DeviceCode       string `json:"deviceCode"`
	ModelName        string `json:"modelName"`
	NormalConnected  bool   `json:"normalConnected"`
	DevicePresentISP bool   `json:"devicePresentIsp"`
	DriverInstalled  bool   `json:"driverInstalled"`
	CurrentVersion   string `json:"currentVersion"`
	ThrmVersion      int    `json:"thrmVersion"`
}

// GetFirmwareUpdateStatus 返回固件更新面板状态。
func (a *App) GetFirmwareUpdateStatus() FirmwareUpdateStatus {
	var st FirmwareUpdateStatus
	resp, err := a.sendRequest(ipc.ReqGetFirmwareUpdateStatus, nil)
	if err == nil && resp.Success {
		json.Unmarshal(resp.Data, &st)
	}
	return st
}

// ListFirmwareUpdates 返回官方 + THRM 托管的可刷固件清单。
func (a *App) ListFirmwareUpdates() ([]fwupdate.FirmwareInfo, error) {
	resp, err := a.sendRequest(ipc.ReqListFirmwareUpdates, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var list []fwupdate.FirmwareInfo
	if err := json.Unmarshal(resp.Data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// InstallFirmwareDriver 为下载模式设备安装 WinUSB 驱动。
func (a *App) InstallFirmwareDriver() error {
	resp, err := a.sendRequest(ipc.ReqInstallFirmwareDriver, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// UninstallFirmwareDriver 卸载 THRM 装的 WinUSB 驱动。
func (a *App) UninstallFirmwareDriver() error {
	resp, err := a.sendRequest(ipc.ReqUninstallFirmwareDriver, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// FlashFirmware 开始刷写（异步）。进度/完成通过 "firmware-update" 事件下发。
func (a *App) FlashFirmware(params ipc.FlashFirmwareParams) error {
	resp, err := a.sendRequest(ipc.ReqFlashFirmware, params)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

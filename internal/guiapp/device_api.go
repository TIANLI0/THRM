package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// ConnectDevice 连接HID设备
func (a *App) ConnectDevice() bool {
	resp, err := a.sendRequest(ipc.ReqConnect, nil)
	if err != nil {
		guiLogger.Errorf("连接设备请求失败: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("连接设备失败: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// DisconnectDevice 断开设备连接
func (a *App) DisconnectDevice() error {
	resp, err := a.sendRequest(ipc.ReqDisconnect, nil)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetDeviceStatus 获取设备连接状态
func (a *App) GetDeviceStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDeviceStatus, nil)
	if err != nil {
		return map[string]any{"connected": false, "error": err.Error()}
	}
	if !resp.Success {
		return map[string]any{"connected": false, "error": resp.Error}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// GetCurrentFanData 获取当前风扇数据
func (a *App) GetCurrentFanData() *FanData {
	resp, err := a.sendRequest(ipc.ReqGetCurrentFanData, nil)
	if err != nil {
		return nil
	}
	var fanData FanData
	if err := json.Unmarshal(resp.Data, &fanData); err != nil {
		return nil
	}
	return &fanData
}

func (a *App) RefreshDeviceSettings() (*DeviceSettings, error) {
	resp, err := a.sendRequest(ipc.ReqRefreshDeviceSettings, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var settings DeviceSettings
	if err := json.Unmarshal(resp.Data, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// ReinitializeDeviceFirmware runs controller initialization. Passing true also
// resets RPM/runtime/startup/LED state; Core restores the current App config
// before returning the fresh readback.
func (a *App) ReinitializeDeviceFirmware(factoryReset bool) (*DeviceSettings, error) {
	resp, err := a.sendRequest(ipc.ReqReinitializeFirmware, factoryReset)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var settings DeviceSettings
	if err := json.Unmarshal(resp.Data, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetAvailableGears 获取可用挡位
func (a *App) GetAvailableGears() map[string][]GearCommand {
	resp, err := a.sendRequest(ipc.ReqGetAvailableGears, nil)
	if err != nil {
		return types.GearCommands
	}
	if !resp.Success {
		return types.GearCommands
	}
	var gears map[string][]GearCommand
	json.Unmarshal(resp.Data, &gears)
	return gears
}

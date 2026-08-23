package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// SetAutoControl 设置智能变频
func (a *App) SetAutoControl(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetAutoControl, ipc.SetAutoControlParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetManualGear 设置手动挡位
func (a *App) SetManualGear(gear, level string) bool {
	resp, err := a.sendRequest(ipc.ReqSetManualGear, ipc.SetManualGearParams{Gear: gear, Level: level})
	if err != nil {
		return false
	}
	if !resp.Success {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// ManualSetFanSpeed 废弃方法
func (a *App) ManualSetFanSpeed(rpm int) bool {
	guiLogger.Warn("ManualSetFanSpeed 已废弃，请使用 SetManualGear")
	return false
}

// SetCustomSpeed 设置自定义转速
func (a *App) SetCustomSpeed(enabled bool, rpm int) error {
	resp, err := a.sendRequest(ipc.ReqSetCustomSpeed, ipc.SetCustomSpeedParams{Enabled: enabled, RPM: rpm})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetGearLight 设置挡位灯
func (a *App) SetGearLight(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetGearLight, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetPowerOnStart 设置通电自启动
func (a *App) SetPowerOnStart(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetPowerOnStart, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetSmartStartStop 设置智能启停
func (a *App) SetSmartStartStop(mode string) bool {
	resp, err := a.sendRequest(ipc.ReqSetSmartStartStop, ipc.SetStringParams{Value: mode})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetBrightness 设置亮度
func (a *App) SetBrightness(percentage int) bool {
	resp, err := a.sendRequest(ipc.ReqSetBrightness, ipc.SetIntParams{Value: percentage})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// GetSmartLightStatus 获取智能温控灯效当前落在哪一段。
func (a *App) GetSmartLightStatus() types.SmartTempLightStatus {
	status := types.SmartTempLightStatus{BandIndex: -1}
	resp, err := a.sendRequest(ipc.ReqGetSmartLightStatus, nil)
	if err != nil || !resp.Success {
		return status
	}
	json.Unmarshal(resp.Data, &status)
	return status
}

// SetLightStrip 设置灯带
func (a *App) SetLightStrip(cfg types.LightStripConfig) error {
	resp, err := a.sendRequest(ipc.ReqSetLightStrip, ipc.SetLightStripParams{Config: cfg})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

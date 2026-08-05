package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// GetConfig 获取当前配置
func (a *App) GetConfig() AppConfig {
	resp, err := a.sendRequest(ipc.ReqGetConfig, nil)
	if err != nil {
		guiLogger.Errorf("获取配置失败: %v", err)
		return types.GetDefaultConfig(false)
	}
	if !resp.Success {
		guiLogger.Errorf("获取配置失败: %s", resp.Error)
		return types.GetDefaultConfig(false)
	}
	var cfg AppConfig
	json.Unmarshal(resp.Data, &cfg)
	return cfg
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(cfg AppConfig) error {
	resp, err := a.sendRequest(ipc.ReqUpdateConfig, cfg)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// PreviewRTSSPosition moves the live OSD without writing the application
// configuration. The final position is persisted through UpdateConfig.
func (a *App) PreviewRTSSPosition(mode string, x, y int) error {
	resp, err := a.sendRequest(ipc.ReqPreviewRTSSPosition, ipc.PreviewRTSSPositionParams{Mode: mode, X: x, Y: y})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

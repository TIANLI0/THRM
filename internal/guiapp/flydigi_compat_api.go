package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/flydigicompat"
	"github.com/TIANLI0/THRM/internal/ipc"
)

// GetFlydigiCompatStatus 查询飞智空间站兼容处理的当前状态。
func (a *App) GetFlydigiCompatStatus() FlydigiCompatStatus {
	resp, err := a.sendRequest(ipc.ReqGetFlydigiCompatStatus, nil)
	if err != nil {
		guiLogger.Errorf("获取飞智兼容状态失败: %v", err)
		return FlydigiCompatStatus{}
	}
	if !resp.Success {
		guiLogger.Errorf("获取飞智兼容状态失败: %s", resp.Error)
		return FlydigiCompatStatus{Error: resp.Error}
	}
	var status FlydigiCompatStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		guiLogger.Errorf("解析飞智兼容状态失败: %v", err)
		return FlydigiCompatStatus{}
	}
	return status
}

// SetFlydigiCompat 开关飞智空间站兼容处理，返回处理后的状态。
func (a *App) SetFlydigiCompat(enabled bool) (FlydigiCompatStatus, error) {
	resp, err := a.sendRequest(ipc.ReqSetFlydigiCompat, enabled)
	if err != nil {
		return FlydigiCompatStatus{}, err
	}
	if !resp.Success {
		return FlydigiCompatStatus{}, fmt.Errorf("%s", resp.Error)
	}
	var status FlydigiCompatStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return FlydigiCompatStatus{}, err
	}
	return status, nil
}

// FlydigiCompatStatus 供 Wails 生成前端绑定模型。
type FlydigiCompatStatus = flydigicompat.Status

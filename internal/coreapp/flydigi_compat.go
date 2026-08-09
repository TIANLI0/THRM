package coreapp

import (
	"github.com/TIANLI0/THRM/internal/flydigicompat"
	"github.com/TIANLI0/THRM/internal/ipc"
)

// 飞智空间站兼容处理。
//
// 飞智空间站的后台服务以 LocalSystem 身份常驻，会持续向散热器下发风扇/灯效指令，
// 和 THRM 抢同一个 HID 接口。开启本功能后，THRM 给散热器的设备节点写入安全描述符，
// 只放行 Administrators / 交互用户 / 当前用户，把 LocalSystem 挡在门外——飞智的
// 手柄映射等功能不受任何影响。
//
// 详见 internal/flydigicompat 包注释。

// GetFlydigiCompatStatus 返回当前兼容处理状态，供设置界面展示。
func (a *CoreApp) GetFlydigiCompatStatus() flydigicompat.Status {
	status := flydigicompat.Detect(a.logger)
	return status
}

// SetFlydigiCompat 开关飞智空间站兼容处理，并立即执行对应的写入/还原。
func (a *CoreApp) SetFlydigiCompat(enabled bool) (flydigicompat.Status, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	stateDir := a.configManager.GetDefaultConfigDir()

	var (
		status flydigicompat.Status
		err    error
	)
	if enabled {
		status, err = flydigicompat.Apply(a.logger, stateDir)
	} else {
		status, err = flydigicompat.Revert(a.logger, stateDir)
	}
	if err != nil {
		a.logError("飞智空间站兼容处理失败(enabled=%v): %v", enabled, err)
		// 写入失败时不落盘开关状态，避免配置和实际状态不一致。
		return status, err
	}

	cfg := a.configManager.Get()
	cfg.FlydigiCompat = enabled
	a.configManager.Set(cfg)
	if saveErr := a.configManager.Save(); saveErr != nil {
		a.logError("保存飞智兼容开关失败: %v", saveErr)
		return status, saveErr
	}

	if enabled {
		a.logInfo("已开启飞智空间站兼容处理: 节点=%d 已写入=%d 需重连=%v",
			status.TotalNodes, status.AppliedNodes, status.NeedsReconnect)
	} else {
		a.logInfo("已关闭飞智空间站兼容处理")
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		a.ipcServer.BroadcastEvent(ipc.EventFlydigiCompatUpdate, status)
	}

	return status, nil
}

// ensureFlydigiCompat 自动兼容处理：开关打开时，确保所有散热器设备节点都已写入
// 安全描述符。散热器重新配对、换设备、换接入方式都会产生新的设备节点，这里负责补齐。
//
// 由启动流程和 30 秒健康检查循环调用，是幂等的：没有需要改动的节点时不写注册表。
func (a *CoreApp) ensureFlydigiCompat(reason string) {
	cfg := a.configManager.Get()
	if !cfg.FlydigiCompat {
		return
	}

	// 先做只读的注册表扫描：没有待处理的节点就直接返回，不打开设备句柄，
	// 避免 30 秒一次的健康检查周期性去戳散热器。
	if !flydigicompat.NeedsApply(a.logger) {
		return
	}

	status, err := flydigicompat.Apply(a.logger, a.configManager.GetDefaultConfigDir())
	if err != nil {
		a.logError("自动飞智兼容处理失败(%s): %v", reason, err)
		return
	}

	a.logInfo("自动飞智兼容处理(%s): 已写入 %d/%d 个设备节点，需重连=%v",
		reason, status.AppliedNodes, status.TotalNodes, status.NeedsReconnect)
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFlydigiCompatUpdate, status)
	}
}

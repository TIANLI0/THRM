package coreapp

import (
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
)

// recordTimelineEvent 记录一次值得在温度趋势图上标注的状态变化，并推送给已连接的 GUI。
//
// 事件由核心统一记录而不是让前端自己从 IPC 事件里推导：核心常驻后台、GUI 只是偶尔
// 打开的观察窗口，前端推导出的时间轴只覆盖"界面开着"的那几分钟，而曲线本身有一小时
// 甚至一天，于是关着界面时发生的断连在图上完全看不出来。
func (a *CoreApp) recordTimelineEvent(eventType, labelKey string) {
	if a == nil || a.tempHistory == nil {
		return
	}
	event, ok := a.tempHistory.AddEvent(eventType, labelKey)
	if !ok {
		return
	}
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventTimelineEvent, event)
	}
}

// recordSmartControlTimelineEvent 仅在智能控温开关真的翻转时记标记，
// 避免重复写入同一状态的配置刷屏。
func (a *CoreApp) recordSmartControlTimelineEvent(enabled bool) {
	if enabled {
		a.recordTimelineEvent(types.TimelineEventTypeMode, types.TimelineKeySmartControlOn)
		return
	}
	a.recordTimelineEvent(types.TimelineEventTypeMode, types.TimelineKeySmartControlOff)
}

package guiapp

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ipcWatchdogInterval 是重连探测周期。
//
// 此前"连接是否还活着"完全依赖前端每 5 秒一次的 UpdateGuiResponseTime 请求：
// 该请求写入的字段只被调试信息面板使用，却事实上承担了唯一的重连触发职责，
// 代价是每天约 1.7 万次完整 IPC 往返，而且窗口隐藏时也照常发送。
// 把探活挪到这里之后，前端那个请求可以降频，并且核心重启这类场景
// 不再依赖"恰好有请求在飞"才能被发现。
const ipcWatchdogInterval = 5 * time.Second

// startIPCWatchdog 周期性检查与核心服务的连接，断开时尝试重连。
// 重连成功后通知前端重新拉取状态，避免界面停留在断开期间的残留状态。
func (a *App) startIPCWatchdog() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				guiLogger.Errorf("IPC 看护协程发生 panic: %v", r)
			}
		}()

		ticker := time.NewTicker(ipcWatchdogInterval)
		defer ticker.Stop()

		for range ticker.C {
			// 退出流程中绝不能重连：QuitAll 会先让核心退出，此时"未连接"是预期状态，
			// 误判成掉线就会把刚刚退出的核心重新拉起来。
			if a.shuttingDown.Load() {
				return
			}
			if a.ctx == nil {
				continue
			}
			if a.ipcClient.IsConnected() {
				continue
			}
			a.reconnectCore()
		}
	}()
}

// reconnectCore 尝试重新建立 IPC 连接。成功后重新注册事件处理器并要求前端重同步。
func (a *App) reconnectCore() {
	if a.shuttingDown.Load() {
		return
	}
	if !EnsureCoreServiceRunning() {
		a.emitCoreServiceError("核心服务未运行且启动失败")
		return
	}
	if err := a.ipcClient.Connect(); err != nil {
		guiLogger.Warnf("IPC 看护重连失败: %v", err)
		a.emitCoreServiceError(err.Error())
		return
	}

	a.ipcClient.SetEventHandler(a.handleCoreEvent)
	guiLogger.Info("IPC 看护重连成功，请求前端重新同步核心状态")
	a.emitCoreServiceOK()
	a.emitCoreResynced()
}

func (a *App) emitCoreResynced() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "core-resynced", nil)
}

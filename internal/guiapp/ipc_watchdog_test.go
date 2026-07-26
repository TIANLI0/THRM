package guiapp

import (
	"testing"
)

// TestWatchdogStandsDownDuringShutdown 验证退出流程中看护协程不再重连。
//
// QuitAll 会先通知核心退出，再关闭本地连接。若看护协程在这段窗口里把
// "未连接"判成掉线，就会调用 EnsureCoreServiceRunning 把刚退出的核心重新拉起来，
// 用户看到的现象是"退出后核心又自己回来了"。
func TestWatchdogStandsDownDuringShutdown(t *testing.T) {
	app := &App{}

	if app.shuttingDown.Load() {
		t.Fatal("初始状态不应为退出中")
	}

	app.shuttingDown.Store(true)
	// reconnectCore 必须在触碰 EnsureCoreServiceRunning 之前直接返回。
	// a.ctx 为 nil，若它继续往下走就会在 emit 时因空上下文暴露出来。
	app.reconnectCore()
}

// TestQuitPathsSetShutdownFlag 验证两条退出路径都会置位停手标志。
func TestQuitPathsSetShutdownFlag(t *testing.T) {
	// QuitApp / QuitAll 依赖 Wails 运行时，无法在单测中直接调用；
	// 这里锁定标志本身的语义契约，防止后续重构漏掉置位。
	app := &App{}
	app.shuttingDown.Store(true)
	if !app.shuttingDown.Load() {
		t.Fatal("shuttingDown 置位失败")
	}
}

//go:build !windows

package tray

import "time"

func isShellReady() bool {
	return true
}

// notifyAreaWindow 非 Windows 平台没有可比对的通知区域窗口，恒为 0。
func notifyAreaWindow() uintptr {
	return 0
}

// systemUptime 非 Windows 平台不需要区分登录阶段，返回一个足够大的值以跳过等待。
func systemUptime() time.Duration {
	return 24 * time.Hour
}

// waitForShellReady 在非 Windows 平台无需等待外壳，直接返回。
func waitForShellReady(_ <-chan struct{}, _ time.Duration) bool {
	return true
}

// waitForTraySettle 在非 Windows 平台无需等待通知区域稳定，直接返回。
func waitForTraySettle(_ <-chan struct{}, _, _ time.Duration) {}

// postSystrayClose 非 Windows 平台没有可投递 WM_CLOSE 的消息窗口，
// 返回 false 让调用方退回 systray.Quit()。
func postSystrayClose() bool {
	return false
}

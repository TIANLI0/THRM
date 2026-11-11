//go:build !windows

package coreapp

// startSessionEndWatcher 在非 Windows 平台上是空实现。
//
// 此监听仅支持 Windows 会话消息。关闭 HID 句柄不等于停转，
// 非 Windows 平台尚未接入系统关机停转通知。
func startSessionEndWatcher(onShutdown, onCancelled func()) {}

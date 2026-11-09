//go:build !windows

package coreapp

// startSessionEndWatcher 在非 Windows 平台上是空实现。
//
// Linux 下关机由 systemd 发送 SIGTERM，进程退出路径本来就会关闭设备句柄，
// 不需要额外的窗口消息机制。
func startSessionEndWatcher(onShutdown, onCancelled func()) {}

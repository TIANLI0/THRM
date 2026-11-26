//go:build darwin

// macOS 适配层：睡眠/唤醒与 HID 插拔通知。
//
// 这两个功能都还没实现，原因是需要 CGO：
//   - 睡眠/唤醒要挂 NSWorkspace 的 willSleepNotification / didWakeNotification
//   - HID 插拔要挂 IO Kit 的 IOService 匹配通知
//
// 纯 Go 拿不到这些回调，而核心控温并不依赖它们——散热器控制本身有
// 短周期轮询，设备重连也能被 HID 枚举发现。所以这里先返回空实现，
// 保证核心服务能在 macOS 上构建运行。
//
// 注意：这是「还没做」，不是「macOS 不需要」。这两个通知在 macOS 上都有意义，
// 只是实现要引入 CGO，暂时跳过了。将来补的话改这两个函数即可，调用方不用动。

package powernotify

// RegisterSuspendResumeNotifications 注册睡眠/唤醒回调。
//
// 未实现：需要 NSWorkspace 通知（CGO）。
func RegisterSuspendResumeNotifications(onSuspend func(), onResume func(userPresent bool)) (func(), error) {
	return func() {}, nil
}

// RegisterHIDInterfaceArrivalNotifications 注册 HID 设备到达通知。
//
// 未实现：需要 IO Kit 的 IOService 匹配（CGO）。
func RegisterHIDInterfaceArrivalNotifications(ids []HIDDeviceID, onArrive func(string)) (func(), error) {
	return func() {}, nil
}

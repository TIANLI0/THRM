//go:build !linux && !windows

package powernotify

import (
	"fmt"
	"runtime"
)

// RegisterSuspendResumeNotifications 在未实现的平台（macOS 等）上返回错误，
// 调用方据此跳过休眠/唤醒联动。
func RegisterSuspendResumeNotifications(func(), func(userPresent bool)) (func(), error) {
	return nil, fmt.Errorf("suspend/resume notifications are not implemented on %s", runtime.GOOS)
}

func RegisterHIDInterfaceArrivalNotifications([]HIDDeviceID, func(string)) (func(), error) {
	return nil, fmt.Errorf("HID interface arrival notifications are not implemented on %s", runtime.GOOS)
}

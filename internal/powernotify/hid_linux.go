//go:build linux

package powernotify

import "fmt"

func RegisterHIDInterfaceArrivalNotifications(uint16, []uint16, func(string)) (func(), error) {
	return nil, fmt.Errorf("HID interface arrival notifications are not implemented on Linux")
}

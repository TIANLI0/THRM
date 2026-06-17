//go:build windows

package guiapp

import "golang.org/x/sys/windows"

// isWindows11 判断当前系统是否为 Windows 11(内部版本号 >= 22000)。
func isWindows11() bool {
	defer func() { _ = recover() }()
	v := windows.RtlGetVersion()
	if v == nil {
		return false
	}
	return v.BuildNumber >= 22000
}

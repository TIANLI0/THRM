//go:build windows

package guiapp

import "golang.org/x/sys/windows"

var procGetDpiForSystem = windows.NewLazySystemDLL("user32.dll").NewProc("GetDpiForSystem")

// isWindows11 判断当前系统是否为 Windows 11(内部版本号 >= 22000)。
func isWindows11() bool {
	defer func() { _ = recover() }()
	v := windows.RtlGetVersion()
	if v == nil {
		return false
	}
	return v.BuildNumber >= 22000
}

// systemDPI 返回系统 DPI（96 = 100% 缩放）。GUI 进程按清单声明了 Per-Monitor V2
// DPI 感知，因此拿到的是真实缩放；API 不可用（Win10 1607 之前）时按 96 处理。
func systemDPI() uint32 {
	defer func() { _ = recover() }()
	if procGetDpiForSystem.Find() != nil {
		return 96
	}
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 96
	}
	return uint32(dpi)
}

//go:build !windows

package guiapp

// isWindows11 在非 Windows 平台恒为 false(窗口模糊为 Windows 专有特性)。
func isWindows11() bool { return false }

// systemDPI 在非 Windows 平台不参与材质判断，按 96 返回。
func systemDPI() uint32 { return 96 }

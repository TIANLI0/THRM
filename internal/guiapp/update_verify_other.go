//go:build !windows

package guiapp

import "os"

// 非 Windows 平台不支持自动安装（见 launchUpdateInstaller），这里没有需要校验的安装包。
func verifyUpdateInstaller(_, _ string) error { return nil }

func createUpdateDir() (string, error) {
	return os.MkdirTemp("", "THRM-update-")
}

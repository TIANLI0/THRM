//go:build !windows && !darwin

package ipc

import "os"

func ipcSocketDir() string {
	return os.TempDir()
}

// prepareIPCSocketDir 在 Linux 上无事可做：socket 直接放在临时目录下，
// 创建后由 listenIPC 收紧为 0600。
func prepareIPCSocketDir(string) error {
	return nil
}

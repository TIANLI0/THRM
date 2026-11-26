//go:build darwin

package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ipcSocketDir 在 macOS 上不用 os.TempDir()：GUI 从 Finder 启动时 $TMPDIR 是
// /var/folders/.../T/，而 launchd 拉起的自启动核心往往没有 $TMPDIR，会回落到
// /tmp，两边算出的 socket 路径对不上，GUI 永远连不上核心。改用固定的按用户
// 区分目录；路径也足够短，不会碰到 sun_path 的 104 字节上限。
func ipcSocketDir() string {
	return filepath.Join("/tmp", fmt.Sprintf("thrm-%d", os.Getuid()))
}

// prepareIPCSocketDir 创建仅当前用户可访问的 socket 目录。/tmp 人人可写，
// 目录若已被别人（或一个符号链接）占住就拒绝使用，防止 socket 被劫持。
func prepareIPCSocketDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("创建 IPC 目录失败: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("检查 IPC 目录失败: %w", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("IPC 目录 %s 不属于当前用户，拒绝使用", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("收紧 IPC 目录权限失败: %w", err)
		}
	}
	return nil
}

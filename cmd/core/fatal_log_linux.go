//go:build linux

package main

import (
	"runtime/debug"
)

func setupFatalOutput() (func(), string) {
	debug.SetTraceback("all")

	// Linux 正常日志与捕获到的 panic 都由 journal 后端处理。保留 stdout/stderr
	// 原样可让 systemd 或其他服务管理器接管 Go runtime 自己打印的致命错误，且不会
	// 在 /usr/bin、~/.local/bin 或需要 root 的 /var/log 下偷偷创建文件。
	return func() {}, ""
}

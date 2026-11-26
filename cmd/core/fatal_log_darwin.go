//go:build darwin

package main

import (
	"runtime/debug"
)

func setupFatalOutput() (func(), string) {
	debug.SetTraceback("all")

	// 不重定向 stdout/stderr：从终端运行时直接可见；被 launchd 或 GUI 拉起时
	// 由父进程接管。正常日志与捕获到的 panic 写在 ~/Library/Logs/THRM。
	return func() {}, ""
}

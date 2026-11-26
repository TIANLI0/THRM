//go:build darwin

package main

import (
	"runtime/debug"
)

// macOS 下致命错误直接走 stderr，由 launchd 或终端接管，
// 不在用户目录里偷偷写文件。
func setupFatalOutput() (func(), string) {
	debug.SetTraceback("all")
	return func() {}, ""
}

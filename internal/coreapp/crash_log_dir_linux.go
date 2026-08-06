//go:build linux

package coreapp

// Linux 崩溃详情与普通日志一起进入 journal，不创建安装目录日志，也不尝试写
// /var/log（后者通常需要 root）。
func platformCrashLogDir() string {
	return ""
}

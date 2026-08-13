package main

import (
	"fmt"
	"os"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/types"
)

// --install-autostart 一次性模式：配置开机自启动后立即退出，不启动核心服务。
//
// Why: 自启动任务的定义必须只有一处来源。安装器早先用 schtasks 命令行创建任务，
// 不带 /xml 就会继承 Task Scheduler 的三个默认值——DisallowStartIfOnBatteries、
// StopIfGoingOnBatteries、ExecutionTimeLimit=PT72H——对一个常驻控温服务来说这三项
// 全是错的（理由见 autostart.buildScheduledTaskXML 的注释），而且这样建出来的任务
// 还没有 RestartOnFailure，核心崩溃后要等到下次登录才会回来。
//
// 核心启动时的 EnsureAutoStartTaskHealthy 本可就地修正，但它自己就是被这个任务拉起
// 的，于是存在一个自锁死循环：DisallowStartIfOnBatteries 意味着用电池开机时任务压根
// 不触发，核心不运行也就永远修不好这个任务，下次再用电池开机依旧如此。对一台笔记本
// 散热器的控制程序来说，"没插电源"恰恰是最常见的状态。
//
// 改由安装器调用本模式：安装器本身已提权（RequestExecutionLevel admin），子进程继承
// 提权令牌，因此不会多弹一次 UAC，任务从创建那一刻就是正确定义。

// runInstallAutoStart 返回进程退出码：0 表示成功。
// 安装器据此决定是否退回注册表自启动方式。
func runInstallAutoStart() int {
	manager := autostart.NewManager(stderrLogger{})
	// SetWindowsAutoStart 在各平台上都走该平台唯一正确的方式
	// （Windows 为任务计划程序 XML 定义，Linux 为 XDG autostart 条目）。
	if err := manager.SetWindowsAutoStart(true); err != nil {
		fmt.Fprintf(os.Stderr, "配置开机自启动失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "开机自启动已配置")
	return 0
}

// stderrLogger 是本一次性模式使用的最简日志实现。
//
// 不用 logger.NewCustomLogger：那会创建轮转日志文件与目录，对一条装完就退的命令
// 是多余的副作用；而这里写到 stdio 的内容会被安装器（nsExec::ExecToStack）直接
// 捕获进安装日志，恰好是排查自启动配置失败最需要的地方。
type stderrLogger struct{}

var _ types.Logger = stderrLogger{}

func (stderrLogger) Info(format string, v ...any)  { writeLogLine("INFO", format, v...) }
func (stderrLogger) Warn(format string, v ...any)  { writeLogLine("WARN", format, v...) }
func (stderrLogger) Error(format string, v ...any) { writeLogLine("ERROR", format, v...) }
func (stderrLogger) Debug(string, ...any)          {}
func (stderrLogger) Close()                        {}
func (stderrLogger) CleanOldLogs()                 {}
func (stderrLogger) SetDebugMode(bool)             {}
func (stderrLogger) GetLogDir() string             { return "" }

func writeLogLine(level, format string, v ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", level, fmt.Sprintf(format, v...))
}

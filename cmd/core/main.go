package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/TIANLI0/THRM/internal/autostart"
	"github.com/TIANLI0/THRM/internal/coreapp"
)

// idleCoreGCPercent 收紧核心进程的 GC 触发阈值。
//
// 核心是常驻后台进程：存活堆只有几 MB，只在 GUI 会话期间因 JSON 序列化与历史
// 快照短暂膨胀。默认 GOGC=100 要等堆涨到存活集的两倍才回收，对一个大部分时间在
// 空转的守护进程来说是纯粹的常驻内存浪费。堆本身很小，把阈值收到 50% 后多出来的
// 那几次标记开销可以忽略，换来的是明显更低的后台 RSS。
const idleCoreGCPercent = 50

func main() {
	// 必须排在 setupFatalOutput 之前：该模式的输出要交给调用它的安装器捕获，
	// 而 setupFatalOutput 会把 stdout/stderr 重定向进日志文件，顺带还会为这条
	// 装完即退的命令留下一个空的 fatal 日志。
	if autostart.IsInstallAutoStartRequest(os.Args[1:]) {
		os.Exit(runInstallAutoStart())
	}

	var app *coreapp.CoreApp
	cleanupFatalOutput, _ := setupFatalOutput()
	defer cleanupFatalOutput()

	// 显式设置了 GOGC 时以用户/调试配置为准，不覆盖。
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(idleCoreGCPercent)
	}

	defer func() {
		if r := recover(); r != nil {
			coreapp.CapturePanic(app, "main", r)

			if app != nil {
				func() {
					defer func() {
						if stopPanic := recover(); stopPanic != nil {
							coreapp.CapturePanic(app, "main.Stop", stopPanic)
						}
					}()
					app.Stop()
				}()
			}

			os.Exit(1)
		}
	}()

	// 检测命令行参数
	debugMode := false
	isAutoStart := false

	for _, arg := range os.Args {
		switch arg {
		case "--debug", "/debug", "-debug":
			debugMode = true
		case "--autostart", "/autostart", "-autostart":
			isAutoStart = true
		}
	}

	// 创建核心应用
	app = coreapp.NewCoreApp(debugMode, isAutoStart, iconData)

	// 启动应用
	if err := app.Start(); err != nil {
		app.LogInfo("启动核心服务失败: %v", err)
		fmt.Fprintf(os.Stderr, "启动核心服务失败: %v\n", err)
		app.Stop()
		os.Exit(1)
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		app.LogInfo("收到系统退出信号")
	case <-app.QuitChan():
		app.LogInfo("收到应用退出请求")
	}

	app.Stop()
}

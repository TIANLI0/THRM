package coreapp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

func CapturePanic(app *CoreApp, source string, recovered any) string {
	stack := debug.Stack()
	var builder strings.Builder
	builder.WriteString("=== THRM Core Crash Report ===\n")
	fmt.Fprintf(&builder, "time: %s\n", time.Now().Format(time.RFC3339Nano))
	fmt.Fprintf(&builder, "source: %s\n", source)
	fmt.Fprintf(&builder, "panic: %v\n", recovered)
	fmt.Fprintf(&builder, "pid: %d\n", os.Getpid())
	fmt.Fprintf(&builder, "args: %v\n", os.Args)
	builder.WriteString("\n--- stack ---\n")
	builder.Write(stack)
	builder.WriteString("\n")

	if app != nil {
		app.logError("[%s] 捕获到panic: %v", source, recovered)
		app.logError("[%s] panic堆栈:\n%s", source, string(stack))
		if app.logger != nil {
			app.logger.Close()
		}
	}

	logDir := resolveCrashLogDir(app)
	if logDir == "" {
		// Linux 的 panic 已写入 journal；app/logger 尚未初始化时，至少保留 stderr，
		// 让启动它的 service manager 接管。
		if app == nil || app.logger == nil {
			_, _ = fmt.Fprint(os.Stderr, builder.String())
		}
		return ""
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建崩溃日志目录失败: %v\n", err)
		_, _ = fmt.Fprint(os.Stderr, builder.String())
		return ""
	}

	fileName := fmt.Sprintf("crash_%s.log", time.Now().Format("2006-01-02_15-04-05.000"))
	filePath := filepath.Join(logDir, fileName)
	if err := os.WriteFile(filePath, []byte(builder.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入崩溃报告失败: %v\n", err)
		_, _ = fmt.Fprint(os.Stderr, builder.String())
		return ""
	}

	fmt.Fprintf(os.Stderr, "程序发生panic，崩溃报告已写入: %s\n", filePath)
	return filePath
}

func resolveCrashLogDir(app *CoreApp) string {
	if app != nil && app.logger != nil {
		if logDir := app.logger.GetLogDir(); logDir != "" {
			return logDir
		}
	}

	return platformCrashLogDir()
}

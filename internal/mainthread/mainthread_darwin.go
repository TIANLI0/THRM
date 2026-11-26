//go:build darwin

package mainthread

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// 运行一次主线程 RunLoop，最多 seconds 秒。返回 kCFRunLoopRunFinished(1)
// 表示当前模式下没有任何输入源，调用会立即返回。
static int thrmRunMainLoopOnce(double seconds) {
	return (int)CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, false);
}
*/
import "C"

import (
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	// exitGrace 是 fn 返回后、主线程仍卡在某个 Call（典型是 NSApp 消息循环）里时，
	// 等它自行结束的最长时间。超时直接退出进程，避免核心在退出流程里挂死。
	exitGrace = 3 * time.Second

	// idleRunLoopSlice 是主线程空闲时每轮跑 RunLoop 的时长，也是 Call 最长的
	// 排队延迟（只有托盘启动时用 Call，延迟无感）。
	idleRunLoopSlice = 500 * time.Millisecond

	// runLoopRunFinished 对应 CFRunLoopRunResult 的 kCFRunLoopRunFinished。
	runLoopRunFinished = 1
)

func init() {
	// init 在主 goroutine 上执行，此时锁定即把 main.main 固定在进程主线程上。
	runtime.LockOSThread()
}

var (
	queue   = make(chan func())
	running atomic.Bool
)

// Run 必须由 main.main 直接调用。fn 在新 goroutine 中运行，主线程转而执行
// Call 投递过来的函数，直到 fn 返回。
//
// 主线程空闲时不能只阻塞在 Go 的 channel 上：热键库注册/注销时会 dispatch_sync
// 到 GCD 主队列，CGEventTap 也挂在主 RunLoop 上。主线程不跑 RunLoop，托盘关闭
// 期间注册热键会永久卡死，快捷键也不会触发。
func Run(fn func()) {
	done := make(chan struct{})
	running.Store(true)
	go func() {
		defer close(done)
		fn()
	}()
	go func() {
		<-done
		time.Sleep(exitGrace)
		os.Exit(0)
	}()

	for {
		select {
		case f := <-queue:
			f()
			continue
		case <-done:
			running.Store(false)
			return
		default:
		}

		if C.thrmRunMainLoopOnce(C.double(idleRunLoopSlice.Seconds())) == runLoopRunFinished {
			// 没有任何输入源时 RunLoop 立即返回，退回 channel 等待，避免空转。
			select {
			case f := <-queue:
				f()
			case <-done:
				running.Store(false)
				return
			case <-time.After(idleRunLoopSlice):
			}
		}
	}
}

// Call 在主线程上执行 f 并等待其返回。Run 尚未接管主线程时（测试、工具命令）
// 就地执行。f 内的 panic 由调用方在 f 里自行 recover。
func Call(f func()) {
	if !running.Load() {
		f()
		return
	}
	finished := make(chan struct{})
	queue <- func() {
		defer close(finished)
		f()
	}
	<-finished
}

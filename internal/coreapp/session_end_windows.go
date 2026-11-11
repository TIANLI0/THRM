//go:build windows

package coreapp

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Windows 会话结束（关机 / 注销）通知。
//
// Core 是以 -H=windowsgui 构建的进程，没有控制台，收不到 CTRL_SHUTDOWN_EVENT；
// 而 WM_QUERYENDSESSION 只广播给顶层窗口。所以这里自建一个不可见的顶层窗口
// 专门接这条消息——message-only 窗口（HWND_MESSAGE）收不到该广播，不能省。
//
// 为什么需要它：2026-11-05 实测确认，设备不会因为"主机不再发命令"而停转，
// 它会一直保持最后收到的目标转速。关机必须主动下发一次 0。
const (
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmDestroy         = 0x0002
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type pointW struct {
	X int32
	Y int32
}

type msgW struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      pointW
}

var (
	sessionEndOnce        sync.Once
	sessionEndCallback    func()
	sessionCancelCallback func()
	sessionEndProc        uintptr // 保持回调存活，避免被 GC 回收
)

// startSessionEndWatcher 注册一个只处理关机 / 注销通知的隐藏窗口。
// onShutdown 在系统广播 WM_QUERYENDSESSION 时被调用，必须尽快返回；
// onCancelled 在用户取消关机（WM_ENDSESSION wParam=0）时调用。
func startSessionEndWatcher(onShutdown, onCancelled func()) {
	sessionEndOnce.Do(func() {
		sessionEndCallback = onShutdown
		sessionCancelCallback = onCancelled
		go runSessionEndWindow()
	})
}

func runSessionEndWindow() {
	// 窗口消息循环必须固定在同一个 OS 线程上。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className, err := syscall.UTF16PtrFromString("THRM_SessionEndWatcher")
	if err != nil {
		return
	}
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	sessionEndProc = syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmQueryEndSession:
			runSessionEndCallback()
			return 1 // TRUE：同意关机，不阻塞系统
		case wmEndSession:
			if wParam != 0 {
				// 已经确定要关机；正常情况下前面那条已经发过了，这里只兜底。
				runSessionEndCallback()
			} else {
				// 关机被取消：把自动控制放回去，否则控温会一直停摆。
				runSessionCancelCallback()
			}
			return 0
		case wmDestroy:
			procPostQuitMessage.Call(0)
			return 0
		}
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	})

	wc := wndClassExW{
		Size:      uint32(unsafe.Sizeof(wndClassExW{})),
		WndProc:   sessionEndProc,
		Instance:  syscall.Handle(hInstance),
		ClassName: className,
	}
	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return
	}

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0, // 不带 WS_VISIBLE：窗口存在但不显示
		0, 0, 0, 0,
		0, 0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return
	}

	var m msgW
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func runSessionEndCallback() {
	cb := sessionEndCallback
	if cb == nil {
		return
	}
	// 关机路径上的 panic 绝不能让窗口过程崩掉，否则系统会判定应用无响应。
	defer func() { _ = recover() }()
	cb()
}

func runSessionCancelCallback() {
	cb := sessionCancelCallback
	if cb == nil {
		return
	}
	defer func() { _ = recover() }()
	cb()
}

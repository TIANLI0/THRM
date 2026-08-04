//go:build windows

package tray

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32          = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW    = modUser32.NewProc("FindWindowW")
	procFindWindowExW  = modUser32.NewProc("FindWindowExW")
	modKernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64 = modKernel32.NewProc("GetTickCount64")
)

// systemUptime 返回系统启动至今的时长，用于区分登录阶段与日常运行。
// 取不到时返回 0，按登录阶段处理（多等一会儿总比丢图标好）。
func systemUptime() time.Duration {
	ticks, _, _ := procGetTickCount64.Call()
	return time.Duration(ticks) * time.Millisecond
}

// findTopWindow 查找指定类名的顶层窗口句柄，未找到返回 0。
func findTopWindow(class string) uintptr {
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(classPtr)), 0)
	return hwnd
}

// findChildWindow 在父窗口下查找指定类名的子窗口句柄，未找到返回 0。
func findChildWindow(parent uintptr, class string) uintptr {
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowExW.Call(parent, 0, uintptr(unsafe.Pointer(classPtr)), 0)
	return hwnd
}

// isShellReady 判断 Windows 任务栏外壳及其通知区域是否已就绪。
//
// 仅判断 Shell_TrayWnd 是不够的：开机快速启动时该窗口可能很早创建，但承载
// 通知图标的 TrayNotifyWnd 尚未就绪，此时调用 Shell_NotifyIcon(NIM_ADD)
// 会“成功返回但图标被静默丢弃”，且因为本进程的消息窗口在 Explorer 广播
// TaskbarCreated 之后才创建，systray 的自动重添机制也无从触发。
// 因此这里进一步要求通知区域窗口 TrayNotifyWnd 也已存在。
func isShellReady() bool {
	return notifyAreaWindow() != 0
}

// notifyAreaWindow 返回任务栏通知区域(TrayNotifyWnd)的窗口句柄，未就绪时返回 0。
// 通知区域被重建时会新建该窗口，因此句柄变化即代表重建过。
func notifyAreaWindow() uintptr {
	tray := findTopWindow("Shell_TrayWnd")
	if tray == 0 {
		return 0
	}
	return findChildWindow(tray, "TrayNotifyWnd")
}

// waitForShellReady 在启动系统托盘前等待外壳就绪。
func waitForShellReady(done <-chan struct{}, timeout time.Duration) bool {
	if isShellReady() {
		return true
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return false
		case <-ticker.C:
			if isShellReady() {
				return true
			}
			if time.Now().After(deadline) {
				return true
			}
		}
	}
}

// waitForTraySettle 在自启动首次注册托盘前等待通知区域稳定。
//
// 即便 isShellReady 已返回 true，开机阶段通知区域仍可能在短时间内被重建
// （Explorer 完成初始化时会广播 TaskbarCreated）。这里要求通知区域在连续
// settle 时长内持续可用后再返回，从而尽量避免在重建窗口期注册图标导致丢失。
// 超过 timeout 后无论是否稳定都会返回，避免在异常环境下永不显示图标。
func waitForTraySettle(done <-chan struct{}, settle, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var stableSince time.Time
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if isShellReady() {
				if stableSince.IsZero() {
					stableSince = time.Now()
				}
				if time.Since(stableSince) >= settle {
					return
				}
			} else {
				// 通知区域消失（仍在重建），重新计时。
				stableSince = time.Time{}
			}
			if time.Now().After(deadline) {
				return
			}
		}
	}
}

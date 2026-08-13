//go:build windows

package tray

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW              = modUser32.NewProc("FindWindowW")
	procFindWindowExW            = modUser32.NewProc("FindWindowExW")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetClassNameW            = modUser32.NewProc("GetClassNameW")
	procGetWindowThreadProcessID = modUser32.NewProc("GetWindowThreadProcessId")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64           = modKernel32.NewProc("GetTickCount64")
)

// systrayWindowClass 是 fyne.io/systray 注册消息窗口时使用的类名
// （见 systray_windows.go 的 initInstance）。
const systrayWindowClass = "SystrayClass"

const wmClose = 0x0010

// postSystrayClose 向本进程的 systray 消息窗口投递 WM_CLOSE，等价于
// systray.Quit() 内部所做的事情，返回是否成功投递。
//
// Why: systray.Quit() 被包级 `quitOnce sync.Once` 保护，且 Register/Run 都不重置它，
// 因此整个进程生命周期内只有第一次调用有效。托盘的所有自愈路径（ready 超时重建、
// 图标/菜单创建失败重建、可见性开关）都依赖"结束消息循环让监督协程重建实例"，
// 一旦 Quit 被用掉，后续调用变成静默空操作——核心还在跑，托盘图标却再也回不来。
// 直接投递 WM_CLOSE 绕开那个 sync.Once。
//
// 绝不能用 FindWindowW(systrayWindowClass, nil)：该类名是 fyne.io/systray 的全局
// 常量，任何用同一个库的第三方程序都会注册同名窗口类，而 FindWindowW 搜索的是
// 整个桌面的顶层窗口，命中别人的窗口就会把别人的托盘图标关掉。因此这里枚举顶层
// 窗口并按进程 ID 过滤，只处理本进程自己的窗口。
func postSystrayClose() bool {
	hwnd := findOwnSystrayWindow()
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procPostMessageW.Call(hwnd, wmClose, 0, 0)
	return ret != 0
}

// 枚举回调的共享状态。EnumWindows 是同步调用，回调在它返回前于同一线程上执行完毕，
// 因此用一把互斥锁串行化并发调用即可，无需每次新建回调。
var (
	enumSystrayMu    sync.Mutex
	enumSystrayPID   uint32
	enumSystrayFound uintptr
	enumSystrayOnce  sync.Once
	enumSystrayProc  uintptr
)

// enumSystrayCallback 惰性创建并复用枚举回调。
//
// windows.NewCallback 每次调用都会占用一个进程级回调槽位，上限约 2000 且永不回收。
// 按调用新建回调会随托盘重建次数线性泄漏，最终把槽位耗尽——这正是本次要一并修掉的
// 问题，不能在修复代码里重新引入。
func enumSystrayCallback() uintptr {
	enumSystrayOnce.Do(func() {
		enumSystrayProc = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var pid uint32
			procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid != enumSystrayPID {
				return 1 // 继续枚举
			}
			if windowClassName(hwnd) != systrayWindowClass {
				return 1
			}
			enumSystrayFound = hwnd
			return 0 // 找到了，停止枚举
		})
	})
	return enumSystrayProc
}

// findOwnSystrayWindow 返回本进程的 systray 消息窗口句柄，未找到返回 0。
func findOwnSystrayWindow() uintptr {
	enumSystrayMu.Lock()
	defer enumSystrayMu.Unlock()

	enumSystrayPID = uint32(windows.GetCurrentProcessId())
	enumSystrayFound = 0
	procEnumWindows.Call(enumSystrayCallback(), 0)
	return enumSystrayFound
}

// windowClassName 读取窗口类名，失败时返回空串。
func windowClassName(hwnd uintptr) string {
	// 窗口类名上限为 256 个字符，加上结尾的 NUL 刚好放得下。
	buf := make([]uint16, 257)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

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

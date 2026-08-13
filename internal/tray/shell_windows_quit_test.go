//go:build windows

package tray

import "testing"

// TestEnumSystrayCallbackReusesSingleSlot 保证枚举回调只创建一次。
//
// windows.NewCallback 每次调用都会占用一个进程级回调槽位，数量有限且永不回收。
// 若查找托盘窗口时按调用新建回调，就会随托盘重建次数线性泄漏——这正是本次一并
// 修掉的问题，不能在修复代码里重新引入。
func TestEnumSystrayCallbackReusesSingleSlot(t *testing.T) {
	first := enumSystrayCallback()
	if first == 0 {
		t.Fatal("枚举回调创建失败")
	}

	for i := range 64 {
		if got := enumSystrayCallback(); got != first {
			t.Fatalf("第 %d 次取回调 = %#x, want %#x（回调被重复创建，会泄漏槽位）", i+1, got, first)
		}
	}
}

// TestFindOwnSystrayWindowIgnoresOtherProcesses 验证只会命中本进程的窗口。
//
// 测试进程没有 systray 消息窗口，因此必须返回 0。这条断言是防回归的关键：
// SystrayClass 是 fyne.io/systray 的全局类名，任何同时在跑的、用同一个库的第三方
// 程序都注册了同名窗口类。若实现退回 FindWindowW(systrayWindowClass, nil)——它搜索
// 的是整个桌面的顶层窗口——就会拿到别人的句柄，随后那条 WM_CLOSE 会把别人的托盘
// 图标关掉。
func TestFindOwnSystrayWindowIgnoresOtherProcesses(t *testing.T) {
	if hwnd := findOwnSystrayWindow(); hwnd != 0 {
		t.Fatalf("本进程没有托盘窗口，却找到句柄 %#x（可能命中了其它进程的窗口）", hwnd)
	}
}

// TestPostSystrayCloseWithoutWindow 没有托盘窗口时应报告失败，
// 让调用方退回 systray.Quit()，而不是误以为已经关掉了消息循环。
func TestPostSystrayCloseWithoutWindow(t *testing.T) {
	if postSystrayClose() {
		t.Fatal("没有托盘窗口时 postSystrayClose 应返回 false")
	}
}

// TestWindowClassNameOfShellTray 用任务栏这个稳定存在的系统窗口验证类名读取。
// 拿不到任务栏句柄（无桌面会话的 CI）时跳过。
func TestWindowClassNameOfShellTray(t *testing.T) {
	hwnd := findTopWindow("Shell_TrayWnd")
	if hwnd == 0 {
		t.Skip("当前会话没有任务栏窗口，跳过")
	}

	if got := windowClassName(hwnd); got != "Shell_TrayWnd" {
		t.Fatalf("windowClassName() = %q, want %q", got, "Shell_TrayWnd")
	}
}

//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void thrmPrepareAccessoryApp(void) {
	[NSApplication sharedApplication];
	// 核心进程只有菜单栏图标：Accessory 不占 Dock、不抢菜单栏。核心与 GUI 同在
	// THRM.app/Contents/MacOS 下时，系统会按 .app 的 Info.plist 把它当成普通
	// 应用，不显式设置就会多出第二个 Dock 图标。
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}
*/
import "C"

// prepareNativeTrayApp 必须在主线程、systray.Run 之前调用。
func prepareNativeTrayApp() {
	C.thrmPrepareAccessoryApp()
}

//go:build windows

package coreapp

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	modKernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGetLastInputInfo = modUser32.NewProc("GetLastInputInfo")
	procGetTickCount     = modKernel32.NewProc("GetTickCount")
)

// lastInputInfo 对应 LASTINPUTINFO。
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// currentInputTick 返回与 GetLastInputInfo 同一时钟（GetTickCount）的当前值。
func currentInputTick() (uint32, bool) {
	if procGetTickCount.Find() != nil {
		return 0, false
	}
	tick, _, _ := procGetTickCount.Call()
	return uint32(tick), true
}

// lastUserInputTick 返回本会话最近一次键盘/鼠标输入的时刻（GetTickCount 时钟）。
// 核心进程以 InteractiveToken 跑在用户会话里，因此能看到用户的输入。
func lastUserInputTick() (uint32, bool) {
	if procGetLastInputInfo.Find() != nil {
		return 0, false
	}
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, false
	}
	return info.dwTime, true
}

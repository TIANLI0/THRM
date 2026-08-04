//go:build windows

package main

import _ "embed"

// 托盘图标：Windows 的 Shell_NotifyIcon 需要 ICO。
//
//go:embed icon.ico
var iconData []byte

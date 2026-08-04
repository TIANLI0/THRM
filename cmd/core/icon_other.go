//go:build !windows

package main

import _ "embed"

// 托盘图标：非 Windows 平台（fyne.io/systray 的 Linux/StatusNotifierItem 后端）
// 只能解码 PNG，喂给它 ICO 会得到一个空白托盘项。
//
// 这里必须是独立于 Windows 的一份资源，否则下游打包者只能在二进制里把嵌入的 ICO
// 字节段替换成 PNG（AUR 的 thrm-bin 一直这么做），每次版本更新都要重新定位偏移。
//
//go:embed icon.png
var iconData []byte

//go:build darwin

package hotkey

import (
	"strings"

	hotkeylib "golang.design/x/hotkey"
)

// parseModifier 把配置里的修饰键名映射到 macOS 键位。配置在各平台间共用，
// 所以 Windows 写法也要认：Alt 即 Option，Win/Super 即 Command。
//
// macOS 上全局快捷键靠 CGEventTap 实现，需要在“系统设置 → 隐私与安全性 →
// 辅助功能”里授权核心进程，否则注册会失败并记录到日志。
func parseModifier(part string) (hotkeylib.Modifier, bool) {
	switch strings.ToUpper(strings.TrimSpace(part)) {
	case "CTRL", "CONTROL":
		return hotkeylib.ModCtrl, true
	case "ALT", "OPTION", "OPT":
		return hotkeylib.ModOption, true
	case "SHIFT":
		return hotkeylib.ModShift, true
	case "WIN", "WINDOWS", "SUPER", "CMD", "COMMAND", "META":
		return hotkeylib.ModCmd, true
	default:
		return hotkeylib.Modifier(0), false
	}
}

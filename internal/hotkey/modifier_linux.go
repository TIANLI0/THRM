//go:build linux

package hotkey

import (
	"strings"

	hotkeylib "golang.design/x/hotkey"
)

func parseModifier(part string) (hotkeylib.Modifier, bool) {
	switch strings.ToUpper(strings.TrimSpace(part)) {
	case "CTRL", "CONTROL":
		return hotkeylib.ModCtrl, true
	case "ALT":
		return hotkeylib.Mod1, true
	case "SHIFT":
		return hotkeylib.ModShift, true
	case "WIN", "WINDOWS", "SUPER":
		return hotkeylib.Mod4, true
	default:
		return hotkeylib.Modifier(0), false
	}
}

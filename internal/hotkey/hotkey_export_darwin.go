//go:build darwin

package hotkey

/*
extern void thrmGoHotkeyFired(int id);
*/
import "C"

//export thrmGoHotkeyFired
func thrmGoHotkeyFired(id C.int) {
	if m := activeManager; m != nil {
		m.dispatch(int(id))
	}
}

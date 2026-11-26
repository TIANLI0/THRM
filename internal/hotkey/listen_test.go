//go:build linux || darwin

package hotkey

import (
	"testing"
	"time"

	hotkeylib "golang.design/x/hotkey"
)

// Unregister 会关闭旧的 keydown 通道；监听协程必须把关闭当作退出，
// 而不是当成一次按键再触发一遍动作。
func TestListenExitsWithoutFiringWhenKeydownClosed(t *testing.T) {
	fired := make(chan string, 4)
	m := &Manager{onAction: func(_ Action, shortcut string) { fired <- shortcut }}
	keydown := make(chan hotkeylib.Event)
	b := &binding{shortcut: "CTRL+ALT+K", keydown: keydown, stopChan: make(chan struct{})}

	done := make(chan struct{})
	go func() {
		m.listen(ActionToggleAutoMode, b)
		close(done)
	}()

	keydown <- hotkeylib.Event{}
	select {
	case got := <-fired:
		if got != "CTRL+ALT+K" {
			t.Fatalf("onAction shortcut = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("按键事件没有触发动作")
	}

	close(keydown)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("keydown 通道关闭后监听协程没有退出")
	}
	select {
	case got := <-fired:
		t.Fatalf("通道关闭被误当成按键，触发了 %q", got)
	default:
	}
}

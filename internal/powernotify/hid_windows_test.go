//go:build windows

package powernotify

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestCMNotifyFilterLayout(t *testing.T) {
	const wantSize = 416
	if got := unsafe.Sizeof(cmNotifyFilter{}); got != wantSize {
		t.Fatalf("CM_NOTIFY_FILTER size = %d, want %d", got, wantSize)
	}
}

func TestHIDInterfacePath(t *testing.T) {
	want := `\\?\HID#VID_37D7&PID_1002#abc`
	encoded := append(utf16.Encode([]rune(want)), 0)
	eventData := make([]byte, cmNotifyEventSymbolicLinkOffset+len(encoded)*2)
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(eventData[cmNotifyEventSymbolicLinkOffset+i*2:], value)
	}

	got := hidInterfacePath(unsafe.Pointer(&eventData[0]), uintptr(len(eventData)))
	if got != want {
		t.Fatalf("hidInterfacePath() = %q, want %q", got, want)
	}
}

func TestMatchesHIDInterfacePath(t *testing.T) {
	identifiers := []string{"vid_37d7&pid_1002", "vid_37d7&pid_1003"}
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "BS2PRO Bluetooth HID", path: `\\?\HID#VID_37D7&PID_1002&Col01#8&abc`, want: true},
		{name: "case insensitive", path: `\\?\hid#vid_37d7&pid_1003#abc`, want: true},
		{name: "other Flydigi product", path: `\\?\HID#VID_37D7&PID_2001#abc`},
		{name: "unrelated HID", path: `\\?\HID#VID_1234&PID_1002#abc`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesHIDInterfacePath(test.path, identifiers); got != test.want {
				t.Fatalf("matchesHIDInterfacePath(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

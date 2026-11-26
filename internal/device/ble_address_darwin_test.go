//go:build darwin

package device

import (
	"strings"
	"testing"

	"tinygo.org/x/bluetooth"
)

// mustBLEAddress 在 macOS 上构造一个 UUID 形式的地址：CoreBluetooth 不暴露 MAC，
// 只给每台外设一个本机唯一的 UUID。把 MAC 的 12 位十六进制放进 UUID 末段，
// 不同输入仍得到不同地址，测试语义不变。
func mustBLEAddress(t *testing.T, mac string) bluetooth.Address {
	t.Helper()
	hex := strings.ToLower(strings.ReplaceAll(mac, ":", ""))
	parsed, err := bluetooth.ParseUUID("00000000-0000-0000-0000-" + hex)
	if err != nil {
		t.Fatalf("ParseUUID(%q): %v", mac, err)
	}
	return bluetooth.Address{UUID: parsed}
}

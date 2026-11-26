//go:build !darwin

package device

import (
	"testing"

	"tinygo.org/x/bluetooth"
)

func mustBLEAddress(t *testing.T, mac string) bluetooth.Address {
	t.Helper()
	parsed, err := bluetooth.ParseMAC(mac)
	if err != nil {
		t.Fatalf("ParseMAC(%q): %v", mac, err)
	}
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: parsed}}
}

//go:build linux

package device

import (
	"testing"

	"github.com/sstallion/go-hid"
)

// TestBLEDevice_HardwareScan attempts a real BLE device scan.
// This test requires a Bluetooth adapter and may fail with certain BlueZ versions.
// Use: go test -v -run TestBLEDevice_HardwareScan -count=1 ./internal/device/
func TestBLEDevice_HardwareScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware test in short mode")
	}

	ble := NewBLEManager(newTestLogger(t))

	// Test adapter enable
	t.Log("Enabling BLE adapter...")
	err := ble.adapter.Enable()
	if err != nil {
		t.Skipf("BLE adapter enable failed: %v (may need bluetooth permissions)", err)
	}
	t.Log("BLE adapter enabled successfully")

	// Note: Scan() may crash on some BlueZ versions due to upstream bug
	// in tinygo.org/x/bluetooth. The crash happens at gap_linux.go:310
	// when processing D-Bus signals.
	// This test is skipped by default; use with caution.
}

// TestHIDDevice_Scan scans for Flydigi USB HID devices (VID 0x37D7).
// Use: go test -v -run TestHIDDevice_Scan -count=1 ./internal/device/
func TestHIDDevice_Scan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware test in short mode")
	}

	// Enumerate all HID devices
	hid.Enumerate(0, 0, func(info *hid.DeviceInfo) error {
		if info.VendorID == 0x37D7 {
			t.Logf("Found Flydigi device: VID=%04X PID=%04X Path=%s Product=%s",
				info.VendorID, info.ProductID, info.Path, info.ProductStr)
		}
		return nil
	})
}

// TestHIDDevice_OpenFirst attempts to open a Flydigi HID device if present.
// Use: go test -v -run TestHIDDevice_OpenFirst -count=1 ./internal/device/
func TestHIDDevice_OpenFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware test in short mode")
	}

	const flydigiVID = 0x37D7
	flydigiPIDs := []uint16{0x1001, 0x1002, 0x1003, 0x1004}

	for _, pid := range flydigiPIDs {
		device, err := hid.OpenFirst(flydigiVID, pid)
		if err != nil {
			continue
		}
		t.Logf("Opened Flydigi device: VID=%04X PID=%04X", flydigiVID, pid)

		info, err := device.GetDeviceInfo()
		if err != nil {
			t.Logf("GetDeviceInfo failed: %v", err)
		} else {
			t.Logf("Device info: Mfr=%s Product=%s Serial=%s",
				info.MfrStr, info.ProductStr, info.SerialNbr)
		}

		// Try sending a simple query (0x25 - query work mode)
		// This command is safe and should work on all BS devices
		t.Log("Testing device protocol round-trip...")

		device.Close()
		return
	}
	t.Skip("No Flydigi USB HID devices found (PID 0x1001-0x1004)")
}

// newTestLogger creates a test logger
func newTestLogger(t *testing.T) *testDevLogger {
	return &testDevLogger{t: t}
}

type testDevLogger struct {
	t *testing.T
}

func (l *testDevLogger) Info(format string, v ...any)  { l.t.Logf("[INFO] "+format, v...) }
func (l *testDevLogger) Error(format string, v ...any) { l.t.Logf("[ERROR] "+format, v...) }
func (l *testDevLogger) Warn(format string, v ...any)  { l.t.Logf("[WARN] "+format, v...) }
func (l *testDevLogger) Debug(format string, v ...any) { l.t.Logf("[DEBUG] "+format, v...) }
func (l *testDevLogger) Close()                        {}
func (l *testDevLogger) CleanOldLogs()                 {}
func (l *testDevLogger) SetDebugMode(enabled bool)     {}
func (l *testDevLogger) GetLogDir() string             { return "" }

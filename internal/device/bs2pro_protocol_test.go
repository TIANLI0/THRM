//go:build linux

package device

import (
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/sstallion/go-hid"
)

// TestBS2PRO_ProtocolRoundTrip tests real protocol communication with BS2PRO device.
// Requires USB or Bluetooth HID connection. udev rules must be installed.
// Use: CGO_ENABLED=1 go test -v -run TestBS2PRO_ProtocolRoundTrip -count=1 ./internal/device/
func TestBS2PRO_ProtocolRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware protocol test in short mode")
	}

	const flydigiVID = 0x37D7
	pids := []uint16{0x1002, 0x1004, 0x1003, 0x1001} // BS2PRO, BS3PRO, BS3, BS2

	var dev *hid.Device
	var foundPID uint16
	for _, pid := range pids {
		d, err := hid.OpenFirst(flydigiVID, pid)
		if err == nil {
			dev = d
			foundPID = pid
			break
		}
	}
	if dev == nil {
		t.Skip("No Flydigi HID device found. Connect device via USB/BLE and install udev rules.")
	}
	defer dev.Close()

	info, err := dev.GetDeviceInfo()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Device: %s (VID=%04X PID=%04X Serial=%s)", info.ProductStr, flydigiVID, foundPID, info.SerialNbr)
	manager := NewManager(testLogger{})
	manager.configureHIDReportFormatLocked(dev)
	buildReport := func(frame []byte) []byte {
		t.Helper()
		report, err := manager.hidReportFormat.build(frame, hidControlReportLen)
		if err != nil {
			t.Fatal(err)
		}
		return report
	}

	// Set non-blocking mode for reads
	if err := dev.SetNonblock(true); err != nil {
		t.Fatal(err)
	}
	// Status notifications prove only USB IN works. Require an actual query
	// reply to prove that the device also accepted the USB OUT framing.
	if err := writeHIDReport(dev, buildReport(deviceproto.BuildFrame(0x01))); err != nil {
		t.Fatal(err)
	}
	if resp := readCommandWithTimeout(t, dev, 0x01, deviceResponseTimeout); resp == nil {
		t.Fatal("No firmware-version response to 0x01 query")
	}

	// Test 1: Build and send Query Work Mode command (0x25)
	t.Log("=== Test 1: Query Work Mode (0x25) ===")
	queryFrame := deviceproto.BuildFrame(0x25)
	report := buildReport(queryFrame)

	if _, err := dev.Write(report); err != nil {
		t.Fatalf("Write 0x25 failed: %v", err)
	}
	t.Logf("Sent 0x25 query: % X", report)

	// Read response with timeout
	resp := readCommandWithTimeout(t, dev, 0x25, deviceResponseTimeout)
	if resp == nil {
		t.Fatal("No response to 0x25 query")
	}
	t.Logf("Received: % X", resp)

	parsed, ok := deviceproto.ParseFrame(resp)
	if !ok {
		t.Fatalf("Failed to parse response frame: % X", resp)
	}
	if !parsed.ChecksumOK {
		t.Error("Response checksum mismatch")
	}
	t.Logf("Parsed: cmd=0x%02X len=%d payload=% X checksumOK=%v",
		parsed.Command, parsed.Length, parsed.Payload, parsed.ChecksumOK)

	// Test 2: Query Gear RPM Table (0x27) - with delay to let device process
	time.Sleep(100 * time.Millisecond)
	t.Log("=== Test 2: Query Gear RPM Table (0x27) ===")
	queryGearFrame := deviceproto.BuildFrame(0x27)
	report2 := buildReport(queryGearFrame)

	if _, err := dev.Write(report2); err != nil {
		t.Fatalf("Write 0x27 failed: %v", err)
	}

	resp2 := readCommandWithTimeout(t, dev, 0x27, deviceResponseTimeout)
	if resp2 == nil {
		t.Error("No response to 0x27 query")
	} else {
		parsed2, ok := deviceproto.ParseFrame(resp2)
		if !ok {
			t.Logf("Failed to parse 0x27 response: % X", resp2)
		} else {
			t.Logf("Gear table response: cmd=0x%02X payload=%d bytes checksumOK=%v",
				parsed2.Command, len(parsed2.Payload), parsed2.ChecksumOK)
		}
	}

	// Test 3: Device status notification (0xEF) - should arrive spontaneously
	t.Log("=== Test 3: Wait for status notification (0xEF) ===")
	resp3 := readWithTimeout(t, dev, 3*time.Second)
	if resp3 != nil {
		parsed3, ok := deviceproto.ParseFrame(resp3)
		if ok && parsed3.Command == 0xEF {
			t.Logf("Status notification received: cmd=0xEF payload=% X", parsed3.Payload)
			if len(parsed3.Payload) >= 5 {
				// Parse RPM (little-endian uint16 at offset 3)
				rpm := uint16(parsed3.Payload[3]) | uint16(parsed3.Payload[4])<<8
				t.Logf("Current RPM: %d", rpm)
				if rpm > 0 {
					t.Logf("Fan is spinning at %d RPM", rpm)
				} else {
					t.Log("Fan appears to be stopped (RPM=0)")
				}
			}
		} else if ok {
			t.Logf("Received unexpected command: 0x%02X", parsed3.Command)
		}
	} else {
		t.Log("No spontaneous status notification received (device may be idle)")
	}

	// Test 4: RGB query (0x45) - safe read-only command
	t.Log("=== Test 4: Query RGB Status (0x45) ===")
	rgbFrame := deviceproto.BuildFrame(0x45)
	report4 := buildReport(rgbFrame)

	if _, err := dev.Write(report4); err != nil {
		t.Fatalf("Write 0x45 failed: %v", err)
	}

	resp4 := readCommandWithTimeout(t, dev, 0x45, deviceResponseTimeout)
	if resp4 != nil {
		parsed4, ok := deviceproto.ParseFrame(resp4)
		if ok {
			t.Logf("RGB status: cmd=0x%02X payload=% X", parsed4.Command, parsed4.Payload)
		}
	} else {
		t.Error("No response to 0x45 query")
	}

	t.Log("=== All hardware protocol tests passed ===")
}

// readWithTimeout reads from a HID device with a timeout
func readWithTimeout(t *testing.T, dev *hid.Device, timeout time.Duration) []byte {
	t.Helper()
	buf := make([]byte, 64)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := dev.ReadWithTimeout(buf, 100*time.Millisecond)
		if err != nil {
			if isInterruptedHIDRead(err) {
				continue
			}
			if !isTimeoutError(err) {
				t.Logf("Read error: %v", err)
				return nil
			}
			continue
		}
		if n > 0 {
			result := make([]byte, n)
			copy(result, buf[:n])
			return result
		}
	}
	return nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, hid.ErrTimeout) || err.Error() == "hid: read timed out"
}

func readCommandWithTimeout(t *testing.T, dev *hid.Device, command byte, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw := readWithTimeout(t, dev, time.Until(deadline))
		if raw == nil {
			return nil
		}
		frame, ok := deviceproto.ParseFrame(raw)
		if ok && frame.ChecksumOK && frame.Command == command {
			return raw
		}
	}
	return nil
}

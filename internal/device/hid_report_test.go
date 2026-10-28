package device

import (
	"bytes"
	"errors"
	"io"
	"syscall"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/sstallion/go-hid"
)

func issue45USBDescriptor(t *testing.T) []byte {
	t.Helper()
	descriptor, err := deviceproto.ParseHex("06a0 ff09 01a1 0109 0315 8025 7f35 0045 ff75 0895 2081 0209 0515 8025 7f35 0045 ff75 0895 1f91 02c0")
	if err != nil {
		t.Fatal(err)
	}
	return descriptor
}

func TestHIDReportFormatIssue45USB(t *testing.T) {
	format := hidReportFormatForDescriptor(hid.BusUSB, issue45USBDescriptor(t))
	for _, command := range []byte{0x01, 0x25, 0x23, 0x46, 0x48, 0x47} {
		frame := deviceproto.BuildFrame(command)
		for _, reportLen := range []int{hidControlReportLen, hidLightReportLen} {
			report, err := format.build(frame, reportLen)
			if err != nil {
				t.Fatal(err)
			}
			// Emulate Linux stripping the hidapi dummy ID before USB OUT.
			if len(report) != 32 || report[0] != 0 {
				t.Fatalf("USB output = % X, want dummy ID 00 + 31 bytes", report)
			}
			wire := report[1:]
			if !bytes.Equal(wire[:len(frame)], frame) {
				t.Fatalf("firmware would see % X instead of % X", wire, frame)
			}
			if !bytes.Equal(wire[len(frame):], make([]byte, 31-len(frame))) {
				t.Fatalf("USB padding is not zero: % X", wire)
			}
		}
	}
	if _, err := format.build(make([]byte, 32), hidControlReportLen); err == nil {
		t.Fatal("oversized USB payload accepted")
	}
}

func TestHIDReportFormatPreservesOtherTransports(t *testing.T) {
	descriptor := issue45USBDescriptor(t)
	numbered := append([]byte{0x85, 0x02}, descriptor...)
	for _, test := range []struct {
		name       string
		bus        hid.BusType
		descriptor []byte
	}{
		{"Bluetooth", hid.BusBluetooth, descriptor},
		{"unknown bus", hid.BusUnknown, descriptor},
		{"numbered USB", hid.BusUSB, numbered},
		{"unreadable descriptor", hid.BusUSB, nil},
		{"truncated descriptor", hid.BusUSB, descriptor[:len(descriptor)-1]},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := deviceproto.BuildFrame(0x01)
			report, err := hidReportFormatForDescriptor(test.bus, test.descriptor).build(frame, hidControlReportLen)
			if err != nil || !bytes.Equal(report, deviceproto.BuildReport(frame, 25)) {
				t.Fatalf("legacy 02 + 24-byte format changed: % X, %v", report, err)
			}
		})
	}
}

func TestIssue45USBStatusDoesNotSatisfyQuery(t *testing.T) {
	manager := NewManager(testLogger{})
	manager.debugCapture.Store(true)
	firmware := manager.responses.register(0x01)
	// Actual unmodified /dev/hidraw capture from the issue, including 03.
	raw, err := deviceproto.ParseHex("035a a5ef 0b32 0005 280a 8c0a aa02 a500 0000 0000 0000 0000 0000 0000 0000 0000")
	if err != nil {
		t.Fatal(err)
	}
	frame, ok := deviceproto.ParseFrame(raw)
	if !ok || !frame.ChecksumOK || frame.ReportID != 0x03 {
		t.Fatalf("USB input did not parse: %#v", frame)
	}
	if manager.responses.deliver(frame) {
		t.Fatal("unsolicited EF incorrectly completed firmware query")
	}
	data := manager.parseFanData(raw, len(raw))
	if data == nil || data.CurrentRPM != 2600 || data.TargetRPM != 2700 {
		t.Fatalf("captured status decoded incorrectly: %#v", data)
	}
	// A USB reply uses the same input prefix. Only the matching command
	// should complete the query, even when a status notification comes first.
	reply := append([]byte{0x03}, deviceproto.BuildFrame(0x01, 0, 0, 3, 5)...)
	reply = append(reply, make([]byte, 32-len(reply))...)
	response, ok := deviceproto.ParseFrame(reply)
	if !ok || !manager.responses.deliver(response) {
		t.Fatal("matching USB response was not delivered")
	}
	if _, err := waitForResponse(manager.responses, firmware, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	manager.recordDebugFrame("rx", "hid", raw)
	if frames := manager.GetDebugFrames(); len(frames) != 1 || !frames[0].ChecksumOK || frames[0].Command != "0xEF" {
		t.Fatalf("USB debug capture lost protocol frame: %#v", frames)
	}
}

type hidReportWriterStub struct {
	n   int
	err error
}

func (w hidReportWriterStub) Write([]byte) (int, error) { return w.n, w.err }

func TestWriteHIDReportRejectsShortWrite(t *testing.T) {
	report := make([]byte, 32)
	if err := writeHIDReport(hidReportWriterStub{n: 31}, report); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write = %v", err)
	}
	if err := writeHIDReport(hidReportWriterStub{n: 32}, report); err != nil {
		t.Fatal(err)
	}
	if err := writeHIDReport(hidReportWriterStub{err: io.EOF}, report); !errors.Is(err, io.EOF) {
		t.Fatalf("write error lost: %v", err)
	}
}

func TestInterruptedHIDRead(t *testing.T) {
	for _, err := range []error{syscall.EINTR, errors.New("Interrupted system call")} {
		if !isInterruptedHIDRead(err) {
			t.Fatalf("EINTR treated as connection failure: %v", err)
		}
	}
	for _, err := range []error{nil, hid.ErrTimeout, io.EOF, errors.New("device disconnected")} {
		if isInterruptedHIDRead(err) {
			t.Fatalf("non-EINTR error ignored: %v", err)
		}
	}
}

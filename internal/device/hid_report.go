package device

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"syscall"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/sstallion/go-hid"
)

// This USB descriptor (issue #45) declares 32 input bytes and 31 output
// bytes, with no Report ID item (0x85). The leading 0x03 in its input is
// firmware data, not evidence that output should use the HOGP Report ID 0x02.
// Match the known descriptor exactly: other firmware may use numbered reports.
var flydigiUSBReportDescriptor = []byte{
	0x06, 0xA0, 0xFF, 0x09, 0x01, 0xA1, 0x01, 0x09, 0x03,
	0x15, 0x80, 0x25, 0x7F, 0x35, 0x00, 0x45, 0xFF,
	0x75, 0x08, 0x95, 0x20, 0x81, 0x02, 0x09, 0x05,
	0x15, 0x80, 0x25, 0x7F, 0x35, 0x00, 0x45, 0xFF,
	0x75, 0x08, 0x95, 0x1F, 0x91, 0x02, 0xC0,
}

type hidReportFormat struct {
	unnumberedUSB bool
}

func hidReportFormatForDescriptor(bus hid.BusType, descriptor []byte) hidReportFormat {
	return hidReportFormat{unnumberedUSB: bus == hid.BusUSB && bytes.Equal(descriptor, flydigiUSBReportDescriptor)}
}

func (m *Manager) configureHIDReportFormatLocked(device *hid.Device) {
	// Re-detect on every connection: a reconnect can switch USB/Bluetooth.
	m.hidReportFormat = hidReportFormat{}
	if runtime.GOOS != "linux" {
		return
	}
	info, err := device.GetDeviceInfo()
	if err != nil || info == nil || info.BusType != hid.BusUSB {
		return
	}
	descriptor := make([]byte, 4096) // HID_MAX_DESCRIPTOR_SIZE
	n, err := device.GetReportDescriptor(descriptor)
	if err != nil {
		m.logWarn("读取 USB HID 描述符失败，保留默认报告格式: %v", err)
		return
	}
	m.hidReportFormat = hidReportFormatForDescriptor(info.BusType, descriptor[:n])
	if m.hidReportFormat.unnumberedUSB {
		m.logInfo("USB HID 使用无编号输出报告: 31 字节数据，hidapi 前缀 0x00")
	}
}

func (f hidReportFormat) build(frame []byte, reportLen int) ([]byte, error) {
	if !f.unnumberedUSB {
		return deviceproto.BuildReport(frame, reportLen), nil
	}
	const outputBytes = 31
	if len(frame) > outputBytes {
		return nil, fmt.Errorf("USB HID 协议帧长度 %d 超出输出报告容量 %d", len(frame), outputBytes)
	}
	// hidapi requires a dummy zero Report ID for unnumbered reports. Linux
	// removes it before the USB transfer, so the firmware sees 5A A5 ...,
	// padded to the descriptor's 31 bytes, rather than 02 5A A5 ... .
	report := make([]byte, 1+outputBytes)
	copy(report[1:], frame)
	return report, nil
}

func writeHIDReport(writer interface{ Write([]byte) (int, error) }, report []byte) error {
	n, err := writer.Write(report)
	if err == nil && n != len(report) {
		return fmt.Errorf("HID 写入 %d/%d 字节: %w", n, len(report), io.ErrShortWrite)
	}
	return err
}

func isInterruptedHIDRead(err error) bool {
	// hidapi exposes strerror(EINTR) as a plain Go error, losing errno.
	return errors.Is(err, syscall.EINTR) || (err != nil && strings.EqualFold(err.Error(), "interrupted system call"))
}

func (m *Manager) writeHIDReportLocked(frame []byte, reportLen int) ([]byte, error) {
	report, err := m.hidReportFormat.build(frame, reportLen)
	if err != nil {
		return nil, err
	}
	m.recordDebugFrame("tx", "hid", report)
	return report, writeHIDReport(m.device, report)
}

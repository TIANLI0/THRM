package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	hidControlReportLen = 25
	hidLightReportLen   = 65
	maxDebugFrames      = 100
)

func DebugCommandPresets() []types.DeviceDebugCommandPreset {
	return []types.DeviceDebugCommandPreset{
		{Name: "Read gear RPM table", CommandHex: "27", Description: "0x27, query saved gear RPM profile table"},
		{Name: "Read work status", CommandHex: "25", Description: "0x25, query current work mode and gear state"},
		{Name: "Read RGB status", CommandHex: "45", Description: "0x45, query RGB/dynamic status"},
	}
}

func (m *Manager) writeHIDFrameLocked(cmd byte, payload []byte, reportLen int) error {
	frame := deviceproto.BuildFrame(cmd, payload...)
	report := deviceproto.BuildReport(frame, reportLen)
	m.recordDebugFrame("tx", types.DeviceTypeHID, report)
	_, err := m.device.Write(report)
	return err
}

func (m *Manager) currentDebugSeq() uint64 {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	return m.debugSeq
}

func (m *Manager) recordDebugFrame(direction, transport string, raw []byte) uint64 {
	copiedRaw := make([]byte, len(raw))
	copy(copiedRaw, raw)

	frameInfo, ok := deviceproto.ParseFrame(copiedRaw)
	debugFrame := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(copiedRaw),
	}
	if ok {
		debugFrame.FrameHex = deviceproto.Hex(frameInfo.Frame)
		debugFrame.Command = fmt.Sprintf("0x%02X", frameInfo.Command)
		debugFrame.Length = int(frameInfo.Length)
		debugFrame.PayloadHex = deviceproto.Hex(frameInfo.Payload)
		debugFrame.ChecksumOK = frameInfo.ChecksumOK
		debugFrame.Description = deviceproto.CommandDescription(frameInfo.Command)
		decoded := deviceproto.DecodeFrame(frameInfo)
		debugFrame.Decoded = decoded.Summary
		debugFrame.Parsed = decoded
	} else {
		debugFrame.Description = "non-protocol data"
	}

	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	m.debugSeq++
	debugFrame.ID = m.debugSeq
	m.debugFrames = append(m.debugFrames, debugFrame)
	if len(m.debugFrames) > maxDebugFrames {
		m.debugFrames = append([]types.DeviceDebugFrame(nil), m.debugFrames[len(m.debugFrames)-maxDebugFrames:]...)
	}
	return debugFrame.ID
}

func (m *Manager) debugFramesAfter(seq uint64) []types.DeviceDebugFrame {
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	frames := make([]types.DeviceDebugFrame, 0, len(m.debugFrames))
	for _, frame := range m.debugFrames {
		if frame.ID > seq {
			frames = append(frames, frame)
		}
	}
	return frames
}

func (m *Manager) GetDebugFrames() []types.DeviceDebugFrame {
	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.GetDebugFrames()
	}
	m.debugMutex.Lock()
	defer m.debugMutex.Unlock()
	frames := make([]types.DeviceDebugFrame, len(m.debugFrames))
	copy(frames, m.debugFrames)
	return frames
}

func (m *Manager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if waitMs < 0 {
		waitMs = 0
	}
	if waitMs > 5000 {
		waitMs = 5000
	}

	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.SendDebugCommand(input, waitMs)
	}

	frame, err := deviceproto.NormalizeDebugInput(input)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	startSeq := m.currentDebugSeq()
	m.mutex.Lock()
	if !m.isConnected || m.device == nil {
		m.mutex.Unlock()
		return types.DeviceDebugCommandResult{}, fmt.Errorf("device is not connected")
	}
	report := deviceproto.BuildReport(frame, hidControlReportLen)
	m.recordDebugFrame("tx", types.DeviceTypeHID, report)
	_, err = m.device.Write(report)
	m.mutex.Unlock()
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTypeHID,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(report),
		WaitMs:    waitMs,
		Frames:    m.debugFramesAfter(startSeq),
	}, nil
}

func (b *BLEManager) recordDebugFrame(direction, transport string, raw []byte) uint64 {
	copiedRaw := make([]byte, len(raw))
	copy(copiedRaw, raw)

	frameInfo, ok := deviceproto.ParseFrame(copiedRaw)
	debugFrame := types.DeviceDebugFrame{
		Direction: direction,
		Transport: transport,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		RawHex:    deviceproto.Hex(copiedRaw),
	}
	if ok {
		debugFrame.FrameHex = deviceproto.Hex(frameInfo.Frame)
		debugFrame.Command = fmt.Sprintf("0x%02X", frameInfo.Command)
		debugFrame.Length = int(frameInfo.Length)
		debugFrame.PayloadHex = deviceproto.Hex(frameInfo.Payload)
		debugFrame.ChecksumOK = frameInfo.ChecksumOK
		debugFrame.Description = deviceproto.CommandDescription(frameInfo.Command)
		decoded := deviceproto.DecodeFrame(frameInfo)
		debugFrame.Decoded = decoded.Summary
		debugFrame.Parsed = decoded
	} else {
		debugFrame.Description = "non-protocol data"
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.debugSeq++
	debugFrame.ID = b.debugSeq
	b.debugFrames = append(b.debugFrames, debugFrame)
	if len(b.debugFrames) > maxDebugFrames {
		b.debugFrames = append([]types.DeviceDebugFrame(nil), b.debugFrames[len(b.debugFrames)-maxDebugFrames:]...)
	}
	return debugFrame.ID
}

func (b *BLEManager) currentDebugSeq() uint64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.debugSeq
}

func (b *BLEManager) debugFramesAfter(seq uint64) []types.DeviceDebugFrame {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	frames := make([]types.DeviceDebugFrame, 0, len(b.debugFrames))
	for _, frame := range b.debugFrames {
		if frame.ID > seq {
			frames = append(frames, frame)
		}
	}
	return frames
}

func (b *BLEManager) GetDebugFrames() []types.DeviceDebugFrame {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	frames := make([]types.DeviceDebugFrame, len(b.debugFrames))
	copy(frames, b.debugFrames)
	return frames
}

func (b *BLEManager) SendDebugCommand(input string, waitMs int) (types.DeviceDebugCommandResult, error) {
	if waitMs < 0 {
		waitMs = 0
	}
	if waitMs > 5000 {
		waitMs = 5000
	}

	frame, err := deviceproto.NormalizeDebugInput(input)
	if err != nil {
		return types.DeviceDebugCommandResult{}, err
	}

	startSeq := b.currentDebugSeq()
	if err := b.WriteCommand(frame); err != nil {
		return types.DeviceDebugCommandResult{}, err
	}
	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return types.DeviceDebugCommandResult{
		Transport: types.DeviceTypeBLE,
		InputHex:  input,
		FrameHex:  deviceproto.Hex(frame),
		RawHex:    deviceproto.Hex(frame),
		WaitMs:    waitMs,
		Frames:    b.debugFramesAfter(startSeq),
	}, nil
}

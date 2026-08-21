package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

var hidDetailedQueryCommands = []byte{
	deviceproto.CmdQueryFirmwareVersion,
	deviceproto.CmdQueryConfigState,
	deviceproto.CmdQueryDeviceID,
	deviceproto.CmdQueryControllerCapability,
	deviceproto.CmdQueryRuntimeProfile,
	deviceproto.CmdQueryIdentityBlock,
	deviceproto.CmdQueryRPMState,
	deviceproto.CmdQueryGearRPMTable,
	deviceproto.CmdQueryWorkMode,
	deviceproto.CmdRGBStatus,
}

const (
	firmwareVersionQueryAttempts = 3
	firmwareVersionRetryDelay    = 80 * time.Millisecond
)

var basicDeviceSettingsQueryCommands = []byte{
	deviceproto.CmdQueryGearRPMTable,
	deviceproto.CmdQueryWorkMode,
	deviceproto.CmdRGBStatus,
}

// All HID models (BS2/BS2PRO/BS3/BS3PRO) expose the same controller command
// set for detailed firmware readback and maintenance. BS1 is the only model
// that remains on the reduced BLE command profile.
func supportsDetailedFirmwareProtocol(productID uint16) bool {
	switch productID {
	case ProductIDBS2, ProductIDBS2PRO, ProductIDBS3, ProductIDBS3PRO:
		return true
	default:
		return false
	}
}

func (m *Manager) QueryDeviceSettings() (types.DeviceSettings, error) {
	if m.GetDeviceType() == types.DeviceTypeBLE {
		return m.bleManager.QueryDeviceSettings()
	}
	m.queryMutex.Lock()
	defer m.queryMutex.Unlock()
	wasCapturing := m.debugCapture.Swap(true)
	defer m.debugCapture.Store(wasCapturing)

	settings := types.DeviceSettings{
		Available: false,
		Source:    types.DeviceTypeHID,
		ReadAt:    time.Now().Format("2006-01-02 15:04:05"),
		Model:     m.GetModelName(),
	}
	if cpuModel, source := deviceCPUModelFromKnownFirmware(m.GetProductID()); cpuModel != "" {
		settings.DeviceCPUModel = cpuModel
		settings.DeviceCPUModelSource = source
	}
	m.applyHIDDescriptorInfo(&settings)

	m.mutex.RLock()
	connected := m.isConnected && m.device != nil
	m.mutex.RUnlock()
	if !connected {
		return settings, fmt.Errorf("device is not connected")
	}

	var lastErr error
	commands := basicDeviceSettingsQueryCommands
	if supportsDetailedFirmwareProtocol(m.GetProductID()) {
		commands = hidDetailedQueryCommands
	} else {
		settings.FirmwareReadStatus = "unsupported"
	}
	for _, cmd := range commands {
		attempts := 1
		if cmd == deviceproto.CmdQueryFirmwareVersion {
			attempts = firmwareVersionQueryAttempts
		}
		var commandErr error
		for attempt := 1; attempt <= attempts; attempt++ {
			frame, debugFrames, err := m.queryCommand(cmd)
			settings.RawFrames = append(settings.RawFrames, queryResponseFrames(cmd, debugFrames)...)
			if err == nil {
				err = applyDeviceSettingsFrame(&settings, frame)
			}
			if err == nil {
				commandErr = nil
				break
			}
			commandErr = err
			if attempt < attempts {
				time.Sleep(firmwareVersionRetryDelay)
			}
		}
		if commandErr != nil {
			lastErr = commandErr
			message := fmt.Sprintf("0x%02X: %v", cmd, commandErr)
			settings.ReadErrors = append(settings.ReadErrors, message)
			if cmd == deviceproto.CmdQueryFirmwareVersion {
				settings.FirmwareReadStatus = "failed"
				settings.FirmwareReadError = commandErr.Error()
			}
		} else if cmd == deviceproto.CmdQueryFirmwareVersion {
			settings.FirmwareReadStatus = "ready"
			settings.FirmwareReadError = ""
		}
	}
	applyCurrentStatus(&settings, m.GetCurrentFanData())

	settings.Available = settings.FirmwareVersion != "" || settings.DeviceIdentifier != "" || len(settings.GearRPMTable) > 0 || settings.QueriedWorkState != "" || settings.Status != nil
	if settings.Available && lastErr != nil {
		m.logWarn("设备详细信息部分读取失败: %v", lastErr)
		return settings, nil
	}
	return settings, lastErr
}

func deviceCPUModelFromKnownFirmware(productID uint16) (model, source string) {
	// The checked-in reverse-engineered image is specifically
	// CH591_For_BS2PRO_Ver0.0.3.5. HID/0x01 does not expose an MCU field, so do
	// not infer the BS3/BS3PRO MCU until its firmware image is verified too.
	if productID == ProductIDBS2PRO {
		return "WCH CH591", "firmware-image: CH591_For_BS2PRO_Ver0.0.3.5"
	}
	return "", ""
}

func (m *Manager) applyHIDDescriptorInfo(settings *types.DeviceSettings) {
	if settings == nil || m.GetDeviceType() != types.DeviceTypeHID {
		return
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if !m.isConnected || m.device == nil {
		return
	}
	info, err := m.device.GetDeviceInfo()
	if err != nil || info == nil {
		return
	}
	settings.HIDManufacturer = info.MfrStr
	settings.HIDProduct = info.ProductStr
	settings.HIDSerialNumber = info.SerialNbr
	settings.HIDReleaseNumber = info.ReleaseNbr
	settings.HIDReleaseNumberHex = fmt.Sprintf("0x%04X", info.ReleaseNbr)
}

func (m *Manager) queryCommand(cmd byte) (deviceproto.Frame, []types.DeviceDebugFrame, error) {
	startSeq := m.currentDebugSeq()

	m.mutex.Lock()
	if !m.isConnected || m.device == nil {
		m.mutex.Unlock()
		return deviceproto.Frame{}, nil, fmt.Errorf("device is not connected")
	}
	waiter := m.responses.register(cmd)
	err := m.writeHIDFrameLocked(cmd, nil, hidControlReportLen)
	m.mutex.Unlock()
	if err != nil {
		m.responses.cancel(waiter)
		return deviceproto.Frame{}, nil, err
	}
	frame, err := waitForResponse(m.responses, waiter, deviceResponseTimeout)
	if err != nil {
		return deviceproto.Frame{}, m.debugFramesAfter(startSeq), err
	}
	return frame, m.debugFramesAfter(startSeq), nil
}

func (b *BLEManager) QueryDeviceSettings() (types.DeviceSettings, error) {
	b.queryMutex.Lock()
	defer b.queryMutex.Unlock()
	wasCapturing := b.debugCapture.Swap(true)
	defer b.debugCapture.Store(wasCapturing)
	settings := types.DeviceSettings{
		Available:          false,
		Source:             types.DeviceTypeBLE,
		ReadAt:             time.Now().Format("2006-01-02 15:04:05"),
		Model:              "BS1",
		FirmwareReadStatus: "unsupported",
	}

	if !b.IsConnected() {
		return settings, fmt.Errorf("BLE device is not connected")
	}

	var lastErr error
	for _, cmd := range basicDeviceSettingsQueryCommands {
		frame, frames, err := b.queryCommand(cmd)
		if err != nil {
			lastErr = err
			settings.ReadErrors = append(settings.ReadErrors, fmt.Sprintf("0x%02X: %v", cmd, err))
			continue
		}
		frames = queryResponseFrames(cmd, frames)
		settings.RawFrames = append(settings.RawFrames, frames...)
		if err := applyDeviceSettingsFrame(&settings, frame); err != nil {
			lastErr = err
			settings.ReadErrors = append(settings.ReadErrors, fmt.Sprintf("0x%02X: %v", cmd, err))
		}
	}
	applyCurrentStatus(&settings, b.GetCurrentFanData())

	settings.Available = settings.FirmwareVersion != "" || settings.DeviceIdentifier != "" || len(settings.GearRPMTable) > 0 || settings.QueriedWorkState != "" || settings.Status != nil
	if settings.Available && lastErr != nil {
		b.logError("设备详细信息部分读取失败: %v", lastErr)
		return settings, nil
	}
	return settings, lastErr
}

// queryResponseFrames accepts only frames with the same protocol command as
// the query just sent. The old sequence-only approach also consumed unrelated
// status notifications and control responses generated by background control.
func queryResponseFrames(cmd byte, frames []types.DeviceDebugFrame) []types.DeviceDebugFrame {
	command := fmt.Sprintf("0x%02X", cmd)
	matched := make([]types.DeviceDebugFrame, 0, len(frames))
	for _, frame := range frames {
		if frame.Direction == "rx" && frame.ChecksumOK && frame.Command == command {
			matched = append(matched, frame)
		}
	}
	return matched
}

func (b *BLEManager) queryCommand(cmd byte) (deviceproto.Frame, []types.DeviceDebugFrame, error) {
	startSeq := b.currentDebugSeq()
	frame, err := b.sendBLECommandAndWait(cmd, nil, deviceResponseTimeout)
	if err != nil {
		return deviceproto.Frame{}, b.debugFramesAfter(startSeq), err
	}
	return frame, b.debugFramesAfter(startSeq), nil
}

func applyDeviceSettingsFrame(settings *types.DeviceSettings, frame deviceproto.Frame) error {
	if !frame.ChecksumOK {
		return fmt.Errorf("响应校验和错误")
	}
	decoded := deviceproto.DecodeFrame(frame)
	if decoded.Type == "" {
		return fmt.Errorf("无法解析命令 0x%02X 响应", frame.Command)
	}
	if frame.Command == deviceproto.CmdQueryFirmwareVersion && decoded.FirmwareVersion == "" {
		return fmt.Errorf("固件版本响应不完整，需要 4 字节，收到 %d 字节", len(frame.Payload))
	}
	applyDecodedDeviceSetting(settings, decoded)
	return nil
}

func applyDecodedDeviceSetting(settings *types.DeviceSettings, decoded deviceproto.DecodedFrame) {
	switch decoded.Type {
	case "firmwareVersion":
		settings.FirmwareVersion = decoded.FirmwareVersion
		settings.FirmwareVersionRaw = decoded.FirmwareVersionRaw
	case "deviceIdentifier":
		settings.DeviceIdentifier = decoded.DeviceIdentifier
	case "identityBlock":
		settings.IdentityMarker = decoded.IdentityMarker
		settings.IdentityHex = decoded.IdentityHex
		if settings.DeviceIdentifier == "" {
			settings.DeviceIdentifier = decoded.DeviceIdentifier
		}
	case "configState":
		settings.ConfigState = decoded.ConfigState
		settings.ConfigStateName = decoded.ConfigStateName
	case "controllerCapability":
		settings.ControllerCapabilityTier = decoded.ControllerCapabilityTier
	case "runtimeProfile":
		settings.RuntimeProfileRaw = decoded.RuntimeProfileRaw
	case "rpmState":
		settings.MeasuredRPM = decoded.MeasuredRPM
		settings.TargetRPM = decoded.RuntimeTargetRPM
	case "gearRpmTable":
		settings.GearRPMTable = make([]types.DeviceGearRPM, 0, len(decoded.GearTable))
		for _, item := range decoded.GearTable {
			settings.GearRPMTable = append(settings.GearRPMTable, types.DeviceGearRPM{
				Gear:  item.Gear,
				Label: item.Label,
				RPM:   item.RPM,
			})
		}
	case "queriedWorkState":
		// 0x25 is an enum with an ambiguous value 4. Keep it separate from
		// the 0xEF bit-field and never use it as authoritative live gear state.
		settings.QueriedWorkState = decoded.Mode
		settings.QueriedWorkStateName = decoded.ModeName
	case "rgbStatus":
		settings.RGBState = decoded.RGBState
		settings.RGBStateName = decoded.RGBName
	case "statusNotification":
		settings.Status = &types.DeviceStatusRead{
			GearSetting:        decoded.GearSetting,
			MaxGear:            decoded.MaxGear,
			Selected:           decoded.Selected,
			Mode:               decoded.Mode,
			ModeName:           decoded.ModeName,
			SmartStartStop:     decoded.SmartStartStop,
			SmartStartStopName: decoded.SmartStartStopName,
			CurrentRPM:         decoded.CurrentRPM,
			TargetRPM:          decoded.TargetRPM,
		}
	}
}

func applyCurrentStatus(settings *types.DeviceSettings, fanData *types.FanData) {
	if fanData == nil {
		return
	}
	// 0xEF is the authoritative live bit-field. It must not overwrite the
	// unrelated 0x25 enum stored in QueriedWorkState.
	settings.LiveModeFlags = fmt.Sprintf("0x%02X", fanData.CurrentMode)
	settings.LiveModeName = deviceproto.ModeName(fanData.CurrentMode)
	realtime := fanData.CurrentMode&0x01 == 1
	settings.RealtimeActive = &realtime
	settings.SelectedGear = deviceproto.DecodeSelectedGear(fanData.GearSettings)
	settings.ActiveGear = settings.SelectedGear
	if realtime {
		settings.ActiveGear = 0
	}
	maxGear, selected := deviceproto.DecodeGearSetting(fanData.GearSettings)
	smartCode, smartName := deviceproto.DecodeSmartStartStop(fanData.CurrentMode)
	settings.Status = &types.DeviceStatusRead{
		GearSetting:        fmt.Sprintf("0x%02X", fanData.GearSettings),
		MaxGear:            maxGear,
		Selected:           selected,
		Mode:               fmt.Sprintf("0x%02X", fanData.CurrentMode),
		ModeName:           deviceproto.ModeName(fanData.CurrentMode),
		SmartStartStop:     smartCode,
		SmartStartStopName: smartName,
		CurrentRPM:         int(fanData.CurrentRPM),
		TargetRPM:          int(fanData.TargetRPM),
	}
}

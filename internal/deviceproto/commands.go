package deviceproto

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	CmdQueryFirmwareVersion      byte = 0x01
	CmdQueryConfigState          byte = 0x02
	CmdInitializeController      byte = 0x03
	CmdQueryDeviceID             byte = 0x04
	CmdClearInitializationLatch  byte = 0x05
	CmdFactoryReinitialize       byte = 0x06
	CmdQueryControllerCapability byte = 0x07
	CmdSetFixedGear              byte = 0x08
	CmdQueryRuntimeProfile       byte = 0x09
	CmdSetRuntimeProfile         byte = 0x0A
	CmdQueryIdentityBlock        byte = 0x0B
	CmdSetPowerOnStart           byte = 0x0C
	CmdSetSmartStartStop         byte = 0x0D
	CmdSetRealtimeRPM            byte = 0x21
	CmdQueryRPMState             byte = 0x22
	CmdEnterRealtimeRPM          byte = 0x23
	CmdExitRealtimeRPM           byte = 0x24
	CmdQueryWorkMode             byte = 0x25
	CmdSetGearRPM                byte = 0x26
	CmdQueryGearRPMTable         byte = 0x27
	CmdRGBUploadInit             byte = 0x41
	CmdRGBChunkWrite             byte = 0x42
	CmdRGBCommit                 byte = 0x43
	CmdRGBDynamicParam           byte = 0x44
	CmdRGBStatus                 byte = 0x45
	CmdRGBEnable                 byte = 0x46
	CmdRGBFrameWrite             byte = 0x47
	CmdGearLight                 byte = 0x48
	CmdStatusNotify              byte = 0xEF

	// Capture-era aliases retained for source compatibility.
	CmdQueryDeviceInfo     = CmdQueryFirmwareVersion
	CmdQueryConfigFlag     = CmdQueryConfigState
	CmdQueryConfigSnapshot = CmdQueryDeviceID

	// Deprecated capture-era names. Keep aliases so third-party debug scripts
	// continue to compile while application code uses the recovered meaning.
	CmdInitializeRuntime    = CmdInitializeController
	CmdClearRuntimeState    = CmdClearInitializationLatch
	CmdQueryControllerState = CmdQueryControllerCapability
	CmdQueryRuntimeEnabled  = CmdQueryRuntimeProfile
	CmdSetRuntimeEnabled    = CmdSetRuntimeProfile
	CmdQueryRuntimeValues   = CmdQueryRPMState
)

type GearRPM struct {
	Gear  int    `json:"gear"`
	Label string `json:"label"`
	RPM   int    `json:"rpm"`
}

type DecodedFrame struct {
	Type                     string         `json:"type,omitempty"`
	Summary                  string         `json:"summary,omitempty"`
	FirmwareVersion          string         `json:"firmwareVersion,omitempty"`
	FirmwareVersionRaw       string         `json:"firmwareVersionRaw,omitempty"`
	DeviceIdentifier         string         `json:"deviceIdentifier,omitempty"`
	IdentityMarker           string         `json:"identityMarker,omitempty"`
	IdentityHex              string         `json:"identityHex,omitempty"`
	ConfigState              string         `json:"configState,omitempty"`
	ConfigStateName          string         `json:"configStateName,omitempty"`
	ControllerCapabilityTier *int           `json:"controllerCapabilityTier,omitempty"`
	RuntimeProfileRaw        *int           `json:"runtimeProfileRaw,omitempty"`
	MeasuredRPM              *int           `json:"measuredRpm,omitempty"`
	RuntimeTargetRPM         *int           `json:"runtimeTargetRpm,omitempty"`
	AckStatus                *int           `json:"ackStatus,omitempty"`
	AckStatusName            string         `json:"ackStatusName,omitempty"`
	GearTable                []GearRPM      `json:"gearTable,omitempty"`
	Mode                     string         `json:"mode,omitempty"`
	ModeName                 string         `json:"modeName,omitempty"`
	ActiveGear               int            `json:"activeGear,omitempty"`
	RealtimeActive           *bool          `json:"realtimeActive,omitempty"`
	RGBState                 string         `json:"rgbState,omitempty"`
	RGBName                  string         `json:"rgbName,omitempty"`
	CurrentRPM               int            `json:"currentRpm,omitempty"`
	TargetRPM                int            `json:"targetRpm,omitempty"`
	GearSetting              string         `json:"gearSetting,omitempty"`
	MaxGear                  string         `json:"maxGear,omitempty"`
	Selected                 string         `json:"selected,omitempty"`
	SmartStartStop           string         `json:"smartStartStop,omitempty"`
	SmartStartStopName       string         `json:"smartStartStopName,omitempty"`
	RawHex                   string         `json:"rawHex,omitempty"`
	Extra                    map[string]any `json:"extra,omitempty"`
	Confidence               string         `json:"confidence,omitempty"`
}

func CommandDescription(cmd byte) string {
	switch cmd {
	case CmdQueryFirmwareVersion:
		return "query firmware version"
	case CmdQueryConfigState:
		return "query firmware initialization state"
	case CmdInitializeController:
		return "initialize controller and select fixed gear 1"
	case CmdQueryDeviceID:
		return "query six-byte device identifier"
	case CmdClearInitializationLatch:
		return "clear firmware initialization latch"
	case CmdFactoryReinitialize:
		return "factory reinitialize runtime and gear RPM table"
	case CmdQueryControllerCapability:
		return "query controller capability tier"
	case CmdSetFixedGear:
		return "set fixed gear"
	case CmdQueryRuntimeProfile:
		return "query persisted runtime profile byte"
	case CmdSetRuntimeProfile:
		return "set persisted runtime profile byte"
	case CmdQueryIdentityBlock:
		return "query firmware identity block"
	case CmdSetPowerOnStart:
		return "set power-on start"
	case CmdSetSmartStartStop:
		return "set smart start/stop"
	case CmdSetRealtimeRPM:
		return "set realtime target RPM"
	case CmdQueryRPMState:
		return "query measured and target RPM"
	case CmdEnterRealtimeRPM:
		return "enter realtime RPM mode"
	case CmdExitRealtimeRPM:
		return "exit realtime RPM mode"
	case CmdQueryWorkMode:
		return "query work state and active gear"
	case CmdSetGearRPM:
		return "set one of four hardware gear RPM slots"
	case CmdQueryGearRPMTable:
		return "query four-slot gear RPM table"
	case CmdRGBUploadInit:
		return "RGB upload init"
	case CmdRGBChunkWrite:
		return "RGB chunk write"
	case CmdRGBCommit:
		return "RGB commit/apply"
	case CmdRGBDynamicParam:
		return "RGB dynamic mode param"
	case CmdRGBStatus:
		return "query RGB enable state"
	case CmdRGBEnable:
		return "RGB enable/disable"
	case CmdRGBFrameWrite:
		return "RGB frame write"
	case CmdGearLight:
		return "gear light enable/disable"
	case CmdStatusNotify:
		return "device status notification"
	default:
		return "unknown/debug command"
	}
}

// ModeName decodes the bit-field carried by the asynchronous 0xEF status frame.
// Command 0x25 uses a different enum and must be decoded by WorkStateName.
func ModeName(mode byte) string {
	if mode&0x01 == 1 {
		return "auto/realtime RPM mode"
	}
	return "manual/fixed gear mode"
}

func WorkStateName(state byte) (name string, realtime bool) {
	switch state {
	case 1, 2, 3:
		return fmt.Sprintf("fixed gear %d", state), false
	case 4:
		// Firmware also returns 4 as its initialization fallback.
		return "fixed gear 4 / initialized fallback", false
	case 5:
		return "auto/realtime RPM mode", true
	default:
		return fmt.Sprintf("unknown work state (0x%02X)", state), false
	}
}

func DecodeSmartStartStop(mode byte) (code, name string) {
	switch mode & 0x0E {
	case 0x02:
		return "off", "关闭"
	case 0x04:
		return "immediate", "即时"
	case 0x08:
		return "delayed", "延时"
	default:
		return "", ""
	}
}

func DecodeGearSetting(value byte) (maxGear, selected string) {
	maxCode := (value >> 4) & 0x0F
	selectedCode := value & 0x0F
	switch maxCode {
	case 0x2:
		maxGear = "standard"
	case 0x4:
		maxGear = "performance"
	case 0x6:
		maxGear = "extreme"
	default:
		maxGear = fmt.Sprintf("unknown(0x%X)", maxCode)
	}
	switch selectedCode {
	case 0x8:
		selected = "quiet"
	case 0xA:
		selected = "standard"
	case 0xC:
		selected = "performance"
	case 0xE:
		selected = "extreme"
	default:
		selected = fmt.Sprintf("unknown(0x%X)", selectedCode)
	}
	return maxGear, selected
}

// DecodeSelectedGear converts the low nibble of the 0xEF gear byte to the
// public 1..4 gear numbering. Zero means the firmware reported an unknown code.
func DecodeSelectedGear(value byte) int {
	switch value & 0x0F {
	case 0x8:
		return 1
	case 0xA:
		return 2
	case 0xC:
		return 3
	case 0xE:
		return 4
	default:
		return 0
	}
}

func formatIdentifier(payload []byte) string {
	parts := make([]string, len(payload))
	for i, value := range payload {
		parts[i] = fmt.Sprintf("%02X", value)
	}
	return strings.Join(parts, ":")
}

func AckStatusName(command, status byte) string {
	if status == 1 {
		return "success"
	}
	switch command {
	case CmdInitializeController:
		if status == 5 {
			return "already initialized"
		}
	case CmdSetRealtimeRPM:
		if status == 2 {
			return "realtime mode is not active"
		}
	case CmdEnterRealtimeRPM:
		if status == 3 {
			return "already in realtime mode"
		}
	case CmdExitRealtimeRPM:
		if status == 2 {
			return "already outside realtime mode"
		}
	case CmdSetGearRPM:
		if status == 0 {
			return "invalid gear index"
		}
	case CmdRGBUploadInit:
		if status == 5 {
			return "RGB stream buffer unavailable"
		}
	}
	switch status {
	case 5:
		return "busy or unavailable"
	default:
		return "invalid or rejected"
	}
}

func decodeAck(command byte, payload []byte) DecodedFrame {
	if len(payload) < 1 {
		return DecodedFrame{Type: "ack", Summary: "acknowledgement: incomplete payload", Confidence: "high"}
	}
	status := int(payload[0])
	name := AckStatusName(command, payload[0])
	return DecodedFrame{Type: "ack", Summary: fmt.Sprintf("%s: status=%d (%s)", CommandDescription(command), status, name), AckStatus: &status, AckStatusName: name, Confidence: "high"}
}

// DecodeRequest decodes host-to-device commands. Query requests intentionally
// have no payload, so they must not be passed to DecodeFrame, which interprets
// the same command number as a device-to-host response.
func DecodeRequest(frame Frame) DecodedFrame {
	payload := frame.Payload
	decoded := DecodedFrame{
		Type:       "request",
		Summary:    "request: " + CommandDescription(frame.Command),
		RawHex:     Hex(payload),
		Confidence: "high",
	}
	switch frame.Command {
	case CmdSetRealtimeRPM:
		if len(payload) >= 2 {
			decoded.TargetRPM = int(binary.LittleEndian.Uint16(payload[:2]))
			decoded.Summary = fmt.Sprintf("request: set realtime target to %d RPM", decoded.TargetRPM)
		}
	case CmdSetGearRPM:
		if len(payload) >= 3 {
			decoded.ActiveGear = int(payload[0]) + 1
			decoded.TargetRPM = int(binary.LittleEndian.Uint16(payload[1:3]))
			decoded.Summary = fmt.Sprintf("request: write hardware gear %d RPM=%d and select it", decoded.ActiveGear, decoded.TargetRPM)
		}
	case CmdSetFixedGear:
		if len(payload) >= 1 {
			decoded.ActiveGear = int(payload[0])
			decoded.Summary = fmt.Sprintf("request: select fixed gear %d", decoded.ActiveGear)
		}
	case CmdSetPowerOnStart:
		if len(payload) >= 1 {
			enabled := payload[0] == 1
			decoded.Summary = fmt.Sprintf("request: set power-on start=%t", enabled)
		}
	case CmdSetSmartStartStop:
		if len(payload) >= 1 {
			names := map[byte]string{0: "off", 1: "immediate", 2: "delayed"}
			name := names[payload[0]]
			if name == "" {
				name = fmt.Sprintf("unknown(%d)", payload[0])
			}
			decoded.Summary = "request: set smart start/stop=" + name
		}
	case CmdRGBEnable, CmdGearLight:
		if len(payload) >= 1 {
			decoded.Summary = fmt.Sprintf("request: %s=%t", CommandDescription(frame.Command), payload[0] != 0)
		}
	case CmdSetRuntimeProfile:
		if len(payload) >= 1 {
			decoded.Summary = fmt.Sprintf("request: set persisted runtime profile byte=%d (0x%02X)", payload[0], payload[0])
		}
	case CmdRGBFrameWrite:
		if len(payload) >= 1 {
			decoded.Summary = fmt.Sprintf("request: RGB frame write index=%d, bytes=%d", payload[0], len(payload)-1)
		}
	case CmdFactoryReinitialize:
		decoded.Summary = "request: factory reinitialize firmware and reset four RPM slots"
	}
	return decoded
}

func DecodeFrame(frame Frame) DecodedFrame {
	payload := frame.Payload
	switch frame.Command {
	case CmdQueryFirmwareVersion:
		if len(payload) < 4 {
			return DecodedFrame{Type: "firmwareVersion", Summary: "firmware version: incomplete payload", Confidence: "high"}
		}
		version := fmt.Sprintf("%d.%d.%d.%d", payload[0], payload[1], payload[2], payload[3])
		return DecodedFrame{Type: "firmwareVersion", Summary: "firmware version: " + version, FirmwareVersion: version, FirmwareVersionRaw: Hex(payload[:4]), Confidence: "high"}
	case CmdQueryConfigState:
		if len(payload) < 1 {
			return DecodedFrame{Type: "configState", Summary: "firmware initialization state: incomplete payload", Confidence: "medium"}
		}
		name := "not initialized"
		if payload[0] == 2 {
			name = "initialized"
		} else if payload[0] != 1 {
			name = "unknown"
		}
		state := fmt.Sprintf("0x%02X", payload[0])
		return DecodedFrame{Type: "configState", Summary: fmt.Sprintf("firmware initialization state: %s (%s)", state, name), ConfigState: state, ConfigStateName: name, Confidence: "medium"}
	case CmdQueryDeviceID:
		if len(payload) < 6 {
			return DecodedFrame{Type: "deviceIdentifier", Summary: "device identifier: incomplete payload", Confidence: "high"}
		}
		identifier := formatIdentifier(payload[:6])
		return DecodedFrame{Type: "deviceIdentifier", Summary: "device identifier: " + identifier, DeviceIdentifier: identifier, Confidence: "high"}
	case CmdQueryControllerCapability:
		if len(payload) < 1 {
			return DecodedFrame{Type: "controllerCapability", Summary: "controller capability tier: incomplete payload", Confidence: "high"}
		}
		tier := int(payload[0])
		return DecodedFrame{Type: "controllerCapability", Summary: fmt.Sprintf("controller capability tier: %d", tier), ControllerCapabilityTier: &tier, Confidence: "high"}
	case CmdQueryRuntimeProfile:
		if len(payload) < 1 {
			return DecodedFrame{Type: "runtimeProfile", Summary: "runtime profile byte: incomplete payload", Confidence: "high"}
		}
		profile := int(payload[0])
		return DecodedFrame{Type: "runtimeProfile", Summary: fmt.Sprintf("runtime profile byte: %d (0x%02X)", profile, profile), RuntimeProfileRaw: &profile, Confidence: "high"}
	case CmdQueryIdentityBlock:
		if len(payload) < 13 {
			return DecodedFrame{Type: "identityBlock", Summary: "identity block: incomplete payload", IdentityHex: strings.ToUpper(hex.EncodeToString(payload)), Confidence: "high"}
		}
		marker := strings.ToUpper(hex.EncodeToString(payload[:7]))
		identifier := formatIdentifier(payload[7:13])
		return DecodedFrame{Type: "identityBlock", Summary: fmt.Sprintf("identity block: marker=%s, device=%s", marker, identifier), IdentityMarker: marker, IdentityHex: strings.ToUpper(hex.EncodeToString(payload)), DeviceIdentifier: identifier, Confidence: "high"}
	case CmdQueryRPMState:
		if len(payload) < 4 {
			return DecodedFrame{Type: "rpmState", Summary: "measured/target RPM: incomplete payload", Confidence: "high"}
		}
		measured := int(binary.LittleEndian.Uint16(payload[:2]))
		target := int(binary.LittleEndian.Uint16(payload[2:4]))
		return DecodedFrame{Type: "rpmState", Summary: fmt.Sprintf("measured RPM=%d, target RPM=%d", measured, target), MeasuredRPM: &measured, RuntimeTargetRPM: &target, Confidence: "high"}
	case CmdQueryGearRPMTable:
		if len(payload) < 8 {
			return DecodedFrame{Type: "gearRpmTable", Summary: "gear RPM table: incomplete payload", Confidence: "high"}
		}
		labels := []string{"quiet", "standard", "performance", "extreme"}
		table := make([]GearRPM, 0, 4)
		for i := range 4 {
			table = append(table, GearRPM{Gear: i + 1, Label: labels[i], RPM: int(binary.LittleEndian.Uint16(payload[i*2 : i*2+2]))})
		}
		return DecodedFrame{Type: "gearRpmTable", Summary: fmt.Sprintf("gear RPM table: quiet=%d, standard=%d, performance=%d, extreme=%d", table[0].RPM, table[1].RPM, table[2].RPM, table[3].RPM), GearTable: table, Confidence: "high"}
	case CmdQueryWorkMode:
		if len(payload) < 1 {
			return DecodedFrame{Type: "queriedWorkState", Summary: "work state: incomplete payload", Confidence: "high"}
		}
		state := payload[0]
		name, realtime := WorkStateName(state)
		return DecodedFrame{Type: "queriedWorkState", Summary: fmt.Sprintf("work state: 0x%02X (%s)", state, name), Mode: fmt.Sprintf("0x%02X", state), ModeName: name, RealtimeActive: &realtime, Confidence: "high"}
	case CmdRGBStatus:
		if len(payload) < 1 {
			return DecodedFrame{Type: "rgbStatus", Summary: "RGB state: incomplete payload", Confidence: "high"}
		}
		name := "off"
		if payload[0] != 0 {
			name = "on"
		}
		state := fmt.Sprintf("0x%02X", payload[0])
		return DecodedFrame{Type: "rgbStatus", Summary: fmt.Sprintf("RGB state: %s (%s)", state, name), RGBState: state, RGBName: name, Confidence: "high"}
	case CmdStatusNotify:
		if len(payload) < 7 {
			return DecodedFrame{Type: "statusNotification", Summary: "status notification: incomplete payload", Confidence: "high"}
		}
		mode := payload[1]
		modeName := ModeName(mode)
		currentRPM := int(binary.LittleEndian.Uint16(payload[3:5]))
		targetRPM := int(binary.LittleEndian.Uint16(payload[5:7]))
		maxGear, selected := DecodeGearSetting(payload[0])
		smartCode, smartName := DecodeSmartStartStop(mode)
		return DecodedFrame{Type: "statusNotification", Summary: fmt.Sprintf("status: mode=0x%02X (%s), current=%d RPM, target=%d RPM", mode, modeName, currentRPM, targetRPM), GearSetting: fmt.Sprintf("0x%02X", payload[0]), MaxGear: maxGear, Selected: selected, SmartStartStop: smartCode, SmartStartStopName: smartName, Mode: fmt.Sprintf("0x%02X", mode), ModeName: modeName, CurrentRPM: currentRPM, TargetRPM: targetRPM, RawHex: Hex(payload[7:]), Confidence: "high"}
	case CmdInitializeController, CmdClearInitializationLatch, CmdFactoryReinitialize, CmdSetFixedGear,
		CmdSetRuntimeProfile, CmdSetPowerOnStart, CmdSetSmartStartStop, CmdSetRealtimeRPM,
		CmdEnterRealtimeRPM, CmdExitRealtimeRPM, CmdSetGearRPM, CmdRGBUploadInit,
		CmdRGBChunkWrite, CmdRGBCommit, CmdRGBDynamicParam, CmdRGBEnable,
		CmdRGBFrameWrite, CmdGearLight:
		return decodeAck(frame.Command, payload)
	default:
		return DecodedFrame{}
	}
}

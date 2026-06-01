package deviceproto

import (
	"encoding/binary"
	"fmt"
)

const (
	CmdQueryDeviceInfo     byte = 0x01
	CmdQueryConfigFlag     byte = 0x02
	CmdQueryConfigSnapshot byte = 0x04
	CmdSetPowerOnStart     byte = 0x0C
	CmdSetSmartStartStop   byte = 0x0D
	CmdSetRealtimeRPM      byte = 0x21
	CmdEnterRealtimeRPM    byte = 0x23
	CmdExitRealtimeRPM     byte = 0x24
	CmdQueryWorkMode       byte = 0x25
	CmdSetGearRPM          byte = 0x26
	CmdQueryGearRPMTable   byte = 0x27
	CmdRGBStatus           byte = 0x45
	CmdRGBEnable           byte = 0x46
	CmdGearLight           byte = 0x48
	CmdStatusNotify        byte = 0xEF
)

type GearRPM struct {
	Gear  int    `json:"gear"`
	Label string `json:"label"`
	RPM   int    `json:"rpm"`
}

type DecodedFrame struct {
	Type               string         `json:"type,omitempty"`
	Summary            string         `json:"summary,omitempty"`
	GearTable          []GearRPM      `json:"gearTable,omitempty"`
	Mode               string         `json:"mode,omitempty"`
	ModeName           string         `json:"modeName,omitempty"`
	RGBState           string         `json:"rgbState,omitempty"`
	RGBName            string         `json:"rgbName,omitempty"`
	CurrentRPM         int            `json:"currentRpm,omitempty"`
	TargetRPM          int            `json:"targetRpm,omitempty"`
	GearSetting        string         `json:"gearSetting,omitempty"`
	MaxGear            string         `json:"maxGear,omitempty"`
	Selected           string         `json:"selected,omitempty"`
	SmartStartStop     string         `json:"smartStartStop,omitempty"`
	SmartStartStopName string         `json:"smartStartStopName,omitempty"`
	RawHex             string         `json:"rawHex,omitempty"`
	Extra              map[string]any `json:"extra,omitempty"`
	Confidence         string         `json:"confidence,omitempty"`
}

func CommandDescription(cmd byte) string {
	switch cmd {
	case CmdQueryDeviceInfo:
		return "query device info block"
	case CmdQueryConfigFlag:
		return "query protocol/config valid flag"
	case CmdQueryConfigSnapshot:
		return "query system config snapshot"
	case 0x07:
		return "query internal status field"
	case 0x08:
		return "set fixed gear"
	case 0x0B:
		return "query fixed/info block"
	case CmdSetPowerOnStart:
		return "set power-on start"
	case CmdSetSmartStartStop:
		return "set smart start/stop"
	case CmdSetRealtimeRPM:
		return "set realtime target RPM"
	case CmdEnterRealtimeRPM:
		return "enter realtime RPM mode"
	case CmdExitRealtimeRPM:
		return "exit realtime RPM mode"
	case CmdQueryWorkMode:
		return "query work status/gear mode"
	case CmdSetGearRPM:
		return "set gear profile RPM"
	case CmdQueryGearRPMTable:
		return "query gear RPM table"
	case 0x41:
		return "RGB upload init"
	case 0x42:
		return "RGB chunk write"
	case 0x43:
		return "RGB commit/apply"
	case 0x44:
		return "RGB dynamic mode param"
	case CmdRGBStatus:
		return "RGB status/heartbeat"
	case CmdRGBEnable:
		return "RGB enable/disable"
	case 0x47:
		return "RGB frame write"
	case CmdGearLight:
		return "gear light enable/disable"
	case CmdStatusNotify:
		return "device status notification"
	default:
		return "unknown/debug command"
	}
}

func ModeName(mode byte) string {
	if mode&0x01 == 1 {
		return "auto/realtime RPM mode"
	}
	return "manual/fixed gear mode"
}

// DecodeSmartStartStop 从 0xEF 状态上报的 mode 字节解析智能启停设置。
// mode 字节中 bit0(0x01) 为实时转速模式标志，bit1..3 表示智能启停状态：
// 0x02=关闭, 0x04=即时, 0x08=延时（经 A/B 抓包验证）。
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

func DecodeFrame(frame Frame) DecodedFrame {
	payload := frame.Payload
	switch frame.Command {
	case CmdQueryGearRPMTable:
		if len(payload) < 8 {
			return DecodedFrame{Type: "gearRpmTable", Summary: "gear RPM table: incomplete payload", Confidence: "high"}
		}
		labels := []string{"quiet", "standard", "performance", "extreme"}
		table := make([]GearRPM, 0, 4)
		for i := range 4 {
			table = append(table, GearRPM{
				Gear:  i,
				Label: labels[i],
				RPM:   int(binary.LittleEndian.Uint16(payload[i*2 : i*2+2])),
			})
		}
		return DecodedFrame{
			Type:       "gearRpmTable",
			Summary:    fmt.Sprintf("gear RPM table: quiet=%d, standard=%d, performance=%d, extreme=%d", table[0].RPM, table[1].RPM, table[2].RPM, table[3].RPM),
			GearTable:  table,
			Confidence: "high",
		}
	case CmdQueryWorkMode:
		if len(payload) < 1 {
			return DecodedFrame{Type: "workMode", Summary: "work mode: incomplete payload", Confidence: "high"}
		}
		mode := payload[0]
		name := ModeName(mode)
		return DecodedFrame{
			Type:       "workMode",
			Summary:    fmt.Sprintf("work mode: 0x%02X (%s)", mode, name),
			Mode:       fmt.Sprintf("0x%02X", mode),
			ModeName:   name,
			Confidence: "high",
		}
	case CmdRGBStatus:
		if len(payload) < 1 {
			return DecodedFrame{Type: "rgbStatus", Summary: "RGB status: incomplete payload", Confidence: "medium"}
		}
		name := "off/idle"
		if payload[0] != 0 {
			name = "on/ready"
		}
		state := fmt.Sprintf("0x%02X", payload[0])
		return DecodedFrame{
			Type:       "rgbStatus",
			Summary:    fmt.Sprintf("RGB status: %s (%s)", state, name),
			RGBState:   state,
			RGBName:    name,
			Confidence: "medium",
		}
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
		return DecodedFrame{
			Type:               "statusNotification",
			Summary:            fmt.Sprintf("status: mode=0x%02X (%s), current=%d RPM, target=%d RPM", mode, modeName, currentRPM, targetRPM),
			GearSetting:        fmt.Sprintf("0x%02X", payload[0]),
			MaxGear:            maxGear,
			Selected:           selected,
			SmartStartStop:     smartCode,
			SmartStartStopName: smartName,
			Mode:               fmt.Sprintf("0x%02X", mode),
			ModeName:           modeName,
			CurrentRPM:         currentRPM,
			TargetRPM:          targetRPM,
			RawHex:             Hex(payload[7:]),
			Confidence:         "high",
		}
	default:
		return DecodedFrame{}
	}
}

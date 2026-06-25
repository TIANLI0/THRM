package deviceproto

import (
	"testing"
)

func TestCommandDescription_Known(t *testing.T) {
	commands := map[byte]string{
		CmdQueryDeviceInfo:     "query device info block",
		CmdQueryConfigFlag:     "query protocol/config valid flag",
		CmdQueryConfigSnapshot: "query system config snapshot",
		CmdSetPowerOnStart:     "set power-on start",
		CmdSetSmartStartStop:   "set smart start/stop",
		CmdSetRealtimeRPM:      "set realtime target RPM",
		CmdEnterRealtimeRPM:    "enter realtime RPM mode",
		CmdExitRealtimeRPM:     "exit realtime RPM mode",
		CmdQueryWorkMode:       "query work status/gear mode",
		CmdSetGearRPM:          "set gear profile RPM",
		CmdQueryGearRPMTable:   "query gear RPM table",
		CmdRGBStatus:           "RGB status/heartbeat",
		CmdRGBEnable:           "RGB enable/disable",
		CmdGearLight:           "gear light enable/disable",
		CmdStatusNotify:        "device status notification",
	}
	for cmd, expected := range commands {
		got := CommandDescription(cmd)
		if got != expected {
			t.Errorf("CommandDescription(0x%02X) = %q, want %q", cmd, got, expected)
		}
	}
}

func TestCommandDescription_Unknown(t *testing.T) {
	got := CommandDescription(0xFF)
	if got != "unknown/debug command" {
		t.Errorf("CommandDescription(0xFF) = %q, want 'unknown/debug command'", got)
	}
}

func TestCommandDescription_AllConstantsNonEmpty(t *testing.T) {
	allCmds := []byte{
		CmdQueryDeviceInfo, CmdQueryConfigFlag, CmdQueryConfigSnapshot,
		CmdSetPowerOnStart, CmdSetSmartStartStop, CmdSetRealtimeRPM,
		CmdEnterRealtimeRPM, CmdExitRealtimeRPM, CmdQueryWorkMode,
		CmdSetGearRPM, CmdQueryGearRPMTable, CmdRGBStatus, CmdRGBEnable,
		CmdGearLight, CmdStatusNotify,
	}
	for _, cmd := range allCmds {
		desc := CommandDescription(cmd)
		if desc == "" || desc == "unknown/debug command" {
			t.Errorf("CommandDescription(0x%02X) returned %q", cmd, desc)
		}
	}
}

func TestModeName(t *testing.T) {
	tests := []struct {
		mode byte
		want string
	}{
		{0x01, "auto/realtime RPM mode"},
		{0x03, "auto/realtime RPM mode"},
		{0x00, "manual/fixed gear mode"},
		{0x02, "manual/fixed gear mode"},
	}
	for _, tt := range tests {
		got := ModeName(tt.mode)
		if got != tt.want {
			t.Errorf("ModeName(0x%02X) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestDecodeSmartStartStop(t *testing.T) {
	tests := []struct {
		mode     byte
		wantCode string
		wantName string
	}{
		{0x02, "off", "关闭"},
		{0x04, "immediate", "即时"},
		{0x08, "delayed", "延时"},
		{0x00, "", ""},
		{0x0F, "", ""},
	}
	for _, tt := range tests {
		code, name := DecodeSmartStartStop(tt.mode)
		if code != tt.wantCode || name != tt.wantName {
			t.Errorf("DecodeSmartStartStop(0x%02X) = (%q, %q), want (%q, %q)",
				tt.mode, code, name, tt.wantCode, tt.wantName)
		}
	}
}

func TestDecodeGearSetting(t *testing.T) {
	tests := []struct {
		value         byte
		wantMaxGear   string
		wantSelected  string
	}{
		{0x28, "standard", "quiet"},
		{0x2A, "standard", "standard"},
		{0x2C, "standard", "performance"},
		{0x2E, "standard", "extreme"},
		{0x4A, "performance", "standard"},
		{0x6C, "extreme", "performance"},
	}
	for _, tt := range tests {
		maxGear, selected := DecodeGearSetting(tt.value)
		if maxGear != tt.wantMaxGear {
			t.Errorf("DecodeGearSetting(0x%02X) maxGear = %q, want %q", tt.value, maxGear, tt.wantMaxGear)
		}
		if selected != tt.wantSelected {
			t.Errorf("DecodeGearSetting(0x%02X) selected = %q, want %q", tt.value, selected, tt.wantSelected)
		}
	}
}

func TestDecodeFrame_GearRPMTable(t *testing.T) {
	payload := make([]byte, 8)
	payload[0] = 0x14 // quiet: 0x0514 = 1300
	payload[1] = 0x05
	payload[2] = 0x34 // standard: 0x0834 = 2100
	payload[3] = 0x08
	payload[4] = 0xF0 // perf: 0x0AF0 = 2800
	payload[5] = 0x0A
	payload[6] = 0xAC // extreme: 0x0DAC = 3500
	payload[7] = 0x0D

	frame := Frame{Command: CmdQueryGearRPMTable, Payload: payload}
	decoded := DecodeFrame(frame)

	if decoded.Type != "gearRpmTable" {
		t.Errorf("Type = %q, want 'gearRpmTable'", decoded.Type)
	}
	if decoded.Confidence != "high" {
		t.Errorf("Confidence = %q, want 'high'", decoded.Confidence)
	}
	if len(decoded.GearTable) != 4 {
		t.Fatalf("GearTable len = %d, want 4", len(decoded.GearTable))
	}
	expectedRPMs := []int{1300, 2100, 2800, 3500}
	expectedLabels := []string{"quiet", "standard", "performance", "extreme"}
	for i := range 4 {
		if decoded.GearTable[i].RPM != expectedRPMs[i] {
			t.Errorf("GearTable[%d].RPM = %d, want %d", i, decoded.GearTable[i].RPM, expectedRPMs[i])
		}
		if decoded.GearTable[i].Label != expectedLabels[i] {
			t.Errorf("GearTable[%d].Label = %q, want %q", i, decoded.GearTable[i].Label, expectedLabels[i])
		}
	}
}

func TestDecodeFrame_GearRPMTable_Short(t *testing.T) {
	frame := Frame{Command: CmdQueryGearRPMTable, Payload: []byte{0x00, 0x00}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "gearRpmTable" {
		t.Errorf("Type = %q", decoded.Type)
	}
	if decoded.Confidence != "high" {
		t.Errorf("Confidence = %q", decoded.Confidence)
	}
}

func TestDecodeFrame_WorkMode(t *testing.T) {
	payload := []byte{0x01}
	frame := Frame{Command: CmdQueryWorkMode, Payload: payload}
	decoded := DecodeFrame(frame)

	if decoded.Type != "workMode" {
		t.Errorf("Type = %q, want 'workMode'", decoded.Type)
	}
	if decoded.ModeName != "auto/realtime RPM mode" {
		t.Errorf("ModeName = %q", decoded.ModeName)
	}
}

func TestDecodeFrame_WorkMode_Short(t *testing.T) {
	frame := Frame{Command: CmdQueryWorkMode, Payload: []byte{}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "workMode" {
		t.Errorf("Type = %q", decoded.Type)
	}
}

func TestDecodeFrame_StatusNotify(t *testing.T) {
	payload := make([]byte, 11)
	payload[0] = 0x2A // gear setting: standard max, standard selected
	payload[1] = 0x02 // smart start stop off, bit0=0 manual
	payload[3] = 0xD0 // current RPM: 0x07D0 = 2000
	payload[4] = 0x07
	payload[5] = 0xE8 // target RPM: 0x03E8 = 1000
	payload[6] = 0x03
	payload[7] = 0x5A
	payload[8] = 0xA5

	frame := Frame{Command: CmdStatusNotify, Payload: payload}
	decoded := DecodeFrame(frame)

	if decoded.Type != "statusNotification" {
		t.Errorf("Type = %q, want 'statusNotification'", decoded.Type)
	}
	if decoded.CurrentRPM != 2000 {
		t.Errorf("CurrentRPM = %d, want 2000", decoded.CurrentRPM)
	}
	if decoded.TargetRPM != 1000 {
		t.Errorf("TargetRPM = %d, want 1000", decoded.TargetRPM)
	}
}

func TestDecodeFrame_StatusNotify_Short(t *testing.T) {
	frame := Frame{Command: CmdStatusNotify, Payload: []byte{0x00, 0x00, 0x00}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "statusNotification" {
		t.Errorf("Type = %q", decoded.Type)
	}
}

func TestDecodeFrame_Unknown(t *testing.T) {
	frame := Frame{Command: 0xFE, Payload: []byte{}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "" {
		t.Errorf("Type = %q, want empty", decoded.Type)
	}
}

func TestDecodeFrame_RGBStatus_On(t *testing.T) {
	frame := Frame{Command: CmdRGBStatus, Payload: []byte{0x01}}
	decoded := DecodeFrame(frame)
	if decoded.RGBName != "on/ready" {
		t.Errorf("RGBName = %q, want 'on/ready'", decoded.RGBName)
	}
}

func TestDecodeFrame_RGBStatus_Off(t *testing.T) {
	frame := Frame{Command: CmdRGBStatus, Payload: []byte{0x00}}
	decoded := DecodeFrame(frame)
	if decoded.RGBName != "off/idle" {
		t.Errorf("RGBName = %q, want 'off/idle'", decoded.RGBName)
	}
}

func TestDecodeFrame_RGBStatus_Short(t *testing.T) {
	frame := Frame{Command: CmdRGBStatus, Payload: []byte{}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "rgbStatus" {
		t.Errorf("Type = %q, want 'rgbStatus'", decoded.Type)
	}
}

func TestRoundTrip_DecodeFrame(t *testing.T) {
	frame := BuildFrame(CmdStatusNotify,
		0x2A, 0x02, 0x00, 0xD0, 0x07, 0xE8, 0x03, 0x5A, 0xA5, 0x00, 0x00)
	parsed, ok := ParseFrame(frame)
	if !ok {
		t.Fatal("parse failed")
	}
	decoded := DecodeFrame(parsed)
	if decoded.Type != "statusNotification" {
		t.Errorf("Type = %q", decoded.Type)
	}
	if decoded.CurrentRPM != 2000 {
		t.Errorf("CurrentRPM = %d", decoded.CurrentRPM)
	}
}

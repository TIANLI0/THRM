package deviceproto

import (
	"strings"
	"testing"
)

func TestCommandDescription_Known(t *testing.T) {
	commands := map[byte]string{
		CmdQueryDeviceInfo:     "query firmware version",
		CmdQueryConfigFlag:     "query firmware initialization state",
		CmdQueryConfigSnapshot: "query six-byte device identifier",
		CmdSetPowerOnStart:     "set power-on start",
		CmdSetSmartStartStop:   "set smart start/stop",
		CmdSetRealtimeRPM:      "set realtime target RPM",
		CmdEnterRealtimeRPM:    "enter realtime RPM mode",
		CmdExitRealtimeRPM:     "exit realtime RPM mode",
		CmdQueryWorkMode:       "query work state and active gear",
		CmdSetGearRPM:          "set one of four hardware gear RPM slots",
		CmdQueryGearRPMTable:   "query four-slot gear RPM table",
		CmdRGBStatus:           "query RGB enable state",
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
		CmdQueryDeviceInfo, CmdQueryConfigFlag, CmdInitializeController, CmdQueryConfigSnapshot,
		CmdClearInitializationLatch, CmdFactoryReinitialize, CmdQueryControllerCapability,
		CmdSetFixedGear, CmdQueryRuntimeProfile, CmdSetRuntimeProfile, CmdQueryIdentityBlock,
		CmdSetPowerOnStart, CmdSetSmartStartStop, CmdSetRealtimeRPM,
		CmdQueryRPMState, CmdEnterRealtimeRPM, CmdExitRealtimeRPM, CmdQueryWorkMode,
		CmdSetGearRPM, CmdQueryGearRPMTable, CmdRGBStatus, CmdRGBEnable,
		CmdRGBUploadInit, CmdRGBChunkWrite, CmdRGBCommit, CmdRGBDynamicParam,
		CmdRGBFrameWrite, CmdGearLight, CmdStatusNotify,
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
		value        byte
		wantMaxGear  string
		wantSelected string
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
		if decoded.GearTable[i].Gear != i+1 {
			t.Errorf("GearTable[%d].Gear = %d, want %d", i, decoded.GearTable[i].Gear, i+1)
		}
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
	payload := []byte{0x05}
	frame := Frame{Command: CmdQueryWorkMode, Payload: payload}
	decoded := DecodeFrame(frame)

	if decoded.Type != "queriedWorkState" {
		t.Errorf("Type = %q, want 'queriedWorkState'", decoded.Type)
	}
	if decoded.ModeName != "auto/realtime RPM mode" || decoded.RealtimeActive == nil || !*decoded.RealtimeActive {
		t.Errorf("ModeName = %q", decoded.ModeName)
	}
}

func TestDecodeFrame_WorkModeFixedGearIsNotNotificationBitField(t *testing.T) {
	decoded := DecodeFrame(Frame{Command: CmdQueryWorkMode, Payload: []byte{0x01}})
	if decoded.ModeName != "fixed gear 1" || decoded.ActiveGear != 0 {
		t.Fatalf("decoded 0x25 state 1 as %#v", decoded)
	}
	if decoded.RealtimeActive == nil || *decoded.RealtimeActive {
		t.Fatalf("state 1 must be fixed-gear mode: %#v", decoded.RealtimeActive)
	}
}

func TestDecodeWorkStateRealtimeGoldenVector(t *testing.T) {
	frame, ok := ParseFrame([]byte{0x5A, 0xA5, 0x25, 0x03, 0x05, 0x2D})
	if !ok || !frame.ChecksumOK {
		t.Fatalf("failed to parse 0x25 golden vector: ok=%v frame=%#v", ok, frame)
	}
	decoded := DecodeFrame(frame)
	if decoded.RealtimeActive == nil || !*decoded.RealtimeActive || decoded.ModeName != "auto/realtime RPM mode" {
		t.Fatalf("decoded work state = %#v", decoded)
	}
}

func TestDecodeFirmwareGoldenVector0035(t *testing.T) {
	raw := []byte{0x5A, 0xA5, 0x01, 0x06, 0x00, 0x00, 0x03, 0x05, 0x0F}
	frame, ok := ParseFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatalf("failed to parse firmware golden vector: ok=%v frame=%#v", ok, frame)
	}
	decoded := DecodeFrame(frame)
	if decoded.FirmwareVersion != "0.0.3.5" || decoded.FirmwareVersionRaw != "00 00 03 05" {
		t.Fatalf("decoded firmware = %#v", decoded)
	}
}

func TestDecodeFirmwareQueryAsRequest(t *testing.T) {
	raw := BuildFrame(CmdQueryFirmwareVersion)
	frame, ok := ParseFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatal("failed to parse firmware query")
	}
	decoded := DecodeRequest(frame)
	if decoded.Type != "request" || decoded.Summary != "request: query firmware version" {
		t.Fatalf("firmware query decoded as %#v", decoded)
	}
	if strings.Contains(decoded.Summary, "incomplete") {
		t.Fatalf("zero-payload query was treated as an incomplete response: %q", decoded.Summary)
	}
}

func TestDecodeFactoryGearTableGoldenVector(t *testing.T) {
	raw := []byte{0x5A, 0xA5, 0x27, 0x0A, 0xA4, 0x06, 0x60, 0x09, 0xB8, 0x0B, 0xA0, 0x0F, 0xB6}
	frame, ok := ParseFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatalf("failed to parse gear-table golden vector: ok=%v frame=%#v", ok, frame)
	}
	decoded := DecodeFrame(frame)
	want := []int{1700, 2400, 3000, 4000}
	if len(decoded.GearTable) != len(want) {
		t.Fatalf("GearTable = %#v", decoded.GearTable)
	}
	for i, rpm := range want {
		if decoded.GearTable[i].RPM != rpm {
			t.Fatalf("GearTable[%d].RPM = %d, want %d", i, decoded.GearTable[i].RPM, rpm)
		}
	}
}

func TestDecodeDeviceIdentifierGoldenVector(t *testing.T) {
	raw := []byte{0x5A, 0xA5, 0x04, 0x08, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x71}
	frame, ok := ParseFrame(raw)
	if !ok || !frame.ChecksumOK {
		t.Fatalf("failed to parse identifier golden vector: ok=%v frame=%#v", ok, frame)
	}
	decoded := DecodeFrame(frame)
	if decoded.DeviceIdentifier != "11:22:33:44:55:66" {
		t.Fatalf("device identifier = %q", decoded.DeviceIdentifier)
	}
}

func TestDecodeACK(t *testing.T) {
	decoded := DecodeFrame(Frame{Command: CmdSetRealtimeRPM, Payload: []byte{0x02}})
	if decoded.AckStatus == nil || *decoded.AckStatus != 2 || decoded.AckStatusName == "" {
		t.Fatalf("decoded ACK = %#v", decoded)
	}
}

func TestAckStatusNameIsCommandSpecific(t *testing.T) {
	if got := AckStatusName(CmdSetRealtimeRPM, 2); got != "realtime mode is not active" {
		t.Fatalf("0x21 status 2 = %q", got)
	}
	if got := AckStatusName(CmdExitRealtimeRPM, 2); got != "already outside realtime mode" {
		t.Fatalf("0x24 status 2 = %q", got)
	}
	if got := AckStatusName(CmdInitializeController, 5); got != "already initialized" {
		t.Fatalf("0x03 status 5 = %q", got)
	}
}

func TestRecoveredScalarFieldsKeepRawMeaning(t *testing.T) {
	capability := DecodeFrame(Frame{Command: CmdQueryControllerCapability, Payload: []byte{3}})
	if capability.ControllerCapabilityTier == nil || *capability.ControllerCapabilityTier != 3 {
		t.Fatalf("capability = %#v", capability)
	}
	profile := DecodeFrame(Frame{Command: CmdQueryRuntimeProfile, Payload: []byte{2}})
	if profile.RuntimeProfileRaw == nil || *profile.RuntimeProfileRaw != 2 {
		t.Fatalf("runtime profile = %#v", profile)
	}
	rpm := DecodeFrame(Frame{Command: CmdQueryRPMState, Payload: []byte{0x34, 0x08, 0xE3, 0x08}})
	if rpm.MeasuredRPM == nil || *rpm.MeasuredRPM != 2100 || rpm.RuntimeTargetRPM == nil || *rpm.RuntimeTargetRPM != 2275 {
		t.Fatalf("RPM state = %#v", rpm)
	}
}

func TestDecodeFrame_WorkMode_Short(t *testing.T) {
	frame := Frame{Command: CmdQueryWorkMode, Payload: []byte{}}
	decoded := DecodeFrame(frame)
	if decoded.Type != "queriedWorkState" {
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
	if decoded.RGBName != "on" {
		t.Errorf("RGBName = %q, want 'on'", decoded.RGBName)
	}
}

func TestDecodeFrame_RGBStatus_Off(t *testing.T) {
	frame := Frame{Command: CmdRGBStatus, Payload: []byte{0x00}}
	decoded := DecodeFrame(frame)
	if decoded.RGBName != "off" {
		t.Errorf("RGBName = %q, want 'off'", decoded.RGBName)
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

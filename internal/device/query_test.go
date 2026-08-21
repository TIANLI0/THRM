package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestQueryResponseFramesFiltersUnrelatedTraffic(t *testing.T) {
	frames := []types.DeviceDebugFrame{
		{Direction: "tx", Command: "0x25", ChecksumOK: true},
		{Direction: "rx", Command: "0xEF", ChecksumOK: true},
		{Direction: "rx", Command: "0x27", ChecksumOK: true},
		{Direction: "rx", Command: "0x25", ChecksumOK: false},
		{Direction: "rx", Command: "0x25", ChecksumOK: true},
	}

	got := queryResponseFrames(0x25, frames)
	if len(got) != 1 || got[0].Command != "0x25" || !got[0].ChecksumOK {
		t.Fatalf("queryResponseFrames() = %#v, want only the valid 0x25 response", got)
	}
}

func TestDetailedFirmwareProtocolModels(t *testing.T) {
	if !supportsDetailedFirmwareProtocol(ProductIDBS2) {
		t.Fatal("BS2 should use detailed firmware queries")
	}
	if !supportsDetailedFirmwareProtocol(ProductIDBS2PRO) {
		t.Fatal("BS2PRO should use detailed firmware queries")
	}
	if !supportsDetailedFirmwareProtocol(ProductIDBS3) {
		t.Fatal("BS3 should use detailed firmware queries")
	}
	if !supportsDetailedFirmwareProtocol(ProductIDBS3PRO) {
		t.Fatal("BS3PRO should use detailed firmware queries")
	}
	if supportsDetailedFirmwareProtocol(0) {
		t.Fatal("unknown/non-HID product must not use the detailed command profile")
	}
}

func TestFirmwareVersionFrameRequiresCompletePayload(t *testing.T) {
	settings := types.DeviceSettings{}
	err := applyDeviceSettingsFrame(&settings, deviceproto.Frame{
		Command: deviceproto.CmdQueryFirmwareVersion, Payload: []byte{0, 0, 3}, ChecksumOK: true,
	})
	if err == nil || settings.FirmwareVersion != "" {
		t.Fatalf("incomplete version should fail, settings=%#v err=%v", settings, err)
	}
	err = applyDeviceSettingsFrame(&settings, deviceproto.Frame{
		Command: deviceproto.CmdQueryFirmwareVersion, Payload: []byte{0, 0, 3, 5}, ChecksumOK: true,
	})
	if err != nil || settings.FirmwareVersion != "0.0.3.5" {
		t.Fatalf("complete version = %#v err=%v", settings, err)
	}
}

func TestDeviceCPUModelRequiresKnownFirmwareEvidence(t *testing.T) {
	model, source := deviceCPUModelFromKnownFirmware(ProductIDBS2PRO)
	if model != "WCH CH591" || source == "" {
		t.Fatalf("BS2PRO CPU mapping = %q, %q", model, source)
	}
	if model, source = deviceCPUModelFromKnownFirmware(ProductIDBS3PRO); model != "" || source != "" {
		t.Fatalf("BS3PRO CPU mapping should remain unknown, got %q, %q", model, source)
	}
}

func TestApplyDecodedDetailedFirmwareSettings(t *testing.T) {
	settings := types.DeviceSettings{}
	frames := []deviceproto.Frame{
		{Command: deviceproto.CmdQueryFirmwareVersion, Payload: []byte{0, 0, 3, 5}},
		{Command: deviceproto.CmdQueryDeviceID, Payload: []byte{1, 2, 3, 4, 5, 6}},
		{Command: deviceproto.CmdQueryWorkMode, Payload: []byte{5}},
	}
	for _, frame := range frames {
		applyDecodedDeviceSetting(&settings, deviceproto.DecodeFrame(frame))
	}
	if settings.FirmwareVersion != "0.0.3.5" {
		t.Fatalf("FirmwareVersion = %q", settings.FirmwareVersion)
	}
	if settings.DeviceIdentifier != "01:02:03:04:05:06" {
		t.Fatalf("DeviceIdentifier = %q", settings.DeviceIdentifier)
	}
	if settings.QueriedWorkState != "0x05" || settings.QueriedWorkStateName != "auto/realtime RPM mode" {
		t.Fatalf("queried work state was not kept separately: %#v", settings)
	}
	if settings.RealtimeActive != nil || settings.ActiveGear != 0 {
		t.Fatalf("0x25 must not be promoted to authoritative live state: %#v", settings)
	}
}

func TestApplyCurrentStatusKeepsQueriedStateSeparate(t *testing.T) {
	settings := types.DeviceSettings{
		QueriedWorkState:     "0x04",
		QueriedWorkStateName: "fixed gear 4 / initialized fallback",
	}

	applyCurrentStatus(&settings, &types.FanData{
		CurrentMode:  0x05,
		GearSettings: 0x4A,
		CurrentRPM:   1420,
		TargetRPM:    1500,
	})

	if settings.QueriedWorkState != "0x04" {
		t.Fatalf("QueriedWorkState was overwritten: %q", settings.QueriedWorkState)
	}
	if settings.LiveModeFlags != "0x05" || settings.LiveModeName != "auto/realtime RPM mode" {
		t.Fatalf("live mode = %q/%q", settings.LiveModeFlags, settings.LiveModeName)
	}
	if settings.SelectedGear != 2 || settings.ActiveGear != 0 {
		t.Fatalf("selected/active gear = %d/%d, want 2/0 in realtime", settings.SelectedGear, settings.ActiveGear)
	}
	if settings.Status == nil {
		t.Fatal("Status was not populated")
	}
	if settings.Status.Mode != "0x05" || settings.Status.TargetRPM != 1500 {
		t.Fatalf("Status = %#v, want latest realtime mode and target RPM", settings.Status)
	}
}

func TestApplyCurrentStatusDerivesManualGearFromEF(t *testing.T) {
	settings := types.DeviceSettings{QueriedWorkState: "0x04"}
	applyCurrentStatus(&settings, &types.FanData{CurrentMode: 0x02, GearSettings: 0x6C})
	if settings.SelectedGear != 3 || settings.ActiveGear != 3 {
		t.Fatalf("selected/active gear = %d/%d, want 3/3", settings.SelectedGear, settings.ActiveGear)
	}
}

func TestStatusSelectedGearUsesEFBitField(t *testing.T) {
	frame := deviceproto.Frame{Command: deviceproto.CmdStatusNotify, Payload: []byte{0x6C, 0x02}, ChecksumOK: true}
	gear, manual, ok := statusSelectedGear(frame)
	if !ok || !manual || gear != 3 {
		t.Fatalf("statusSelectedGear = gear %d manual %t ok %t", gear, manual, ok)
	}
	frame.Payload[1] = 0x05
	gear, manual, ok = statusSelectedGear(frame)
	if !ok || manual || gear != 3 {
		t.Fatalf("realtime status = gear %d manual %t ok %t", gear, manual, ok)
	}
}

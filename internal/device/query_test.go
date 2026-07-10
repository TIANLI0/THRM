package device

import (
	"testing"

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

func TestApplyCurrentStatusOverridesStaleWorkMode(t *testing.T) {
	settings := types.DeviceSettings{
		WorkMode:     "0x04",
		WorkModeName: "挡位工作模式",
	}

	applyCurrentStatus(&settings, &types.FanData{
		CurrentMode:  0x05,
		GearSettings: 0x4A,
		CurrentRPM:   1420,
		TargetRPM:    1500,
	})

	if settings.WorkMode != "0x05" {
		t.Fatalf("WorkMode = %q, want latest fan-data mode 0x05", settings.WorkMode)
	}
	if settings.Status == nil {
		t.Fatal("Status was not populated")
	}
	if settings.Status.Mode != "0x05" || settings.Status.TargetRPM != 1500 {
		t.Fatalf("Status = %#v, want latest realtime mode and target RPM", settings.Status)
	}
}

package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestLightProgramFirmwareLayout(t *testing.T) {
	program := newLightProgram(5, 10, 25)
	if got, want := program[0][:6], []byte{0, 2, 0, 5, 10, 25}; !equalBytes(got, want) {
		t.Fatalf("header = %v, want %v", got, want)
	}

	first := types.RGBColor{R: 255, G: 128, B: 64}
	last := types.RGBColor{R: 1, G: 2, B: 3}
	program.setColor(0, 0, first)
	program.setColor(5, 9, last)

	if got := program[0][6:9]; !equalBytes(got, []byte{255, 128, 64}) {
		t.Fatalf("first LED/keyframe color = %v", got)
	}
	if got := program[18][3:6]; !equalBytes(got, []byte{1, 2, 3}) {
		t.Fatalf("last LED/keyframe color = %v", got)
	}
	if program[0][5] != 25 || program[0][6] != 255 {
		t.Fatal("RGB values must remain unscaled; firmware applies the header brightness")
	}
}

func TestLightProgramUsesLEDMajorColorOrder(t *testing.T) {
	program := newLightProgram(1, 5, 100)
	program.setColor(1, 0, types.RGBColor{R: 10, G: 20, B: 30})
	program.setColor(0, 1, types.RGBColor{R: 40, G: 50, B: 60})

	// Header(6) + LED(1)*10*RGB(3) = byte 36.
	if got := program[3][6:9]; !equalBytes(got, []byte{10, 20, 30}) {
		t.Fatalf("LED-major offset = %v", got)
	}
	// Header(6) + keyframe(1)*RGB(3) = byte 9, crossing a 10-byte upload frame.
	if program[0][9] != 40 || program[1][0] != 50 || program[1][1] != 60 {
		t.Fatalf("cross-frame RGB = [%d %d %d]", program[0][9], program[1][0], program[1][1])
	}
}

func TestSmartLightPresetForTemperature(t *testing.T) {
	tests := []struct {
		name        string
		temperature int
		current     byte
		want        byte
	}{
		{name: "invalid", temperature: 0, want: 0},
		{name: "initial green", temperature: 70, want: 1},
		{name: "initial yellow", temperature: 71, want: 2},
		{name: "initial red", temperature: 81, want: 3},
		{name: "green hysteresis", temperature: 72, current: 1, want: 1},
		{name: "green to yellow", temperature: 73, current: 1, want: 2},
		{name: "yellow hysteresis down", temperature: 68, current: 2, want: 2},
		{name: "yellow to green", temperature: 67, current: 2, want: 1},
		{name: "yellow to red", temperature: 83, current: 2, want: 3},
		{name: "red hysteresis", temperature: 78, current: 3, want: 3},
		{name: "red to yellow", temperature: 77, current: 3, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := smartLightPresetForTemperature(test.temperature, test.current); got != test.want {
				t.Fatalf("preset(%d°C, current=%d) = %d, want %d", test.temperature, test.current, got, test.want)
			}
		})
	}
}

func TestSmartLightActivationSequenceResetsCustomAnimation(t *testing.T) {
	steps := smartLightActivationSequence(2)
	want := []struct {
		command byte
		payload byte
	}{
		{command: 0x46, payload: 0x00},
		{command: 0x44, payload: 0x00},
		{command: 0x46, payload: 0x01},
		{command: 0x44, payload: 0x02},
	}
	if len(steps) != len(want) {
		t.Fatalf("activation steps = %d, want %d", len(steps), len(want))
	}
	for i := range want {
		if steps[i].command != want[i].command || len(steps[i].payload) != 1 || steps[i].payload[0] != want[i].payload {
			t.Fatalf("step %d = command %#02x payload %v, want command %#02x payload %#02x",
				i, steps[i].command, steps[i].payload, want[i].command, want[i].payload)
		}
	}
}

func equalBytes(got, want []byte) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

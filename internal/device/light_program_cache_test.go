package device

import "testing"

func TestRepeatedLightProgramDoesNotUploadOrCommitAgain(t *testing.T) {
	program := newLightProgram(0, lightSpeedMedium, 100)
	m := NewManager(testLogger{})
	m.lastLightProgram, m.hasLightProgram = program, true
	m.hasRGBEnabled, m.rgbEnabled = true, true
	// No HID handle: any accidental retransmission would fail. A same-program
	// replay must complete without entering the transport or flash-write path.
	if err := m.applyLightFramesLocked(program); err != nil {
		t.Fatal(err)
	}
	if m.linkHealth.txCount != 0 || !m.lastFlashWriteAt.IsZero() {
		t.Fatal("same-program replay wrote to the device")
	}
	m.resetLightStateCacheLocked()
	if m.hasLightProgram {
		t.Fatal("new connection/maintenance must invalidate the program cache")
	}
}

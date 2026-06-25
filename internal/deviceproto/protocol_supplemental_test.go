package deviceproto

import (
	"testing"
)

// TestFrameRoundTripFull covers additional payload scenarios
func TestFrameRoundTripFull(t *testing.T) {
	tests := []struct {
		cmd     byte
		payload []byte
	}{
		{0xEF, nil},
		{0xEF, []byte{0x0B, 0x01, 0x02, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{0x26, []byte{0x01, 0x00, 0x00}},
		{0x08, []byte{0x01}},
		{0x45, []byte{0x00}},
	}

	for _, tt := range tests {
		var frame []byte
		if tt.payload == nil {
			frame = BuildFrame(tt.cmd)
		} else {
			frame = BuildFrame(tt.cmd, tt.payload...)
		}
		parsed, ok := ParseFrame(frame)
		if !ok {
			t.Fatalf("ParseFrame failed on valid frame for cmd=0x%02X", tt.cmd)
		}
		if parsed.Command != tt.cmd {
			t.Errorf("Parsed command = 0x%02X, want 0x%02X", parsed.Command, tt.cmd)
		}
		if len(parsed.Payload) != len(tt.payload) {
			t.Errorf("Parsed payload len = %d, want %d", len(parsed.Payload), len(tt.payload))
		}
		if !parsed.ChecksumOK {
			t.Error("Checksum should be valid on round-trip")
		}
		if len(parsed.Frame) != len(frame) {
			t.Errorf("Parsed frame len = %d, want %d", len(parsed.Frame), len(frame))
		}
	}
}

// TestParseFrameInvalidInputs verifies additional invalid frame scenarios
func TestParseFrameInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"zero length field", []byte{0x5A, 0xA5, 0xEF, 0x01, 0x00}},
		{"empty slice", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := ParseFrame(tt.data)
			if ok {
				t.Error("Expected ParseFrame to fail on invalid input")
			}
		})
	}
}

// TestNormalizeDebugInputVariants covers input formats not in existing tests
func TestNormalizeDebugInputVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ok    bool
	}{
		{"colon separated", "5A:A5:EF:02:00:F1", true},
		{"comma separated", "5A,A5,EF,02,00,F1", true},
		{"raw cmd with payload", "EF 00", true},
		{"no separator", "5AA5EF0200F1", true},
		{"invalid hex", "xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeDebugInput(tt.input)
			if tt.ok && err != nil {
				t.Errorf("NormalizeDebugInput(%q) unexpected error: %v", tt.input, err)
			}
			if !tt.ok && err == nil {
				t.Errorf("NormalizeDebugInput(%q) should have failed", tt.input)
			}
		})
	}
}

// TestHexOutputIsParseable verifies Hex() output can be parsed back
func TestHexOutputIsParseable(t *testing.T) {
	original := []byte{0x5A, 0xA5, 0xEF, 0x02, 0x00, 0xF1}
	hexStr := Hex(original)

	parsed, err := ParseHex(hexStr)
	if err != nil {
		t.Fatalf("ParseHex error: %v", err)
	}
	if len(parsed) != len(original) {
		t.Fatalf("Round-trip len = %d, want %d", len(parsed), len(original))
	}
	for i := range original {
		if parsed[i] != original[i] {
			t.Errorf("Round-trip byte[%d] = 0x%02X, want 0x%02X", i, parsed[i], original[i])
		}
	}
}

// TestBuildFrameLengthField verifies length field = 2 + len(payload)
func TestBuildFrameLengthField(t *testing.T) {
	for _, n := range []int{0, 1, 3, 5, 11} {
		payload := make([]byte, n)
		for i := range payload {
			payload[i] = byte(i)
		}
		var frame []byte
		if n == 0 {
			frame = BuildFrame(0x26)
		} else {
			frame = BuildFrame(0x26, payload...)
		}
		expectedLen := 2 + n
		if int(frame[3]) != expectedLen {
			t.Errorf("Frame with %d-byte payload: length field = %d, want %d", n, frame[3], expectedLen)
		}
	}
}

// TestParseFrameWithReportIDFullChain verifies full parse chain from HID report
func TestParseFrameWithReportIDFullChain(t *testing.T) {
	frame := BuildFrame(0xEF, 0x0B)
	report := BuildReport(frame, 25)

	parsed, ok := ParseFrame(report)
	if !ok {
		t.Fatal("ParseFrame failed on HID report")
	}
	if parsed.ReportID != ReportID {
		t.Errorf("ReportID = 0x%02X, want 0x%02X", parsed.ReportID, ReportID)
	}
	if parsed.Command != 0xEF {
		t.Errorf("Command = 0x%02X, want 0xEF", parsed.Command)
	}
	if len(parsed.Payload) != 1 || parsed.Payload[0] != 0x0B {
		t.Error("Payload mismatch when parsing from report")
	}
}

// TestReportZeroPadding verifies report is zero-padded to full length
func TestReportZeroPadding(t *testing.T) {
	frame := BuildFrame(0xEF)
	report := BuildReport(frame, 25)

	if report[0] != ReportID {
		t.Errorf("Report[0] = 0x%02X, want 0x%02X", report[0], ReportID)
	}
	if len(report) != 25 {
		t.Errorf("Report len = %d, want 25", len(report))
	}
	// Remaining bytes after ReportID + frame should be zero
	for i := len(frame) + 1; i < len(report); i++ {
		if report[i] != 0 {
			t.Errorf("Report[%d] = 0x%02X, want 0x00 (zero-padded)", i, report[i])
		}
	}
}

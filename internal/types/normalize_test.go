package types

import (
	"testing"
)

func TestNormalizeThemeMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{ThemeModeSystem, ThemeModeSystem},
		{ThemeModeLight, ThemeModeLight},
		{ThemeModeDark, ThemeModeDark},
		{ThemeModeTHRM, ThemeModeTHRM},
	}
	for _, tt := range tests {
		got := NormalizeThemeMode(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeThemeMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
	// unknown values pass through
	if got := NormalizeThemeMode("unknown"); got != "unknown" {
		t.Errorf("NormalizeThemeMode('unknown') = %q, expect passthrough", got)
	}
}

func TestNormalizeTempSource(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{TempSourceMax, TempSourceMax},
		{TempSourceCPU, TempSourceCPU},
		{TempSourceGPU, TempSourceGPU},
		{"unknown", TempSourceMax},
		{"", TempSourceMax},
	}
	for _, tt := range tests {
		got := NormalizeTempSource(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTempSource(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeWindowBlur(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{WindowBlurAuto, WindowBlurAuto},
		{WindowBlurOn, WindowBlurOn},
		{"acrylic", "acrylic"},
		{"mica", "mica"},
		{"tabbed", "tabbed"},
		{WindowBlurOff, WindowBlurOff},
		{"invalid", WindowBlurAuto},
	}
	for _, tt := range tests {
		got := NormalizeWindowBlur(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeWindowBlur(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeSensorSelection(t *testing.T) {
	if got := NormalizeSensorSelection(TempSensorAuto); got != TempSensorAuto {
		t.Errorf("NormalizeSensorSelection(%q) = %q", TempSensorAuto, got)
	}
	if got := NormalizeSensorSelection(""); got != TempSensorAuto {
		t.Errorf("NormalizeSensorSelection('') = %q, want %q", got, TempSensorAuto)
	}
}

func TestNormalizeSensorSelections(t *testing.T) {
	got := NormalizeSensorSelections([]string{"cpu", "gpu"})
	if len(got) != 2 || got[0] != "cpu" || got[1] != "gpu" {
		t.Errorf("NormalizeSensorSelections([cpu, gpu]) = %v", got)
	}
	got = NormalizeSensorSelections(nil)
	if got != nil {
		t.Errorf("NormalizeSensorSelections(nil) = %v, want nil", got)
	}
}

func TestNormalizeDeviceSelection(t *testing.T) {
	if got := NormalizeDeviceSelection(TempDeviceAuto); got != TempDeviceAuto {
		t.Errorf("NormalizeDeviceSelection(%q) = %q", TempDeviceAuto, got)
	}
	if got := NormalizeDeviceSelection(""); got != TempDeviceAuto {
		t.Errorf("NormalizeDeviceSelection('') = %q, want %q", got, TempDeviceAuto)
	}
}

func TestNormalizeLearningBias(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{LearningBiasBalanced, LearningBiasBalanced},
		{LearningBiasCooling, LearningBiasCooling},
		{LearningBiasQuiet, LearningBiasQuiet},
		{"invalid", LearningBiasBalanced},
	}
	for _, tt := range tests {
		got := NormalizeLearningBias(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLearningBias(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetDefaultTemperatureSelection(t *testing.T) {
	sel := GetDefaultTemperatureSelection()
	if sel.TempSource == "" {
		t.Error("TempSource should not be empty")
	}
}

func TestNormalizeTemperatureSelection(t *testing.T) {
	valid := TemperatureSelection{TempSource: TempSourceCPU, GpuDevice: TempDeviceAuto}
	got := NormalizeTemperatureSelection(valid)
	if got.TempSource != TempSourceCPU {
		t.Errorf("TempSource = %q", got.TempSource)
	}

	invalid := TemperatureSelection{TempSource: "bad"}
	got = NormalizeTemperatureSelection(invalid)
	if got.TempSource == "bad" {
		t.Errorf("should normalize invalid TempSource: got %q", got.TempSource)
	}
}

func TestGearIndex(t *testing.T) {
	tests := []struct {
		gear      string
		wantFound bool
	}{
		{"静音", true},
		{"标准", true},
		{"强劲", true},
		{"超频", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		idx, found := GearIndex(tt.gear)
		if found != tt.wantFound {
			t.Errorf("GearIndex(%q) found = %v, want %v", tt.gear, found, tt.wantFound)
		}
		if found && idx < 0 {
			t.Errorf("GearIndex(%q) idx = %d, want >= 0", tt.gear, idx)
		}
	}
}

func TestDefaultGearRPM(t *testing.T) {
	gears := []string{"静音", "标准", "强劲", "超频"}
	levels := []string{"低", "中", "高"}
	for _, gear := range gears {
		for _, level := range levels {
			rpm := DefaultGearRPM(gear, level)
			if rpm < ManualGearMinRPM || rpm > ManualGearMaxRPM {
				t.Errorf("DefaultGearRPM(%q, %q) = %d, out of range [%d, %d]",
					gear, level, rpm, ManualGearMinRPM, ManualGearMaxRPM)
			}
		}
	}
}

func TestBuildGearRPMCommand(t *testing.T) {
	cmd := BuildGearRPMCommand(0, 2000)
	if len(cmd) < 5 {
		t.Errorf("BuildGearRPMCommand too short: %d bytes", len(cmd))
	}
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig(false)
	if len(cfg.FanCurve) == 0 {
		t.Error("DefaultConfig FanCurve should not be empty")
	}
	if FanCurveMaxTemperature != 110 {
		t.Errorf("FanCurveMaxTemperature = %d, want 110", FanCurveMaxTemperature)
	}
}

func TestCloneDefaultManualGearRPM(t *testing.T) {
	original := CloneDefaultManualGearRPM()
	if len(original) == 0 {
		t.Fatal("CloneDefaultManualGearRPM returned empty map")
	}
	clone := CloneDefaultManualGearRPM()
	for gear := range original {
		for level := range original[gear] {
			clone[gear][level] = 9999
			if original[gear][level] == 9999 {
				t.Errorf("clone should be independent: %s/%s modified original", gear, level)
			}
		}
	}
}

func TestNormalizeManualGearRPM(t *testing.T) {
	cfg := GetDefaultConfig(false)

	aboveMax := cfg.ManualGearRPM["静音"]["低"] + 10000
	cfg.ManualGearRPM["静音"]["低"] = aboveMax
	NormalizeManualGearRPM(&cfg)

	if cfg.ManualGearRPM["静音"]["低"] > ManualGearMaxRPM {
		t.Errorf("RPM not clamped: %d > %d", cfg.ManualGearRPM["静音"]["低"], ManualGearMaxRPM)
	}
	if cfg.ManualGearRPM["静音"]["低"] < ManualGearMinRPM {
		t.Errorf("RPM clamped too low: %d < %d", cfg.ManualGearRPM["静音"]["低"], ManualGearMinRPM)
	}
}

func TestDeviceTypeConstants(t *testing.T) {
	if DeviceTypeHID != "hid" {
		t.Errorf("DeviceTypeHID = %q, want 'hid'", DeviceTypeHID)
	}
	if DeviceTypeBLE != "ble" {
		t.Errorf("DeviceTypeBLE = %q, want 'ble'", DeviceTypeBLE)
	}
}

func TestBS1GearCommands(t *testing.T) {
	if len(BS1GearCommands) != 4 {
		t.Errorf("BS1GearCommands len = %d, want 4", len(BS1GearCommands))
	}
	for _, name := range []string{"静音", "标准", "强劲", "超频"} {
		if _, ok := BS1GearCommands[name]; !ok {
			t.Errorf("BS1GearCommands missing key %q", name)
		}
	}
}

func TestGetDefaultSmartControlConfig(t *testing.T) {
	curve := GetDefaultFanCurve()
	cfg := GetDefaultSmartControlConfig(curve)
	if cfg.TargetTemp < 45 || cfg.TargetTemp > 90 {
		t.Errorf("TargetTemp = %d out of range", cfg.TargetTemp)
	}
	if len(cfg.LearnedOffsets) != len(curve) {
		t.Errorf("LearnedOffsets len = %d, want %d", len(cfg.LearnedOffsets), len(curve))
	}
}

func TestGetDefaultLegionFnQConfig(t *testing.T) {
	cfg := GetDefaultLegionFnQConfig()
	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.ModeMapping == nil {
		t.Error("ModeMapping should not be nil")
	}
}

func TestNormalizeLegionFnQConfig(t *testing.T) {
	cfg := LegionFnQConfig{
		Enabled:     true,
		TakeOverFan: true,
		ModeMapping: nil,
	}
	normalized := NormalizeLegionFnQConfig(cfg)
	if normalized.ModeMapping == nil {
		t.Error("ModeMapping should be populated from defaults")
	}
	if len(normalized.ModeMapping) == 0 {
		t.Error("ModeMapping should have entries")
	}
}

func TestBS1Checksum(t *testing.T) {
	cs := BS1Checksum([]byte{0x08, 0x01})
	if cs != 0x0A {
		t.Errorf("BS1Checksum([0x08, 0x01]) = 0x%02X, want 0x0A (sum+1)", cs)
	}
}

func TestBuildBS1RPMCommand(t *testing.T) {
	cmd := BuildBS1RPMCommand(2000)
	if len(cmd) < 5 {
		t.Errorf("BuildBS1RPMCommand too short")
	}
}

func TestGetDefaultLightStripConfig(t *testing.T) {
	cfg := GetDefaultLightStripConfig()
	if cfg.Brightness < 0 || cfg.Brightness > 100 {
		t.Errorf("Brightness = %d out of range", cfg.Brightness)
	}
}

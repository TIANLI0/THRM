package types

import (
	"testing"
)

func TestGetDefaultSpeedAvoidanceConfig(t *testing.T) {
	cfg := GetDefaultSpeedAvoidanceConfig()
	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.MinRPM <= 0 {
		t.Error("MinRPM should be positive")
	}
	if cfg.MaxRPM <= cfg.MinRPM {
		t.Error("MaxRPM should be greater than MinRPM")
	}
}

func TestGetDefaultTimeCurveScheduleConfig(t *testing.T) {
	cfg := GetDefaultTimeCurveScheduleConfig()
	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.Rules == nil {
		t.Error("Rules should not be nil")
	}
}

func TestNormalizeSpeedAvoidanceConfig(t *testing.T) {
	cfg := SpeedAvoidanceConfig{
		Enabled:   true,
		MinRPM:    0,
		MaxRPM:    0,
		MarginRPM: 0,
	}
	normalized := NormalizeSpeedAvoidanceConfig(cfg)
	if normalized.MinRPM <= 0 {
		t.Error("MinRPM should be replaced with default when <= 0")
	}
	if normalized.MaxRPM <= 0 {
		t.Error("MaxRPM should be replaced with default when <= 0")
	}
	if normalized.MinRPM > normalized.MaxRPM {
		t.Error("MinRPM should not exceed MaxRPM after normalization")
	}
	if normalized.MarginRPM < 50 || normalized.MarginRPM > 500 {
		t.Errorf("MarginRPM = %d, want [50, 500]", normalized.MarginRPM)
	}
}

func TestNormalizeSpeedAvoidanceConfig_Clamp(t *testing.T) {
	cfg := SpeedAvoidanceConfig{
		MinRPM:              5000,
		MaxRPM:              1000,
		MarginRPM:           550,
		EmergencyBypassTemp: 100,
	}
	normalized := NormalizeSpeedAvoidanceConfig(cfg)
	if normalized.MinRPM > normalized.MaxRPM {
		t.Error("MinRPM and MaxRPM should be swapped")
	}
	if normalized.EmergencyBypassTemp > 95 {
		t.Errorf("EmergencyBypassTemp = %d, should be clamped to max 95", normalized.EmergencyBypassTemp)
	}
}

func TestNormalizeTimeCurveScheduleConfig(t *testing.T) {
	profiles := []FanCurveProfile{
		{ID: "p1", Name: "profile1"},
	}
	cfg := TimeCurveScheduleConfig{
		Enabled: true,
		Rules: []TimeCurveScheduleRule{
			{Enabled: true, Weekdays: []int{1, 3, 5}, CurveProfileID: "p1"},
		},
	}
	normalized := NormalizeTimeCurveScheduleConfig(cfg, profiles, "p1")
	if len(normalized.Rules) != 1 {
		t.Fatalf("rules len = %d", len(normalized.Rules))
	}
	if normalized.Rules[0].ID == "" {
		t.Error("rule ID should be populated")
	}
	if normalized.Rules[0].Name == "" {
		t.Error("rule Name should be populated")
	}
}

func TestNormalizeTimeCurveScheduleConfig_InvalidProfile(t *testing.T) {
	cfg := TimeCurveScheduleConfig{
		Rules: []TimeCurveScheduleRule{
			{CurveProfileID: "nonexistent"},
		},
	}
	normalized := NormalizeTimeCurveScheduleConfig(cfg, nil, "fallback")
	if normalized.Rules[0].CurveProfileID != "fallback" {
		t.Errorf("CurveProfileID = %q, want 'fallback'", normalized.Rules[0].CurveProfileID)
	}
}

func TestNormalizeTimeCurveScheduleConfig_EmptyWeekdays(t *testing.T) {
	cfg := TimeCurveScheduleConfig{
		Rules: []TimeCurveScheduleRule{{}},
	}
	normalized := NormalizeTimeCurveScheduleConfig(cfg, nil, "")
	if len(normalized.Rules[0].Weekdays) != 7 {
		t.Errorf("Weekdays len = %d, want 7 (default all days)", len(normalized.Rules[0].Weekdays))
	}
}

func TestNormalizeTimeCurveScheduleConfig_InvalidClock(t *testing.T) {
	cfg := TimeCurveScheduleConfig{
		Rules: []TimeCurveScheduleRule{{StartTime: "99:99"}},
	}
	normalized := NormalizeTimeCurveScheduleConfig(cfg, nil, "")
	if normalized.Rules[0].StartTime != "00:00" {
		t.Errorf("StartTime = %q, want '00:00'", normalized.Rules[0].StartTime)
	}
}

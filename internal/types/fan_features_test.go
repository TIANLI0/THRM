package types

import (
	"testing"
)

func TestGetDefaultTimeCurveScheduleConfig(t *testing.T) {
	cfg := GetDefaultTimeCurveScheduleConfig()
	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.Rules == nil {
		t.Error("Rules should not be nil")
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

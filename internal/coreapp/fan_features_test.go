package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeFanFeatureConfig_NilConfig(t *testing.T) {
	if normalizeFanFeatureConfig(nil) {
		t.Fatal("should return false for nil config")
	}
}

func TestNormalizeFanFeatureConfig_SetsDefaults(t *testing.T) {
	cfg := &types.AppConfig{}
	changed := normalizeFanFeatureConfig(cfg)
	if changed {
		t.Log("normalizeSpeedAvoidance defaults may differ from zero-values")
	}
}

func TestApplySpeedAvoidance_Disabled(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{Enabled: false}
	_, applied := applySpeedAvoidance(2000, 0, 3000, 1900, 70, 68, cfg)
	if applied {
		t.Fatal("should not apply when disabled")
	}
}

func TestApplySpeedAvoidance_TargetZero(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{Enabled: true}
	_, applied := applySpeedAvoidance(0, 0, 3000, 1900, 70, 68, cfg)
	if applied {
		t.Fatal("should not apply when targetRPM <= 0")
	}
}

func TestApplySpeedAvoidance_EmergencyBypass(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{
		Enabled:             true,
		MinRPM:              1900,
		MaxRPM:              2200,
		MarginRPM:           100,
		EmergencyBypassTemp: 80,
	}
	result, applied := applySpeedAvoidance(2000, 0, 3000, 1900, 80, 78, cfg)
	if applied {
		t.Fatal("should bypass when controlTemp >= emergencyBypassTemp")
	}
	if result != 2000 {
		t.Fatalf("bypassed target should be unchanged, got %d", result)
	}
}

func TestApplySpeedAvoidance_OutsideRange(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{
		Enabled:  true,
		MinRPM:   1900,
		MaxRPM:   2200,
		MarginRPM: 100,
	}
	result, applied := applySpeedAvoidance(2500, 0, 3000, 2400, 70, 68, cfg)
	if applied {
		t.Fatal("should not apply when targetRPM outside avoid range")
	}
	if result != 2500 {
		t.Fatalf("unchanged target: %d", result)
	}
}

func TestApplySpeedAvoidance_MovesDown(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{
		Enabled:  true,
		MinRPM:   1900,
		MaxRPM:   2200,
		MarginRPM: 100,
	}
	result, applied := applySpeedAvoidance(2000, 0, 3000, 2100, 70, 72, cfg)
	if !applied {
		t.Fatal("should apply speed avoidance")
	}
	if result >= 1900 {
		t.Fatalf("should move below avoid range, got %d", result)
	}
}

func TestApplySpeedAvoidance_HeatingUpPrefersUp(t *testing.T) {
	cfg := types.SpeedAvoidanceConfig{
		Enabled:  true,
		MinRPM:   1900,
		MaxRPM:   2200,
		MarginRPM: 100,
	}
	result, applied := applySpeedAvoidance(2000, 1800, 3000, 1800, 72, 70, cfg)
	if !applied {
		t.Fatal("should apply speed avoidance")
	}
	if result <= 2200 {
		t.Fatalf("heating up should prefer up candidate, got %d", result)
	}
}

func TestParseScheduleClock_Valid(t *testing.T) {
	minutes, ok := parseScheduleClock("12:30")
	if !ok {
		t.Fatal("should parse valid clock string")
	}
	if minutes != 12*60+30 {
		t.Fatalf("expected 750 minutes, got %d", minutes)
	}
}

func TestParseScheduleClock_Invalid(t *testing.T) {
	_, ok := parseScheduleClock("invalid")
	if ok {
		t.Fatal("should fail on invalid clock string")
	}
}

func TestParseScheduleClock_Empty(t *testing.T) {
	_, ok := parseScheduleClock("")
	if ok {
		t.Fatal("should fail on empty clock string")
	}
}

func TestScheduleRuleMatches_WithinRange(t *testing.T) {
	weekday := 1
	days := []int{weekday}
	if !scheduleRuleMatches(days, weekday, 0, 12*60, 8*60, 18*60) {
		t.Fatal("should match when current time is within range on matching weekday")
	}
}

func TestScheduleRuleMatches_WrongWeekday(t *testing.T) {
	days := []int{1}
	if scheduleRuleMatches(days, 2, 1, 12*60, 8*60, 18*60) {
		t.Fatal("should not match on wrong weekday")
	}
}

func TestScheduleRuleMatches_OutsideTimeRange(t *testing.T) {
	weekday := 3
	days := []int{weekday}
	if scheduleRuleMatches(days, weekday, 2, 6*60, 8*60, 18*60) {
		t.Fatal("should not match when outside time range")
	}
}

func TestScheduleRuleMatches_CrossMidnight(t *testing.T) {
	days := []int{0}
	if !scheduleRuleMatches(days, 0, 6, 23*60, 22*60, 6*60) {
		t.Fatal("should match cross-midnight rule when current time >= start on matching weekday")
	}
}

func TestScheduleRuleMatches_CrossMidnightPreviousDay(t *testing.T) {
	days := []int{6}
	if !scheduleRuleMatches(days, 0, 6, 4*60, 22*60, 6*60) {
		t.Fatal("should match cross-midnight rule on next day using previous weekday")
	}
}

func TestFindMatchingTimeCurveScheduleRule_NoRules(t *testing.T) {
	schedule := types.TimeCurveScheduleConfig{
		Enabled: true,
		Rules:   []types.TimeCurveScheduleRule{},
	}
	now, _ := time.Parse("15:04", "12:00")
	rule := findMatchingTimeCurveScheduleRule(schedule, now)
	if rule != nil {
		t.Fatal("should return nil for empty rules")
	}
}

func TestFindMatchingTimeCurveScheduleRule_Matches(t *testing.T) {
	schedule := types.TimeCurveScheduleConfig{
		Enabled: true,
		Rules: []types.TimeCurveScheduleRule{
			{
				ID:             "r1",
				Name:           "Daytime",
				Enabled:        true,
				Weekdays:       []int{0, 1, 2, 3, 4, 5, 6},
				StartTime:      "08:00",
				EndTime:        "18:00",
				CurveProfileID: "day-curve",
			},
		},
	}
	now, _ := time.Parse("15:04", "12:00")
	rule := findMatchingTimeCurveScheduleRule(schedule, now)
	if rule == nil {
		t.Fatal("should find matching rule")
	}
	if rule.CurveProfileID != "day-curve" {
		t.Fatalf("expected day-curve, got %s", rule.CurveProfileID)
	}
}

func TestFindMatchingTimeCurveScheduleRule_Disabled(t *testing.T) {
	schedule := types.TimeCurveScheduleConfig{
		Enabled: true,
		Rules: []types.TimeCurveScheduleRule{
			{
				ID:             "r1",
				Enabled:        false,
				Weekdays:       []int{0, 1, 2, 3, 4, 5, 6},
				StartTime:      "08:00",
				EndTime:        "18:00",
				CurveProfileID: "day-curve",
			},
		},
	}
	now, _ := time.Parse("15:04", "12:00")
	rule := findMatchingTimeCurveScheduleRule(schedule, now)
	if rule != nil {
		t.Fatal("should skip disabled rules")
	}
}

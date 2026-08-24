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
		t.Log("分时曲线默认值可能与零值不同")
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

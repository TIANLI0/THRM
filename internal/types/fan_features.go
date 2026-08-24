package types

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type TimeCurveScheduleConfig struct {
	Enabled bool                    `json:"enabled"`
	Rules   []TimeCurveScheduleRule `json:"rules"`
}

type TimeCurveScheduleRule struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	Weekdays       []int  `json:"weekdays"`
	StartTime      string `json:"startTime"`
	EndTime        string `json:"endTime"`
	CurveProfileID string `json:"curveProfileId"`
}

func GetDefaultTimeCurveScheduleConfig() TimeCurveScheduleConfig {
	return TimeCurveScheduleConfig{
		Enabled: false,
		Rules:   []TimeCurveScheduleRule{},
	}
}

func NormalizeTimeCurveScheduleConfig(cfg TimeCurveScheduleConfig, profiles []FanCurveProfile, activeProfileID string) TimeCurveScheduleConfig {
	cfg.Rules = append([]TimeCurveScheduleRule(nil), cfg.Rules...)
	if cfg.Rules == nil {
		cfg.Rules = []TimeCurveScheduleRule{}
	}

	fallbackProfileID := strings.TrimSpace(activeProfileID)
	if fallbackProfileID == "" && len(profiles) > 0 {
		fallbackProfileID = strings.TrimSpace(profiles[0].ID)
	}

	for index := range cfg.Rules {
		rule := &cfg.Rules[index]
		if rule.ID == "" {
			rule.ID = fmt.Sprintf("schedule-%d", index+1)
		}
		if strings.TrimSpace(rule.Name) == "" {
			rule.Name = fmt.Sprintf("时段 %d", index+1)
		} else {
			rule.Name = strings.TrimSpace(rule.Name)
		}
		rule.StartTime = normalizeScheduleClock(rule.StartTime, "00:00")
		rule.EndTime = normalizeScheduleClock(rule.EndTime, "23:59")
		rule.Weekdays = normalizeScheduleWeekdays(rule.Weekdays)
		rule.CurveProfileID = normalizeScheduleProfileID(rule.CurveProfileID, profiles, fallbackProfileID)
	}

	return cfg
}

func normalizeScheduleClock(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := time.Parse("15:04", trimmed)
	if err != nil {
		return fallback
	}
	return parsed.Format("15:04")
}

func normalizeScheduleWeekdays(days []int) []int {
	if len(days) == 0 {
		return []int{0, 1, 2, 3, 4, 5, 6}
	}
	unique := make([]int, 0, 7)
	seen := map[int]struct{}{}
	for _, day := range days {
		if day < 0 || day > 6 {
			continue
		}
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		unique = append(unique, day)
	}
	if len(unique) == 0 {
		return []int{0, 1, 2, 3, 4, 5, 6}
	}
	slices.Sort(unique)
	return unique
}

func normalizeScheduleProfileID(profileID string, profiles []FanCurveProfile, fallback string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID != "" {
		for _, profile := range profiles {
			if strings.TrimSpace(profile.ID) == profileID {
				return profileID
			}
		}
	}
	return strings.TrimSpace(fallback)
}

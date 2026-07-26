package types

import (
	"maps"
	"slices"
)

// Clone 返回 AppConfig 的深拷贝。
//
// AppConfig 里混有切片与映射字段，直接按值复制只会共享底层数组/哈希表。
// config.Manager 在 Get/Set 两个方向上都必须切断这种共享，否则调用方持有的
// cfg 与管理器内部状态互为别名：一方原地写入切片元素（例如学习偏移、曲线点）
// 就会在另一方毫无同步的情况下被观察到，形成数据竞争。
func (c AppConfig) Clone() AppConfig {
	cloned := c

	cloned.ManualGearLevels = maps.Clone(c.ManualGearLevels)
	cloned.ManualGearRPM = cloneNestedIntMap(c.ManualGearRPM)
	cloned.FanCurve = slices.Clone(c.FanCurve)
	cloned.FanCurveProfiles = cloneFanCurveProfiles(c.FanCurveProfiles)
	cloned.CpuSensors = slices.Clone(c.CpuSensors)

	cloned.LegionFnQ.ModeMapping = maps.Clone(c.LegionFnQ.ModeMapping)
	cloned.LightStrip.Colors = slices.Clone(c.LightStrip.Colors)
	cloned.TimeCurveSchedule.Rules = cloneTimeCurveRules(c.TimeCurveSchedule.Rules)
	cloned.SmartControl = c.SmartControl.Clone()

	return cloned
}

// Clone 返回 SmartControlConfig 的深拷贝。学习偏移是运行期唯一会被高频改写的
// 配置区域，单独暴露该方法便于智能控温路径在不复制整份配置时也能安全传递。
func (s SmartControlConfig) Clone() SmartControlConfig {
	cloned := s

	cloned.LearnedOffsets = slices.Clone(s.LearnedOffsets)
	cloned.LearnedOffsetsHeat = slices.Clone(s.LearnedOffsetsHeat)
	cloned.LearnedOffsetsCool = slices.Clone(s.LearnedOffsetsCool)
	cloned.LearnedRateHeat = slices.Clone(s.LearnedRateHeat)
	cloned.LearnedRateCool = slices.Clone(s.LearnedRateCool)
	cloned.NoiseProfile = slices.Clone(s.NoiseProfile)
	cloned.TargetTempByProfile = maps.Clone(s.TargetTempByProfile)
	cloned.LearningBiasByProfile = maps.Clone(s.LearningBiasByProfile)

	if s.LearnedOffsetsByProfile != nil {
		byProfile := make(map[string][]int, len(s.LearnedOffsetsByProfile))
		for id, offsets := range s.LearnedOffsetsByProfile {
			byProfile[id] = slices.Clone(offsets)
		}
		cloned.LearnedOffsetsByProfile = byProfile
	}

	return cloned
}

func cloneNestedIntMap(source map[string]map[string]int) map[string]map[string]int {
	if source == nil {
		return nil
	}
	cloned := make(map[string]map[string]int, len(source))
	for key, inner := range source {
		cloned[key] = maps.Clone(inner)
	}
	return cloned
}

func cloneFanCurveProfiles(source []FanCurveProfile) []FanCurveProfile {
	if source == nil {
		return nil
	}
	cloned := make([]FanCurveProfile, len(source))
	for i, profile := range source {
		profile.Curve = slices.Clone(profile.Curve)
		cloned[i] = profile
	}
	return cloned
}

func cloneTimeCurveRules(source []TimeCurveScheduleRule) []TimeCurveScheduleRule {
	if source == nil {
		return nil
	}
	cloned := make([]TimeCurveScheduleRule, len(source))
	for i, rule := range source {
		rule.Weekdays = slices.Clone(rule.Weekdays)
		cloned[i] = rule
	}
	return cloned
}

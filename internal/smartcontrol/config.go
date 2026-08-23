package smartcontrol

import "github.com/TIANLI0/THRM/internal/types"

// NormalizeConfig 归一化智能控温配置。
func NormalizeConfig(cfg types.SmartControlConfig, curve []types.FanCurvePoint, _ bool) (types.SmartControlConfig, bool) {
	defaults := types.GetDefaultSmartControlConfig(curve)
	changed := false

	if cfg.TargetTemp < 45 || cfg.TargetTemp > 90 {
		cfg.TargetTemp = defaults.TargetTemp
		changed = true
	}
	if cfg.Aggressiveness < 1 || cfg.Aggressiveness > 10 {
		cfg.Aggressiveness = defaults.Aggressiveness
		changed = true
	}
	if cfg.Hysteresis < 0 || cfg.Hysteresis > 8 {
		cfg.Hysteresis = defaults.Hysteresis
		changed = true
	}
	if cfg.MinRPMChange < 20 || cfg.MinRPMChange > 400 {
		cfg.MinRPMChange = defaults.MinRPMChange
		changed = true
	}
	if cfg.RampUpLimit < 50 || cfg.RampUpLimit > 1200 {
		cfg.RampUpLimit = defaults.RampUpLimit
		changed = true
	}
	if cfg.RampDownLimit < 50 || cfg.RampDownLimit > 1200 {
		cfg.RampDownLimit = defaults.RampDownLimit
		changed = true
	}
	if cfg.LearnRate < 1 || cfg.LearnRate > 10 {
		cfg.LearnRate = defaults.LearnRate
		changed = true
	}
	if normalizedBias := types.NormalizeLearningBias(cfg.LearningBias); normalizedBias != cfg.LearningBias {
		cfg.LearningBias = normalizedBias
		changed = true
	}
	if cfg.LearnWindow < 3 || cfg.LearnWindow > 24 {
		cfg.LearnWindow = defaults.LearnWindow
		changed = true
	}
	if cfg.LearnDelay < 1 || cfg.LearnDelay > 8 {
		cfg.LearnDelay = defaults.LearnDelay
		changed = true
	}
	if cfg.OverheatWeight < 1 || cfg.OverheatWeight > 12 {
		cfg.OverheatWeight = defaults.OverheatWeight
		changed = true
	}
	if cfg.RPMDeltaWeight < 1 || cfg.RPMDeltaWeight > 12 {
		cfg.RPMDeltaWeight = defaults.RPMDeltaWeight
		changed = true
	}
	if cfg.NoiseWeight < 0 || cfg.NoiseWeight > 12 {
		cfg.NoiseWeight = defaults.NoiseWeight
		changed = true
	}
	if cfg.TrendGain < 1 || cfg.TrendGain > 12 {
		cfg.TrendGain = defaults.TrendGain
		changed = true
	}
	if cfg.MaxLearnOffset < 100 || cfg.MaxLearnOffset > 2000 {
		cfg.MaxLearnOffset = defaults.MaxLearnOffset
		changed = true
	}

	if len(cfg.LearnedOffsets) != len(curve) {
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsets)
		cfg.LearnedOffsets = next
		changed = true
	}
	if sanitized, updated := constrainOffsetsToCurveBounds(cfg.LearnedOffsets, curve, cfg.MaxLearnOffset); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}
	if sanitized, updated := constrainOffsetsToLearningBias(cfg.LearnedOffsets, cfg.LearningBias); updated {
		cfg.LearnedOffsets = sanitized
		changed = true
	}

	if len(cfg.LearnedOffsetsHeat) != len(curve) {
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsetsHeat)
		cfg.LearnedOffsetsHeat = next
		changed = true
	}
	if len(cfg.LearnedOffsetsCool) != len(curve) {
		next := make([]int, len(curve))
		copy(next, cfg.LearnedOffsetsCool)
		cfg.LearnedOffsetsCool = next
		changed = true
	}

	rateLen := rateBucketMax - rateBucketMin + 1
	if len(cfg.LearnedRateHeat) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateHeat)
		cfg.LearnedRateHeat = next
		changed = true
	}
	if len(cfg.LearnedRateCool) != rateLen {
		next := make([]int, rateLen)
		copy(next, cfg.LearnedRateCool)
		cfg.LearnedRateCool = next
		changed = true
	}

	if cfg.RampDownLimit > cfg.RampUpLimit+300 {
		cfg.RampDownLimit = cfg.RampUpLimit + 300
		changed = true
	}

	if sanitized, updated := sanitizeNoiseProfile(cfg.NoiseProfile); updated {
		cfg.NoiseProfile = sanitized
		changed = true
	}
	if len(cfg.NoiseProfile) == 0 && cfg.NoiseProfileUpdatedAt != 0 {
		cfg.NoiseProfileUpdatedAt = 0
		changed = true
	}

	return cfg, changed
}

const maxNoiseProfilePoints = 64

// sanitizeNoiseProfile 清洗噪音测试曲线：去除非法点、按转速升序排序、
// 同转速去重、限制点数，并把噪音值平移为以最小值为 0 的相对量。
func sanitizeNoiseProfile(profile []types.NoiseProfilePoint) ([]types.NoiseProfilePoint, bool) {
	if len(profile) == 0 {
		return profile, false
	}

	cleaned := make([]types.NoiseProfilePoint, 0, len(profile))
	for _, p := range profile {
		if p.RPM <= 0 || p.RPM > 20000 {
			continue
		}
		if p.DB != p.DB || p.DB < -200 || p.DB > 200 { // NaN 或离谱值
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) < 2 {
		// 单点曲线没有斜率信息，视为无效
		if len(profile) == len(cleaned) && len(cleaned) == 0 {
			return profile, false
		}
		return nil, true
	}

	sortNoiseProfile(cleaned)

	deduped := cleaned[:0]
	for _, p := range cleaned {
		if len(deduped) > 0 && deduped[len(deduped)-1].RPM == p.RPM {
			deduped[len(deduped)-1] = p
			continue
		}
		deduped = append(deduped, p)
	}
	if len(deduped) > maxNoiseProfilePoints {
		deduped = deduped[:maxNoiseProfilePoints]
	}

	minDB := deduped[0].DB
	for _, p := range deduped[1:] {
		if p.DB < minDB {
			minDB = p.DB
		}
	}
	for i := range deduped {
		deduped[i].DB -= minDB
	}

	changed := len(deduped) != len(profile)
	if !changed {
		for i := range deduped {
			if deduped[i] != profile[i] {
				changed = true
				break
			}
		}
	}
	return deduped, changed
}

func sortNoiseProfile(points []types.NoiseProfilePoint) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].RPM < points[j-1].RPM; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}

// BlendOffsets 保留旧接口所需的 Heat/Cool 融合行为。
func BlendOffsets(heatOffsets, coolOffsets []int) []int {
	if len(heatOffsets) == 0 && len(coolOffsets) == 0 {
		return nil
	}
	size := max(len(coolOffsets), len(heatOffsets))
	out := make([]int, size)
	for i := range size {
		h, c := 0, 0
		if i < len(heatOffsets) {
			h = heatOffsets[i]
		}
		if i < len(coolOffsets) {
			c = coolOffsets[i]
		}
		out[i] = (h + c) / 2
	}
	return out
}

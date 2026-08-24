package smartcontrol

import "github.com/TIANLI0/THRM/internal/types"

func getCurveEdgeRPMBounds(curve []types.FanCurvePoint) (int, int) {
	return GetCurveRPMBounds(curve)
}

// GetCurveRPMBounds 返回用户曲线的最小/最大 RPM 边界。
func GetCurveRPMBounds(curve []types.FanCurvePoint) (int, int) {
	if len(curve) == 0 {
		return 0, 4000
	}
	minRPM := curve[0].RPM
	maxRPM := curve[0].RPM
	for i := 1; i < len(curve); i++ {
		rpm := curve[i].RPM
		if rpm < minRPM {
			minRPM = rpm
		}
		if rpm > maxRPM {
			maxRPM = rpm
		}
	}
	return minRPM, maxRPM
}

func clampOffsetForPoint(offset, baseRPM, leftMinRPM, rightMaxRPM, maxLearnOffset int) int {
	minOffset := leftMinRPM - baseRPM
	maxOffset := rightMaxRPM - baseRPM
	minOffset = max(minOffset, -maxLearnOffset)
	maxOffset = min(maxOffset, maxLearnOffset)
	if minOffset > maxOffset {
		return 0
	}
	return clampInt(offset, minOffset, maxOffset)
}

func constrainOffsetsToCurveBounds(offsets []int, curve []types.FanCurvePoint, maxLearnOffset int) ([]int, bool) {
	if len(offsets) == 0 || len(curve) == 0 {
		return offsets, false
	}
	leftMinRPM, rightMaxRPM := getCurveEdgeRPMBounds(curve)
	updated := false
	normalized := make([]int, len(offsets))
	copy(normalized, offsets)
	for i := range normalized {
		if i >= len(curve) {
			normalized[i] = 0
			updated = true
			continue
		}
		clamped := clampOffsetForPoint(normalized[i], curve[i].RPM, leftMinRPM, rightMaxRPM, maxLearnOffset)
		if clamped != normalized[i] {
			normalized[i] = clamped
			updated = true
		}
	}
	return normalized, updated
}

func constrainOffsetsToLearningBias(offsets []int, learningBias string) ([]int, bool) {
	if len(offsets) == 0 {
		return offsets, false
	}

	bias := types.NormalizeLearningBias(learningBias)
	if bias == types.LearningBiasBalanced {
		return offsets, false
	}

	updated := false
	normalized := make([]int, len(offsets))
	copy(normalized, offsets)
	for i, offset := range normalized {
		switch bias {
		case types.LearningBiasCooling:
			if offset < 0 {
				normalized[i] = 0
				updated = true
			}
		case types.LearningBiasQuiet:
			if offset > 0 {
				normalized[i] = 0
				updated = true
			}
		}
	}
	return normalized, updated
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func medianOfThree(a, b, c int) int {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b, c = c, b
	}
	if a > b {
		a, b = b, a
	}
	return b
}

// FilterTransientSample 在进入移动平均前抑制最近稳定区间中的单点温度跳变。
func FilterTransientSample(currentTemp int, recentTemps []int, hysteresis int) (int, bool) {
	if len(recentTemps) < 3 {
		return currentTemp, false
	}

	last3 := recentTemps[len(recentTemps)-3:]
	baseline := medianOfThree(last3[0], last3[1], last3[2])
	minRecent := last3[0]
	maxRecent := last3[0]
	for _, temp := range last3[1:] {
		if temp < minRecent {
			minRecent = temp
		}
		if temp > maxRecent {
			maxRecent = temp
		}
	}

	stableBand := max(2, hysteresis+1)
	if maxRecent-minRecent > stableBand {
		return currentTemp, false
	}

	spikeBand := max(5, hysteresis+4)
	if absInt(currentTemp-baseline) >= spikeBand {
		return baseline, true
	}

	return currentTemp, false
}

// FilterTransientSpike 在控制环节抑制 1 个采样点的短时温度尖峰。
func FilterTransientSpike(currentTemp int, recentTemps []int, targetTemp, hysteresis int) (int, bool) {
	if len(recentTemps) < 3 {
		return currentTemp, false
	}

	// 高温区优先保守，避免误抑制真实过热。
	if currentTemp >= targetTemp+10 {
		return currentTemp, false
	}

	last3 := recentTemps[len(recentTemps)-3:]
	baseline := medianOfThree(last3[0], last3[1], last3[2])
	spikeBand := max(2, hysteresis+2)
	if currentTemp-baseline >= spikeBand {
		return baseline, true
	}

	return currentTemp, false
}

func enforceNonDecreasingRPM(curve []types.FanCurvePoint) {
	for i := 1; i < len(curve); i++ {
		if curve[i].RPM < curve[i-1].RPM {
			curve[i].RPM = curve[i-1].RPM
		}
	}
}

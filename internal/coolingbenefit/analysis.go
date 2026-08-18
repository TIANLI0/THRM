// Package coolingbenefit 量化散热器在不同转速下带来的实际收益。
//
// 它刻意不依赖 smartcontrol：学习模式回答"该吹多少转"，本包回答"吹到这个转速
// 值不值"。两者的可信度要求不同——学习可以带着噪声慢慢收敛，收益报告则是要给
// 用户看结论的，宁可标注"数据不可信"也不能给出一个看起来精确的假数字。
package coolingbenefit

import (
	"math"
	"sort"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	// 判定"有显著变化"的阈值。低于它的差异用传感器噪声就能解释，不该拿去讲故事。
	significantTempDropC  = 2.0
	significantPowerRiseW = 3.0

	// 采样窗口内波动超过这些值，说明该档位其实没热稳定。
	unsettledTempRangeC  = 3.0
	unsettledPowerRangeW = 15.0

	// 各档功耗中位数偏离超过该比例，说明用户的负载中途变了，横向对比不再成立。
	loadDriftRatio = 0.25
	// 全程功耗低于该值基本是待机，测不出任何区分度。
	minMeaningfulPowerW = 15.0

	// 实际转速低于目标这么多，视为风扇够不到该档位。
	rpmUnreachableGap = 250

	minUsefulSteps = 3

	// 拐点判定：某段的单位转速收益低于全程最佳段的该比例时，认为收益已经衰减。
	kneeDecayRatio = 0.35
)

// PowerBucketBounds 是日常统计的功耗分档上界 (W)，最后一档为无上界。
// 分档而不是连续回归，是因为被动数据太脏，拟合只会给出虚假的精度。
var PowerBucketBounds = []float64{25, 50, 80, 120}

// PowerBucketCount 是功耗档位总数。
func PowerBucketCount() int { return len(PowerBucketBounds) + 1 }

// PowerBucketOf 返回功耗所属档位下标。
func PowerBucketOf(watts float64) int {
	for i, bound := range PowerBucketBounds {
		if watts < bound {
			return i
		}
	}
	return len(PowerBucketBounds)
}

// AnalyzeReport 解读一次扫描测试。noise 为可选的实测噪音档案（没有就传 nil），
// 仅用于把"收益拐点"细化成"每分贝噪音换到最多收益"的甜点转速。
func AnalyzeReport(steps []types.CoolingBenefitStep, noise []types.NoiseProfilePoint) types.CoolingBenefitAnalysis {
	analysis := types.CoolingBenefitAnalysis{
		Regime:   types.CoolingRegimeInconclusive,
		Warnings: []string{},
	}

	ordered := make([]types.CoolingBenefitStep, len(steps))
	copy(ordered, steps)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].TargetRPM < ordered[j].TargetRPM })
	if len(ordered) < 2 {
		analysis.Warnings = append(analysis.Warnings, types.CoolingWarnFewSteps)
		return analysis
	}
	if len(ordered) < minUsefulSteps {
		analysis.Warnings = append(analysis.Warnings, types.CoolingWarnFewSteps)
	}

	base, top := ordered[0], ordered[len(ordered)-1]
	analysis.BaselineRPM = base.TargetRPM
	analysis.TopRPM = top.TargetRPM

	analysis.TempDelta = round1(controlTemp(top) - controlTemp(base))
	analysis.PowerDelta = round1(totalPower(top) - totalPower(base))
	analysis.LaptopFanDelta = laptopFan(top) - laptopFan(base)

	if span := float64(top.TargetRPM - base.TargetRPM); span > 0 {
		perKilo := 1000 / span
		analysis.TempPerKiloRPM = round1(analysis.TempDelta * perKilo)
		analysis.PowerPerKiloRPM = round1(analysis.PowerDelta * perKilo)
	}

	analysis.Regime = classifyRegime(analysis.TempDelta, analysis.PowerDelta)
	analysis.SensorDeltas = sensorDeltas(base, top)
	analysis.Warnings = append(analysis.Warnings, detectWarnings(ordered)...)

	analysis.KneeRPM = findKnee(ordered)
	analysis.SweetSpotRPM, analysis.SweetSpotHasNoise = findSweetSpot(ordered, noise)
	if analysis.SweetSpotRPM == 0 {
		analysis.SweetSpotRPM = analysis.KneeRPM
	}
	return analysis
}

// classifyRegime 判定这次测试里散热收益体现在哪一侧。
//
// 温度墙下温度被硬顶住不动，收益全部转化为功耗；功耗墙下功耗恒定，收益全部转化为
// 温度。两种形态对用户的意义完全不同（一个是"更快"，一个是"更凉"），必须分开讲。
func classifyRegime(tempDelta, powerDelta float64) string {
	cooler := tempDelta <= -significantTempDropC
	faster := powerDelta >= significantPowerRiseW
	switch {
	case cooler && faster:
		return types.CoolingRegimeMixed
	case faster:
		return types.CoolingRegimeThermal
	case cooler:
		return types.CoolingRegimePower
	default:
		return types.CoolingRegimeInconclusive
	}
}

// sensorDeltas 逐传感器算出基准档到最高档的变化，按降温幅度排序。
// 只保留两档都出现过的传感器：中途才冒出来的读数没有可比的基准。
func sensorDeltas(base, top types.CoolingBenefitStep) []types.CoolingSensorDelta {
	baseline := make(map[string]types.CoolingSensorReading, len(base.Sensors))
	for _, sensor := range base.Sensors {
		baseline[sensor.Key] = sensor
	}

	deltas := make([]types.CoolingSensorDelta, 0, len(top.Sensors))
	for _, sensor := range top.Sensors {
		from, ok := baseline[sensor.Key]
		if !ok || from.Value <= 0 || sensor.Value <= 0 {
			continue
		}
		deltas = append(deltas, types.CoolingSensorDelta{
			Key:      sensor.Key,
			Name:     sensor.Name,
			Group:    sensor.Group,
			Baseline: round1(from.Value),
			Best:     round1(sensor.Value),
			Delta:    round1(sensor.Value - from.Value),
		})
	}
	// 降幅最大的排在最前面，用户最想知道的就是"哪个部件最受益"。
	sort.SliceStable(deltas, func(i, j int) bool { return deltas[i].Delta < deltas[j].Delta })
	return deltas
}

// detectWarnings 找出会让这份报告不可信的因素。
//
// 这些检查比分析本身更重要：一次负载中途掉线的测试会画出一条漂亮但完全错误的
// 曲线，而用户没有任何办法自己看出来。
func detectWarnings(ordered []types.CoolingBenefitStep) []string {
	warnings := make([]string, 0, 4)

	powers := make([]float64, 0, len(ordered))
	for _, step := range ordered {
		powers = append(powers, totalPower(step))
	}
	medianPower := median(powers)

	// "太轻"看峰值而不是中位数：只要有任何一档跑出过真实负载，这次测试的问题就不是
	// 负载太轻，而是负载没稳住——中途把游戏关掉正是这种情况，它会把中位数拉到阈值
	// 以下，若用中位数判定就会把最需要提示的失败模式误报成"你没加负载"。
	peakPower := 0.0
	for _, power := range powers {
		peakPower = math.Max(peakPower, power)
	}
	if peakPower < minMeaningfulPowerW {
		warnings = append(warnings, types.CoolingWarnLoadTooLight)
	}

	// 漂移检查独立进行。温度墙下功耗本来就会随转速上升，所以只在偏离中位数过大时
	// 报警，而不是要求功耗恒定——那会把最有价值的一类结果误判成故障。
	// 分母取中位数与最低有效功耗的较大者，避免中位数很小时把正常波动放大成告警。
	driftBase := math.Max(medianPower, minMeaningfulPowerW)
	for _, power := range powers {
		if math.Abs(power-medianPower) > driftBase*loadDriftRatio {
			warnings = append(warnings, types.CoolingWarnLoadUnstable)
			break
		}
	}

	for _, step := range ordered {
		if step.TempRange > unsettledTempRangeC || step.PowerRange > unsettledPowerRangeW {
			warnings = append(warnings, types.CoolingWarnNotSettled)
			break
		}
	}

	for _, step := range ordered {
		if step.ActualRPM > 0 && step.TargetRPM-step.ActualRPM > rpmUnreachableGap {
			warnings = append(warnings, types.CoolingWarnRPMUnreachable)
			break
		}
	}

	return warnings
}

// stepBenefit 把一个档位相对基准的收益折算成一个标量。
// 降温和增功耗都是收益，用各自的显著性阈值归一化后相加，
// 这样温度墙和功耗墙两种形态都能用同一套拐点逻辑处理。
func stepBenefit(base, step types.CoolingBenefitStep) float64 {
	cooling := (controlTemp(base) - controlTemp(step)) / significantTempDropC
	power := (totalPower(step) - totalPower(base)) / significantPowerRiseW
	return math.Max(cooling, 0) + math.Max(power, 0)
}

// findKnee 找收益拐点：越过它继续提速，单位转速换到的改善明显变小。
// 返回最后一个"仍值得"的档位转速。
func findKnee(ordered []types.CoolingBenefitStep) int {
	if len(ordered) < 3 {
		return ordered[len(ordered)-1].TargetRPM
	}
	base := ordered[0]

	slopes := make([]float64, len(ordered)-1)
	best := 0.0
	for i := 0; i < len(ordered)-1; i++ {
		span := float64(ordered[i+1].TargetRPM - ordered[i].TargetRPM)
		if span <= 0 {
			continue
		}
		slopes[i] = (stepBenefit(base, ordered[i+1]) - stepBenefit(base, ordered[i])) / span
		best = math.Max(best, slopes[i])
	}
	if best <= 0 {
		// 全程没有可测量的收益，最低档就是拐点：再快也只是白吵。
		return ordered[0].TargetRPM
	}

	for i, slope := range slopes {
		if slope < best*kneeDecayRatio {
			return ordered[i].TargetRPM
		}
	}
	return ordered[len(ordered)-1].TargetRPM
}

// findSweetSpot 在有实测噪音档案时，找"每分贝噪音换到收益最多"的转速。
// 拐点只看收益衰减，甜点还要看这份收益的噪音代价——两台机器拐点相同，
// 噪音曲线不同，值得停的位置就不同。
func findSweetSpot(ordered []types.CoolingBenefitStep, noise []types.NoiseProfilePoint) (int, bool) {
	if len(noise) < 2 || len(ordered) < 2 {
		return 0, false
	}
	base := ordered[0]
	baseNoise := noiseAt(base.TargetRPM, noise)

	bestRPM, bestRatio := 0, 0.0
	for _, step := range ordered[1:] {
		cost := noiseAt(step.TargetRPM, noise) - baseNoise
		if cost < 0.5 {
			// 噪音几乎没涨，收益是白拿的，直接采纳这一档继续往上看。
			cost = 0.5
		}
		if ratio := stepBenefit(base, step) / cost; ratio > bestRatio {
			bestRatio, bestRPM = ratio, step.TargetRPM
		}
	}
	if bestRPM == 0 {
		return 0, false
	}
	return bestRPM, true
}

func noiseAt(rpm int, profile []types.NoiseProfilePoint) float64 {
	if len(profile) == 0 {
		return 0
	}
	if rpm <= profile[0].RPM {
		return profile[0].DB
	}
	last := len(profile) - 1
	if rpm >= profile[last].RPM {
		return profile[last].DB
	}
	for i := range last {
		if rpm < profile[i+1].RPM {
			span := float64(profile[i+1].RPM - profile[i].RPM)
			if span <= 0 {
				return profile[i].DB
			}
			t := float64(rpm-profile[i].RPM) / span
			return profile[i].DB + t*(profile[i+1].DB-profile[i].DB)
		}
	}
	return profile[last].DB
}

// controlTemp 用 CPU/GPU 中较高的一个代表整机热状态：
// 决定这台机器烫不烫、会不会降频的，永远是更热的那一侧。
func controlTemp(step types.CoolingBenefitStep) float64 {
	return math.Max(step.CPUTemp, step.GPUTemp)
}

func totalPower(step types.CoolingBenefitStep) float64 {
	return step.CPUPower + step.GPUPower
}

func laptopFan(step types.CoolingBenefitStep) int {
	return max(step.LaptopCPUFanRPM, step.LaptopGPUFanRPM)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

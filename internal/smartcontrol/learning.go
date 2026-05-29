package smartcontrol

import (
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// StableObserver + 稳态学习用到的常量。
const (
	// rpmPerDegree     = 50
	hardOffsetCap    = 600
	stableTempBand   = 2
	stableMinSamples = 6
	neighborShare    = 3

	// 冷却效率估计与转速寻优相关常量。
	effHistoryLen    = 6      // 每个温度桶保留的稳态 (转速,温度) 样本数
	minRPMSpanForEff = 80     // 估计冷却效率所需的最小转速跨度 (RPM)
	effFloorPerRPM   = 0.0008 // 冷却效率下限 (°C/RPM)，防止步长发散
	effCeilPerRPM    = 0.05   // 冷却效率上限 (°C/RPM)
	defaultEffPerRPM = 0.004  // 无历史时的保守冷却效率 (≈0.4°C/100RPM)
	maxLearnStep     = 300    // 单次学习的最大转速调整 (RPM)
	learnStepDeadRPM = 12     // 小于此调整量则忽略，避免抖动
	minSafetyStep    = 15     // 温度超目标时的最小降温步长 (RPM)
	defaultTargetTmp = 70     // TargetTemp 未配置时的回退目标温度 (°C)
)

// eqPoint 记录一次稳态 (转速, 温度) 平衡点。
type eqPoint struct {
	rpm  int
	temp int
}

// SteadyResult 是一次稳态观测的结果。
type SteadyResult struct {
	BucketIdx int     // 命中的曲线点索引；-1 表示无效
	MeanTemp  int     // 稳态平均温度 (°C)
	MeanRPM   int     // 稳态期间的平均下发转速 (RPM)
	LocalEff  float64 // 局部冷却效率 (°C/RPM)，正值
	HaveEff   bool    // 是否成功估计出冷却效率
	Ready     bool    // 是否达到稳态、可触发一次学习
}

// StableObserver 为每个曲线点累积稳态采样，并维护 (转速,温度) 平衡点历史。
type StableObserver struct {
	curveLen   int
	samples    [][]int     // 每个温度桶的温度采样
	rpmSamples [][]int     // 与 samples 平行的转速采样
	history    [][]eqPoint // 每个温度桶最近的稳态平衡点
}

// NewStableObserver 创建针对当前曲线长度的观察者。
func NewStableObserver(curveLen int) *StableObserver {
	if curveLen <= 0 {
		curveLen = 1
	}
	o := &StableObserver{curveLen: curveLen}
	o.allocBuffers(curveLen)
	return o
}

func (o *StableObserver) allocBuffers(curveLen int) {
	o.samples = make([][]int, curveLen)
	o.rpmSamples = make([][]int, curveLen)
	o.history = make([][]eqPoint, curveLen)
	for i := range o.samples {
		o.samples[i] = make([]int, 0, stableMinSamples*2)
		o.rpmSamples[i] = make([]int, 0, stableMinSamples*2)
		o.history[i] = make([]eqPoint, 0, effHistoryLen)
	}
}

// Resize 在曲线长度变化时调整内部缓冲。曲线变化会使历史失效，因此一并清空。
func (o *StableObserver) Resize(curveLen int) {
	if curveLen <= 0 {
		curveLen = 1
	}
	if o.curveLen == curveLen {
		o.Reset()
		return
	}
	o.curveLen = curveLen
	o.allocBuffers(curveLen)
}

// Reset 清空进行中的采样缓冲，但保留已学到的效率历史。
func (o *StableObserver) Reset() {
	for i := range o.samples {
		o.samples[i] = o.samples[i][:0]
		o.rpmSamples[i] = o.rpmSamples[i][:0]
	}
}

// CurveLen 返回当前观察者的曲线长度。
func (o *StableObserver) CurveLen() int {
	return o.curveLen
}

// pickBucketIndex 按最近邻选择温度所属的曲线点。
func pickBucketIndex(temp int, curve []types.FanCurvePoint) int {
	if len(curve) == 0 {
		return -1
	}
	if temp <= curve[0].Temperature {
		return 0
	}
	if temp >= curve[len(curve)-1].Temperature {
		return len(curve) - 1
	}
	for i := 0; i < len(curve)-1; i++ {
		if temp >= curve[i].Temperature && temp < curve[i+1].Temperature {
			midpoint := (curve[i].Temperature + curve[i+1].Temperature) / 2
			if temp < midpoint {
				return i
			}
			return i + 1
		}
	}
	return len(curve) - 1
}

// Observe 把一次 (温度, 下发转速) 采样放入对应温度桶。
// 达到稳态时返回平均温度、平均转速及局部冷却效率估计。
func (o *StableObserver) Observe(temp, effectiveRPM int, curve []types.FanCurvePoint) SteadyResult {
	idx := pickBucketIndex(temp, curve)
	if idx < 0 || idx >= len(o.samples) {
		return SteadyResult{BucketIdx: -1}
	}

	o.samples[idx] = append(o.samples[idx], temp)
	o.rpmSamples[idx] = append(o.rpmSamples[idx], effectiveRPM)
	if len(o.samples[idx]) > stableMinSamples*2 {
		o.samples[idx] = o.samples[idx][len(o.samples[idx])-stableMinSamples*2:]
		o.rpmSamples[idx] = o.rpmSamples[idx][len(o.rpmSamples[idx])-stableMinSamples*2:]
	}

	if len(o.samples[idx]) < stableMinSamples {
		return SteadyResult{BucketIdx: idx}
	}
	minT, maxT, sumT, sumR := o.samples[idx][0], o.samples[idx][0], 0, 0
	for i, t := range o.samples[idx] {
		if t < minT {
			minT = t
		}
		if t > maxT {
			maxT = t
		}
		sumT += t
		sumR += o.rpmSamples[idx][i]
	}
	if maxT-minT > stableTempBand {
		return SteadyResult{BucketIdx: idx}
	}

	meanT := sumT / len(o.samples[idx])
	meanR := sumR / len(o.rpmSamples[idx])
	o.samples[idx] = o.samples[idx][:0]
	o.rpmSamples[idx] = o.rpmSamples[idx][:0]

	o.recordEquilibrium(idx, meanR, meanT)
	eff, haveEff := o.localEfficiency(idx)

	return SteadyResult{
		BucketIdx: idx,
		MeanTemp:  meanT,
		MeanRPM:   meanR,
		LocalEff:  eff,
		HaveEff:   haveEff,
		Ready:     true,
	}
}

// recordEquilibrium 把一次稳态平衡点写入桶历史（环形保留最近 effHistoryLen 条）。
// 同一转速附近的旧样本会被新样本覆盖，使历史反映最新的热行为。
func (o *StableObserver) recordEquilibrium(idx, rpm, temp int) {
	if idx < 0 || idx >= len(o.history) {
		return
	}
	hist := o.history[idx]
	for i := range hist {
		if absInt(hist[i].rpm-rpm) < minRPMSpanForEff {
			hist[i] = eqPoint{rpm: rpm, temp: temp}
			o.history[idx] = hist
			return
		}
	}
	hist = append(hist, eqPoint{rpm: rpm, temp: temp})
	if len(hist) > effHistoryLen {
		hist = hist[len(hist)-effHistoryLen:]
	}
	o.history[idx] = hist
}

// localEfficiency 用历史中转速跨度最大的两点估计局部冷却效率 (°C/RPM, 正值)。
// 更高转速对应更低温度时效率为正；若数据不足或冷却无效则保守处理。
func (o *StableObserver) localEfficiency(idx int) (float64, bool) {
	if idx < 0 || idx >= len(o.history) {
		return 0, false
	}
	hist := o.history[idx]
	if len(hist) < 2 {
		return 0, false
	}
	lo, hi := hist[0], hist[0]
	for _, p := range hist[1:] {
		if p.rpm < lo.rpm {
			lo = p
		}
		if p.rpm > hi.rpm {
			hi = p
		}
	}
	span := hi.rpm - lo.rpm
	if span < minRPMSpanForEff {
		return 0, false
	}
	// 低转速点温度应更高；冷却有效时 (lo.temp - hi.temp) > 0。
	eff := float64(lo.temp-hi.temp) / float64(span)
	if eff < effFloorPerRPM {
		// 冷却几乎无效（甚至负相关）：视为最低效率，让寻优倾向于降转速省噪音。
		eff = effFloorPerRPM
	}
	if eff > effCeilPerRPM {
		eff = effCeilPerRPM
	}
	return eff, true
}

// alphaFromLearnRate 把 1..10 的 LearnRate 映射成反馈系数。
func alphaFromLearnRate(learnRate int) float64 {
	if learnRate < 1 {
		learnRate = 1
	}
	if learnRate > 10 {
		learnRate = 10
	}
	return 0.05 + float64(learnRate-1)*0.05
}

// effectiveOffsetCap 取 cfg.MaxLearnOffset 和 hardOffsetCap 的较小值。
func effectiveOffsetCap(cfg types.SmartControlConfig) int {
	cap := cfg.MaxLearnOffset
	if cap <= 0 || cap > hardOffsetCap {
		cap = hardOffsetCap
	}
	return cap
}

// targetTempCeiling 返回学习寻优使用的目标温度上限。
func targetTempCeiling(cfg types.SmartControlConfig) int {
	if cfg.TargetTemp > 0 {
		return cfg.TargetTemp
	}
	return defaultTargetTmp
}

// comfortBandWidth 返回目标温度下方的舒适带宽度 (°C)。
// 舒适带内不动作，避免无意义的转速抖动；带宽随滞回温差略微放宽。
func comfortBandWidth(cfg types.SmartControlConfig) int {
	band := cfg.Hysteresis + 3
	if band < 3 {
		band = 3
	}
	return band
}

// solveLearnStep 依据稳态温度、目标温度带与冷却效率，求出本次应施加的转速调整 (RPM)。
//
// 策略：
//   - 温度高于目标温度  → 加转速降温，步长 = α·(超出°C)/效率，确保把温度压回目标附近。
//   - 温度处于舒适带内  → 保持不动（这是消除“无脑降温”的关键：温度够低就不再加速）。
//   - 温度低于舒适带    → 主动降转速省噪音，可降幅 = α·(可上升°C)/效率；
//     冷却越低效（效率小），同样的降速带来的升温越小，于是越敢大幅降速。
//
// 冷却效率 eff (°C/RPM) 把“温度误差”换算成“转速需求”，使步长物理合理、收敛快且不易过冲。
func solveLearnStep(steadyTemp int, eff float64, haveEff bool, cfg types.SmartControlConfig) int {
	ceiling := targetTempCeiling(cfg)
	lowTarget := ceiling - comfortBandWidth(cfg)
	alpha := alphaFromLearnRate(cfg.LearnRate)

	if !haveEff || eff < effFloorPerRPM {
		eff = defaultEffPerRPM
	}
	if eff > effCeilPerRPM {
		eff = effCeilPerRPM
	}

	var step float64
	switch {
	case steadyTemp > ceiling:
		step = alpha * float64(steadyTemp-ceiling) / eff
		if step < minSafetyStep {
			step = minSafetyStep
		}
	case steadyTemp < lowTarget:
		step = -alpha * float64(lowTarget-steadyTemp) / eff
	default:
		return 0
	}

	if step > maxLearnStep {
		step = maxLearnStep
	}
	if step < -maxLearnStep {
		step = -maxLearnStep
	}

	delta := roundFloat(step)
	if steadyTemp <= ceiling && absInt(delta) < learnStepDeadRPM {
		return 0
	}
	return delta
}

// LearnSteadyOffset 根据一次稳态观测（温度 + 冷却效率）更新学习偏移。
func LearnSteadyOffset(
	bucketIdx int,
	steadyMeanTemp int,
	localEff float64,
	haveEff bool,
	curve []types.FanCurvePoint,
	prevOffsets []int,
	cfg types.SmartControlConfig,
) ([]int, bool) {
	if bucketIdx < 0 || bucketIdx >= len(curve) {
		return prevOffsets, false
	}

	offsets := make([]int, len(curve))
	for i := range offsets {
		if i < len(prevOffsets) {
			offsets[i] = prevOffsets[i]
		}
	}

	mainDelta := solveLearnStep(steadyMeanTemp, localEff, haveEff, cfg)
	if mainDelta == 0 {
		return offsets, false
	}

	neighborDelta := mainDelta / neighborShare
	cap := effectiveOffsetCap(cfg)
	leftMin, rightMax := GetCurveRPMBounds(curve)

	apply := func(idx, delta int) {
		if idx < 0 || idx >= len(offsets) || delta == 0 {
			return
		}
		offsets[idx] = clampOffsetForPoint(
			offsets[idx]+delta,
			curve[idx].RPM,
			leftMin,
			rightMax,
			cap,
		)
	}
	apply(bucketIdx, mainDelta)
	apply(bucketIdx-1, neighborDelta)
	apply(bucketIdx+1, neighborDelta)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
	}

	enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)
	if biased, updated := constrainOffsetsToLearningBias(offsets, cfg.LearningBias); updated {
		offsets = biased
		enforceMonotonicWithOffsets(curve, offsets, cap, leftMin, rightMax)
	}

	changed := false
	for i := range offsets {
		if i >= len(prevOffsets) || offsets[i] != prevOffsets[i] {
			changed = true
			break
		}
	}
	return offsets, changed
}

// roundFloat 四舍五入到最近整数
func roundFloat(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// enforceMonotonicWithOffsets 确保 (RPM_i + Δ_i) 沿 i 非递减；
// 如果某点违反，向上调整 Δ_i 直至单调（仍受 cap 与曲线 RPM 上限约束）。
func enforceMonotonicWithOffsets(curve []types.FanCurvePoint, offsets []int, cap, leftMin, rightMax int) {
	for i := 1; i < len(curve) && i < len(offsets); i++ {
		prevEffective := curve[i-1].RPM + offsets[i-1]
		currEffective := curve[i].RPM + offsets[i]
		if currEffective < prevEffective {
			needed := prevEffective - curve[i].RPM
			offsets[i] = clampOffsetForPoint(needed, curve[i].RPM, leftMin, rightMax, cap)
		}
	}
}

// ResetLearnedState 清空学习相关字段（保留可学习开关本身）。
// 旧字段也清空以保证存档一致。
func ResetLearnedState(cfg types.SmartControlConfig, curve []types.FanCurvePoint) types.SmartControlConfig {
	// rateBucketCount 来自 doc.go (rateBucketMax - rateBucketMin + 1)；
	// 这里仅为保持旧字段长度合法，不再被新算法读取。
	rateLen := rateBucketMax - rateBucketMin + 1
	cfg.LearnedOffsets = make([]int, len(curve))
	cfg.LearnedOffsetsHeat = make([]int, len(curve))
	cfg.LearnedOffsetsCool = make([]int, len(curve))
	cfg.LearnedRateHeat = make([]int, rateLen)
	cfg.LearnedRateCool = make([]int, rateLen)
	return cfg
}

package smartcontrol

import (
	"math"
	"sort"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
自适应学习 2.0 的在线热模型。

1.0 学的是"用户曲线上每个点该加减多少 RPM"，学到的东西离不开那条曲线：换曲线、
换方案，偏移就作废。2.0 学的是机器本身，与任何曲线无关：

	T_稳态 ≈ Baseline + Load × k(RPM)

  - Baseline 是空载基线温度（机器什么都不干时的落点）；
  - k(RPM) 是"单位负载造成多少温升"，随转速单调不增并饱和——这是散热器唯一
    能改变的量；
  - Load 在能读到 CPU/GPU 功耗时就是瓦数（k 的单位是 °C/W）；读不到功耗的机器
    退化成无量纲负载因子（k 的单位是"该转速下的典型温升"），两条路径的解算公式
    完全一致。

只读得到一半功耗（比如有 CPU 没 GPU）不会破坏模型：辨识与反解用的是同一个偏小的
瓦数口径，load = rise/k 里这个系统偏差会自行抵消。真正会造成一段偏差的是"功耗读数
的可得性中途变了"（换了显卡驱动之类），此时 EMA 需要几个样本才能跟过来。

k(RPM) 不做参数化拟合，而是按转速分桶做加权 EMA，再用保序回归（PAVA）强制单调。
理由是在线数据稀疏且有偏（用户常年只在两三个转速档上跑），非线性拟合很容易被
一小撮样本带跑；分桶 + 保序则最差退化成"没学到的地方保持原样"，不会发散。
*/

const (
	// adaptiveBucketWidth 是转速分桶宽度。太窄则每桶样本不足，太宽则抹平饱和特性。
	adaptiveBucketWidth = 200
	adaptiveMaxBuckets  = 28

	// adaptiveMinPowerW 以下的功耗读数信噪比太差（也可能是根本没读到），
	// 不参与每瓦温升估计，只计入退化模型。
	adaptiveMinPowerW = 8.0
	// adaptiveMinRiseC 以下的温升同样无法可靠地除以功耗。
	adaptiveMinRiseC = 2.0

	// adaptiveEMASpan 是 EMA 的等效样本数上限：桶内样本达到该数量后，
	// 新样本恒定占 1/(span+1) 的权重，使模型能持续跟随硬件老化/换环境。
	adaptiveEMASpan   = 8.0
	adaptiveMaxWeight = 64.0

	// 基线温度用非对称跟踪：见到更冷的落点就快速下修，否则极慢上浮。
	// 快下修保证换季/开空调能立刻反映；慢上浮避免一次异常低读数把基线永久钉死。
	adaptiveBaselineDown = 0.30
	adaptiveBaselineUp   = 0.004
	adaptiveBaselineMin  = 25.0
	adaptiveBaselineMax  = 72.0

	adaptiveRisePerWattMin = 0.02
	adaptiveRisePerWattMax = 3.0
	adaptiveRiseMax        = 60.0

	// 模型外推：表尾之外允许沿最后一段斜率再延伸这么多转速，
	// 给寻优一个"再快一点也许还有收益"的探索方向，但收益下限被 floor 夹住。
	adaptiveExtrapolateRPM   = 400
	adaptiveExtrapolateFloor = 0.55
)

// AdaptiveObservation 是一次稳态观测，热模型的唯一输入。
type AdaptiveObservation struct {
	RPM         int     // 稳态期间实际下发的转速
	ObservedRPM int     // 设备回报的实测转速，仅用于记录可达上限
	Temp        int     // 稳态平均温度 (°C)
	Power       float64 // 稳态平均 CPU+GPU 功耗 (W)，<=0 表示不可用
}

// NewAdaptiveThermalModel 返回一个空模型。基线先落在常见的空载温度附近，
// 第一批观测就会把它拉到这台机器真实的位置。
func NewAdaptiveThermalModel() types.AdaptiveThermalModel {
	return types.AdaptiveThermalModel{Baseline: 45}
}

// UpdateAdaptiveThermalModel 把一次稳态观测并入模型，返回更新后的副本。
func UpdateAdaptiveThermalModel(model types.AdaptiveThermalModel, obs AdaptiveObservation) types.AdaptiveThermalModel {
	if obs.RPM <= 0 || obs.Temp <= 0 {
		return model
	}
	if model.Baseline < adaptiveBaselineMin || model.Baseline > adaptiveBaselineMax {
		model.Baseline = clampFloat(float64(obs.Temp), adaptiveBaselineMin, adaptiveBaselineMax)
	}

	temp := float64(obs.Temp)
	if temp < model.Baseline {
		model.Baseline += adaptiveBaselineDown * (temp - model.Baseline)
	} else {
		model.Baseline += adaptiveBaselineUp * (temp - model.Baseline)
	}
	model.Baseline = clampFloat(model.Baseline, adaptiveBaselineMin, adaptiveBaselineMax)

	if obs.ObservedRPM > model.MaxObservedRPM {
		model.MaxObservedRPM = obs.ObservedRPM
	}

	rise := math.Max(temp-model.Baseline, 0)
	center := adaptiveBucketCenter(obs.RPM)

	buckets := make([]types.AdaptiveThermalBucket, len(model.Buckets))
	copy(buckets, model.Buckets)
	idx := adaptiveFindBucket(buckets, center)
	if idx < 0 {
		buckets = append(buckets, types.AdaptiveThermalBucket{RPM: center})
		sort.Slice(buckets, func(i, j int) bool { return buckets[i].RPM < buckets[j].RPM })
		idx = adaptiveFindBucket(buckets, center)
	}

	bucket := &buckets[idx]
	bucket.Rise = emaUpdate(bucket.Rise, clampFloat(rise, 0, adaptiveRiseMax), bucket.Weight)
	bucket.Weight = math.Min(bucket.Weight+1, adaptiveMaxWeight)

	if obs.Power >= adaptiveMinPowerW && rise >= adaptiveMinRiseC {
		perWatt := clampFloat(rise/obs.Power, adaptiveRisePerWattMin, adaptiveRisePerWattMax)
		bucket.RisePerWatt = emaUpdate(bucket.RisePerWatt, perWatt, bucket.PowerWeight)
		bucket.PowerWeight = math.Min(bucket.PowerWeight+1, adaptiveMaxWeight)
	}

	// 桶数超限时丢掉权重最低的那个：它要么是刚建立的过渡态，要么是很久没再
	// 命中的孤立档位，无论哪种都最不值得占住名额。
	if len(buckets) > adaptiveMaxBuckets {
		weakest := 0
		for i := range buckets {
			if buckets[i].Weight < buckets[weakest].Weight {
				weakest = i
			}
		}
		buckets = append(buckets[:weakest], buckets[weakest+1:]...)
	}

	model.Buckets = buckets
	model.Samples++
	model.UpdatedAt = time.Now().Unix()
	return model
}

func adaptiveBucketCenter(rpm int) int {
	idx := rpm / adaptiveBucketWidth
	return idx*adaptiveBucketWidth + adaptiveBucketWidth/2
}

func adaptiveFindBucket(buckets []types.AdaptiveThermalBucket, center int) int {
	for i := range buckets {
		if buckets[i].RPM == center {
			return i
		}
	}
	return -1
}

// emaUpdate 按已累计权重做变步长 EMA：样本少时几乎等于直接采纳（快速收敛），
// 样本多后步长收敛到 1/(span+1)（稳定但仍能跟随变化）。
func emaUpdate(prev, sample, weight float64) float64 {
	if weight <= 0 {
		return sample
	}
	w := math.Min(weight, adaptiveEMASpan)
	return (prev*w + sample) / (w + 1)
}

/* ── 模型查询 ── */

// adaptiveResponse 是把稀疏分桶整理成的单调查询表。
// perWatt 为真时 values 的单位是 °C/W，否则是"该转速下的典型温升 °C"。
type adaptiveResponse struct {
	rpms    []float64
	values  []float64
	perWatt bool
}

// adaptiveModelResponse 构建查询表：优先使用每瓦温升（物理意义明确、跨负载可迁移），
// 覆盖不足时退回相对温升。两种表都强制"转速越高、单位负载温升越低"的单调性。
func adaptiveModelResponse(model types.AdaptiveThermalModel) (adaptiveResponse, bool) {
	if table, ok := buildAdaptiveResponse(model.Buckets, true); ok {
		return table, true
	}
	return buildAdaptiveResponse(model.Buckets, false)
}

func buildAdaptiveResponse(buckets []types.AdaptiveThermalBucket, perWatt bool) (adaptiveResponse, bool) {
	rpms := make([]float64, 0, len(buckets))
	values := make([]float64, 0, len(buckets))
	weights := make([]float64, 0, len(buckets))
	for _, b := range buckets {
		weight, value := b.Weight, b.Rise
		if perWatt {
			weight, value = b.PowerWeight, b.RisePerWatt
		}
		if weight < 1 || value <= 0 {
			continue
		}
		rpms = append(rpms, float64(b.RPM))
		values = append(values, value)
		weights = append(weights, weight)
	}
	if len(rpms) < 2 || rpms[len(rpms)-1]-rpms[0] < float64(adaptiveBucketWidth)*2 {
		return adaptiveResponse{}, false
	}
	return adaptiveResponse{rpms: rpms, values: isotonicDecreasing(values, weights), perWatt: perWatt}, true
}

// isotonicDecreasing 用 PAVA (pool adjacent violators) 把序列投影成加权最小二乘
// 意义下最接近的非增序列。物理上散热只会越吹越好，观测里出现的反转必然是噪声
// 或工况漂移；直接丢弃反转点会损失信息，合并成区段均值则能保留其权重。
func isotonicDecreasing(values, weights []float64) []float64 {
	n := len(values)
	sums := make([]float64, 0, n)
	wsum := make([]float64, 0, n)
	count := make([]int, 0, n)

	for i := range n {
		s, w, c := values[i]*weights[i], weights[i], 1
		for len(sums) > 0 && sums[len(sums)-1]/wsum[len(wsum)-1] < s/w {
			s += sums[len(sums)-1]
			w += wsum[len(wsum)-1]
			c += count[len(count)-1]
			sums = sums[:len(sums)-1]
			wsum = wsum[:len(wsum)-1]
			count = count[:len(count)-1]
		}
		sums = append(sums, s)
		wsum = append(wsum, w)
		count = append(count, c)
	}

	out := make([]float64, 0, n)
	for i := range sums {
		mean := sums[i] / wsum[i]
		for range count[i] {
			out = append(out, mean)
		}
	}
	return out
}

// valueAt 返回给定转速下的单位负载温升。表内线性插值；
// 表尾之外沿最后一段斜率有限外推，让寻优保留探索更高转速的动机。
func (r adaptiveResponse) valueAt(rpm float64) float64 {
	n := len(r.rpms)
	if n == 0 {
		return 0
	}
	if rpm <= r.rpms[0] {
		return r.values[0]
	}
	if rpm >= r.rpms[n-1] {
		last := r.values[n-1]
		if n < 2 {
			return last
		}
		span := r.rpms[n-1] - r.rpms[n-2]
		if span <= 0 {
			return last
		}
		slope := (last - r.values[n-2]) / span
		extra := math.Min(rpm-r.rpms[n-1], adaptiveExtrapolateRPM)
		return math.Max(last+slope*extra, last*adaptiveExtrapolateFloor)
	}
	for i := 0; i < n-1; i++ {
		if rpm < r.rpms[i+1] {
			span := r.rpms[i+1] - r.rpms[i]
			if span <= 0 {
				return r.values[i]
			}
			t := (rpm - r.rpms[i]) / span
			return r.values[i] + t*(r.values[i+1]-r.values[i])
		}
	}
	return r.values[n-1]
}

// rpmForValue 是 valueAt 的反函数：求达到目标单位负载温升所需的最低转速。
// 目标比表内任何一点都宽松时返回 loRPM（不需要吹那么快），
// 比表尾还严苛时返回 hiRPM（吹到头也只能这样）。
func (r adaptiveResponse) rpmForValue(target, loRPM, hiRPM float64) float64 {
	n := len(r.rpms)
	if n == 0 || target <= 0 {
		return hiRPM
	}
	if target >= r.values[0] {
		return loRPM
	}
	for i := 0; i < n-1; i++ {
		hi, lo := r.values[i], r.values[i+1]
		if target <= hi && target >= lo {
			if hi-lo <= 0 {
				return r.rpms[i]
			}
			t := (hi - target) / (hi - lo)
			return r.rpms[i] + t*(r.rpms[i+1]-r.rpms[i])
		}
	}
	// 目标严于表尾：沿外推段找，找不到就交给硬件上限。
	last := r.values[n-1]
	if target >= last*adaptiveExtrapolateFloor {
		for rpm := r.rpms[n-1]; rpm <= r.rpms[n-1]+adaptiveExtrapolateRPM; rpm += 25 {
			if r.valueAt(rpm) <= target {
				return rpm
			}
		}
	}
	return hiRPM
}

// AdaptiveModelConfidence 返回 0..1 的模型可信度，决定合成曲线在多大程度上
// 采用模型解、多大程度上退回保守种子曲线。
//
// 三项取最小而不是取平均：样本够多但只覆盖一个转速档，或者覆盖够宽但每档只有
// 一两个点，都不足以支撑"放手让模型决定曲线"。任何一项不达标都必须保守。
//
// 门槛（4 个桶 / 600 RPM 跨度 / 30 个样本）刻意定得不高，因为覆盖不足的后果本身
// 是偏安全的：查询表在两端都取平端值，缺乏数据时反解只会得到更高的转速，而不是
// 更低。安全下限更是完全不看模型。真正需要靠置信度挡住的，只是模型样本太少时
// 解出的曲线形状可能古怪，而不是"会把机器烤了"。
//
// 另一方面，门槛也不能按"理想覆盖"来定：安静倾向的用户风扇常年只在一条窄带里
// 转，若要求宽跨度才肯信任模型，这类用户会永远停在种子曲线上，等于 2.0 对他们
// 从不生效。
func AdaptiveModelConfidence(model types.AdaptiveThermalModel) float64 {
	usable := 0
	minRPM, maxRPM := math.MaxInt32, 0
	for _, b := range model.Buckets {
		if b.Weight < 2 {
			continue
		}
		usable++
		minRPM = min(minRPM, b.RPM)
		maxRPM = max(maxRPM, b.RPM)
	}
	if usable < 2 {
		return 0
	}
	byCount := math.Min(float64(usable)/4, 1)
	bySpan := math.Min(float64(maxRPM-minRPM)/600, 1)
	bySamples := math.Min(float64(model.Samples)/30, 1)
	return math.Min(byCount, math.Min(bySpan, bySamples))
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

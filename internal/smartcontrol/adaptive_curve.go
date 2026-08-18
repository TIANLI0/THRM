package smartcontrol

import (
	"math"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
曲线合成。

给定热模型和倾向，解算出完整的温度—转速曲线。

# 按负载参数化，而不是按温度逐点求解

直觉上会想"对曲线上每个温度点求一次最优转速"，但那个提法是自指的：要评估某个
转速好不好，得知道当前负载；而负载只能由"温度 + 当时的转速"反推，那个转速正是
待求量。代入进去会形成正反馈——假设的转速越高，反推出的负载就越大，于是解出的
转速更高——同一个温度点可能收敛到好几个不同的解，曲线上就会冒出上千 RPM 的陡坎。

改成按负载参数化就没有这个问题：

	对每个负载 P：  rpm*(P) = argmin_rpm [ 噪音(rpm) + ThermalWeight × 热代价(T(P, rpm)) ]
	                T*(P)   = Baseline + P × k(rpm*(P))

每次优化都是良定义的（P 是自变量，不依赖待求量），得到的一串 (T*, rpm*) 就是这台
机器在这个倾向下的工作点轨迹——也就是风扇曲线本身。把它插值到温度栅格即可。
P 增大时 rpm* 与 T* 都单调上升，于是曲线天然单调且平滑。

# 代价函数里没有目标温度

	J(rpm) = 噪音(rpm) + ThermalWeight × exp((T_pred − 80) / scale)

热代价是一条处处光滑、处处递增的指数曲线：60°C 时多一度几乎不算代价，90°C 时多
一度贵得离谱。稳态落在哪里是权衡的结果，而不是被规定的数字；倾向只调
ThermalWeight，也就是"一度热折算多少分贝"。

用指数而不是分段惩罚，正是为了消掉拐点：任何"低于 X 不管、高于 X 开罚"的写法都会
让曲线在 X 处折一下，而 X 的位置纯属人为规定。同理，噪音项也必须处处有像样的梯度
（见 adaptiveNoiseDB），否则 argmin 会贴着边界不动然后突然跳走。

模型没学明白的时候（置信度低），结果按置信度与保守种子曲线混合。种子曲线单调
上扬，会在高温段主动给出更高转速，从而制造出模型缺失区间的观测——探索是靠这个
自然发生的，不需要专门的探索策略。
*/

const (
	adaptiveCurveSearchStep = 25

	// 负载采样：按几何级数取点，低负载段同样有分辨率。
	// perWatt 模型下单位是瓦；退化模型下是无量纲负载因子。
	adaptiveLoadSamples   = 56
	adaptiveLoadWattMin   = 4.0
	adaptiveLoadWattMax   = 320.0
	adaptiveLoadFactorMin = 0.08
	adaptiveLoadFactorMax = 4.0

	// 热代价 exp((T − anchor) / scale)。anchor 只是标尺原点，与 ThermalWeight
	// 共同决定绝对量级；scale 决定"热得多快就贵得多快"——7°C 一个 e 倍，意味着
	// 从 70°C 到 90°C 代价涨约 17 倍，足以让高温段压过任何省噪音的诱惑。
	adaptiveThermalAnchorC = 80.0
	adaptiveThermalScaleC  = 14.0
	// 预测温度先夹一夹再取指数：模型在数据稀疏处可能外推出离谱的值，
	// 让它进 exp 会瞬间放大成天文数字，把整条曲线钉在满速。
	adaptiveThermalMinC = 20.0
	adaptiveThermalMaxC = 130.0
	// 代价差小于该值（dB 当量）视为持平，不足以换更高的转速。
	adaptiveCostTieDB = 0.05

	// 曲线允许的最大陡度。5°C 一格的栅格上相当于每格 600 RPM，
	// 对应"整个转速区间铺开在 25°C 温区内"这样一条正常的风扇曲线。
	adaptiveMaxSlopeRPMPerC = 120

	// adaptiveCurveStepRPM 限制单次重新合成相对上一版曲线的变化量，
	// 让曲线随学习平滑演进而不是一跳一个样。
	adaptiveCurveStepRPM = 150

	adaptiveCurveRounding = 10
)

// adaptiveTempGrid 与默认曲线共用温度栅格，这样合成结果能直接喂给既有的
// 查表插值与前端图表，不需要任何特殊处理。
func adaptiveTempGrid() []int {
	base := types.GetDefaultFanCurve()
	grid := make([]int, len(base))
	for i, p := range base {
		grid[i] = p.Temperature
	}
	return grid
}

// adaptiveSeedCurve 是置信度不足时的回退曲线：默认曲线按倾向整体平移。
// 平移量存在的意义是让全新安装的安静倾向用户第一次就得到偏安静的曲线，
// 而不是先吵上半小时再等模型学明白。
func adaptiveSeedCurve(tuning AdaptiveTuning) []types.FanCurvePoint {
	base := types.GetDefaultFanCurve()
	p := float64(tuning.Preference) / 100
	shift := roundFloat(lerp(-450, 350, p))

	out := make([]types.FanCurvePoint, len(base))
	for i, point := range base {
		rpm := clampInt(point.RPM+shift, tuning.RPMFloor, tuning.RPMCeil)
		rpm = max(rpm, adaptiveSafetyFloor(point.Temperature, tuning))
		out[i] = types.FanCurvePoint{Temperature: point.Temperature, RPM: clampInt(rpm, adaptiveHWMinRPM, adaptiveHWMaxRPM)}
	}
	return out
}

// SynthesizeAdaptiveCurve 由热模型与倾向解算出完整风扇曲线。
// previous 为上一版曲线（可为 nil），用于限制单次变化幅度。
func SynthesizeAdaptiveCurve(
	model types.AdaptiveThermalModel,
	tuning AdaptiveTuning,
	noiseProfile []types.NoiseProfilePoint,
	previous []types.FanCurvePoint,
) []types.FanCurvePoint {
	grid := adaptiveTempGrid()
	seed := adaptiveSeedCurve(tuning)
	response, haveModel := adaptiveModelResponse(model)
	confidence := AdaptiveModelConfidence(model)
	if !haveModel {
		confidence = 0
	}

	var operating []adaptiveOperatingPoint
	if confidence > 0 {
		operating = adaptiveOperatingPoints(model.Baseline, response, tuning, noiseProfile)
	}

	out := make([]types.FanCurvePoint, len(grid))
	for i, temp := range grid {
		rpm := seed[i].RPM
		if confidence > 0 && len(operating) > 0 {
			solved := adaptiveRPMAtTemp(operating, temp, tuning)
			rpm = roundFloat(lerp(float64(seed[i].RPM), float64(solved), confidence))
		}
		out[i] = types.FanCurvePoint{Temperature: temp, RPM: clampInt(rpm, adaptiveHWMinRPM, adaptiveHWMaxRPM)}
	}

	// 先给代价驱动的部分限陡，再叠安全网：这样曲线上唯一可能陡的地方只剩下逼近
	// 红线时的安全爬升，而那一段的陡峭是有意为之。
	limitAdaptiveCurveSlope(out)
	for i := range out {
		out[i].RPM = clampInt(max(out[i].RPM, adaptiveSafetyFloor(out[i].Temperature, tuning)), adaptiveHWMinRPM, adaptiveHWMaxRPM)
	}

	limitAdaptiveCurveStep(out, previous)
	enforceNonDecreasingRPM(out)
	for i := range out {
		out[i].RPM = clampInt(roundToStep(out[i].RPM, adaptiveCurveRounding), adaptiveHWMinRPM, adaptiveHWMaxRPM)
	}
	enforceNonDecreasingRPM(out)
	return out
}

// adaptiveOperatingPoint 是某个负载下解出的最优工作点。
type adaptiveOperatingPoint struct {
	temp float64
	rpm  float64
}

// adaptiveOperatingPoints 扫过一串负载，解出这台机器在当前倾向下的工作点轨迹。
// 结果按温度升序且转速非递减——插值成曲线后天然单调。
func adaptiveOperatingPoints(
	baseline float64,
	response adaptiveResponse,
	tuning AdaptiveTuning,
	noiseProfile []types.NoiseProfilePoint,
) []adaptiveOperatingPoint {
	loMin, loMax := adaptiveLoadFactorMin, adaptiveLoadFactorMax
	if response.perWatt {
		loMin, loMax = adaptiveLoadWattMin, adaptiveLoadWattMax
	}

	out := make([]adaptiveOperatingPoint, 0, adaptiveLoadSamples)
	runningRPM := 0
	for i := range adaptiveLoadSamples {
		t := float64(i) / float64(adaptiveLoadSamples-1)
		load := loMin * math.Pow(loMax/loMin, t)

		// 负载只增不减，转速也不该回退。取运行最大值而不是丢弃回退点：
		// 丢点会在轨迹上留下空档，插值时那段空档就变成曲线上的陡坎。
		rpm := max(minimizeAdaptiveCost(load, baseline, response, tuning, noiseProfile), runningRPM)
		runningRPM = rpm
		temp := baseline + load*response.valueAt(float64(rpm))

		if len(out) > 0 && temp <= out[len(out)-1].temp {
			// 提速把温度压回去了：同一温度下保留更高的转速，不新增横坐标。
			out[len(out)-1].rpm = float64(rpm)
			continue
		}
		out = append(out, adaptiveOperatingPoint{temp: temp, rpm: float64(rpm)})
	}
	return out
}

// adaptiveRPMAtTemp 在工作点轨迹上插值出某个温度对应的转速。
// 低于最凉的工作点说明负载比采样下限还轻——最低转速足矣；
// 高于最热的工作点说明吹到头也就这样了。
func adaptiveRPMAtTemp(points []adaptiveOperatingPoint, temp int, tuning AdaptiveTuning) int {
	t := float64(temp)
	if len(points) == 0 {
		return tuning.RPMFloor
	}
	if t <= points[0].temp {
		return tuning.RPMFloor
	}
	last := len(points) - 1
	if t >= points[last].temp {
		return tuning.RPMCeil
	}
	for i := 0; i < last; i++ {
		if t < points[i+1].temp {
			span := points[i+1].temp - points[i].temp
			if span <= 0 {
				return roundFloat(points[i].rpm)
			}
			ratio := (t - points[i].temp) / span
			return roundFloat(points[i].rpm + ratio*(points[i+1].rpm-points[i].rpm))
		}
	}
	return roundFloat(points[last].rpm)
}

// minimizeAdaptiveCost 在允许的转速区间里全程扫描取代价最小点。
// 全程扫描而不是在某个解附近搜：代价函数没有阈值也没有解析解，
// 而这条路径每 90 秒才跑一次，几千次求值的开销可以忽略。
func minimizeAdaptiveCost(
	load, baseline float64,
	response adaptiveResponse,
	tuning AdaptiveTuning,
	noiseProfile []types.NoiseProfilePoint,
) int {
	// 升序扫描 + 显著性阈值：代价景观常有一整段近乎持平（噪音的对数增长恰好抵消
	// 热代价的指数下降），此时 argmin 会被浮点级差异左右，相邻负载解出的转速能差
	// 上千转，曲线随之出现陡坎。要求"明显更优"才换，等价于在持平区里稳定地取最低
	// 转速——既消除抖动，也符合"收益差不多就选安静的那个"。
	bestRPM, bestCost := tuning.RPMFloor, math.Inf(1)
	for cand := tuning.RPMFloor; cand <= tuning.RPMCeil; cand += adaptiveCurveSearchStep {
		predicted := baseline + load*response.valueAt(float64(cand))
		if cost := adaptiveCost(predicted, cand, tuning, noiseProfile); cost < bestCost-adaptiveCostTieDB {
			bestCost, bestRPM = cost, cand
		}
	}
	return bestRPM
}

// adaptiveCost 是转速取舍的代价函数，单位统一折算成 dB 当量。
//
// 处处光滑、对温度处处递增、没有任何阈值——这是 2.0 与目标温度式控温的根本差别。
// 温度每上升 adaptiveThermalScaleC 度，多一度的代价就翻 e 倍，于是低温区几乎只看
// 噪音、高温区几乎只看温度，中间是连续过渡而不是一个突变的拐点。
func adaptiveCost(predictedTemp float64, rpm int, tuning AdaptiveTuning, noiseProfile []types.NoiseProfilePoint) float64 {
	clamped := clampFloat(predictedTemp, adaptiveThermalMinC, adaptiveThermalMaxC)
	thermal := tuning.ThermalWeight * math.Exp((clamped-adaptiveThermalAnchorC)/adaptiveThermalScaleC)
	return adaptiveNoiseDB(rpm, noiseProfile) + thermal
}

// limitAdaptiveCurveSlope 限制曲线的陡度（RPM/°C）。
//
// 强散热倾向下，最优工作点轨迹会把整个转速区间压进两三度温差里——机器"无论如何
// 都要稳在 61°C"。那样的曲线不只是看着像个拐点，控制上也站不住：温度动一度，
// 转速就冲一千转，接着温度被压回去、转速又掉下来，来回摆动。
//
// 反向传播地抬高低温侧而不是压低高温侧：高温侧的转速关系到散热安全，只能保留；
// 把提速的时机提前，才是既平缓又安全的做法。
func limitAdaptiveCurveSlope(curve []types.FanCurvePoint) {
	for i := len(curve) - 1; i > 0; i-- {
		span := curve[i].Temperature - curve[i-1].Temperature
		if span <= 0 {
			continue
		}
		if floor := curve[i].RPM - adaptiveMaxSlopeRPMPerC*span; curve[i-1].RPM < floor {
			curve[i-1].RPM = floor
		}
	}
}

// limitAdaptiveCurveStep 限制相对上一版曲线的单点变化幅度。
// 温度栅格固定，因此按下标一一对应即可。
func limitAdaptiveCurveStep(curve []types.FanCurvePoint, previous []types.FanCurvePoint) {
	if len(previous) != len(curve) {
		return
	}
	for i := range curve {
		if previous[i].Temperature != curve[i].Temperature || previous[i].RPM <= 0 {
			continue
		}
		delta := curve[i].RPM - previous[i].RPM
		if delta > adaptiveCurveStepRPM {
			curve[i].RPM = previous[i].RPM + adaptiveCurveStepRPM
		} else if delta < -adaptiveCurveStepRPM {
			curve[i].RPM = previous[i].RPM - adaptiveCurveStepRPM
		}
	}
}

func roundToStep(value, step int) int {
	if step <= 1 {
		return value
	}
	return (value + step/2) / step * step
}

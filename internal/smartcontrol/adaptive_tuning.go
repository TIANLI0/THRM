package smartcontrol

import (
	"math"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
倾向派生。

2.0 对用户只暴露一个 0..100 的滑块（0 = 安静优先，100 = 低温优先）。所有过去
需要手动调的量——目标温度、噪音权重、升降速限幅、滞回、前馈增益、最低生效变化、
可用转速区间——都从这一个数派生。

这么做的前提是这些参数本来就不独立：想要低温，就必须接受更高转速、更快响应、
更窄滞回；想要安静，就得容忍更高的稳态温度和更迟钝的响应。让用户逐个去调，
既是负担，也几乎必然调出互相矛盾的组合（比如目标 60°C 配 120 RPM 的升速限幅，
风扇永远追不上）。派生保证了任意滑块位置上，这组参数都是自洽的。
*/

const (
	// 自动模式的硬件转速包络。设备协议上限是 4000 RPM；下限取 1000 而不是设备
	// 能接受的更低值，是因为自动模式不该主动把用户带进"非官方最低转速"的风险区。
	adaptiveHWMinRPM = 1000
	adaptiveHWMaxRPM = 4000

	// 热代价权重的两端。数值本身没有物理含义，只是"一度热折算多少分贝"的标尺；
	// 区间宽度决定了滑块两端的稳态温度大致能拉开多远（在参考机型上约 80°C ↔ 60°C）。
	adaptiveThermalWeightQuiet = 5.0
	adaptiveThermalWeightCool  = 25.0

	// 安全网提前多少度开始爬升。太窄会在介入点附近形成陡坎，太宽会让安全网
	// 长期压着代价函数，把倾向变成摆设。
	adaptiveSafetyRampSpanC = 20

	// 风扇定律系数：声功率 ∝ 转速^5 → 分贝 = 50·log10(转速比)。
	adaptiveNoiseFanLawDB = 50.0
)

// AdaptiveTuning 是由倾向派生出的一整套控温参数。
//
// 这里刻意没有"目标温度"。目标温度会在曲线上制造一个拐点：低于它只看噪音、
// 高于它突然开始惩罚，同一台机器在目标温度两侧的行为判若两机，而那个拐点的
// 位置纯属人为规定。2.0 改用连续的热代价（见 adaptiveCost），温度每高一点
// 代价就重一点，没有任何阈值——稳态落在哪里是权衡的结果，不是被规定的。
type AdaptiveTuning struct {
	Preference  int // 归一化后的倾向值
	CeilingTemp int // 安全介入温度：越过它开始无视噪音强制拉速（安全网，不是控温目标）
	LimitTemp   int // 安全红线：达到即全速

	// ThermalWeight 是热代价相对噪音代价的权重，倾向的核心体现。
	// 它不规定温度落点，只规定"一度热值多少分贝"。
	ThermalWeight float64

	RPMFloor int // 允许的最低转速
	RPMCeil  int // 允许的最高转速

	RampUp       int // 单周期最大升速
	RampDown     int // 单周期最大降速
	Hysteresis   int // 滞回温差
	TrendGain    int // 前馈增益
	MinRPMChange int // 最小生效转速变化
}

// NormalizeAdaptivePreference 把倾向夹到合法区间。
func NormalizeAdaptivePreference(preference int) int {
	return clampInt(preference, types.AdaptivePreferenceMin, types.AdaptivePreferenceMax)
}

// NormalizeAdaptiveTempLimit 把安全红线夹到合法区间，非法值回落默认。
func NormalizeAdaptiveTempLimit(limit int) int {
	if limit < types.AdaptiveTempLimitMin || limit > types.AdaptiveTempLimitMax {
		return types.DefaultAdaptiveTempLimit
	}
	return limit
}

// DeriveAdaptiveTuning 由倾向解出完整参数组。
func DeriveAdaptiveTuning(cfg types.AdaptiveConfig) AdaptiveTuning {
	preference := NormalizeAdaptivePreference(cfg.Preference)
	limit := NormalizeAdaptiveTempLimit(cfg.TempLimit)
	p := float64(preference) / 100

	// 安全介入温度直接由红线倒推，不再挂靠任何"目标"：安静倾向下让安全网尽量
	// 晚介入（把决定权留给代价函数），低温倾向下提前介入。
	ceiling := limit - roundFloat(lerp(3, 14, p))
	ceiling = clampInt(ceiling, 55, limit-2)

	span := float64(adaptiveHWMaxRPM - adaptiveHWMinRPM)
	tuning := AdaptiveTuning{
		Preference:  preference,
		CeilingTemp: ceiling,
		LimitTemp:   limit,

		// 按对数插值：热代价本身是指数的，线性扫权重会让滑块前半段几乎没反应、
		// 后半段突变。对数插值使滑块每移动同样距离，稳态温度平移的度数大致相同。
		ThermalWeight: math.Pow(10, lerp(math.Log10(adaptiveThermalWeightQuiet), math.Log10(adaptiveThermalWeightCool), p)),

		RPMFloor: roundFloat(float64(adaptiveHWMinRPM) + lerp(0, 0.18, p)*span),
		RPMCeil:  roundFloat(float64(adaptiveHWMinRPM) + lerp(0.60, 1.0, p)*span),

		RampUp:       roundFloat(lerp(120, 420, p)),
		RampDown:     roundFloat(lerp(80, 280, p)),
		Hysteresis:   roundFloat(lerp(4, 1, p)),
		TrendGain:    roundFloat(lerp(3, 10, p)),
		MinRPMChange: roundFloat(lerp(90, 40, p)),
	}

	// 安全红线永远压过倾向：即便倾向把可用上限压到 60%，红线温度下仍必须能全速。
	// 这里不改 RPMCeil（那是"日常允许用到多少"），而由 adaptiveSafetyFloor 在
	// 高温段直接抬到硬件上限，两者分工明确。
	tuning.RPMCeil = clampInt(tuning.RPMCeil, tuning.RPMFloor+200, adaptiveHWMaxRPM)
	return tuning
}

// adaptiveSafetyFloor 返回与倾向无关的最低转速要求。
//
// 早先的写法在安全介入温度处是个阶跃：低于它完全不约束，到了它一步跳到七成转速。
// 那正是用户会感觉到的"某个温度突然拐点"——风扇在一度之内从安静变成呼啸。现在改成
// 从 adaptiveSafetyRampSpanC 度之前就开始爬，用 t^2.5 的幂曲线：入口处斜率为零
// （接得上代价函数给的转速，没有折角），越接近红线越陡，到红线正好满速。
//
// 幂次选 2.5 而不是线性或 smoothstep，是为了让它在中段尽量不抢戏：安全网的职责是
// 兜底，不是替代权衡。只有真正逼近红线时它才该压过用户的安静倾向。
func adaptiveSafetyFloor(temp int, tuning AdaptiveTuning) int {
	if temp >= tuning.LimitTemp {
		return adaptiveHWMaxRPM
	}
	start := tuning.CeilingTemp - adaptiveSafetyRampSpanC
	if temp <= start {
		return 0
	}
	span := float64(tuning.LimitTemp - start)
	if span <= 0 {
		return adaptiveHWMaxRPM
	}
	t := float64(temp-start) / span
	eased := math.Pow(t, 2.5)
	return roundFloat(lerp(float64(tuning.RPMFloor), float64(adaptiveHWMaxRPM), eased))
}

func lerp(from, to, t float64) float64 {
	return from + (to-from)*clampFloat(t, 0, 1)
}

// adaptiveNoiseDB 估计某转速的相对噪音水平 (dB)。
// 有麦克风实测档案就用实测值插值；没有则按风扇定律兜底。
//
// 兜底模型必须是对数的：轴流风扇的声功率大致正比于转速的五次方，折成分贝就是
// 50·log10(rpm)，其导数正比于 1/rpm——也就是说边际噪音代价在低转速处最高，
// 从 1000 加到 1200 转（+4 dB）比从 3000 加到 3200 转（+1.4 dB）吵得多。
//
// 这一点关系重大，不只是精度问题。早先的幂律写法 22·t^1.7 在下限处导数为零，
// 等于告诉寻优"刚离开最低转速时提速不要钱"，于是最优解会一直贴着下限不动，
// 直到热代价突然压过来才一步跳上去——曲线上就出现一个上千 RPM 的陡坎。
// 代价函数光滑并不能保证 argmin 光滑；某一项梯度趋零就会产生这种跳变。
func adaptiveNoiseDB(rpm int, profile []types.NoiseProfilePoint) float64 {
	if len(profile) >= noiseProfileMinPoints {
		span := profile[len(profile)-1].RPM - profile[0].RPM
		rise := profile[len(profile)-1].DB - profile[0].DB
		if span >= noiseProfileMinSpanRPM && rise >= noiseProfileMinRiseDB {
			return interpolateNoiseProfile(rpm, profile)
		}
	}
	if rpm <= adaptiveHWMinRPM {
		return 0
	}
	return adaptiveNoiseFanLawDB * math.Log10(float64(rpm)/float64(adaptiveHWMinRPM))
}

func interpolateNoiseProfile(rpm int, profile []types.NoiseProfilePoint) float64 {
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

// ApplyAdaptiveTuning 把派生参数覆盖到一份仅在本个控温周期内使用的配置副本上。
//
// 覆盖而不是写回配置，是为了让"用户看到的设置"与"算法实际用的参数"分开：用户在
// 1.0 里调过的目标温度、滞回、限幅原封不动地留着，关掉 2.0 就立刻恢复，不会出现
// 自动模式悄悄改掉手动设置的情况。
//
// Learning 被置为 false 是关键一步：2.0 的曲线已经是完整答案，再叠加 1.0 学到的
// 逐点偏移等于把两套互不知情的控制器串起来，结果既不可预测也无法解释。
func ApplyAdaptiveTuning(cfg types.SmartControlConfig, tuning AdaptiveTuning) types.SmartControlConfig {
	cfg.Learning = false
	cfg.FilterTransientSpike = true
	// 2.0 没有目标温度。TargetTemp 在控温环里只剩一个用途——告诉尖峰过滤器
	// "到多热就别再抑制了"，安全介入温度正是这个语义。
	cfg.TargetTemp = clampInt(tuning.CeilingTemp, 45, 90)
	cfg.Hysteresis = tuning.Hysteresis
	cfg.RampUpLimit = tuning.RampUp
	cfg.RampDownLimit = tuning.RampDown
	cfg.TrendGain = tuning.TrendGain
	cfg.MinRPMChange = tuning.MinRPMChange
	cfg.LearnedOffsets = nil
	return cfg
}

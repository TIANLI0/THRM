package smartcontrol

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
自适应学习 2.0 的运行期引擎：把稳态检测、模型更新和曲线合成串成一条线，
供控温环每个采样周期调用。

引擎本身不持久化任何东西——模型和曲线都存在配置里，由调用方决定何时落盘。
这样休眠恢复、桥接重启、设备重连都可以直接 Reset 掉进行中的稳态窗口，
而不会丢掉已经学到的模型。
*/

const (
	// adaptiveSteadyWindow 是判定稳态所需的连续采样点数。
	adaptiveSteadyWindow = 6
	// adaptiveSteadyTempBand / adaptiveSteadyRPMBand 是窗口内允许的波动幅度。
	// 转速带取得很小：自动模式下转速由曲线查表得到，稳态时本就该几乎不动，
	// 一旦在变就说明还在追温度，此时的 (转速,温度) 配对不构成平衡点。
	adaptiveSteadyTempBand = 2
	adaptiveSteadyRPMBand  = 60

	// adaptiveResynthInterval 是两次曲线重算之间的最小间隔。
	// 曲线重算不贵，但让它慢一点能避免风扇行为在用户眼皮底下频繁变化。
	adaptiveResynthInterval = 90 * time.Second
	// adaptiveResynthSamples 是触发重算所需的新增样本数：模型没长进就不必重算。
	adaptiveResynthSamples = 3
)

// AdaptiveActive 判断自适应学习 2.0 当前是否接管曲线。
func AdaptiveActive(cfg types.SmartControlConfig) bool {
	return cfg.Adaptive.Enabled
}

type adaptiveWindowSample struct {
	temp     int
	rpm      int
	observed int
	power    float64
}

// AdaptiveEngine 持有一次监控会话内的稳态窗口与最近合成的曲线。
type AdaptiveEngine struct {
	window []adaptiveWindowSample

	curve       []types.FanCurvePoint
	signature   string
	lastSynth   time.Time
	lastSamples int
}

func NewAdaptiveEngine() *AdaptiveEngine {
	return &AdaptiveEngine{window: make([]adaptiveWindowSample, 0, adaptiveSteadyWindow)}
}

// Reset 丢弃进行中的稳态窗口。设备重连、休眠恢复、模式切换后必须调用：
// 跨越这些事件的采样点属于不同工况，凑在一起会伪造出不存在的平衡点。
func (e *AdaptiveEngine) Reset() {
	if e == nil {
		return
	}
	e.window = e.window[:0]
}

// Observe 累积一次采样，达到稳态时返回可用于更新模型的观测。
func (e *AdaptiveEngine) Observe(temp, commandedRPM, observedRPM int, power float64) (AdaptiveObservation, bool) {
	if e == nil || temp <= 0 || commandedRPM <= 0 {
		return AdaptiveObservation{}, false
	}

	sample := adaptiveWindowSample{temp: temp, rpm: commandedRPM, observed: observedRPM, power: power}
	if len(e.window) > 0 {
		prev := e.window[len(e.window)-1]
		if absInt(temp-prev.temp) > adaptiveSteadyTempBand+1 || absInt(commandedRPM-prev.rpm) > adaptiveSteadyRPMBand {
			e.window = e.window[:0]
		}
	}
	e.window = append(e.window, sample)
	if len(e.window) > adaptiveSteadyWindow {
		e.window = e.window[len(e.window)-adaptiveSteadyWindow:]
	}
	if len(e.window) < adaptiveSteadyWindow {
		return AdaptiveObservation{}, false
	}

	minTemp, maxTemp := e.window[0].temp, e.window[0].temp
	minRPM, maxRPM := e.window[0].rpm, e.window[0].rpm
	sumTemp, sumRPM, sumObserved := 0, 0, 0
	sumPower, powerCount := 0.0, 0
	for _, s := range e.window {
		minTemp, maxTemp = min(minTemp, s.temp), max(maxTemp, s.temp)
		minRPM, maxRPM = min(minRPM, s.rpm), max(maxRPM, s.rpm)
		sumTemp += s.temp
		sumRPM += s.rpm
		sumObserved += s.observed
		if s.power > 0 {
			sumPower += s.power
			powerCount++
		}
	}
	if maxTemp-minTemp > adaptiveSteadyTempBand || maxRPM-minRPM > adaptiveSteadyRPMBand {
		return AdaptiveObservation{}, false
	}

	count := len(e.window)
	obs := AdaptiveObservation{
		RPM:         sumRPM / count,
		ObservedRPM: sumObserved / count,
		Temp:        sumTemp / count,
	}
	// 功耗读数缺失的采样点不参与平均，否则一个 0 会把整段拉低，
	// 让模型以为同样的温升只需要更少的瓦数。
	if powerCount == count {
		obs.Power = sumPower / float64(powerCount)
	}

	e.window = e.window[:0]
	return obs, true
}

// Curve 返回当前应当生效的曲线，必要时重新合成。
// 第二个返回值指出这次调用是否产生了新曲线，调用方据此决定是否落盘/广播。
func (e *AdaptiveEngine) Curve(cfg types.SmartControlConfig, now time.Time) ([]types.FanCurvePoint, bool) {
	tuning := DeriveAdaptiveTuning(cfg.Adaptive)
	signature := adaptiveSignature(cfg, tuning)
	// 样本数倒退只可能来自"重置模型"，此时缓存的曲线已经无从谈起，必须重算。
	modelReset := cfg.Adaptive.Model.Samples < e.lastSamples
	userChanged := signature != e.signature

	if len(e.curve) > 0 && !userChanged && !modelReset {
		grown := cfg.Adaptive.Model.Samples-e.lastSamples >= adaptiveResynthSamples
		if !grown || now.Sub(e.lastSynth) < adaptiveResynthInterval {
			return e.curve, false
		}
	}

	// 限幅只用来约束"模型自己慢慢学出来的漂移"。用户刚拖完倾向滑块却要等好几轮
	// 才看到曲线到位，会被当成没生效，所以显式改动一步到位。
	var previous []types.FanCurvePoint
	if !userChanged {
		previous = e.curve
		if len(previous) == 0 {
			// 会话刚开始：从配置里恢复上次的曲线，避免每次开机都从种子曲线重新爬。
			previous = cfg.Adaptive.AutoCurve
		}
	}
	next := SynthesizeAdaptiveCurve(cfg.Adaptive.Model, tuning, cfg.NoiseProfile, previous)

	changed := !sameCurve(next, e.curve)
	e.curve = next
	e.signature = signature
	e.lastSynth = now
	e.lastSamples = cfg.Adaptive.Model.Samples
	return e.curve, changed
}

// adaptiveSignature 指纹化所有会改变曲线形状的输入。指纹变化即强制重算，
// 这样用户拖动倾向滑块时曲线立刻响应，而不必等到下一个重算周期。
func adaptiveSignature(cfg types.SmartControlConfig, tuning AdaptiveTuning) string {
	return fmt.Sprintf("%d|%d|%d|%d", tuning.Preference, tuning.LimitTemp, len(cfg.NoiseProfile), cfg.NoiseProfileUpdatedAt)
}

func sameCurve(a, b []types.FanCurvePoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ResetAdaptiveModel 清空学到的热模型与合成曲线，保留开关与倾向。
func ResetAdaptiveModel(cfg types.AdaptiveConfig) types.AdaptiveConfig {
	cfg.Model = NewAdaptiveThermalModel()
	cfg.AutoCurve = nil
	cfg.AutoCurveUpdatedAt = 0
	return cfg
}

// NormalizeAdaptiveConfig 归一化 2.0 配置，返回是否发生了修改。
func NormalizeAdaptiveConfig(cfg types.AdaptiveConfig) (types.AdaptiveConfig, bool) {
	changed := false

	if normalized := NormalizeAdaptivePreference(cfg.Preference); normalized != cfg.Preference {
		cfg.Preference = normalized
		changed = true
	}
	if normalized := NormalizeAdaptiveTempLimit(cfg.TempLimit); normalized != cfg.TempLimit {
		cfg.TempLimit = normalized
		changed = true
	}
	if cfg.Model.Baseline < adaptiveBaselineMin || cfg.Model.Baseline > adaptiveBaselineMax {
		cfg.Model.Baseline = NewAdaptiveThermalModel().Baseline
		changed = true
	}
	if sanitized, updated := sanitizeAdaptiveBuckets(cfg.Model.Buckets); updated {
		cfg.Model.Buckets = sanitized
		changed = true
	}
	if cfg.Model.Samples < 0 {
		cfg.Model.Samples = 0
		changed = true
	}
	if len(cfg.AutoCurve) > 0 {
		if err := validateAdaptiveCurve(cfg.AutoCurve); err != nil {
			cfg.AutoCurve = nil
			cfg.AutoCurveUpdatedAt = 0
			changed = true
		}
	}
	return cfg, changed
}

// sanitizeAdaptiveBuckets 清洗持久化的桶：剔除越界/非数值项，按转速升序，同桶去重。
// 配置文件是用户可编辑的，模型直接喂给控温环，必须假设它可能被写坏。
func sanitizeAdaptiveBuckets(buckets []types.AdaptiveThermalBucket) ([]types.AdaptiveThermalBucket, bool) {
	if len(buckets) == 0 {
		return buckets, false
	}
	cleaned := make([]types.AdaptiveThermalBucket, 0, len(buckets))
	for _, b := range buckets {
		if b.RPM <= 0 || b.RPM > adaptiveHWMaxRPM+adaptiveBucketWidth {
			continue
		}
		if isBadFloat(b.RisePerWatt) || isBadFloat(b.Rise) || isBadFloat(b.Weight) || isBadFloat(b.PowerWeight) {
			continue
		}
		b.RPM = adaptiveBucketCenter(b.RPM)
		b.Rise = clampFloat(b.Rise, 0, adaptiveRiseMax)
		b.RisePerWatt = clampFloat(b.RisePerWatt, 0, adaptiveRisePerWattMax)
		b.Weight = clampFloat(b.Weight, 0, adaptiveMaxWeight)
		b.PowerWeight = clampFloat(b.PowerWeight, 0, adaptiveMaxWeight)
		cleaned = append(cleaned, b)
	}

	sortAdaptiveBuckets(cleaned)
	deduped := cleaned[:0]
	for _, b := range cleaned {
		if len(deduped) > 0 && deduped[len(deduped)-1].RPM == b.RPM {
			deduped[len(deduped)-1] = b
			continue
		}
		deduped = append(deduped, b)
	}
	if len(deduped) > adaptiveMaxBuckets {
		deduped = deduped[:adaptiveMaxBuckets]
	}

	if len(deduped) != len(buckets) {
		return deduped, true
	}
	for i := range deduped {
		if deduped[i] != buckets[i] {
			return deduped, true
		}
	}
	return deduped, false
}

func sortAdaptiveBuckets(buckets []types.AdaptiveThermalBucket) {
	for i := 1; i < len(buckets); i++ {
		for j := i; j > 0 && buckets[j].RPM < buckets[j-1].RPM; j-- {
			buckets[j], buckets[j-1] = buckets[j-1], buckets[j]
		}
	}
}

func isBadFloat(v float64) bool {
	return v != v || v < -1e6 || v > 1e6
}

func validateAdaptiveCurve(curve []types.FanCurvePoint) error {
	if len(curve) < 2 {
		return fmt.Errorf("自动曲线点数不足")
	}
	for i, p := range curve {
		if p.RPM < 0 || p.RPM > adaptiveHWMaxRPM {
			return fmt.Errorf("自动曲线第 %d 点转速越界", i+1)
		}
		if i > 0 && p.Temperature <= curve[i-1].Temperature {
			return fmt.Errorf("自动曲线温度未递增")
		}
	}
	return nil
}

// AdaptiveStatus 汇总 2.0 的运行状态，供 GUI 展示"它现在学到哪了、打算怎么吹"。
type AdaptiveStatus struct {
	Enabled    bool `json:"enabled"`
	Preference int  `json:"preference"`
	TempLimit  int  `json:"tempLimit"`
	// CeilingTemp 是安全网的介入温度，不是控温目标——2.0 没有目标温度。
	CeilingTemp int                   `json:"ceilingTemp"`
	RPMFloor    int                   `json:"rpmFloor"`
	RPMCeil     int                   `json:"rpmCeil"`
	Confidence  float64               `json:"confidence"`
	Samples     int                   `json:"samples"`
	Baseline    float64               `json:"baseline"`
	UsingPower  bool                  `json:"usingPower"`
	Curve       []types.FanCurvePoint `json:"curve"`
	UpdatedAt   int64                 `json:"updatedAt"`
}

// BuildAdaptiveStatus 由配置计算当前状态；曲线为空时现场合成一条，
// 这样 GUI 在自动模式还没跑起来时也能预览"开启后大概会怎么吹"。
func BuildAdaptiveStatus(cfg types.SmartControlConfig) AdaptiveStatus {
	adaptive := cfg.Adaptive
	tuning := DeriveAdaptiveTuning(adaptive)
	_, usingPower := buildAdaptiveResponse(adaptive.Model.Buckets, true)

	curve := adaptive.AutoCurve
	if len(curve) == 0 {
		curve = SynthesizeAdaptiveCurve(adaptive.Model, tuning, cfg.NoiseProfile, nil)
	}

	return AdaptiveStatus{
		Enabled:     adaptive.Enabled,
		Preference:  tuning.Preference,
		TempLimit:   tuning.LimitTemp,
		CeilingTemp: tuning.CeilingTemp,
		RPMFloor:    tuning.RPMFloor,
		RPMCeil:     tuning.RPMCeil,
		Confidence:  AdaptiveModelConfidence(adaptive.Model),
		Samples:     adaptive.Model.Samples,
		Baseline:    adaptive.Model.Baseline,
		UsingPower:  usingPower,
		Curve:       curve,
		UpdatedAt:   adaptive.AutoCurveUpdatedAt,
	}
}

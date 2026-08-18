package smartcontrol

import (
	"math"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

// fakeMachine 是一台服从 T = baseline + P·k(rpm) 的虚拟机器，
// k 随转速指数衰减并趋于饱和——真实压风散热器的定性行为就是这样。
// 模型事先并不知道这条曲线，测试要检验的正是它能否把它辨识出来。
type fakeMachine struct {
	baseline float64
	kFloor   float64
	kSpan    float64
	kDecay   float64
}

func newFakeMachine() fakeMachine {
	return fakeMachine{baseline: 45, kFloor: 0.15, kSpan: 0.9, kDecay: 900}
}

func (m fakeMachine) k(rpm int) float64 {
	return m.kFloor + m.kSpan*math.Exp(-float64(rpm)/m.kDecay)
}

func (m fakeMachine) temp(rpm int, watts float64) int {
	return int(math.Round(m.baseline + watts*m.k(rpm)))
}

func adaptiveTestConfig(preference int) types.SmartControlConfig {
	cfg := types.GetDefaultSmartControlConfig(types.GetDefaultFanCurve())
	cfg.Adaptive.Enabled = true
	cfg.Adaptive.Preference = preference
	cfg.Adaptive.Model = NewAdaptiveThermalModel()
	return cfg
}

// runAdaptiveSimulation 让 2.0 在虚拟机器上跑一段时间，返回收敛后的配置。
// 循环刻意按"温度 → 观测 → 重算曲线 → 查表得到下一步转速"的真实顺序推进，
// 并对转速施加阻尼模拟限幅，否则瞬时平衡假设会制造现实中不存在的振荡。
func runAdaptiveSimulation(t *testing.T, cfg types.SmartControlConfig, machine fakeMachine, loads []float64, stepsPerLoad int) (types.SmartControlConfig, *AdaptiveEngine) {
	t.Helper()

	engine := NewAdaptiveEngine()
	now := time.Now()
	rpm := 2000

	for _, watts := range loads {
		for range stepsPerLoad {
			now = now.Add(3 * time.Second)

			measured := machine.temp(rpm, watts)
			if obs, ok := engine.Observe(measured, rpm, rpm, watts); ok {
				cfg.Adaptive.Model = UpdateAdaptiveThermalModel(cfg.Adaptive.Model, obs)
			}

			curve, _ := engine.Curve(cfg, now)
			target := temperature.CalculateTargetRPM(measured, curve)
			// 阻尼相当于升降速限幅：一步到位会让瞬时平衡假设产生假振荡。
			rpm += (target - rpm) / 2
		}
	}
	return cfg, engine
}

func TestAdaptiveModelRecoversCoolingResponse(t *testing.T) {
	machine := newFakeMachine()
	model := NewAdaptiveThermalModel()

	// 在多个转速与负载上采样，模拟用户日常在不同工况间切换。
	for _, rpm := range []int{1200, 1600, 2000, 2400, 2800, 3200} {
		for _, watts := range []float64{35, 60, 90} {
			for range 4 {
				model = UpdateAdaptiveThermalModel(model, AdaptiveObservation{
					RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, watts), Power: watts,
				})
			}
		}
	}

	response, ok := adaptiveModelResponse(model)
	if !ok {
		t.Fatal("模型应当已能给出查询表")
	}
	if !response.perWatt {
		t.Fatal("有功耗读数时应当使用每瓦温升模型")
	}

	// 单调性：转速越高，单位负载温升越低。
	for rpm := 1200; rpm < 3200; rpm += 100 {
		if response.valueAt(float64(rpm+100)) > response.valueAt(float64(rpm))+1e-9 {
			t.Fatalf("查询表在 %d RPM 处不单调", rpm)
		}
	}

	// 精度：辨识出的每瓦温升应当接近真值。基线由观测反推而来，本身带误差，
	// 所以这里检验的是量级正确而不是逐点吻合。
	for _, rpm := range []int{1600, 2400, 3200} {
		got := response.valueAt(float64(rpm))
		want := machine.k(rpm)
		if math.Abs(got-want) > 0.12 {
			t.Errorf("%d RPM 的每瓦温升估计 %.3f 偏离真值 %.3f 过多", rpm, got, want)
		}
	}
}

func TestAdaptiveModelFallsBackWithoutPowerReadings(t *testing.T) {
	machine := newFakeMachine()
	model := NewAdaptiveThermalModel()

	for _, rpm := range []int{1200, 1800, 2400, 3000} {
		for range 6 {
			model = UpdateAdaptiveThermalModel(model, AdaptiveObservation{
				RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, 70), Power: 0,
			})
		}
	}

	response, ok := adaptiveModelResponse(model)
	if !ok {
		t.Fatal("没有功耗读数时也应当能构建退化模型")
	}
	if response.perWatt {
		t.Fatal("没有功耗读数时不应当声称使用每瓦温升模型")
	}
	if response.valueAt(3000) >= response.valueAt(1200) {
		t.Error("退化模型同样应当反映'转速越高越凉'")
	}
}

func TestIsotonicDecreasingPoolsViolators(t *testing.T) {
	// 中间那个 0.9 违反单调（比右邻的 0.4 还小却排在前面是合法的；
	// 真正的违反是 0.3 之后又出现 0.9），应当与相邻点合并成区段均值。
	got := isotonicDecreasing([]float64{1.0, 0.3, 0.9, 0.2}, []float64{1, 1, 1, 1})
	for i := 1; i < len(got); i++ {
		if got[i] > got[i-1]+1e-9 {
			t.Fatalf("保序回归结果仍不单调: %v", got)
		}
	}
	// 保序回归保持总加权和不变。
	sumIn, sumOut := 1.0+0.3+0.9+0.2, 0.0
	for _, v := range got {
		sumOut += v
	}
	if math.Abs(sumIn-sumOut) > 1e-9 {
		t.Errorf("保序回归改变了总量: %.4f -> %.4f", sumIn, sumOut)
	}
}

func TestAdaptiveTuningIsMonotoneInPreference(t *testing.T) {
	prev := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: 0, TempLimit: types.DefaultAdaptiveTempLimit})
	for p := 10; p <= 100; p += 10 {
		cur := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: p, TempLimit: types.DefaultAdaptiveTempLimit})
		if cur.ThermalWeight < prev.ThermalWeight {
			t.Errorf("倾向 %d: 热代价权重不该随'更想要低温'而下降 (%.2f -> %.2f)", p, prev.ThermalWeight, cur.ThermalWeight)
		}
		if cur.CeilingTemp > prev.CeilingTemp {
			t.Errorf("倾向 %d: 安全介入温度不该随'更想要低温'而推后 (%d -> %d)", p, prev.CeilingTemp, cur.CeilingTemp)
		}
		if cur.RPMCeil < prev.RPMCeil {
			t.Errorf("倾向 %d: 转速上限不该随'更想要低温'而下降", p)
		}
		if cur.RampUp < prev.RampUp {
			t.Errorf("倾向 %d: 升速限幅不该随'更想要低温'而收紧", p)
		}
		prev = cur
	}
}

func TestAdaptiveTuningKeepsSafetyOrdering(t *testing.T) {
	for p := 0; p <= 100; p += 5 {
		for _, limit := range []int{75, 90, 100} {
			tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: p, TempLimit: limit})
			if tuning.CeilingTemp >= tuning.LimitTemp {
				t.Fatalf("倾向 %d 红线 %d: 安全介入温度(%d) 必须低于红线(%d)",
					p, limit, tuning.CeilingTemp, tuning.LimitTemp)
			}
			if tuning.ThermalWeight <= 0 {
				t.Fatalf("倾向 %d 红线 %d: 热代价权重必须为正", p, limit)
			}
			if tuning.RPMFloor >= tuning.RPMCeil {
				t.Fatalf("倾向 %d 红线 %d: 转速区间为空 [%d, %d]", p, limit, tuning.RPMFloor, tuning.RPMCeil)
			}
		}
	}
}

func TestSynthesizeCurveWithoutModelUsesSeed(t *testing.T) {
	tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: 50, TempLimit: types.DefaultAdaptiveTempLimit})
	curve := SynthesizeAdaptiveCurve(NewAdaptiveThermalModel(), tuning, nil, nil)
	seed := adaptiveSeedCurve(tuning)

	if len(curve) != len(seed) {
		t.Fatalf("曲线长度应与种子一致: %d != %d", len(curve), len(seed))
	}
	for i := range curve {
		// 合成结果会做 10 RPM 取整，允许这点差异。
		if absInt(curve[i].RPM-seed[i].RPM) > adaptiveCurveRounding {
			t.Errorf("零置信度下第 %d 点应贴近种子曲线: %d vs %d", i, curve[i].RPM, seed[i].RPM)
		}
	}
}

func TestSynthesizedCurveIsMonotoneAndSafe(t *testing.T) {
	machine := newFakeMachine()
	model := NewAdaptiveThermalModel()
	for _, rpm := range []int{1200, 1800, 2400, 3000} {
		for range 8 {
			model = UpdateAdaptiveThermalModel(model, AdaptiveObservation{
				RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, 75), Power: 75,
			})
		}
	}

	for _, p := range []int{0, 25, 50, 75, 100} {
		tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: p, TempLimit: types.DefaultAdaptiveTempLimit})
		curve := SynthesizeAdaptiveCurve(model, tuning, nil, nil)

		for i := 1; i < len(curve); i++ {
			if curve[i].RPM < curve[i-1].RPM {
				t.Errorf("倾向 %d: 曲线在 %d°C 处回落 (%d < %d)", p, curve[i].Temperature, curve[i].RPM, curve[i-1].RPM)
			}
		}
		for _, point := range curve {
			if point.RPM < adaptiveHWMinRPM || point.RPM > adaptiveHWMaxRPM {
				t.Errorf("倾向 %d: %d°C 处转速 %d 越过硬件包络", p, point.Temperature, point.RPM)
			}
			// 安全红线之上必须全速，倾向再安静也不例外。
			if point.Temperature >= tuning.LimitTemp && point.RPM != adaptiveHWMaxRPM {
				t.Errorf("倾向 %d: 红线温度 %d°C 处未全速 (%d RPM)", p, point.Temperature, point.RPM)
			}
		}
	}
}

func TestQuietPreferenceRunsSlowerThanCoolingPreference(t *testing.T) {
	machine := newFakeMachine()
	model := NewAdaptiveThermalModel()
	for _, rpm := range []int{1200, 1800, 2400, 3000} {
		for _, watts := range []float64{40, 80} {
			for range 5 {
				model = UpdateAdaptiveThermalModel(model, AdaptiveObservation{
					RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, watts), Power: watts,
				})
			}
		}
	}

	quiet := SynthesizeAdaptiveCurve(model, DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: 0, TempLimit: 90}), nil, nil)
	cool := SynthesizeAdaptiveCurve(model, DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: 100, TempLimit: 90}), nil, nil)

	compared := 0
	for i := range quiet {
		// 只比较安全网未介入的温度段；红线以上两者都必须全速，比较没有意义。
		if quiet[i].Temperature >= 80 {
			continue
		}
		compared++
		if quiet[i].RPM > cool[i].RPM {
			t.Errorf("%d°C: 安静倾向 (%d RPM) 不应快于低温倾向 (%d RPM)",
				quiet[i].Temperature, quiet[i].RPM, cool[i].RPM)
		}
	}
	if compared == 0 {
		t.Fatal("没有可比较的温度点，测试本身失效")
	}
}

// 2.0 没有目标温度，所以不能检验"落点离目标多远"。能检验的是它是否落在一个
// 讲得通的区间里——既没有热到危险，也没有为了几度白吹到满速。
func TestAdaptiveSettlesInReasonableRange(t *testing.T) {
	machine := newFakeMachine()
	loads := []float64{30, 70, 45, 95, 60, 80, 40, 90, 55, 75}

	for _, preference := range []int{20, 50, 80} {
		cfg := adaptiveTestConfig(preference)
		cfg, engine := runAdaptiveSimulation(t, cfg, machine, loads, 40)

		tuning := DeriveAdaptiveTuning(cfg.Adaptive)
		if confidence := AdaptiveModelConfidence(cfg.Adaptive.Model); confidence < 0.9 {
			t.Fatalf("倾向 %d: 跑完整段模拟后置信度仍只有 %.2f", preference, confidence)
		}

		curve, _ := engine.Curve(cfg, time.Now().Add(time.Hour))
		settled, rpm := settleOnCurve(machine, curve, 70)

		if settled >= tuning.LimitTemp {
			t.Errorf("倾向 %d: 平衡温度 %d°C 已经到安全红线 %d°C", preference, settled, tuning.LimitTemp)
		}
		if settled > tuning.CeilingTemp+3 {
			t.Errorf("倾向 %d: 平衡温度 %d°C 明显越过安全介入温度 %d°C", preference, settled, tuning.CeilingTemp)
		}
		// 落到最低转速说明代价函数完全没在乎温度；顶到最高说明它完全没在乎噪音。
		if rpm <= tuning.RPMFloor || rpm >= tuning.RPMCeil {
			t.Errorf("倾向 %d: 平衡转速 %d 顶在区间边界 [%d, %d]，权衡没有生效",
				preference, rpm, tuning.RPMFloor, tuning.RPMCeil)
		}
	}
}

// settleOnCurve 让虚拟机器在给定曲线上自行找平衡点。
func settleOnCurve(machine fakeMachine, curve []types.FanCurvePoint, watts float64) (int, int) {
	rpm, temp := 2000, 0
	for range 200 {
		temp = machine.temp(rpm, watts)
		rpm += (temperature.CalculateTargetRPM(temp, curve) - rpm) / 2
	}
	return temp, rpm
}

func TestAdaptivePreferenceChangesEquilibrium(t *testing.T) {
	machine := newFakeMachine()
	loads := []float64{35, 65, 50, 85, 60, 75}

	settle := func(preference int) (int, int) {
		cfg := adaptiveTestConfig(preference)
		cfg, engine := runAdaptiveSimulation(t, cfg, machine, loads, 40)
		curve, _ := engine.Curve(cfg, time.Now().Add(time.Hour))
		return settleOnCurve(machine, curve, 70)
	}

	quietTemp, quietRPM := settle(0)
	coolTemp, coolRPM := settle(100)

	if quietTemp <= coolTemp {
		t.Errorf("安静倾向的平衡温度 (%d°C) 应当高于低温倾向 (%d°C)", quietTemp, coolTemp)
	}
	if quietRPM >= coolRPM {
		t.Errorf("安静倾向的平衡转速 (%d) 应当低于低温倾向 (%d)", quietRPM, coolRPM)
	}
}

func TestAdaptiveEngineEmitsOnlyOnSteadyState(t *testing.T) {
	engine := NewAdaptiveEngine()

	// 温度还在爬升：不构成平衡点。
	for i := range 10 {
		if _, ok := engine.Observe(60+i*3, 2000, 2000, 50); ok {
			t.Fatalf("第 %d 步温度仍在变化，不该判定为稳态", i)
		}
	}

	// 稳下来之后应当在窗口填满时给出一次观测。
	emitted := 0
	for range adaptiveSteadyWindow * 2 {
		if _, ok := engine.Observe(70, 2000, 1980, 50); ok {
			emitted++
		}
	}
	if emitted != 2 {
		t.Errorf("连续 %d 个稳定采样应当产生 2 次观测，实际 %d 次", adaptiveSteadyWindow*2, emitted)
	}
}

func TestAdaptiveEngineDropsWindowOnRPMJump(t *testing.T) {
	engine := NewAdaptiveEngine()
	for range adaptiveSteadyWindow - 1 {
		engine.Observe(70, 2000, 2000, 50)
	}
	// 转速跳变说明控制器还在追温度，之前攒的样本作废；
	// 跳变后的这一拍成为新窗口的第一个样本。
	if _, ok := engine.Observe(70, 2600, 2600, 50); ok {
		t.Fatal("转速跳变后不该立刻判定为稳态")
	}
	for i := range adaptiveSteadyWindow - 2 {
		if _, ok := engine.Observe(70, 2600, 2600, 50); ok {
			t.Fatalf("新窗口只攒了 %d 个样本，不该提前给出观测", i+2)
		}
	}
	if _, ok := engine.Observe(70, 2600, 2600, 50); !ok {
		t.Fatal("新窗口攒满后应当给出观测")
	}
}

func TestAdaptiveEngineSkipsPowerAverageWhenIncomplete(t *testing.T) {
	engine := NewAdaptiveEngine()
	for i := range adaptiveSteadyWindow {
		power := 50.0
		if i == 2 {
			power = 0 // 某一拍读不到功耗
		}
		obs, ok := engine.Observe(70, 2000, 2000, power)
		if ok && obs.Power != 0 {
			t.Fatalf("功耗读数不完整时不该给出平均功耗，得到 %.1f", obs.Power)
		}
	}
}

func TestAdaptiveEngineResynthesizesImmediatelyOnPreferenceChange(t *testing.T) {
	cfg := adaptiveTestConfig(50)
	engine := NewAdaptiveEngine()
	now := time.Now()

	first, _ := engine.Curve(cfg, now)
	firstCopy := append([]types.FanCurvePoint(nil), first...)

	// 同一配置下的紧邻调用应当命中缓存。
	if _, changed := engine.Curve(cfg, now.Add(time.Second)); changed {
		t.Error("配置未变时不该重算曲线")
	}

	// 改倾向必须立刻生效，而且不受渐变限幅约束。
	cfg.Adaptive.Preference = 100
	next, changed := engine.Curve(cfg, now.Add(2*time.Second))
	if !changed {
		t.Fatal("倾向变化后应当立刻重算曲线")
	}

	maxDelta := 0
	for i := range next {
		maxDelta = max(maxDelta, absInt(next[i].RPM-firstCopy[i].RPM))
	}
	if maxDelta <= adaptiveCurveStepRPM {
		t.Errorf("倾向从 50 跳到 100，曲线最大变化仅 %d RPM，说明被渐变限幅卡住了", maxDelta)
	}
}

func TestAdaptiveEngineRecomputesAfterModelReset(t *testing.T) {
	machine := newFakeMachine()
	cfg := adaptiveTestConfig(50)
	for _, rpm := range []int{1400, 2000, 2600, 3200} {
		for range 8 {
			cfg.Adaptive.Model = UpdateAdaptiveThermalModel(cfg.Adaptive.Model, AdaptiveObservation{
				RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, 80), Power: 80,
			})
		}
	}

	engine := NewAdaptiveEngine()
	now := time.Now()
	learned, _ := engine.Curve(cfg, now)
	learnedCopy := append([]types.FanCurvePoint(nil), learned...)

	cfg.Adaptive = ResetAdaptiveModel(cfg.Adaptive)
	after, changed := engine.Curve(cfg, now.Add(time.Second))
	if !changed {
		t.Fatal("重置模型后应当重算曲线")
	}
	if sameCurve(after, learnedCopy) {
		t.Error("重置模型后曲线应当退回种子形态")
	}
}

func TestNormalizeAdaptiveConfigSanitizesGarbage(t *testing.T) {
	cfg := types.AdaptiveConfig{
		Preference: 500,
		TempLimit:  12,
		Model: types.AdaptiveThermalModel{
			Baseline: 9999,
			Buckets: []types.AdaptiveThermalBucket{
				{RPM: 3100, Rise: 12, Weight: 4},
				{RPM: -50, Rise: 5, Weight: 2},
				{RPM: 1100, Rise: math.NaN(), Weight: 3},
				{RPM: 1500, Rise: 8, Weight: 1e9},
			},
			Samples: -7,
		},
		AutoCurve: []types.FanCurvePoint{{Temperature: 60, RPM: 2000}, {Temperature: 50, RPM: 2200}},
	}

	normalized, changed := NormalizeAdaptiveConfig(cfg)
	if !changed {
		t.Fatal("这份配置到处都是非法值，应当报告已修改")
	}
	if normalized.Preference != types.AdaptivePreferenceMax {
		t.Errorf("倾向应被夹到上限，得到 %d", normalized.Preference)
	}
	if normalized.TempLimit != types.DefaultAdaptiveTempLimit {
		t.Errorf("非法红线应回落默认，得到 %d", normalized.TempLimit)
	}
	if normalized.Model.Baseline < adaptiveBaselineMin || normalized.Model.Baseline > adaptiveBaselineMax {
		t.Errorf("基线未被修正，得到 %.1f", normalized.Model.Baseline)
	}
	if normalized.Model.Samples != 0 {
		t.Errorf("负样本数应归零，得到 %d", normalized.Model.Samples)
	}
	if normalized.AutoCurve != nil {
		t.Error("温度非递增的自动曲线应被丢弃")
	}
	for _, b := range normalized.Model.Buckets {
		if b.RPM <= 0 || b.Weight > adaptiveMaxWeight || b.Rise != b.Rise {
			t.Errorf("桶未被清洗干净: %+v", b)
		}
	}
	for i := 1; i < len(normalized.Model.Buckets); i++ {
		if normalized.Model.Buckets[i].RPM <= normalized.Model.Buckets[i-1].RPM {
			t.Errorf("桶未按转速升序去重: %+v", normalized.Model.Buckets)
		}
	}
}

func TestApplyAdaptiveTuningDisablesLegacyLearning(t *testing.T) {
	cfg := adaptiveTestConfig(70)
	cfg.Learning = true
	cfg.LearnedOffsets = []int{100, 200, 300}
	cfg.TargetTemp = 68

	tuning := DeriveAdaptiveTuning(cfg.Adaptive)
	applied := ApplyAdaptiveTuning(cfg, tuning)

	if applied.Learning {
		t.Error("2.0 接管时必须关闭 1.0 学习，否则两套控制器互相叠加")
	}
	if applied.LearnedOffsets != nil {
		t.Error("2.0 接管时不应残留 1.0 的学习偏移")
	}
	// 2.0 没有目标温度；TargetTemp 只剩"尖峰过滤器到多热就别再抑制"这一个用途。
	if applied.TargetTemp != clampInt(tuning.CeilingTemp, 45, 90) {
		t.Errorf("TargetTemp 应改用安全介入温度 %d，得到 %d", tuning.CeilingTemp, applied.TargetTemp)
	}
	// 原始配置不能被改动：关掉 2.0 后用户的手动设置必须原样回来。
	if cfg.TargetTemp != 68 || !cfg.Learning {
		t.Error("ApplyAdaptiveTuning 不该改动传入的配置")
	}
	// 子开关在 2.0 下应当仍然判定为生效。
	if !PredictiveBoostActive(applied) || !LaptopFanGuardActive(applied) {
		t.Error("2.0 接管时前馈与缓降应当生效")
	}
}

func TestBuildAdaptiveStatusPreviewsCurveBeforeFirstRun(t *testing.T) {
	cfg := adaptiveTestConfig(30)
	cfg.Adaptive.AutoCurve = nil

	status := BuildAdaptiveStatus(cfg)
	if len(status.Curve) == 0 {
		t.Fatal("还没跑起来时也应当能预览曲线")
	}
	if status.Confidence != 0 || status.Samples != 0 {
		t.Error("全新模型的置信度与样本数都应为 0")
	}
	tuning := DeriveAdaptiveTuning(cfg.Adaptive)
	if status.CeilingTemp != tuning.CeilingTemp || status.Preference != 30 {
		t.Errorf("状态应回报派生后的参数，得到 ceiling=%d preference=%d", status.CeilingTemp, status.Preference)
	}
}

// 控温环每个采样周期都会对配置跑一次 NormalizeConfig，并在它报告"有修改"时
// 同步写盘。归一化若不是幂等的，2.0 就会变成每秒一次磁盘写入。
func TestNormalizeConfigIsIdempotentUnderAdaptiveLearning(t *testing.T) {
	machine := newFakeMachine()
	curve := types.GetDefaultFanCurve()
	cfg := types.GetDefaultSmartControlConfig(curve)
	cfg.Adaptive.Enabled = true
	cfg.Adaptive.Model = NewAdaptiveThermalModel()

	for _, rpm := range []int{1300, 1900, 2500, 3100} {
		for range 6 {
			cfg.Adaptive.Model = UpdateAdaptiveThermalModel(cfg.Adaptive.Model, AdaptiveObservation{
				RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, 70), Power: 70,
			})
		}
	}
	cfg.Adaptive.AutoCurve = SynthesizeAdaptiveCurve(cfg.Adaptive.Model, DeriveAdaptiveTuning(cfg.Adaptive), nil, nil)

	// 第一次可以有修改（旧配置补全字段），此后必须稳定。
	cfg, _ = NormalizeConfig(cfg, curve, false)
	for i := range 3 {
		var changed bool
		cfg, changed = NormalizeConfig(cfg, curve, false)
		if changed {
			t.Fatalf("第 %d 次归一化仍报告有修改，会导致每个采样周期都写盘", i+2)
		}
	}
}

/* ── 2.0 的核心承诺：任何温度上都没有"突然拐点" ── */

// 代价函数本身必须处处光滑。目标温度式的写法（低于 X 不管、高于 X 开罚）会在 X
// 处让增量突变，这个测试就是拦住那种写法重新溜回来。
func TestAdaptiveCostHasNoThreshold(t *testing.T) {
	tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: 50, TempLimit: types.DefaultAdaptiveTempLimit})

	const stepC = 0.5
	prevCost := adaptiveCost(40, 2000, tuning, nil)
	prevDelta := 0.0
	for temp := 40 + stepC; temp <= 100; temp += stepC {
		cost := adaptiveCost(temp, 2000, tuning, nil)
		delta := cost - prevCost
		if delta <= 0 {
			t.Fatalf("%.1f°C 处代价没有随温度上升 (Δ=%.6f)", temp, delta)
		}
		if prevDelta > 0 {
			// 指数代价的相邻增量之比是常数 e^(step/scale)；任何阈值都会让某一步
			// 的比值突然偏离，这里给一点余量容纳浮点误差。
			ratio := delta / prevDelta
			want := math.Exp(stepC / adaptiveThermalScaleC)
			if math.Abs(ratio-want) > 0.02 {
				t.Fatalf("%.1f°C 处代价增量突变: 比值 %.4f，期望 %.4f —— 说明存在阈值", temp, ratio, want)
			}
		}
		prevCost, prevDelta = cost, delta
	}
}

// 安全网早先在介入温度处是个阶跃（一度之内从不约束跳到七成转速）。
// 它必须连续，否则用户会在那个温度上听见风扇突然炸起来。
func TestAdaptiveSafetyFloorIsContinuous(t *testing.T) {
	for _, preference := range []int{0, 50, 100} {
		tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: preference, TempLimit: 90})

		prev := adaptiveSafetyFloor(30, tuning)
		for temp := 31; temp <= tuning.LimitTemp; temp++ {
			cur := adaptiveSafetyFloor(temp, tuning)
			if cur < prev {
				t.Fatalf("倾向 %d: 安全下限在 %d°C 处回落 (%d < %d)", preference, temp, cur, prev)
			}
			// 逐度跨越 500 RPM 就已经是听得见的突变了。
			if prev > 0 && cur-prev > 500 {
				t.Fatalf("倾向 %d: 安全下限在 %d°C 处跳变 %d RPM，存在阶跃", preference, temp, cur-prev)
			}
			prev = cur
		}
		if adaptiveSafetyFloor(tuning.LimitTemp, tuning) != adaptiveHWMaxRPM {
			t.Errorf("倾向 %d: 红线温度必须满速", preference)
		}
	}
}

func wellLearnedModel() types.AdaptiveThermalModel {
	machine := newFakeMachine()
	model := NewAdaptiveThermalModel()
	for _, rpm := range []int{1200, 1800, 2400, 3000, 3600} {
		for _, watts := range []float64{35, 70, 105} {
			for range 5 {
				model = UpdateAdaptiveThermalModel(model, AdaptiveObservation{
					RPM: rpm, ObservedRPM: rpm, Temp: machine.temp(rpm, watts), Power: watts,
				})
			}
		}
	}
	return model
}

// 用户感受到的不是栅格上的折线，而是控温环查表插值后的实际转速。
// 所以按 1°C 分辨率走一遍真实查表路径，看有没有"温度动一度、转速冲一大截"的地方。
//
// 阈值按整条曲线的总升幅折算：允许安全网在逼近红线时陡起来（那是有意为之），
// 但不允许任何一度独占过多升幅——那就是用户说的"到某个温度突然拐点"。
func TestEffectiveCurveHasNoCliff(t *testing.T) {
	model := wellLearnedModel()

	for _, preference := range []int{0, 30, 60, 100} {
		tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: preference, TempLimit: 90})
		curve := SynthesizeAdaptiveCurve(model, tuning, nil, nil)

		prev := temperature.CalculateTargetRPM(curve[0].Temperature, curve)
		worstJump, worstAt := 0, 0
		for temp := curve[0].Temperature + 1; temp <= curve[len(curve)-1].Temperature; temp++ {
			cur := temperature.CalculateTargetRPM(temp, curve)
			if jump := cur - prev; jump > worstJump {
				worstJump, worstAt = jump, temp
			}
			prev = cur
		}
		if worstJump > 320 {
			t.Errorf("倾向 %d: %d°C 处一度之内转速跳了 %d RPM，存在拐点", preference, worstAt, worstJump)
		}
	}
}

// 正常工作区（安全网尚未主导时）的曲线必须平缓，且各档升幅不能忽大忽小。
func TestSynthesizedCurveRisesGraduallyInWorkingRange(t *testing.T) {
	model := wellLearnedModel()

	for _, preference := range []int{0, 30, 60, 100} {
		tuning := DeriveAdaptiveTuning(types.AdaptiveConfig{Preference: preference, TempLimit: 90})
		curve := SynthesizeAdaptiveCurve(model, tuning, nil, nil)

		for i := 1; i < len(curve); i++ {
			span := curve[i].Temperature - curve[i-1].Temperature
			// 安全网自带的陡峭爬升不在此列：它只在逼近红线时接管。
			// 比较时放宽一个取整步长，否则曲线做完 10 RPM 取整后就对不上了。
			if adaptiveSafetyFloor(curve[i].Temperature, tuning) >= curve[i].RPM-adaptiveCurveRounding {
				continue
			}
			allowed := adaptiveMaxSlopeRPMPerC * span
			if delta := curve[i].RPM - curve[i-1].RPM; delta > allowed {
				t.Errorf("倾向 %d: 曲线在 %d°C 处一步跳了 %d RPM，超过限陡 %d",
					preference, curve[i].Temperature, delta, allowed)
			}
		}
	}
}

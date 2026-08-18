package coolingbenefit

import (
	"slices"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func step(rpm int, cpuTemp, gpuTemp, cpuPower, gpuPower float64) types.CoolingBenefitStep {
	return types.CoolingBenefitStep{
		TargetRPM: rpm,
		ActualRPM: rpm,
		CPUTemp:   cpuTemp,
		GPUTemp:   gpuTemp,
		CPUPower:  cpuPower,
		GPUPower:  gpuPower,
		Samples:   12,
	}
}

func hasWarning(analysis types.CoolingBenefitAnalysis, code string) bool {
	return slices.Contains(analysis.Warnings, code)
}

// 功耗墙：功耗恒定，多吹的转速全部换成温度。
func TestAnalyzeDetectsPowerLimitedRegime(t *testing.T) {
	steps := []types.CoolingBenefitStep{
		step(1000, 88, 80, 45, 40),
		step(2000, 82, 75, 45, 40),
		step(3000, 78, 71, 45, 40),
		step(4000, 76, 69, 45, 40),
	}
	analysis := AnalyzeReport(steps, nil)

	if analysis.Regime != types.CoolingRegimePower {
		t.Errorf("功耗恒定、温度下降应判定为功耗墙，得到 %s", analysis.Regime)
	}
	if analysis.TempDelta != -12 {
		t.Errorf("控温温度应取 CPU/GPU 较高者，从 88 降到 76，期望 -12，得到 %.1f", analysis.TempDelta)
	}
	if analysis.PowerDelta != 0 {
		t.Errorf("功耗未变时不该报告功耗收益，得到 %.1f", analysis.PowerDelta)
	}
	if analysis.TempPerKiloRPM != -4 {
		t.Errorf("3000 RPM 跨度降 12°C，每千转应为 -4°C，得到 %.1f", analysis.TempPerKiloRPM)
	}
}

// 温度墙：温度顶死不动，多吹的转速全部换成功耗释放。
func TestAnalyzeDetectsThermalLimitedRegime(t *testing.T) {
	steps := []types.CoolingBenefitStep{
		step(1000, 94, 84, 30, 60),
		step(2000, 94, 84, 38, 68),
		step(3000, 95, 85, 44, 74),
		step(4000, 95, 85, 47, 77),
	}
	analysis := AnalyzeReport(steps, nil)

	if analysis.Regime != types.CoolingRegimeThermal {
		t.Errorf("温度不降、功耗上升应判定为温度墙，得到 %s", analysis.Regime)
	}
	if analysis.PowerDelta != 34 {
		t.Errorf("总功耗从 90W 升到 124W，期望 +34，得到 %.1f", analysis.PowerDelta)
	}
}

func TestAnalyzeDetectsMixedAndInconclusive(t *testing.T) {
	mixed := AnalyzeReport([]types.CoolingBenefitStep{
		step(1000, 92, 85, 30, 55),
		step(2500, 86, 80, 36, 62),
		step(4000, 83, 77, 40, 66),
	}, nil)
	if mixed.Regime != types.CoolingRegimeMixed {
		t.Errorf("温度和功耗同时改善应判定为 mixed，得到 %s", mixed.Regime)
	}

	// 负载足够重但散热器毫无作用：必须诚实地说"测不出来"，而不是把噪声当收益。
	flat := AnalyzeReport([]types.CoolingBenefitStep{
		step(1000, 70, 65, 30, 30),
		step(2500, 70, 65, 30, 30),
		step(4000, 69, 65, 30, 30),
	}, nil)
	if flat.Regime != types.CoolingRegimeInconclusive {
		t.Errorf("温度功耗都没变应判定为 inconclusive，得到 %s", flat.Regime)
	}
}

func TestAnalyzeRanksSensorsByBenefit(t *testing.T) {
	base := step(1000, 88, 84, 40, 50)
	base.Sensors = []types.CoolingSensorReading{
		{Key: "cpu/pkg", Name: "CPU Package", Group: "cpu", Value: 88},
		{Key: "gpu/hotspot", Name: "GPU Hot Spot", Group: "gpu", Value: 102},
		{Key: "gpu/core", Name: "GPU Core", Group: "gpu", Value: 84},
		{Key: "gpu/late", Name: "只在高档出现", Group: "gpu", Value: 0},
	}
	top := step(4000, 82, 76, 40, 50)
	top.Sensors = []types.CoolingSensorReading{
		{Key: "cpu/pkg", Name: "CPU Package", Group: "cpu", Value: 82},
		{Key: "gpu/hotspot", Name: "GPU Hot Spot", Group: "gpu", Value: 87},
		{Key: "gpu/core", Name: "GPU Core", Group: "gpu", Value: 76},
		{Key: "gpu/late", Name: "只在高档出现", Group: "gpu", Value: 70},
	}

	analysis := AnalyzeReport([]types.CoolingBenefitStep{base, step(2500, 85, 80, 40, 50), top}, nil)

	if len(analysis.SensorDeltas) != 3 {
		t.Fatalf("基准档没有有效读数的传感器应被排除，期望 3 项，得到 %d", len(analysis.SensorDeltas))
	}
	if analysis.SensorDeltas[0].Key != "gpu/hotspot" {
		t.Errorf("降幅最大的 GPU 热点应排在最前，得到 %s", analysis.SensorDeltas[0].Key)
	}
	if analysis.SensorDeltas[0].Delta != -15 {
		t.Errorf("GPU 热点应降 15°C，得到 %.1f", analysis.SensorDeltas[0].Delta)
	}
	for i := 1; i < len(analysis.SensorDeltas); i++ {
		if analysis.SensorDeltas[i].Delta < analysis.SensorDeltas[i-1].Delta {
			t.Fatalf("传感器未按降幅排序: %+v", analysis.SensorDeltas)
		}
	}
}

// 负载中途掉线会画出一条漂亮但完全错误的曲线，用户没法自己看出来，必须报警。
func TestAnalyzeWarnsOnUnstableLoad(t *testing.T) {
	analysis := AnalyzeReport([]types.CoolingBenefitStep{
		step(1000, 88, 80, 45, 45),
		step(2000, 60, 55, 8, 5), // 用户把游戏关了
		step(3000, 58, 54, 8, 5),
		step(4000, 57, 53, 8, 5),
	}, nil)

	if !hasWarning(analysis, types.CoolingWarnLoadUnstable) {
		t.Errorf("负载中途骤降应报 loadUnstable，实际告警: %v", analysis.Warnings)
	}
}

func TestAnalyzeWarnsOnLightLoadAndUnsettledSteps(t *testing.T) {
	light := AnalyzeReport([]types.CoolingBenefitStep{
		step(1000, 45, 40, 4, 3),
		step(2500, 44, 39, 4, 3),
		step(4000, 43, 39, 4, 3),
	}, nil)
	if !hasWarning(light, types.CoolingWarnLoadTooLight) {
		t.Errorf("待机负载应报 loadTooLight，实际告警: %v", light.Warnings)
	}

	drifting := []types.CoolingBenefitStep{
		step(1000, 88, 80, 45, 45),
		step(2500, 84, 76, 45, 45),
		step(4000, 80, 72, 45, 45),
	}
	drifting[1].TempRange = 7 // 该档位其实还在漂
	unsettled := AnalyzeReport(drifting, nil)
	if !hasWarning(unsettled, types.CoolingWarnNotSettled) {
		t.Errorf("采样窗口波动过大应报 notSettled，实际告警: %v", unsettled.Warnings)
	}
}

func TestAnalyzeWarnsWhenFanCannotReachTarget(t *testing.T) {
	steps := []types.CoolingBenefitStep{
		step(1000, 88, 80, 45, 45),
		step(2500, 83, 76, 45, 45),
		step(4000, 80, 73, 45, 45),
	}
	steps[2].ActualRPM = 2600 // 这台设备上不去 4000
	analysis := AnalyzeReport(steps, nil)

	if !hasWarning(analysis, types.CoolingWarnRPMUnreachable) {
		t.Errorf("实际转速远低于目标应报 rpmUnreachable，实际告警: %v", analysis.Warnings)
	}
}

func TestAnalyzeFindsKneeWhereBenefitFlattens(t *testing.T) {
	// 收益集中在前半段：1000→2000 降 8°C，之后每档只降 0.5°C。
	analysis := AnalyzeReport([]types.CoolingBenefitStep{
		step(1000, 90, 82, 45, 45),
		step(2000, 82, 74, 45, 45),
		step(3000, 81.5, 73.5, 45, 45),
		step(4000, 81, 73, 45, 45),
	}, nil)

	if analysis.KneeRPM != 2000 {
		t.Errorf("收益在 2000 RPM 之后明显衰减，拐点应为 2000，得到 %d", analysis.KneeRPM)
	}
	// 没有噪音档案时甜点退化为拐点，且必须标明这一点。
	if analysis.SweetSpotHasNoise {
		t.Error("没有噪音档案时不该声称甜点是按噪音算出来的")
	}
	if analysis.SweetSpotRPM != analysis.KneeRPM {
		t.Errorf("无噪音档案时甜点应等于拐点，得到 %d vs %d", analysis.SweetSpotRPM, analysis.KneeRPM)
	}
}

func TestAnalyzeSweetSpotAccountsForNoiseCost(t *testing.T) {
	steps := []types.CoolingBenefitStep{
		step(1000, 90, 82, 45, 45),
		step(2000, 84, 76, 45, 45),
		step(3000, 82, 74, 45, 45),
		step(4000, 80, 72, 45, 45),
	}
	// 噪音在 3000 RPM 之后陡增：再往上换到的每一度都很贵。
	noise := []types.NoiseProfilePoint{
		{RPM: 1000, DB: 0},
		{RPM: 2000, DB: 2},
		{RPM: 3000, DB: 5},
		{RPM: 4000, DB: 18},
	}

	analysis := AnalyzeReport(steps, noise)
	if !analysis.SweetSpotHasNoise {
		t.Fatal("提供了噪音档案时应按噪音代价计算甜点")
	}
	if analysis.SweetSpotRPM >= 4000 {
		t.Errorf("4000 RPM 的噪音代价远高于收益，不该被选为甜点，得到 %d", analysis.SweetSpotRPM)
	}
}

func TestAnalyzeRejectsTooFewSteps(t *testing.T) {
	analysis := AnalyzeReport([]types.CoolingBenefitStep{step(1000, 80, 70, 40, 40)}, nil)
	if analysis.Regime != types.CoolingRegimeInconclusive {
		t.Errorf("单档无法比较，应为 inconclusive，得到 %s", analysis.Regime)
	}
	if !hasWarning(analysis, types.CoolingWarnFewSteps) {
		t.Errorf("档位不足应报 fewSteps，实际告警: %v", analysis.Warnings)
	}
}

func TestAnalyzeSortsStepsBeforeComparing(t *testing.T) {
	// 前端乱序提交也必须得到同样的结论。
	shuffled := AnalyzeReport([]types.CoolingBenefitStep{
		step(4000, 76, 69, 45, 40),
		step(1000, 88, 80, 45, 40),
		step(3000, 78, 71, 45, 40),
		step(2000, 82, 75, 45, 40),
	}, nil)

	if shuffled.BaselineRPM != 1000 || shuffled.TopRPM != 4000 {
		t.Errorf("应按转速排序后取首尾，得到 %d..%d", shuffled.BaselineRPM, shuffled.TopRPM)
	}
	if shuffled.TempDelta != -12 {
		t.Errorf("乱序输入应得到相同温差，得到 %.1f", shuffled.TempDelta)
	}
}

func TestAnalyzeReportsLaptopFanRelief(t *testing.T) {
	base := step(1000, 88, 80, 45, 45)
	base.LaptopCPUFanRPM = 4800
	top := step(4000, 80, 72, 45, 45)
	top.LaptopCPUFanRPM = 3400

	analysis := AnalyzeReport([]types.CoolingBenefitStep{base, step(2500, 84, 76, 45, 45), top}, nil)
	if analysis.LaptopFanDelta != -1400 {
		t.Errorf("本机风扇从 4800 降到 3400，期望 -1400，得到 %d", analysis.LaptopFanDelta)
	}
}

func TestPowerBucketBoundaries(t *testing.T) {
	cases := map[float64]int{0: 0, 24.9: 0, 25: 1, 49.9: 1, 50: 2, 79.9: 2, 80: 3, 119.9: 3, 120: 4, 400: 4}
	for watts, want := range cases {
		if got := PowerBucketOf(watts); got != want {
			t.Errorf("%.1fW 应落在档位 %d，得到 %d", watts, want, got)
		}
	}
	if PowerBucketCount() != len(PowerBucketBounds)+1 {
		t.Error("档位总数应比边界数多一")
	}
}

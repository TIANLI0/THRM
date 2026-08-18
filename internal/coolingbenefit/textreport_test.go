package coolingbenefit

import (
	"strings"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func sampleReport() *types.CoolingBenefitReport {
	base := step(1000, 92, 88, 45, 55)
	base.Sensors = []types.CoolingSensorReading{
		{Key: "cpu/Core Max", Name: "Core Max", Group: "cpu", Value: 92},
		{Key: "gpu/0/Hot Spot", Name: "GPU Hot Spot", Group: "gpu", Value: 104},
		{Key: "storage/nvme0", Name: "Samsung 990 Pro", Group: "other", Value: 58},
	}
	mid := step(2200, 88, 83, 48, 60)
	mid.Sensors = []types.CoolingSensorReading{
		{Key: "cpu/Core Max", Name: "Core Max", Group: "cpu", Value: 88},
		{Key: "gpu/0/Hot Spot", Name: "GPU Hot Spot", Group: "gpu", Value: 95},
		{Key: "storage/nvme0", Name: "Samsung 990 Pro", Group: "other", Value: 55},
	}
	top := step(4000, 85, 79, 52, 66)
	top.Sensors = []types.CoolingSensorReading{
		{Key: "cpu/Core Max", Name: "Core Max", Group: "cpu", Value: 85},
		{Key: "gpu/0/Hot Spot", Name: "GPU Hot Spot", Group: "gpu", Value: 88},
		{Key: "storage/nvme0", Name: "Samsung 990 Pro", Group: "other", Value: 52},
		// 只在最高档出现的传感器：矩阵里该留空，不能编一个基准值出来。
		{Key: "board/VRM", Name: "VRM", Group: "other", Value: 61},
	}
	base.LaptopCPUFanRPM = 5200
	top.LaptopCPUFanRPM = 4100

	steps := []types.CoolingBenefitStep{base, mid, top}
	return &types.CoolingBenefitReport{
		CreatedAt:   time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC).Unix(),
		DeviceModel: "BS2PRO",
		CPUModel:    "Ryzen 9 7945HX",
		GPUModel:    "RTX 4080 Laptop",
		LoadLabel:   "Cyberpunk 2077",
		Steps:       steps,
		Analysis:    AnalyzeReport(steps, nil),
	}
}

func TestFormatTextReportCoversRawDataAndSemantics(t *testing.T) {
	out := FormatTextReport(TextReportInput{
		Report:      sampleReport(),
		AppVersion:  "3.7.0",
		Platform:    "windows/amd64",
		GeneratedAt: time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC),
	})

	// 机器与工况必须在场，否则模型无从判断结论适用于什么。
	for _, want := range []string{"BS2PRO", "Ryzen 9 7945HX", "RTX 4080 Laptop", "Cyberpunk 2077"} {
		if !strings.Contains(out, want) {
			t.Errorf("导出内容缺少 %q", want)
		}
	}

	// 每个档位的原始行都要在，包括中间档——界面只展示首尾，导出不能也只给首尾。
	for _, want := range []string{"1000", "2200", "4000"} {
		if !strings.Contains(out, want) {
			t.Errorf("导出内容缺少档位 %s", want)
		}
	}

	// 逐传感器矩阵：每个传感器一行，且含中间档读数。
	for _, want := range []string{"cpu/Core Max", "gpu/0/Hot Spot", "storage/nvme0", "board/VRM"} {
		if !strings.Contains(out, want) {
			t.Errorf("导出内容缺少传感器 %q", want)
		}
	}
	if !strings.Contains(out, "95.0") {
		t.Error("逐传感器矩阵应包含中间档读数 95.0")
	}

	// 模型拿不到本仓库上下文，regime 的含义必须写进文件——光给个 "mixed" 没用。
	regime := sampleReport().Analysis.Regime
	if !strings.Contains(out, regimeExplanation(regime)) {
		t.Errorf("判定 %s 的含义说明未出现在导出内容里", regime)
	}

	// 单位与采集方法要自带说明。
	for _, want := range []string{"degrees Celsius", "How the data was collected"} {
		if !strings.Contains(out, want) {
			t.Errorf("导出内容缺少说明段 %q", want)
		}
	}
}

func TestFormatTextReportLeavesMissingSensorReadingsBlank(t *testing.T) {
	out := FormatTextReport(TextReportInput{Report: sampleReport()})

	var vrmLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "board/VRM") {
			vrmLine = line
			break
		}
	}
	if vrmLine == "" {
		t.Fatal("未找到 board/VRM 行")
	}
	// 只在最高档出现：前两档留空，末尾 delta 无从计算，只能是 '-'。
	if strings.Count(vrmLine, "61.0") != 1 {
		t.Errorf("VRM 只应有一个读数: %q", vrmLine)
	}
	if !strings.HasSuffix(strings.TrimRight(vrmLine, " "), "-") {
		t.Errorf("缺少基准读数时 delta 应为 '-': %q", vrmLine)
	}
}

// 带告警的报告如果不解释，模型会把一次负载掉线的测试当成真实散热特性来分析。
func TestFormatTextReportExplainsWarnings(t *testing.T) {
	steps := []types.CoolingBenefitStep{
		step(1000, 88, 80, 45, 45),
		step(2200, 60, 55, 8, 5), // 用户中途把负载关了
		step(4000, 58, 54, 8, 5),
	}
	report := &types.CoolingBenefitReport{Steps: steps, Analysis: AnalyzeReport(steps, nil)}

	out := FormatTextReport(TextReportInput{Report: report})
	if !strings.Contains(out, "READ BEFORE DRAWING CONCLUSIONS") {
		t.Error("有告警时应当有醒目提示")
	}
	if !strings.Contains(out, types.CoolingWarnLoadUnstable) {
		t.Error("应当列出告警码")
	}
	if !strings.Contains(out, "cannot be attributed to the cooler") {
		t.Error("应当解释告警意味着什么，而不只是给个代码")
	}
}

func TestFormatTextReportIncludesNoiseProfileBehindSweetSpot(t *testing.T) {
	report := sampleReport()
	noise := []types.NoiseProfilePoint{{RPM: 1000, DB: 0}, {RPM: 2200, DB: 6.5}, {RPM: 4000, DB: 19}}
	report.Analysis = AnalyzeReport(report.Steps, noise)

	out := FormatTextReport(TextReportInput{Report: report, NoiseProfile: noise})
	if !strings.Contains(out, "Measured noise profile") {
		t.Error("甜点转速依赖噪音档案，档案本身也应导出以便复核")
	}
	if !strings.Contains(out, "19.0") {
		t.Error("噪音档案数据点缺失")
	}
}

// 被动统计与实测混在一起会让模型给出过度自信的结论。
func TestFormatTextReportSeparatesPassiveStats(t *testing.T) {
	out := FormatTextReport(TextReportInput{
		Report: sampleReport(),
		Passive: types.CoolingPassiveStats{Cells: []types.CoolingPassiveCell{
			{PowerBucket: 3, RPM: 1400, CPUTemp: 84, GPUTemp: 79, Power: 95, Samples: 12},
			{PowerBucket: 3, RPM: 3400, CPUTemp: 77, GPUTemp: 72, Power: 97, Samples: 9},
		}},
		Comparisons: []types.CoolingPassiveComparison{
			{PowerBucket: 3, LowRPM: 1400, HighRPM: 3400, TempDelta: -7, Samples: 9},
		},
	})

	if !strings.Contains(out, "LOW CONFIDENCE") {
		t.Error("被动统计必须显著标注可信度低")
	}
	if !strings.Contains(out, "80-120W") {
		t.Error("应当标出功耗分桶区间，否则跨桶数字会被误当成可比")
	}
	// 可信度声明必须出现在被动数据之前。
	if strings.Index(out, "LOW CONFIDENCE") > strings.Index(out, "80-120W") {
		t.Error("可信度声明应当先于被动数据出现")
	}
}

func TestFormatTextReportHandlesMissingReport(t *testing.T) {
	out := FormatTextReport(TextReportInput{})
	if !strings.Contains(out, "No active measurement report") {
		t.Error("没有报告时应当说明，而不是产出一份空壳")
	}
	if strings.Contains(out, "Per-step measurements") {
		t.Error("没有报告时不该出现档位表头")
	}
}

// 四种形态对用户的意义完全不同，解释不能雷同，也不能漏掉任何一种。
func TestRegimeExplanationsAreDistinct(t *testing.T) {
	regimes := []string{
		types.CoolingRegimeThermal,
		types.CoolingRegimePower,
		types.CoolingRegimeMixed,
		types.CoolingRegimeInconclusive,
	}
	seen := map[string]string{}
	for _, regime := range regimes {
		text := regimeExplanation(regime)
		if text == "" {
			t.Errorf("%s 没有解释", regime)
		}
		if other, dup := seen[text]; dup {
			t.Errorf("%s 与 %s 的解释相同", regime, other)
		}
		seen[text] = regime
	}
}

// 每个告警码都要有对应解释，否则导出的文件里会出现模型看不懂的裸代码。
func TestWarningExplanationsCoverEveryCode(t *testing.T) {
	codes := []string{
		types.CoolingWarnLoadUnstable,
		types.CoolingWarnLoadTooLight,
		types.CoolingWarnNotSettled,
		types.CoolingWarnRPMUnreachable,
		types.CoolingWarnFewSteps,
	}
	fallback := warningExplanation("definitely-not-a-real-code")
	for _, code := range codes {
		if text := warningExplanation(code); text == "" || text == fallback {
			t.Errorf("告警码 %s 缺少专属解释", code)
		}
	}
}

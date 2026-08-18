package coolingbenefit

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
纯文本导出。

这份文件的读者是语言模型，不是人。因此它的写法和界面上的报告不一样：

  - 全英文。传感器名本来就是 LibreHardwareMonitor 给的英文，混排只会让分词更糟；
    而且中文 .txt 在部分 Windows 工具里不加 BOM 会显示成乱码，英文彻底绕开这个问题。
  - 自带语义说明。模型拿不到本仓库的上下文，"thermal / power regime"、各条告警的
    含义、被动统计为什么不可靠，都必须写在文件里，否则它只能凭字面猜。
  - 原始数据优先于结论。每个档位的逐传感器读数全部导出，而不只是界面上那份
    "基准档 → 最高档"的差值——差值是我们的解读，模型应该能自己重新解读。
  - 明确标注数据可信度。带告警的报告如果不加说明，模型会把一次负载掉线的测试
    当成真实的散热特性来分析。
*/

const textReportIndent = "  "

// TextReportInput 汇总导出所需的全部数据。
type TextReportInput struct {
	Report *types.CoolingBenefitReport
	// NoiseProfile 是可选的实测噪音档案。甜点转速依赖它，不带上的话模型无法复核那个结论。
	NoiseProfile []types.NoiseProfilePoint
	AppVersion   string
	Platform     string
	GeneratedAt  time.Time
}

// FormatTextReport 把一次散热收益测试渲染成便于模型分析的纯文本。
func FormatTextReport(in TextReportInput) string {
	var b strings.Builder

	writeTextHeader(&b, in)
	if in.Report == nil {
		b.WriteString("\nNo measurement report is stored yet.\n")
		return b.String()
	}

	writeMachineSection(&b, in)
	writeMethodSection(&b)
	writeStepSection(&b, in.Report.Steps)
	writeSensorMatrix(&b, in.Report.Steps)
	writeAnalysisSection(&b, in.Report.Analysis)
	writeNoiseSection(&b, in.NoiseProfile)
	return b.String()
}

func writeTextHeader(b *strings.Builder, in TextReportInput) {
	generated := in.GeneratedAt
	if generated.IsZero() {
		generated = time.Now()
	}
	fmt.Fprintf(b, "THRM cooling benefit report\n")
	fmt.Fprintf(b, "===========================\n\n")
	fmt.Fprintf(b, "Generated: %s\n", generated.Format(time.RFC3339))
	if in.AppVersion != "" {
		fmt.Fprintf(b, "App version: %s\n", in.AppVersion)
	}
	if in.Platform != "" {
		fmt.Fprintf(b, "Platform: %s\n", in.Platform)
	}
	b.WriteString(`
What this is
------------
THRM controls an external laptop cooling pad (a fan that blows onto the
underside of a laptop). This report measures what that cooler actually buys at
each of its speeds on this specific machine: how much cooler components run,
and how much extra sustained power the laptop can hold.

All temperatures are degrees Celsius, power is watts, fan speeds are RPM.
"Cooler" always means the external pad; "laptop fan" means the laptop's own
internal fans.
`)
}

func writeMachineSection(b *strings.Builder, in TextReportInput) {
	r := in.Report
	b.WriteString("\nMachine and workload\n--------------------\n")
	fmt.Fprintf(b, "Measured at: %s\n", time.Unix(r.CreatedAt, 0).Format(time.RFC3339))
	writeOptional(b, "Cooler model", r.DeviceModel)
	writeOptional(b, "CPU", r.CPUModel)
	writeOptional(b, "GPU", r.GPUModel)
	if strings.TrimSpace(r.LoadLabel) != "" {
		fmt.Fprintf(b, "Workload (user-supplied note): %s\n", r.LoadLabel)
	} else {
		b.WriteString("Workload: not recorded by the user\n")
	}
}

func writeOptional(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) != "" {
		fmt.Fprintf(b, "%s: %s\n", label, value)
	}
}

func writeMethodSection(b *strings.Builder) {
	b.WriteString(`
How the data was collected
--------------------------
The user starts one sustained workload and keeps it running unchanged. The
cooler is then locked to each speed in turn. At every step the tool waits until
the control temperature stops drifting (rolling range within 1.5 C, at least
30 s, at most 150 s) before it samples for ~15 s. Averages over that sampling
window are what appears below.

This matters for interpretation: because the workload is held constant across
steps, differences between steps can be attributed to the cooler.
`)
}

func writeStepSection(b *strings.Builder, steps []types.CoolingBenefitStep) {
	b.WriteString("\nPer-step measurements\n---------------------\n")
	b.WriteString(`Columns:
  target   cooler speed commanded (RPM)
  actual   cooler speed reported by the device (RPM); far below target means
           the fan could not reach that step
  cpuT     CPU temperature (C)         gpuT   GPU temperature (C)
  cpuW     CPU package power (W)       gpuW   GPU power (W)
  lapFan   highest laptop internal fan speed (RPM), 0 = not readable
  n        samples averaged
  dT/dW    spread of control temperature / total power inside the sampling
           window; large values mean that step had not settled

`)

	ordered := sortedSteps(steps)
	fmt.Fprintf(b, "%-7s %-7s %-6s %-6s %-7s %-7s %-7s %-4s %-6s %s\n",
		"target", "actual", "cpuT", "gpuT", "cpuW", "gpuW", "lapFan", "n", "dT", "dW")
	for _, s := range ordered {
		fmt.Fprintf(b, "%-7d %-7d %-6.1f %-6.1f %-7.1f %-7.1f %-7d %-4d %-6.1f %.1f\n",
			s.TargetRPM, s.ActualRPM, s.CPUTemp, s.GPUTemp, s.CPUPower, s.GPUPower,
			laptopFan(s), s.Samples, s.TempRange, s.PowerRange)
	}
}

// writeSensorMatrix 导出逐传感器 × 逐档位的完整读数矩阵。
// 界面上只展示"基准档 → 最高档"的差值，那是我们的解读；模型应该拿到原始矩阵，
// 才能自己看出中间档位的非线性、某个部件提前饱和之类界面没讲的事。
func writeSensorMatrix(b *strings.Builder, steps []types.CoolingBenefitStep) {
	ordered := sortedSteps(steps)
	names, values := collectSensorMatrix(ordered)
	if len(names) == 0 {
		b.WriteString(`
Per-sensor readings
-------------------
No named sensors were recorded. Extended sensors (memory, storage,
motherboard, PSU) are only enabled during the test itself; if the bridge could
not read them, only CPU and GPU aggregates above are available.
`)
		return
	}

	b.WriteString("\nPer-sensor readings (C) at each cooler speed\n")
	b.WriteString("-------------------------------------------\n")
	b.WriteString(`Sensor keys are prefixed by component group: cpu/, gpu/, memory/, storage/,
board/, ec/, psu/, battery/. A blank cell means that sensor had no valid
reading at that step.

`)

	width := 0
	for _, name := range names {
		width = max(width, len(name))
	}
	width = min(max(width, 20), 46)

	fmt.Fprintf(b, "%-*s", width, "sensor")
	for _, s := range ordered {
		fmt.Fprintf(b, " %8d", s.TargetRPM)
	}
	b.WriteString("   delta\n")

	for _, name := range names {
		row := values[name]
		fmt.Fprintf(b, "%-*s", width, truncate(name, width))
		for i := range ordered {
			if v, ok := row[i]; ok {
				fmt.Fprintf(b, " %8.1f", v)
			} else {
				fmt.Fprintf(b, " %8s", "")
			}
		}
		first, firstOK := row[0]
		last, lastOK := row[len(ordered)-1]
		if firstOK && lastOK {
			fmt.Fprintf(b, "   %+.1f\n", last-first)
		} else {
			b.WriteString("       -\n")
		}
	}
}

// collectSensorMatrix 返回排序后的传感器展示名，以及 名称 -> (档位下标 -> 读数)。
// 用 Key 去重、用 Name 展示：多显卡时 Name 会带设备前缀，Key 才是稳定标识。
func collectSensorMatrix(ordered []types.CoolingBenefitStep) ([]string, map[string]map[int]float64) {
	labels := map[string]string{}
	values := map[string]map[int]float64{}

	for idx, step := range ordered {
		for _, sensor := range step.Sensors {
			if sensor.Value <= 0 {
				continue
			}
			// Name 往往就是 Key 的末段（board/VRM 对 "VRM"），附注一遍纯属噪声。
			// 只有多显卡这类 Name 带了设备前缀、确实多出信息时才附上。
			label := sensor.Key
			name := strings.TrimSpace(sensor.Name)
			if name != "" && name != sensor.Key && name != keyTail(sensor.Key) {
				label = fmt.Sprintf("%s (%s)", sensor.Key, name)
			}
			labels[sensor.Key] = label
			row, ok := values[label]
			if !ok {
				row = map[int]float64{}
				values[label] = row
			}
			row[idx] = sensor.Value
		}
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, values
}

func writeAnalysisSection(b *strings.Builder, a types.CoolingBenefitAnalysis) {
	b.WriteString("\nAnalysis produced by THRM\n-------------------------\n")
	b.WriteString("(This is THRM's own reading of the data above. Feel free to disagree with\nit — the raw numbers are all present.)\n\n")

	fmt.Fprintf(b, "Compared range: %d RPM -> %d RPM\n", a.BaselineRPM, a.TopRPM)
	fmt.Fprintf(b, "Control temperature change: %+.1f C\n", a.TempDelta)
	fmt.Fprintf(b, "Total power change: %+.1f W\n", a.PowerDelta)
	if a.LaptopFanDelta != 0 {
		fmt.Fprintf(b, "Laptop internal fan change: %+d RPM\n", a.LaptopFanDelta)
	}
	fmt.Fprintf(b, "Per 1000 RPM: %+.1f C, %+.1f W\n", a.TempPerKiloRPM, a.PowerPerKiloRPM)
	fmt.Fprintf(b, "Diminishing-returns knee: %d RPM\n", a.KneeRPM)
	if a.SweetSpotHasNoise {
		fmt.Fprintf(b, "Best benefit per dB of added noise: %d RPM (from a measured noise profile)\n", a.SweetSpotRPM)
	}

	fmt.Fprintf(b, "\nRegime: %s\n", a.Regime)
	fmt.Fprintf(b, "%s%s\n", textReportIndent, regimeExplanation(a.Regime))

	if len(a.Warnings) == 0 {
		b.WriteString("\nReliability: no problems detected.\n")
		return
	}
	b.WriteString("\nReliability warnings — READ BEFORE DRAWING CONCLUSIONS\n")
	for _, code := range a.Warnings {
		fmt.Fprintf(b, "%s- %s: %s\n", textReportIndent, code, warningExplanation(code))
	}
}

// regimeExplanation 说明这次测试里散热收益体现在哪一侧。
// 模型没有本仓库的上下文，"thermal" 这个词本身不足以传达"温度顶住不动、收益全在功耗"。
func regimeExplanation(regime string) string {
	switch regime {
	case types.CoolingRegimeThermal:
		return "Thermally limited. Temperature stayed pinned near its ceiling, so the " +
			"extra airflow turned into higher sustained power rather than lower " +
			"temperatures. The benefit here is performance, not a cooler machine."
	case types.CoolingRegimePower:
		return "Power limited. Power draw stayed flat, so all of the extra airflow " +
			"turned into lower temperatures. The benefit here is temperature and " +
			"component life, not performance."
	case types.CoolingRegimeMixed:
		return "Both. Temperature fell and sustained power rose at the same time, so " +
			"under this workload the cooler makes the machine both cooler and faster."
	default:
		return "Inconclusive. Neither temperature nor power moved meaningfully. Likely " +
			"causes: the workload was too light, this machine is not thermally " +
			"constrained, or the cooler is not seated against the intake."
	}
}

func warningExplanation(code string) string {
	switch code {
	case types.CoolingWarnLoadUnstable:
		return "Power draw varied too much between steps, so the workload changed " +
			"during the test. Temperature differences between steps therefore " +
			"include load differences and cannot be attributed to the cooler."
	case types.CoolingWarnLoadTooLight:
		return "The load was too light throughout for any speed to make a measurable " +
			"difference."
	case types.CoolingWarnNotSettled:
		return "At least one step was still drifting when sampled, so its readings sit " +
			"off the true steady-state value."
	case types.CoolingWarnRPMUnreachable:
		return "The cooler could not reach some commanded speeds, so the high-speed " +
			"steps were not actually measured at those speeds. Use the 'actual' " +
			"column rather than 'target'."
	case types.CoolingWarnFewSteps:
		return "Too few valid steps to establish a trend."
	default:
		return "Unrecognised warning code."
	}
}

func writeNoiseSection(b *strings.Builder, profile []types.NoiseProfilePoint) {
	if len(profile) < 2 {
		return
	}
	b.WriteString("\nMeasured noise profile\n----------------------\n")
	b.WriteString("Relative A-weighted level from a microphone sweep, normalised so the\nquietest measured speed is 0 dB. Not an absolute SPL.\n\n")
	fmt.Fprintf(b, "%-8s %s\n", "rpm", "dB above quietest")
	for _, p := range profile {
		fmt.Fprintf(b, "%-8d %.1f\n", p.RPM, p.DB)
	}
}

func sortedSteps(steps []types.CoolingBenefitStep) []types.CoolingBenefitStep {
	ordered := make([]types.CoolingBenefitStep, len(steps))
	copy(ordered, steps)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].TargetRPM < ordered[j].TargetRPM })
	return ordered
}

// keyTail 返回 Key 的最后一段（"gpu/0/Hot Spot" -> "Hot Spot"）。
func keyTail(key string) string {
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "~"
}

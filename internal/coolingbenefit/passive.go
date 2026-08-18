package coolingbenefit

import (
	"math"
	"sort"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

/*
日常被动统计。

主动测试可信但要花十分钟，被动统计零成本但样本来自不同时间的不同负载。要让后者
不至于胡说八道，唯一的办法是承认这个混杂并把它建进数据结构里：按「功耗档 × 转速档」
二维分桶，只在同一功耗档内部横向比较转速。

只按转速分桶是行不通的——用户在低转速时多半在做轻活、高转速时在打游戏，直接比较
两个桶的平均温度，得到的"高转速反而更热"完全是负载差异，跟散热器没关系。

即便二维分桶，功耗也只是负载的粗糙代理（同样 60W，CPU 满载和 GPU 满载的热分布
完全不同）。所以这条数据在界面上必须始终标注为仅供参考，不能和主动测试并列陈述。
*/

const (
	rpmBucketWidth = 400
	maxCells       = 60

	// EMA 等效样本数：格子命中够多之后新样本恒定占 1/(span+1)，
	// 既稳定又能跟随季节、灰尘、硅脂老化带来的长期变化。
	emaSpan = 12.0

	// 稳态判据。被动路径不像主动测试那样能主动等，只能挑"恰好稳住"的片段。
	steadyWindow    = 5
	steadyRPMRange  = 80
	steadyTempRange = 3

	minRecordablePowerW = 10.0
	minRecordableRPM    = 300
)

// MinCellSamples 是一格可以拿出来展示的最低样本数。
const MinCellSamples = 5

type passiveSample struct {
	rpm     int
	cpuTemp int
	gpuTemp int
	power   float64
}

// PassiveObserver 从日常采样里挑出稳态片段并归入统计格。
// 它只在一次监控会话内有效，累积结果由调用方持久化。
type PassiveObserver struct {
	window []passiveSample
}

func NewPassiveObserver() *PassiveObserver {
	return &PassiveObserver{window: make([]passiveSample, 0, steadyWindow)}
}

// Reset 丢弃进行中的窗口。设备重连、休眠恢复后必须调用：
// 跨越这些事件的采样点属于不同工况，凑在一起会造出不存在的稳态。
func (o *PassiveObserver) Reset() {
	if o == nil {
		return
	}
	o.window = o.window[:0]
}

// Observe 累积一次采样。达到稳态时把它并入 stats 并返回更新后的副本与 true。
func (o *PassiveObserver) Observe(stats types.CoolingPassiveStats, coolerRPM, cpuTemp, gpuTemp int, power float64) (types.CoolingPassiveStats, bool) {
	if o == nil || coolerRPM < minRecordableRPM || power < minRecordablePowerW {
		// 待机或读不到功耗的样本没有分析价值：所有转速下都一样凉，
		// 记进去只会把格子填满没有区分度的数据。
		o.Reset()
		return stats, false
	}
	if cpuTemp <= 0 && gpuTemp <= 0 {
		o.Reset()
		return stats, false
	}

	sample := passiveSample{rpm: coolerRPM, cpuTemp: cpuTemp, gpuTemp: gpuTemp, power: power}
	if len(o.window) > 0 {
		prev := o.window[len(o.window)-1]
		if absInt(coolerRPM-prev.rpm) > steadyRPMRange ||
			absInt(maxInt(cpuTemp, gpuTemp)-maxInt(prev.cpuTemp, prev.gpuTemp)) > steadyTempRange ||
			math.Abs(power-prev.power) > power*0.35 {
			o.window = o.window[:0]
		}
	}
	o.window = append(o.window, sample)
	if len(o.window) > steadyWindow {
		o.window = o.window[len(o.window)-steadyWindow:]
	}
	if len(o.window) < steadyWindow {
		return stats, false
	}

	minRPM, maxRPM := o.window[0].rpm, o.window[0].rpm
	minTemp, maxTemp := maxInt(o.window[0].cpuTemp, o.window[0].gpuTemp), maxInt(o.window[0].cpuTemp, o.window[0].gpuTemp)
	sumRPM, sumCPU, sumGPU := 0, 0, 0
	sumPower := 0.0
	for _, s := range o.window {
		minRPM, maxRPM = min(minRPM, s.rpm), max(maxRPM, s.rpm)
		hottest := maxInt(s.cpuTemp, s.gpuTemp)
		minTemp, maxTemp = min(minTemp, hottest), max(maxTemp, hottest)
		sumRPM += s.rpm
		sumCPU += s.cpuTemp
		sumGPU += s.gpuTemp
		sumPower += s.power
	}
	if maxRPM-minRPM > steadyRPMRange || maxTemp-minTemp > steadyTempRange {
		return stats, false
	}

	count := len(o.window)
	meanPower := sumPower / float64(count)
	o.window = o.window[:0]

	return recordCell(stats, sumRPM/count, float64(sumCPU)/float64(count), float64(sumGPU)/float64(count), meanPower), true
}

func recordCell(stats types.CoolingPassiveStats, rpm int, cpuTemp, gpuTemp, power float64) types.CoolingPassiveStats {
	bucket := PowerBucketOf(power)
	center := rpmBucketCenter(rpm)

	cells := make([]types.CoolingPassiveCell, len(stats.Cells))
	copy(cells, stats.Cells)

	idx := -1
	for i := range cells {
		if cells[i].PowerBucket == bucket && cells[i].RPM == center {
			idx = i
			break
		}
	}
	if idx < 0 {
		cells = append(cells, types.CoolingPassiveCell{PowerBucket: bucket, RPM: center})
		idx = len(cells) - 1
	}

	cell := &cells[idx]
	weight := float64(cell.Samples)
	cell.CPUTemp = ema(cell.CPUTemp, cpuTemp, weight)
	cell.GPUTemp = ema(cell.GPUTemp, gpuTemp, weight)
	cell.Power = ema(cell.Power, power, weight)
	cell.Samples++

	// 格子超限时丢样本最少的那个：它最没有统计意义，也最可能是一次性的偶发工况。
	if len(cells) > maxCells {
		weakest := 0
		for i := range cells {
			if cells[i].Samples < cells[weakest].Samples {
				weakest = i
			}
		}
		cells = append(cells[:weakest], cells[weakest+1:]...)
	}

	sortCells(cells)
	stats.Cells = cells
	stats.UpdatedAt = time.Now().Unix()
	return stats
}

func ema(prev, sample, weight float64) float64 {
	if weight <= 0 {
		return sample
	}
	w := math.Min(weight, emaSpan)
	return (prev*w + sample) / (w + 1)
}

func rpmBucketCenter(rpm int) int {
	idx := rpm / rpmBucketWidth
	return idx*rpmBucketWidth + rpmBucketWidth/2
}

func sortCells(cells []types.CoolingPassiveCell) {
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].PowerBucket != cells[j].PowerBucket {
			return cells[i].PowerBucket < cells[j].PowerBucket
		}
		return cells[i].RPM < cells[j].RPM
	})
}

// SanitizePassiveStats 清洗持久化的统计格。配置文件是用户可编辑的，
// 而这些数字会直接画进图表，必须假设它可能被写坏。
func SanitizePassiveStats(stats types.CoolingPassiveStats) (types.CoolingPassiveStats, bool) {
	if len(stats.Cells) == 0 {
		return stats, false
	}
	cleaned := make([]types.CoolingPassiveCell, 0, len(stats.Cells))
	for _, cell := range stats.Cells {
		if cell.RPM <= 0 || cell.RPM > 20000 || cell.Samples <= 0 {
			continue
		}
		if cell.PowerBucket < 0 || cell.PowerBucket >= PowerBucketCount() {
			continue
		}
		if isBadFloat(cell.CPUTemp) || isBadFloat(cell.GPUTemp) || isBadFloat(cell.Power) {
			continue
		}
		cell.RPM = rpmBucketCenter(cell.RPM)
		cleaned = append(cleaned, cell)
	}

	sortCells(cleaned)
	deduped := cleaned[:0]
	for _, cell := range cleaned {
		if len(deduped) > 0 {
			last := &deduped[len(deduped)-1]
			if last.PowerBucket == cell.PowerBucket && last.RPM == cell.RPM {
				if cell.Samples > last.Samples {
					*last = cell
				}
				continue
			}
		}
		deduped = append(deduped, cell)
	}
	if len(deduped) > maxCells {
		deduped = deduped[:maxCells]
	}

	if len(deduped) != len(stats.Cells) {
		stats.Cells = deduped
		return stats, true
	}
	for i := range deduped {
		if deduped[i] != stats.Cells[i] {
			stats.Cells = deduped
			return stats, true
		}
	}
	stats.Cells = deduped
	return stats, false
}

// ComparePassive 在每个功耗档里挑出样本足够的最低与最高转速格做对比。
// 跨功耗档的比较一律不做——那正是被动数据不可信的根源。
func ComparePassive(stats types.CoolingPassiveStats) []types.CoolingPassiveComparison {
	byBucket := make(map[int][]types.CoolingPassiveCell)
	for _, cell := range stats.Cells {
		if cell.Samples < MinCellSamples {
			continue
		}
		byBucket[cell.PowerBucket] = append(byBucket[cell.PowerBucket], cell)
	}

	out := make([]types.CoolingPassiveComparison, 0, len(byBucket))
	for bucket, cells := range byBucket {
		if len(cells) < 2 {
			continue
		}
		sort.Slice(cells, func(i, j int) bool { return cells[i].RPM < cells[j].RPM })
		low, high := cells[0], cells[len(cells)-1]
		if high.RPM-low.RPM < rpmBucketWidth {
			continue
		}
		out = append(out, types.CoolingPassiveComparison{
			PowerBucket: bucket,
			LowRPM:      low.RPM,
			HighRPM:     high.RPM,
			TempDelta:   round1(math.Max(high.CPUTemp, high.GPUTemp) - math.Max(low.CPUTemp, low.GPUTemp)),
			Samples:     min(low.Samples, high.Samples),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PowerBucket < out[j].PowerBucket })
	return out
}

func isBadFloat(v float64) bool {
	return v != v || v < -1e6 || v > 1e6
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

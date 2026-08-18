package coolingbenefit

import (
	"math"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

// feed 连续投喂同一个工况，返回累积后的统计与产生的观测次数。
func feed(observer *PassiveObserver, stats types.CoolingPassiveStats, ticks, rpm, cpuTemp, gpuTemp int, power float64) (types.CoolingPassiveStats, int) {
	recorded := 0
	for range ticks {
		next, ok := observer.Observe(stats, rpm, cpuTemp, gpuTemp, power)
		stats = next
		if ok {
			recorded++
		}
	}
	return stats, recorded
}

func findCell(stats types.CoolingPassiveStats, bucket, rpm int) (types.CoolingPassiveCell, bool) {
	for _, cell := range stats.Cells {
		if cell.PowerBucket == bucket && cell.RPM == rpm {
			return cell, true
		}
	}
	return types.CoolingPassiveCell{}, false
}

func TestPassiveObserverRecordsOnlySteadySegments(t *testing.T) {
	observer := NewPassiveObserver()
	stats := types.CoolingPassiveStats{}

	// 温度还在爬升：不构成稳态。
	for i := range 10 {
		var ok bool
		stats, ok = observer.Observe(stats, 2000, 60+i*4, 55, 60)
		if ok {
			t.Fatalf("第 %d 拍温度仍在变化，不该判定为稳态", i)
		}
	}

	stats, recorded := feed(observer, stats, steadyWindow*2, 2000, 78, 70, 60)
	if recorded != 2 {
		t.Errorf("连续 %d 个稳定采样应产生 2 次观测，得到 %d", steadyWindow*2, recorded)
	}
	if len(stats.Cells) != 1 {
		t.Fatalf("同一工况应只占一格，得到 %d", len(stats.Cells))
	}
}

// 待机数据在任何转速下都一样凉，记进去只会用没有区分度的样本把格子填满。
func TestPassiveObserverIgnoresIdleAndUnreadableSamples(t *testing.T) {
	observer := NewPassiveObserver()
	stats := types.CoolingPassiveStats{}

	stats, recorded := feed(observer, stats, steadyWindow*3, 2000, 50, 45, 4)
	if recorded != 0 || len(stats.Cells) != 0 {
		t.Errorf("低功耗样本不该入库，记录了 %d 次", recorded)
	}

	// 读不到功耗（0W）同样跳过：无法归入功耗档，也就无法与其他转速比较。
	stats, recorded = feed(observer, stats, steadyWindow*3, 2000, 80, 75, 0)
	if recorded != 0 || len(stats.Cells) != 0 {
		t.Errorf("无功耗读数的样本不该入库，记录了 %d 次", recorded)
	}

	// 温度读不到时也不该记。
	stats, recorded = feed(observer, stats, steadyWindow*3, 2000, 0, 0, 60)
	if recorded != 0 || len(stats.Cells) != 0 {
		t.Errorf("无温度读数的样本不该入库，记录了 %d 次", recorded)
	}
}

func TestPassiveObserverSeparatesPowerBuckets(t *testing.T) {
	observer := NewPassiveObserver()
	stats := types.CoolingPassiveStats{}

	// 同一转速、两种负载：必须落进不同的功耗档，否则轻活会把重载的温度冲淡。
	stats, _ = feed(observer, stats, steadyWindow*2, 2200, 65, 60, 30)
	observer.Reset()
	stats, _ = feed(observer, stats, steadyWindow*2, 2200, 88, 80, 100)

	if len(stats.Cells) != 2 {
		t.Fatalf("不同功耗档应分开存放，得到 %d 格: %+v", len(stats.Cells), stats.Cells)
	}
	light, okLight := findCell(stats, PowerBucketOf(30), 2200)
	heavy, okHeavy := findCell(stats, PowerBucketOf(100), 2200)
	if !okLight || !okHeavy {
		t.Fatalf("两个功耗档都应存在: %+v", stats.Cells)
	}
	if light.CPUTemp >= heavy.CPUTemp {
		t.Errorf("重载格温度应更高: 轻载 %.1f vs 重载 %.1f", light.CPUTemp, heavy.CPUTemp)
	}
}

func TestPassiveObserverResetDropsPartialWindow(t *testing.T) {
	observer := NewPassiveObserver()
	stats := types.CoolingPassiveStats{}

	for range steadyWindow - 1 {
		stats, _ = observer.Observe(stats, 2000, 78, 70, 60)
	}
	observer.Reset()
	// 重置后必须重新攒满窗口才允许出结果。
	for i := range steadyWindow - 1 {
		var ok bool
		stats, ok = observer.Observe(stats, 2000, 78, 70, 60)
		if ok {
			t.Fatalf("重置后第 %d 拍就出结果，说明窗口没清干净", i+1)
		}
	}
}

// 只在同一功耗档内部比较转速，这是被动数据唯一站得住的用法。
func TestComparePassiveStaysWithinPowerBucket(t *testing.T) {
	observer := NewPassiveObserver()
	stats := types.CoolingPassiveStats{}

	// 同一重载下的低转速与高转速。
	for range MinCellSamples {
		observer.Reset()
		stats, _ = feed(observer, stats, steadyWindow, 1200, 90, 84, 100)
	}
	for range MinCellSamples {
		observer.Reset()
		stats, _ = feed(observer, stats, steadyWindow, 3400, 81, 75, 100)
	}
	// 另一个功耗档只有一格，不足以比较，不该产出结果。
	for range MinCellSamples {
		observer.Reset()
		stats, _ = feed(observer, stats, steadyWindow, 1200, 62, 58, 30)
	}

	comparisons := ComparePassive(stats)
	if len(comparisons) != 1 {
		t.Fatalf("只有重载档凑齐了两个转速格，期望 1 条对比，得到 %d: %+v", len(comparisons), comparisons)
	}
	got := comparisons[0]
	if got.PowerBucket != PowerBucketOf(100) {
		t.Errorf("对比应落在重载功耗档，得到 %d", got.PowerBucket)
	}
	if got.TempDelta >= 0 {
		t.Errorf("高转速档应更凉，得到温差 %.1f", got.TempDelta)
	}
	if got.Samples < MinCellSamples {
		t.Errorf("对比应报告较少一侧的样本数，得到 %d", got.Samples)
	}
}

func TestComparePassiveSkipsThinCells(t *testing.T) {
	stats := types.CoolingPassiveStats{Cells: []types.CoolingPassiveCell{
		{PowerBucket: 3, RPM: 1200, CPUTemp: 90, Samples: MinCellSamples - 1},
		{PowerBucket: 3, RPM: 3400, CPUTemp: 80, Samples: 40},
	}}
	if got := ComparePassive(stats); len(got) != 0 {
		t.Errorf("样本不足的格子不该参与对比，得到 %+v", got)
	}
}

func TestSanitizePassiveStatsDropsGarbage(t *testing.T) {
	stats := types.CoolingPassiveStats{Cells: []types.CoolingPassiveCell{
		{PowerBucket: 2, RPM: 2200, CPUTemp: 80, Samples: 10},
		{PowerBucket: -1, RPM: 2200, CPUTemp: 80, Samples: 10},        // 档位越界
		{PowerBucket: 99, RPM: 2200, CPUTemp: 80, Samples: 10},        // 档位越界
		{PowerBucket: 1, RPM: 0, CPUTemp: 70, Samples: 10},            // 转速非法
		{PowerBucket: 1, RPM: 1800, CPUTemp: math.NaN(), Samples: 10}, // NaN
		{PowerBucket: 1, RPM: 1800, CPUTemp: 70, Samples: 0},          // 无样本
	}}

	cleaned, changed := SanitizePassiveStats(stats)
	if !changed {
		t.Fatal("这份统计到处是非法值，应报告已修改")
	}
	if len(cleaned.Cells) != 1 {
		t.Fatalf("只有一格合法，得到 %d: %+v", len(cleaned.Cells), cleaned.Cells)
	}

	// 幂等：清洗过的数据再洗一遍不该继续报告修改，否则会引发反复写盘。
	if _, again := SanitizePassiveStats(cleaned); again {
		t.Error("清洗应当幂等")
	}
}

func TestSanitizePassiveStatsMergesDuplicateCells(t *testing.T) {
	stats := types.CoolingPassiveStats{Cells: []types.CoolingPassiveCell{
		{PowerBucket: 2, RPM: 2200, CPUTemp: 80, Samples: 4},
		{PowerBucket: 2, RPM: 2200, CPUTemp: 76, Samples: 20},
	}}
	cleaned, changed := SanitizePassiveStats(stats)
	if !changed || len(cleaned.Cells) != 1 {
		t.Fatalf("同一格重复应合并，得到 %+v", cleaned.Cells)
	}
	if cleaned.Cells[0].Samples != 20 {
		t.Errorf("合并应保留样本更多的一份，得到 %d", cleaned.Cells[0].Samples)
	}
}

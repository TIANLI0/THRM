package config

import (
	"sync"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func sampleConfig() types.AppConfig {
	return types.AppConfig{
		FanCurve: []types.FanCurvePoint{{Temperature: 40, RPM: 1000}, {Temperature: 70, RPM: 3000}},
		FanCurveProfiles: []types.FanCurveProfile{
			{ID: "p1", Name: "默认", Curve: []types.FanCurvePoint{{Temperature: 40, RPM: 1000}}},
		},
		CpuSensors:       []string{"core0", "core1"},
		ManualGearLevels: map[string]string{"gear1": "low"},
		ManualGearRPM:    map[string]map[string]int{"gear1": {"low": 1200}},
		LegionFnQ: types.LegionFnQConfig{
			ModeMapping: map[string]types.FanGearTarget{"quiet": {}},
		},
		LightStrip: types.LightStripConfig{
			Colors: []types.RGBColor{{R: 255}},
		},
		TimeCurveSchedule: types.TimeCurveScheduleConfig{
			Rules: []types.TimeCurveScheduleRule{{ID: "r1", Weekdays: []int{1, 2, 3}}},
		},
		SmartControl: types.SmartControlConfig{
			LearnedOffsets:          []int{10, 20},
			LearnedOffsetsHeat:      []int{1},
			LearnedOffsetsCool:      []int{2},
			LearnedRateHeat:         []int{3},
			LearnedRateCool:         []int{4},
			LearnedOffsetsByProfile: map[string][]int{"p1": {7, 8}},
			TargetTempByProfile:     map[string]int{"p1": 68},
			LearningBiasByProfile:   map[string]string{"p1": "balanced"},
			NoiseProfile:            []types.NoiseProfilePoint{{RPM: 2000, DB: 12.5}},
		},
	}
}

// TestGetReturnsDeepCopy 是 #9 的回归测试。
//
// Get() 过去返回浅拷贝：切片与映射字段与管理器内部状态共享底层存储，
// 调用方原地改写元素即可在没有任何同步的情况下改到内部状态。
func TestGetReturnsDeepCopy(t *testing.T) {
	m := NewManager(t.TempDir(), testLoggerStub{})
	m.Set(sampleConfig())

	got := m.Get()

	// 原地改写调用方拿到的每一个引用型字段。
	got.FanCurve[0].RPM = 9999
	got.FanCurveProfiles[0].Curve[0].RPM = 9999
	got.FanCurveProfiles[0].Name = "改过的"
	got.CpuSensors[0] = "改过的"
	got.ManualGearLevels["gear1"] = "改过的"
	got.ManualGearRPM["gear1"]["low"] = 9999
	got.LegionFnQ.ModeMapping["quiet"] = types.FanGearTarget{}
	got.LightStrip.Colors[0].R = 1
	got.TimeCurveSchedule.Rules[0].Weekdays[0] = 9
	got.SmartControl.LearnedOffsets[0] = 9999
	got.SmartControl.LearnedOffsetsHeat[0] = 9999
	got.SmartControl.LearnedOffsetsCool[0] = 9999
	got.SmartControl.LearnedRateHeat[0] = 9999
	got.SmartControl.LearnedRateCool[0] = 9999
	got.SmartControl.LearnedOffsetsByProfile["p1"][0] = 9999
	got.SmartControl.TargetTempByProfile["p1"] = 99
	got.SmartControl.LearningBiasByProfile["p1"] = "改过的"
	got.SmartControl.NoiseProfile[0].DB = 99

	fresh := m.Get()
	if fresh.FanCurve[0].RPM != 1000 {
		t.Error("FanCurve 与内部状态共享底层数组")
	}
	if fresh.FanCurveProfiles[0].Curve[0].RPM != 1000 {
		t.Error("FanCurveProfiles[].Curve 与内部状态共享底层数组")
	}
	if fresh.CpuSensors[0] != "core0" {
		t.Error("CpuSensors 与内部状态共享底层数组")
	}
	if fresh.ManualGearLevels["gear1"] != "low" {
		t.Error("ManualGearLevels 与内部状态共享同一映射")
	}
	if fresh.ManualGearRPM["gear1"]["low"] != 1200 {
		t.Error("ManualGearRPM 的内层映射被共享")
	}
	if len(fresh.LegionFnQ.ModeMapping) != 1 {
		t.Error("LegionFnQ.ModeMapping 被共享")
	}
	if fresh.LightStrip.Colors[0].R != 255 {
		t.Error("LightStrip.Colors 被共享")
	}
	if fresh.TimeCurveSchedule.Rules[0].Weekdays[0] != 1 {
		t.Error("TimeCurveSchedule.Rules[].Weekdays 被共享")
	}
	if fresh.SmartControl.LearnedOffsets[0] != 10 {
		t.Error("SmartControl.LearnedOffsets 被共享")
	}
	if fresh.SmartControl.LearnedOffsetsHeat[0] != 1 ||
		fresh.SmartControl.LearnedOffsetsCool[0] != 2 ||
		fresh.SmartControl.LearnedRateHeat[0] != 3 ||
		fresh.SmartControl.LearnedRateCool[0] != 4 {
		t.Error("SmartControl 的学习偏移切片被共享")
	}
	if fresh.SmartControl.LearnedOffsetsByProfile["p1"][0] != 7 {
		t.Error("LearnedOffsetsByProfile 的内层切片被共享")
	}
	if fresh.SmartControl.TargetTempByProfile["p1"] != 68 {
		t.Error("TargetTempByProfile 被共享")
	}
	if fresh.SmartControl.LearningBiasByProfile["p1"] != "balanced" {
		t.Error("LearningBiasByProfile 被共享")
	}
	if fresh.SmartControl.NoiseProfile[0].DB != 12.5 {
		t.Error("NoiseProfile 被共享")
	}
}

// TestSetCopiesInput 验证 Set 也切断共享：调用方在 Set 之后继续改写
// 自己手上的切片，不应间接改到管理器内部状态。
func TestSetCopiesInput(t *testing.T) {
	m := NewManager(t.TempDir(), testLoggerStub{})

	cfg := sampleConfig()
	m.Set(cfg)
	cfg.FanCurve[0].RPM = 9999
	cfg.SmartControl.LearnedOffsets[0] = 9999

	got := m.Get()
	if got.FanCurve[0].RPM != 1000 || got.SmartControl.LearnedOffsets[0] != 10 {
		t.Fatal("Set() 未拷贝入参，调用方仍能改到内部状态")
	}
}

// TestConcurrentGetAndSetNoRace 在 -race 下验证并发读写配置不产生数据竞争。
func TestConcurrentGetAndSetNoRace(t *testing.T) {
	m := NewManager(t.TempDir(), testLoggerStub{})
	m.Set(sampleConfig())

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				cfg := m.Get()
				// 读取方原地改写自己的副本，模拟真实调用模式。
				if len(cfg.FanCurve) > 0 {
					cfg.FanCurve[0].RPM++
				}
				if len(cfg.SmartControl.LearnedOffsets) > 0 {
					cfg.SmartControl.LearnedOffsets[0]++
				}
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range 200 {
				m.Set(sampleConfig())
			}
		})
	}
	wg.Wait()
}

type testLoggerStub struct{}

func (testLoggerStub) Info(string, ...any)  {}
func (testLoggerStub) Error(string, ...any) {}
func (testLoggerStub) Warn(string, ...any)  {}
func (testLoggerStub) Debug(string, ...any) {}
func (testLoggerStub) Close()               {}
func (testLoggerStub) CleanOldLogs()        {}
func (testLoggerStub) SetDebugMode(bool)    {}
func (testLoggerStub) GetLogDir() string    { return "" }

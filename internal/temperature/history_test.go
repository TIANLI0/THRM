package temperature

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func enableRecorderForTest(t *testing.T, recorder *HistoryRecorder) {
	t.Helper()
	if err := recorder.SetEnabled(true); err != nil {
		t.Fatalf("enable recorder: %v", err)
	}
}

func TestHistoryRecorderDefaultsEnabled(t *testing.T) {
	t.Parallel()

	recorder := NewHistoryRecorder(filepath.Join(t.TempDir(), "history.bin"), 8, 5*time.Second, nil)
	if !recorder.IsEnabled() {
		t.Fatal("expected history recorder to default enabled")
	}
}

func TestHistoryRecorderAddNormalizesSecondTimestamp(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)

	baseSeconds := int64(1_717_000_000)
	point, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:    61,
		GPUTemp:    58,
		UpdateTime: baseSeconds,
	}, &types.FanData{CurrentRPM: 1680})
	if !recorded {
		t.Fatal("expected first history point to be recorded")
	}
	if want := baseSeconds * 1000; point.Timestamp != want {
		t.Fatalf("expected normalized timestamp %d, got %d", want, point.Timestamp)
	}

	if _, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:    62,
		GPUTemp:    59,
		UpdateTime: baseSeconds + 1,
	}, &types.FanData{CurrentRPM: 1720}); recorded {
		t.Fatal("expected sample inside 5s window to be skipped")
	}

	if _, recorded := recorder.Add(types.TemperatureData{
		CPUTemp:    64,
		GPUTemp:    60,
		UpdateTime: baseSeconds + 5,
	}, &types.FanData{CurrentRPM: 1760}); !recorded {
		t.Fatal("expected sample at 5s boundary to be recorded")
	}
}

func TestHistoryRecorderPersistsBinarySnapshot(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)
	_, _ = recorder.Add(types.TemperatureData{CPUTemp: 60, GPUTemp: 54, UpdateTime: 1_717_000_000}, &types.FanData{CurrentRPM: 1500})
	_, _ = recorder.Add(types.TemperatureData{CPUTemp: 62, GPUTemp: 55, CPUPower: 47.4, GPUPower: 102.6, UpdateTime: 1_717_000_005}, &types.FanData{CurrentRPM: 1550})
	if err := recorder.Flush(); err != nil {
		t.Fatalf("flush binary history: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read binary history: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(historyBinaryMagic)) {
		t.Fatalf("expected binary history to start with %q", historyBinaryMagic)
	}

	reloaded := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	snapshot := reloaded.Snapshot()
	if len(snapshot.Points) != 2 {
		t.Fatalf("expected 2 reloaded points, got %d", len(snapshot.Points))
	}
	if snapshot.Points[1].FanRPM != 1550 {
		t.Fatalf("expected fan rpm 1550, got %d", snapshot.Points[1].FanRPM)
	}
	if snapshot.Points[1].CPUPower != 47.4 || snapshot.Points[1].GPUPower != 102.6 {
		t.Fatalf("expected persisted powers 47.4/102.6 W, got %.1f/%.1f W", snapshot.Points[1].CPUPower, snapshot.Points[1].GPUPower)
	}
}

func TestHistoryRecorderLoadsLegacyV1WithoutPower(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)

	data := make([]byte, 0, 48)
	data = append(data, historyBinaryMagic...)
	data = binary.LittleEndian.AppendUint16(data, historyBinaryVersionLegacy)
	data = append(data, historyEnabledFlag, 0)
	data = binary.LittleEndian.AppendUint32(data, 5)
	data = binary.LittleEndian.AppendUint32(data, 1)
	data = binary.LittleEndian.AppendUint64(data, uint64(1_717_000_000_000))
	data = binary.LittleEndian.AppendUint64(data, uint64(1_717_000_000_000))
	data = binary.LittleEndian.AppendUint32(data, uint32(61))
	data = binary.LittleEndian.AppendUint32(data, uint32(58))
	data = binary.LittleEndian.AppendUint32(data, uint32(1650))

	if err := recorder.loadBinaryData(data); err != nil {
		t.Fatalf("load legacy v1 history: %v", err)
	}
	points := recorder.Snapshot().Points
	if len(points) != 1 {
		t.Fatalf("expected one legacy point, got %d", len(points))
	}
	if points[0].CPUPower != 0 || points[0].GPUPower != 0 {
		t.Fatalf("legacy v1 power should be unavailable, got %.1f/%.1f", points[0].CPUPower, points[0].GPUPower)
	}
	if events := recorder.Snapshot().Events; len(events) != 0 {
		t.Fatalf("legacy v1 file carries no events, got %d", len(events))
	}
}

// 时间轴事件必须跨进程重启活下来：GUI 多数时间是关着的，"关着界面时断的线"
// 只有落盘才能在下次打开时补回图上。
func TestHistoryRecorderPersistsTimelineEvents(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "history.bin")
	recorder := NewHistoryRecorder(filePath, 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)

	if _, ok := recorder.AddEvent(types.TimelineEventTypeDisconnect, types.TimelineKeyDeviceDisconnected); !ok {
		t.Fatal("expected first event to be recorded")
	}
	if _, ok := recorder.AddEvent(types.TimelineEventTypeResume, types.TimelineKeyResumeFromSleep); !ok {
		t.Fatal("expected second event to be recorded")
	}
	if err := recorder.Flush(); err != nil {
		t.Fatalf("flush history: %v", err)
	}

	events := NewHistoryRecorder(filePath, 8, 5*time.Second, nil).Snapshot().Events
	if len(events) != 2 {
		t.Fatalf("expected 2 reloaded events, got %d", len(events))
	}
	if events[0].LabelKey != types.TimelineKeyDeviceDisconnected || events[0].Type != types.TimelineEventTypeDisconnect {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[1].LabelKey != types.TimelineKeyResumeFromSleep {
		t.Fatalf("unexpected second event: %+v", events[1])
	}
	if events[0].Timestamp <= 0 || events[1].Timestamp < events[0].Timestamp {
		t.Fatalf("events must carry ascending timestamps, got %d then %d", events[0].Timestamp, events[1].Timestamp)
	}
}

// Windows 唤醒会连发 PBT_APMRESUMESUSPEND 与 PBT_APMRESUMEAUTOMATIC，
// 两条通知必须折叠成一个标记，否则图上会叠出两条重合的参考线。
func TestHistoryRecorderDedupesRepeatedEvent(t *testing.T) {
	t.Parallel()

	recorder := NewHistoryRecorder(filepath.Join(t.TempDir(), "history.bin"), 8, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)

	if _, ok := recorder.AddEvent(types.TimelineEventTypeResume, types.TimelineKeyResumeFromSleep); !ok {
		t.Fatal("expected first resume event to be recorded")
	}
	if _, ok := recorder.AddEvent(types.TimelineEventTypeResume, types.TimelineKeyResumeFromSleep); ok {
		t.Fatal("expected duplicate resume event inside the dedupe window to be dropped")
	}
	// 不同事件不受去重影响。
	if _, ok := recorder.AddEvent(types.TimelineEventTypeDisconnect, types.TimelineKeyDeviceDisconnected); !ok {
		t.Fatal("expected a different event to be recorded")
	}
	if events := recorder.Snapshot().Events; len(events) != 2 {
		t.Fatalf("expected 2 stored events, got %d", len(events))
	}
}

// 关闭后台历史记录时事件不落盘，但仍要返回给调用方广播——那个开关管的是后台
// 常驻记录，不该让正开着界面的用户连本次会话的状态变化都看不到。
func TestHistoryRecorderEventsWhenDisabled(t *testing.T) {
	t.Parallel()

	recorder := NewHistoryRecorder(filepath.Join(t.TempDir(), "history.bin"), 8, 5*time.Second, nil)
	if err := recorder.SetEnabled(false); err != nil {
		t.Fatalf("disable recorder: %v", err)
	}

	if _, ok := recorder.AddEvent(types.TimelineEventTypeDisconnect, types.TimelineKeyDeviceDisconnected); !ok {
		t.Fatal("expected event to still be reported for live broadcast while disabled")
	}
	if events := recorder.Snapshot().Events; len(events) != 0 {
		t.Fatalf("expected no persisted events while disabled, got %d", len(events))
	}
}

// 事件跟采样点共享保留窗口：早于最旧曲线数据的标记在图上无处可标，必须丢弃。
func TestHistoryRecorderPrunesEventsOutsideRetention(t *testing.T) {
	t.Parallel()

	recorder := NewHistoryRecorder(filepath.Join(t.TempDir(), "history.bin"), 4, 5*time.Second, nil)
	enableRecorderForTest(t, recorder)

	now := time.Now().UnixMilli()
	retention := int64(4) * (5 * time.Second).Milliseconds() // capacity * sampleInterval = 20s
	recorder.events = []types.TimelineEvent{
		{Timestamp: now - retention - 5_000, Type: types.TimelineEventTypeMode, LabelKey: types.TimelineKeyCoreStarted},
		{Timestamp: now - 1_000, Type: types.TimelineEventTypeMode, LabelKey: types.TimelineKeyDeviceConnected},
	}
	recorder.pruneEventsLocked(now)

	if len(recorder.events) != 1 {
		t.Fatalf("expected the out-of-window event to be pruned, got %d events", len(recorder.events))
	}
	if recorder.events[0].LabelKey != types.TimelineKeyDeviceConnected {
		t.Fatalf("pruned the wrong event: %+v", recorder.events[0])
	}
}

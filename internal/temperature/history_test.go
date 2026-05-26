package temperature

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
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
	_, _ = recorder.Add(types.TemperatureData{CPUTemp: 62, GPUTemp: 55, UpdateTime: 1_717_000_005}, &types.FanData{CurrentRPM: 1550})
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
}

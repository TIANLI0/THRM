package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestLatestControlTempPrefersControlTemp(t *testing.T) {
	app := &CoreApp{}
	app.currentTemp = types.TemperatureData{ControlTemp: 61, MaxTemp: 74}
	if got := app.latestControlTemp(); got != 61 {
		t.Fatalf("latestControlTemp() = %d, want 61", got)
	}
}

func TestLatestControlTempFallsBackToMaxTemp(t *testing.T) {
	app := &CoreApp{}
	app.currentTemp = types.TemperatureData{MaxTemp: 74}
	if got := app.latestControlTemp(); got != 74 {
		t.Fatalf("latestControlTemp() = %d, want 74", got)
	}
}

func TestAwaitFirstControlTempReturnsImmediatelyWhenReady(t *testing.T) {
	app := &CoreApp{}
	app.currentTemp = types.TemperatureData{ControlTemp: 55}

	start := time.Now()
	app.awaitFirstControlTemp(2 * time.Second)
	if elapsed := time.Since(start); elapsed > startupControlTempPollInterval {
		t.Fatalf("已有有效温度时不应等待，实际等待 %v", elapsed)
	}
}

func TestAwaitFirstControlTempStopsAtDeadline(t *testing.T) {
	app := &CoreApp{}

	start := time.Now()
	app.awaitFirstControlTemp(200 * time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 200*time.Millisecond {
		t.Fatalf("等待提前结束: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("等待超过预期上限: %v", elapsed)
	}
}

func TestAwaitFirstControlTempStopsWhenSuspended(t *testing.T) {
	app := &CoreApp{}
	app.systemSuspended.Store(true)

	start := time.Now()
	app.awaitFirstControlTemp(5 * time.Second)
	if elapsed := time.Since(start); elapsed > startupControlTempPollInterval {
		t.Fatalf("挂起时应立即放弃等待，实际等待 %v", elapsed)
	}
}

func TestAwaitFirstControlTempWaitsForFirstReading(t *testing.T) {
	app := &CoreApp{}

	go func() {
		time.Sleep(300 * time.Millisecond)
		app.mutex.Lock()
		app.currentTemp = types.TemperatureData{ControlTemp: 48}
		app.mutex.Unlock()
	}()

	start := time.Now()
	app.awaitFirstControlTemp(5 * time.Second)
	elapsed := time.Since(start)
	if app.latestControlTemp() != 48 {
		t.Fatal("等待结束时应已读到温度")
	}
	if elapsed >= 5*time.Second {
		t.Fatalf("读到温度后应立即返回，实际等待 %v", elapsed)
	}
}

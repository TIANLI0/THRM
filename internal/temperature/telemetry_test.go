package temperature

import (
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	now atomic.Int64
}

func (c *fakeClock) set(t time.Time) { c.now.Store(t.UnixNano()) }
func (c *fakeClock) advance(d time.Duration) {
	c.now.Store(c.now.Load() + int64(d))
}
func (c *fakeClock) get() time.Time { return time.Unix(0, c.now.Load()) }

func withFakeClock(t *testing.T) *fakeClock {
	t.Helper()
	clock := &fakeClock{}
	clock.set(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	oldNow := readTimeNow
	readTimeNow = clock.get
	t.Cleanup(func() { readTimeNow = oldNow })
	return clock
}

func fakeExpensiveTelemetry(reading telemetryReading) (platformTelemetryReader, *atomic.Int64) {
	var calls atomic.Int64
	reader := func(_ *Reader, disableGPU bool) telemetryReading {
		if disableGPU {
			return telemetryReading{}
		}
		calls.Add(1)
		return reading
	}
	return reader, &calls
}

func TestExternalTelemetryThrottlesReads(t *testing.T) {
	clock := withFakeClock(t)
	reader, calls := fakeExpensiveTelemetry(telemetryReading{})
	r := NewReader(nil, testLogger{})

	const ticks = 30
	const tickInterval = 2 * time.Second
	for range ticks {
		r.readNativeTelemetryWith(false, reader, true)
		clock.advance(tickInterval)
	}

	if reads := calls.Load(); reads > 8 {
		t.Fatalf("降级路径在 %d 个采样周期内采样了 %d 次，节流未生效", ticks, reads)
	}
}

func TestExternalTelemetryReusesCacheWithinFreshWindow(t *testing.T) {
	clock := withFakeClock(t)
	reader, calls := fakeExpensiveTelemetry(telemetryReading{gpuTemp: 71, gpuPower: 123.45})
	r := NewReader(nil, testLogger{})

	first := r.readNativeTelemetryWith(false, reader, true)
	if first.gpuTemp != 71 || first.gpuPower != 123.45 {
		t.Fatalf("首次读取 = %+v", first)
	}
	afterFirst := calls.Load()

	clock.advance(externalFreshInterval - time.Second)
	second := r.readNativeTelemetryWith(false, reader, true)
	if calls.Load() != afterFirst {
		t.Fatalf("刷新窗口内又执行了 %d 次采样", calls.Load()-afterFirst)
	}
	if second.gpuTemp != first.gpuTemp || second.gpuPower != first.gpuPower {
		t.Fatalf("缓存读数 = %+v, want %+v", second, first)
	}

	clock.advance(2 * time.Second)
	r.readNativeTelemetryWith(false, reader, true)
	if calls.Load() <= afterFirst {
		t.Fatal("超过刷新窗口后未重新采样")
	}
}

func TestExternalCPUSkippedWhenNotNeeded(t *testing.T) {
	withFakeClock(t)
	reader, calls := fakeExpensiveTelemetry(telemetryReading{gpuTemp: 71, gpuPower: 30})
	r := NewReader(nil, testLogger{})

	got := r.readExternalTelemetryThrottled(false, true, false, reader)
	if calls.Load() == 0 {
		t.Fatal("未读取 GPU")
	}
	if got.gpuTemp != 71 {
		t.Fatalf("GPU 温度 = %d, want 71", got.gpuTemp)
	}
}

func TestDisableGpuSkipsGPUReader(t *testing.T) {
	withFakeClock(t)
	reader, calls := fakeExpensiveTelemetry(telemetryReading{gpuTemp: 71, gpuPower: 30})
	r := NewReader(nil, testLogger{})

	r.readNativeTelemetryWith(true, reader, true)
	if calls.Load() != 0 {
		t.Fatalf("停用 GPU 时仍读取了 %d 次 GPU", calls.Load())
	}
}

func TestExternalTelemetryBacksOffWhenUseless(t *testing.T) {
	clock := withFakeClock(t)
	reader, _ := fakeExpensiveTelemetry(telemetryReading{})
	r := NewReader(nil, testLogger{})

	r.readNativeTelemetryWith(false, reader, true)
	if got := r.externalThrottle.interval; got != externalBackoffStart {
		t.Fatalf("首次无效读取后间隔 = %s, want %s", got, externalBackoffStart)
	}
	previous := r.externalThrottle.interval
	for range 5 {
		clock.advance(previous + time.Second)
		r.readNativeTelemetryWith(false, reader, true)
		if r.externalThrottle.interval < previous {
			t.Fatalf("退避间隔回退: %s -> %s", previous, r.externalThrottle.interval)
		}
		previous = r.externalThrottle.interval
	}
	if previous != externalMaxInterval {
		t.Fatalf("退避未封顶到 %s, 实际 %s", externalMaxInterval, previous)
	}
}

func TestExternalTelemetryRefreshesWhenGpuSelectionChanges(t *testing.T) {
	withFakeClock(t)
	reader, calls := fakeExpensiveTelemetry(telemetryReading{gpuTemp: 71, gpuPower: 123.45})
	r := NewReader(nil, testLogger{})

	disabled := r.readNativeTelemetryWith(true, reader, true)
	if disabled.gpuTemp != 0 {
		t.Fatalf("停用 GPU 时仍读到 GPU 温度: %+v", disabled)
	}
	afterDisabled := calls.Load()

	enabled := r.readNativeTelemetryWith(false, reader, true)
	if calls.Load() <= afterDisabled {
		t.Fatal("切换 GPU 选项后未强制重新采样")
	}
	if enabled.gpuTemp != 71 {
		t.Fatalf("切回启用 GPU 后 = %+v, want gpuTemp=71", enabled)
	}
}

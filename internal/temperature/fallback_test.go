package temperature

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock 让降级节流的时间推进可控。
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

// countingExec 替换外部命令执行，统计进程创建次数。
func countingExec(t *testing.T, output string) *atomic.Int64 {
	t.Helper()
	var calls atomic.Int64

	oldExec := execHelperCommand
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		calls.Add(1)
		return []byte(output), nil
	}
	t.Cleanup(func() { execHelperCommand = oldExec })
	return &calls
}

// TestFallbackThrottlesProcessSpawns 是 #5 的核心回归测试。
//
// 修复前：桥接故障期间每个采样周期都会执行降级读取，按默认 2s 采样率意味着
// 每 2 秒创建 wmic + nvidia-smi(温度) + nvidia-smi(功耗) 三个进程。
// 修复后：降级读取自带刷新节流，30 个采样周期内只应真正采样很少几次。
func TestFallbackThrottlesProcessSpawns(t *testing.T) {
	clock := withFakeClock(t)
	// 返回无法解析的输出，模拟"降级路径也拿不到数据"这一最常见的故障态。
	calls := countingExec(t, "N/A\n")

	r := NewReader(nil, testLogger{})

	const ticks = 30
	const tickInterval = 2 * time.Second
	for range ticks {
		r.readFallback(false)
		clock.advance(tickInterval)
	}

	spawns := calls.Load()
	// 60 秒窗口内：首次 + 15s 退避 + 30s 退避 + 60s 封顶，采样次数应为个位数。
	// 每次采样最多 2 个进程（wmic 与合并后的 nvidia-smi），故上限取 8。
	if spawns > 8 {
		t.Fatalf("降级路径在 %d 个采样周期内创建了 %d 次进程，节流未生效", ticks, spawns)
	}
	t.Logf("30 个采样周期(60s)内降级路径进程创建次数: %d（修复前为每周期 2~3 次，约 %d 次）",
		spawns, ticks*3)
}

// TestFallbackReusesCacheWithinFreshWindow 验证有效读数在刷新窗口内被复用。
func TestFallbackReusesCacheWithinFreshWindow(t *testing.T) {
	clock := withFakeClock(t)
	calls := countingExec(t, "71, 123.45\n")

	r := NewReader(nil, testLogger{})
	// 预置厂商缓存，避免 detectGPUVendor 的探测调用干扰计数。
	r.cachedGPUVendor = "nvidia"
	r.cachedVendorAt = clock.get()

	first := r.readFallback(false)
	if first.gpuTemp != 71 || first.gpuPower != 123.45 {
		t.Fatalf("首次读取 = %+v, want gpuTemp=71 gpuPower=123.45", first)
	}
	afterFirst := calls.Load()

	// 刷新窗口内重复读取不应再起进程。
	clock.advance(fallbackFreshInterval - time.Second)
	second := r.readFallback(false)
	if calls.Load() != afterFirst {
		t.Fatalf("刷新窗口内又执行了 %d 次外部命令", calls.Load()-afterFirst)
	}
	if second != first {
		t.Fatalf("缓存读数 = %+v, want %+v", second, first)
	}

	// 超过刷新窗口后应重新采样。
	clock.advance(2 * time.Second)
	r.readFallback(false)
	if calls.Load() <= afterFirst {
		t.Fatal("超过刷新窗口后未重新采样")
	}
}

// TestExternalCPUSkippedWhenNotNeeded 验证廉价读取已拿到 CPU 温度时，
// 不会再去创建 wmic 进程。
//
// 节流只针对需要起外部进程的部分：进程内的廉价读取（gopsutil 传感器、
// Linux sysfs）必须每次取新值，否则在没有硬件桥接的平台上——那里降级路径
// 就是主读取路径——控温会依据 5 秒前的旧温度。
func TestExternalCPUSkippedWhenNotNeeded(t *testing.T) {
	withFakeClock(t)

	var wmicCalls, nvidiaCalls int
	oldExec := execHelperCommand
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		switch name {
		case "wmic":
			wmicCalls++
		case "nvidia-smi":
			nvidiaCalls++
		}
		return []byte("71, 30.0\n"), nil
	}
	t.Cleanup(func() { execHelperCommand = oldExec })

	r := NewReader(nil, testLogger{})
	r.cachedGPUVendor = "nvidia"
	r.cachedVendorAt = readTimeNow()

	// needCPU=false 表示廉价路径已取到 CPU 温度。
	got := r.readThrottledExternal(false, true, false)
	if wmicCalls != 0 {
		t.Fatalf("已有 CPU 温度时仍调用了 wmic %d 次", wmicCalls)
	}
	if nvidiaCalls == 0 {
		t.Fatal("未读取 GPU")
	}
	if got.gpuTemp != 71 {
		t.Fatalf("GPU 温度 = %d, want 71", got.gpuTemp)
	}
}

// TestDisableGpuSkipsNvidiaSmi 验证停用 GPU 监测时完全不碰 nvidia-smi，
// 避免轮询唤醒独显。
func TestDisableGpuSkipsNvidiaSmi(t *testing.T) {
	withFakeClock(t)
	calls := countingExec(t, "71, 30.0\n")

	r := NewReader(nil, testLogger{})
	r.readFallback(true)

	// 停用 GPU 且廉价路径没结果时，最多只应有 CPU 那条外部命令。
	if calls.Load() > 1 {
		t.Fatalf("停用 GPU 时执行了 %d 次外部命令，应至多 1 次(CPU)", calls.Load())
	}
}

// TestFallbackBacksOffWhenUseless 验证连续无效读数会逐级退避。
func TestFallbackBacksOffWhenUseless(t *testing.T) {
	clock := withFakeClock(t)
	countingExec(t, "N/A\n")

	r := NewReader(nil, testLogger{})

	r.readFallback(false)
	if got := r.fallback.interval; got != fallbackBackoffStart {
		t.Fatalf("首次无效读取后间隔 = %s, want %s", got, fallbackBackoffStart)
	}

	// 逐级退避直到封顶。
	previous := r.fallback.interval
	for range 5 {
		clock.advance(previous + time.Second)
		r.readFallback(false)
		if r.fallback.interval < previous {
			t.Fatalf("退避间隔回退: %s -> %s", previous, r.fallback.interval)
		}
		previous = r.fallback.interval
	}
	if previous != fallbackMaxInterval {
		t.Fatalf("退避未封顶到 %s, 实际 %s", fallbackMaxInterval, previous)
	}
}

// TestFallbackRefreshesWhenGpuSelectionChanges 验证切换"停用 GPU"后强制重新采样，
// 不会把停用期间缓存的空 GPU 读数一直沿用。
func TestFallbackRefreshesWhenGpuSelectionChanges(t *testing.T) {
	clock := withFakeClock(t)
	calls := countingExec(t, "71, 123.45\n")

	r := NewReader(nil, testLogger{})
	r.cachedGPUVendor = "nvidia"
	r.cachedVendorAt = clock.get()

	disabled := r.readFallback(true)
	if disabled.gpuTemp != 0 {
		t.Fatalf("停用 GPU 时仍读到 GPU 温度: %+v", disabled)
	}
	afterDisabled := calls.Load()

	// 立刻切回启用 GPU：即便还在刷新窗口内，也必须重新采样。
	enabled := r.readFallback(false)
	if calls.Load() <= afterDisabled {
		t.Fatal("切换 GPU 选项后未强制重新采样")
	}
	if enabled.gpuTemp != 71 {
		t.Fatalf("切回启用 GPU 后 = %+v, want gpuTemp=71", enabled)
	}
}

package bridge

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

// TestEnsureRunningIsNonBlocking 是 #6 的回归测试。
//
// 修复前 EnsureRunning 在持有 m.mutex 的状态下完成启动握手（最长 30 秒），
// 而温控循环每个周期都要经由 GetTemperature 抢同一把锁。后果是：控制循环停摆
// 几十秒且期间无法响应 stop；挂起清理的 2 秒宽限被这把锁耗尽，于是带着未关闭的
// HID 句柄进入睡眠——正是代码本意要避免的唤醒崩溃场景。
func TestEnsureRunningIsNonBlocking(t *testing.T) {
	m := NewManager(testLogger{})

	start := time.Now()
	err := m.EnsureRunning()
	elapsed := time.Since(start)

	if !errors.Is(err, ErrStarting) {
		t.Fatalf("EnsureRunning() 首次调用应立即返回 ErrStarting，实际: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("EnsureRunning() 阻塞了 %v，启动握手仍在调用方协程上同步执行", elapsed)
	}
	t.Logf("EnsureRunning() 在 %v 内返回 ErrStarting（修复前会阻塞到握手结束，最长 %s）",
		elapsed.Round(time.Millisecond), bridgeStartupTimeout)
}

// TestStatusStaysResponsiveDuringStart 验证启动期间状态查询不被阻塞。
func TestStatusStaysResponsiveDuringStart(t *testing.T) {
	m := NewManager(testLogger{})
	_ = m.EnsureRunning()

	start := time.Now()
	status := m.GetStatus()
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("GetStatus() 在启动期间阻塞了 %v", elapsed)
	}
	if status["state"] == nil {
		t.Fatal("GetStatus() 未返回 state")
	}
}

// TestGetStatusSendsNoCommand 是 #7 的回归测试。
//
// GetStatus 过去会真的发一次 GetTemperature 来填充 working/testData。前端恰恰是在
// 检测到 bridgeOk == false 时才来查状态的，此时那次读取必然走满 10 秒超时，
// 而服务端按连接串行处理请求，这 10 秒会把同一连接上后续请求全部拖超时。
func TestGetStatusSendsNoCommand(t *testing.T) {
	m := NewManager(testLogger{})

	start := time.Now()
	status := m.GetStatus()
	elapsed := time.Since(start)

	// 桥接从未启动：若 GetStatus 仍会尝试通信，就会先触发一次启动并等待握手。
	if elapsed > 500*time.Millisecond {
		t.Fatalf("GetStatus() 耗时 %v，疑似仍在主动向桥接发命令", elapsed)
	}
	if m.IsStarting() {
		t.Fatal("GetStatus() 不应触发桥接启动")
	}
	// 从未成功读取过时不应伪造诊断数据。
	if _, ok := status["testData"]; ok {
		t.Fatal("从未读取成功时不应带 testData")
	}
	t.Logf("GetStatus() 在 %v 内完成且未发起任何桥接通信", elapsed.Round(time.Millisecond))
}

// TestConcurrentEnsureRunningStartsOnce 验证并发调用只触发一次启动。
func TestConcurrentEnsureRunningStartsOnce(t *testing.T) {
	m := NewManager(testLogger{})
	var starts atomic.Int32
	release := make(chan struct{})
	m.startProcess = func() error {
		starts.Add(1)
		<-release
		return errors.New("test launch failed")
	}
	defer func() {
		close(release)
		waitStartFinished(t, m)
	}()

	done := make(chan error, 16)
	for range 16 {
		go func() { done <- m.EnsureRunning() }()
	}

	for range 16 {
		if err := <-done; !errors.Is(err, ErrStarting) {
			t.Fatalf("并发 EnsureRunning() 返回了非 ErrStarting: %v", err)
		}
	}
	// All callers have returned while the injected launch remains pending.
	// The real executable may fail before other goroutines even run on CI.
	deadline := time.After(time.Second)
	for starts.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("background launch did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("started %d processes, want 1", got)
	}
}

// TestIsStartingMessageRecognised 验证过渡态文案可被上层识别。
// 上层据此避免把"正在启动"当成故障去触发自愈重启或风扇安全兜底。
func TestIsStartingMessageRecognised(t *testing.T) {
	if !IsStarting(StartingMessage) {
		t.Fatal("IsStarting 应识别 StartingMessage")
	}
	if !IsStarting("桥接程序通信失败: " + StartingMessage) {
		t.Fatal("IsStarting 应识别被包裹的过渡态文案")
	}
	if IsStarting("桥接程序通信失败: 读取响应超时") {
		t.Fatal("IsStarting 不应把真实故障判为过渡态")
	}
	if IsStarting("") {
		t.Fatal("IsStarting 不应把空信息判为过渡态")
	}
}

func waitStartFinished(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for m.IsStarting() {
		if time.Now().After(deadline) {
			t.Fatal("后台启动未在 5 秒内结束")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestEnsureRunningReportsFailureAfterFailedStart 验证启动失败后不再冒充"正在启动"。
//
// 修复前 EnsureRunning 对失败的启动同样返回 ErrStarting：温度读取因此一直被当作过渡态，
// 既不走内置传感器降级，也不计入温度失效，风扇安全兜底永远不会触发；
// 而且每个采样周期都会重新拉起一次子进程。
func TestEnsureRunningReportsFailureAfterFailedStart(t *testing.T) {
	m := NewManager(testLogger{})
	if err := m.EnsureRunning(); !errors.Is(err, ErrStarting) {
		t.Fatalf("首次调用应返回 ErrStarting，实际: %v", err)
	}
	// 测试二进制旁边没有桥接程序，启动必然失败。
	waitStartFinished(t, m)

	err := m.EnsureRunning()
	if err == nil || errors.Is(err, ErrStarting) || IsStarting(err.Error()) {
		t.Fatalf("启动失败后应返回真实原因，实际: %v", err)
	}
	if m.IsStarting() {
		t.Fatal("退避期内不应再次拉起桥接进程")
	}

	data := m.GetTemperature(types.TemperatureSelection{})
	if data.Success || IsStarting(data.Error) {
		t.Fatalf("GetTemperature 不应再上报过渡态，实际: %+v", data)
	}
}

func TestBridgeStartBackoffGrowsAndCaps(t *testing.T) {
	if got := bridgeStartBackoff(1); got != bridgeStartRetryBase {
		t.Fatalf("首次失败退避 = %v，期望 %v", got, bridgeStartRetryBase)
	}
	if got := bridgeStartBackoff(2); got != 2*bridgeStartRetryBase {
		t.Fatalf("第二次失败退避 = %v，期望 %v", got, 2*bridgeStartRetryBase)
	}
	if got := bridgeStartBackoff(100); got != bridgeStartRetryMax {
		t.Fatalf("退避上限 = %v，期望 %v", got, bridgeStartRetryMax)
	}
}

// TestHoldSuppressesAutoStart 验证 Hold 期间不会被温控循环重新拉起。
func TestHoldSuppressesAutoStart(t *testing.T) {
	m := NewManager(testLogger{})
	m.Hold()
	if err := m.EnsureRunning(); !errors.Is(err, ErrHeld) {
		t.Fatalf("Hold 期间应返回 ErrHeld，实际: %v", err)
	}
	if m.IsStarting() {
		t.Fatal("Hold 期间不应拉起桥接进程")
	}

	m.Release()
	if err := m.EnsureRunning(); !errors.Is(err, ErrStarting) {
		t.Fatalf("Release 后应重新开始启动，实际: %v", err)
	}
	waitStartFinished(t, m)
}

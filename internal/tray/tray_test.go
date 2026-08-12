package tray

import (
	"testing"
	"time"
)

type testLogger struct{}

func (l testLogger) Info(string, ...any)  {}
func (l testLogger) Error(string, ...any) {}
func (l testLogger) Warn(string, ...any)  {}
func (l testLogger) Debug(string, ...any) {}
func (l testLogger) Close()               {}
func (l testLogger) CleanOldLogs()        {}
func (l testLogger) SetDebugMode(bool)    {}
func (l testLogger) GetLogDir() string    { return "" }

func TestWaitForShellReady_ReturnsTrue(t *testing.T) {
	if !waitForShellReady(nil, time.Second) {
		t.Fatal("waitForShellReady should always return true on non-Windows")
	}
}

func TestWaitForTraySettle_NoPanic(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("waitForTraySettle panicked: %v", r)
			}
		}()
		waitForTraySettle(make(chan struct{}), time.Millisecond, time.Second)
	}()
}

// 注册时通知区域为 0（非 Windows，或注册那一刻通知区域已消失）时监视协程直接退出，
// 不能空转，也不能因为"当前句柄 != 0"就误判成重建。
func TestWatchNotifyAreaRebuild_ReturnsWithoutHandle(t *testing.T) {
	m := NewManager(testLogger{}, []byte{0x89, 0x50, 0x4e, 0x47})
	done := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		m.watchNotifyAreaRebuild(done, 0)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("watchNotifyAreaRebuild 在没有注册句柄时应立即返回")
	}
}

// 实例停止信号一到，监视协程必须退出，否则每次托盘重建都会多留一个协程。
func TestWatchNotifyAreaRebuild_StopsOnInstanceDone(t *testing.T) {
	m := NewManager(testLogger{}, []byte{0x89, 0x50, 0x4e, 0x47})
	instanceDone := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		m.watchNotifyAreaRebuild(instanceDone, 0x1234)
		close(finished)
	}()

	close(instanceDone)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("watchNotifyAreaRebuild 未响应实例停止信号")
	}
}

func TestNewManager_CreatesInstance(t *testing.T) {
	iconData := []byte{0x89, 0x50, 0x4e, 0x47}
	m := NewManager(testLogger{}, iconData)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.logger == nil {
		t.Fatal("logger should not be nil")
	}
	if m.done == nil {
		t.Fatal("done channel should not be nil")
	}
	if m.uiQueue == nil {
		t.Fatal("uiQueue should not be nil")
	}
	if len(m.iconData) != len(iconData) {
		t.Fatal("iconData length mismatch")
	}
	if m.curveMenuItems == nil {
		t.Fatal("curveMenuItems should not be nil")
	}
}

func TestManager_IsReady_NotInitially(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if m.IsReady() {
		t.Fatal("IsReady should return false before Init")
	}
}

func TestManager_IsInitialized_NotInitially(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if m.IsInitialized() {
		t.Fatal("IsInitialized should return false before Init")
	}
}

// 托盘默认可见；SetEnabled 在 Init 之前调用也必须生效，否则关掉托盘的用户
// 每次启动都会先闪出一个图标。
func TestManager_SetEnabled_TogglesVisibility(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	if !m.IsEnabled() {
		t.Fatal("托盘默认应为启用状态")
	}

	m.SetEnabled(false)
	if m.IsEnabled() {
		t.Fatal("SetEnabled(false) 之后应报告为已关闭")
	}
	// 重复关闭不应堆积唤醒信号，否则重新启用后监督协程会多跑一轮。
	m.SetEnabled(false)
	if len(m.enableCh) != 0 {
		t.Fatal("关闭托盘不应写入唤醒信号")
	}

	m.SetEnabled(true)
	if !m.IsEnabled() {
		t.Fatal("SetEnabled(true) 之后应报告为已启用")
	}
	if len(m.enableCh) != 1 {
		t.Fatal("重新启用托盘应唤醒监督协程")
	}
}

// 关闭状态下的托盘不是"坏了"：健康检查必须放行，不能触发重建。
func TestManager_CheckHealth_SkipsWhileDisabled(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	m.initialized = 1 // 模拟 Init 之后的状态
	m.SetEnabled(false)

	m.CheckHealth()

	if m.lastRestartTry.Load() != 0 {
		t.Fatal("关闭状态的托盘不应触发重建")
	}
}

// waitForEnable 必须响应进程退出信号，否则监督协程在关闭托盘时永远退不出来。
func TestManager_WaitForEnable_ReturnsOnDone(t *testing.T) {
	m := NewManager(testLogger{}, nil)
	m.SetEnabled(false)

	result := make(chan bool, 1)
	go func() { result <- m.waitForEnable() }()

	close(m.done)
	select {
	case enabled := <-result:
		if enabled {
			t.Fatal("进程退出时 waitForEnable 应返回 false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForEnable 未响应进程退出信号")
	}
}

func TestStatusEqual(t *testing.T) {
	base := Status{
		Connected:            true,
		CPUTemp:              65,
		GPUTemp:              70,
		CurrentRPM:           1800,
		AutoControlState:     true,
		ActiveCurveProfileID: "balanced",
		CurveProfiles:        []CurveOption{{ID: "balanced", Name: "Balanced"}},
	}
	if !statusEqual(base, base) {
		t.Fatal("equal tray status was treated as changed")
	}

	changed := base
	changed.CurrentRPM++
	if statusEqual(base, changed) {
		t.Fatal("changed fan speed was treated as equal")
	}

	changed = base
	changed.CurveProfiles = []CurveOption{{ID: "balanced", Name: "Quiet"}}
	if statusEqual(base, changed) {
		t.Fatal("changed curve menu entry was treated as equal")
	}
}

package coreapp

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTickAfterHandlesWraparound(t *testing.T) {
	tests := []struct {
		name string
		a, b uint32
		want bool
	}{
		{"later tick", 2000, 1000, true},
		{"same tick is not after", 1000, 1000, false},
		{"earlier tick", 1000, 2000, false},
		{"after GetTickCount wraps", 50, 0xFFFFFF00, true},
		{"before the wrap", 0xFFFFFF00, 50, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickAfter(tc.a, tc.b); got != tc.want {
				t.Fatalf("tickAfter(%d, %d) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// 挂起后进程被零星调度（现代待机、无人值守的自动唤醒）时，计时间隔变长不能被当成
// 用户回来了：那会重连设备、启动智能控温，风扇在睡眠中转起来。
func TestGapWhileSuspendedWithoutUserInputDoesNotRecover(t *testing.T) {
	app := &CoreApp{}
	app.systemSuspended.Store(true)
	// 没有有效的挂起时刻，等价于"无法确认用户输入"。
	app.hasSuspendInputTick.Store(false)

	if !app.maybeRecoverFromSystemResume("health-monitor", 10*time.Minute, 30*time.Second) {
		t.Fatal("挂起期间的长间隔应被吞掉（返回 true 让调用方跳过本轮），而不是交给常规处理")
	}
	if atomic.LoadInt64(&app.lastResumeRecoveryUnix) != 0 || app.resumeRecoveryRunning.Load() {
		t.Fatal("用户尚未操作时不应启动唤醒恢复")
	}
	if !app.systemSuspended.Load() {
		t.Fatal("挂起标记被清掉了：之后的温控与重连会在睡眠中运行")
	}
}

// PBT_APMRESUMEAUTOMATIC 单独到达时不能立即恢复设备控制。
func TestAutomaticResumeDefersRecoveryWhileSuspended(t *testing.T) {
	app := &CoreApp{}
	app.systemSuspended.Store(true)
	app.stopping.Store(true) // 让观察协程第一次检查就退出，避免测试里真的计时
	before := app.automaticResumeGeneration.Load()

	app.onSystemResume(false)

	if atomic.LoadInt64(&app.lastResumeRecoveryUnix) != 0 || app.resumeRecoveryRunning.Load() {
		t.Fatal("自动唤醒不应立即触发唤醒恢复")
	}
	if !app.systemSuspended.Load() {
		t.Fatal("自动唤醒后仍应保持挂起状态，直到用户出现")
	}
	if app.automaticResumeGeneration.Load() == before {
		t.Fatal("自动唤醒应开启一轮新的等待（递增 generation）")
	}
}

// 再次挂起必须让上一轮"等待用户"的观察协程失效，即使 systemSuspended 仍为 true、
// onSystemSuspend 的其余部分被 CAS 跳过。
func TestSuspendAfterAutomaticResumeInvalidatesWaiter(t *testing.T) {
	app := &CoreApp{}
	app.systemSuspended.Store(true)
	gen := app.automaticResumeGeneration.Add(1)

	app.onSystemSuspend()

	if app.automaticResumeGeneration.Load() == gen {
		t.Fatal("再次挂起没有让自动唤醒的等待失效，它可能在睡眠中到点恢复设备")
	}
}

// 普通睡眠（没有自动唤醒）期间的 HID 到达通知不能去连设备压风扇：那会和挂起清理撞车。
func TestQuietHoldNotWantedOutsideAutomaticWake(t *testing.T) {
	app := &CoreApp{}
	app.systemSuspended.Store(true)
	app.resumeReconnectWanted.Store(true)
	gen := app.automaticResumeGeneration.Load()

	if app.automaticWakeQuietHoldWanted(gen) {
		t.Fatal("没有进行中的自动唤醒，却判定需要压风扇")
	}
}

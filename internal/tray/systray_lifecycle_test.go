package tray

import (
	"sync/atomic"
	"testing"
)

// TestSystrayBudgetExhausted 覆盖托盘重建次数上限。
//
// 每个 systray 实例都会占用一个不可回收的进程级窗口过程回调槽位，无限重建终将耗尽
// 它们，之后连 systray 自己的初始化都会 panic。上限存在的意义是把这种病态状态变成
// 一条明确的日志，而不是滑向无从诊断的静默失效。
func TestSystrayBudgetExhausted(t *testing.T) {
	m := NewManager(testLogger{}, []byte{0x01})

	if m.systrayBudgetExhausted() {
		t.Fatal("新建的管理器不应被判定为已用尽实例预算")
	}

	atomic.StoreInt32(&m.instanceCount, maxSystrayInstances-1)
	if m.systrayBudgetExhausted() {
		t.Fatalf("实例数 %d 仍应允许再建一次", maxSystrayInstances-1)
	}

	atomic.StoreInt32(&m.instanceCount, maxSystrayInstances)
	if !m.systrayBudgetExhausted() {
		t.Fatalf("实例数达到 %d 时应停止继续重建", maxSystrayInstances)
	}
}

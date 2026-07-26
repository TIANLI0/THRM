package device

import (
	"testing"
	"time"
)

// TestIdlePollingInterval 覆盖 #10：空闲时放慢 HID 空转轮询。
//
// HID 句柄设为非阻塞后，读循环靠"空转即休眠"避免占满 CPU，这意味着常驻
// 10 次/秒的唤醒。空闲（无 GUI 且未开智能控温）时没人消费实时转速，
// 托盘 5 秒才刷新一次，因此可以降到 2 次/秒。
func TestIdlePollingInterval(t *testing.T) {
	m := NewManager(testLogger{})

	if got := m.readPollInterval(); got != hidReadPollInterval {
		t.Fatalf("默认轮询间隔 = %s, want %s", got, hidReadPollInterval)
	}

	m.SetIdlePolling(true)
	if got := m.readPollInterval(); got != hidIdleReadPollInterval {
		t.Fatalf("空闲轮询间隔 = %s, want %s", got, hidIdleReadPollInterval)
	}

	m.SetIdlePolling(false)
	if got := m.readPollInterval(); got != hidReadPollInterval {
		t.Fatalf("恢复后轮询间隔 = %s, want %s", got, hidReadPollInterval)
	}

	wakeupsPerSecond := float64(time.Second) / float64(hidIdleReadPollInterval)
	t.Logf("空闲唤醒频率 %.0f 次/秒（修复前恒为 %.0f 次/秒）",
		wakeupsPerSecond, float64(time.Second)/float64(hidReadPollInterval))
}

// TestWaitPollIntervalRespondsToStop 验证拉长间隔后仍能立即响应停止信号。
// 用 select 而非 time.Sleep 是必要的：否则空闲模式下断连/重连要多等半秒。
func TestWaitPollIntervalRespondsToStop(t *testing.T) {
	m := NewManager(testLogger{})
	m.SetIdlePolling(true)

	stop := make(chan struct{})
	close(stop)

	start := time.Now()
	if m.waitPollInterval(stop) {
		t.Fatal("收到停止信号时 waitPollInterval 应返回 false")
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("停止信号响应耗时 %v，未能立即返回", elapsed)
	}
}

// TestWaitPollIntervalElapses 验证无停止信号时正常等满间隔。
func TestWaitPollIntervalElapses(t *testing.T) {
	m := NewManager(testLogger{})
	stop := make(chan struct{})

	start := time.Now()
	if !m.waitPollInterval(stop) {
		t.Fatal("无停止信号时 waitPollInterval 应返回 true")
	}
	if elapsed := time.Since(start); elapsed < hidReadPollInterval {
		t.Fatalf("等待时间 %v 短于轮询间隔 %s", elapsed, hidReadPollInterval)
	}
}

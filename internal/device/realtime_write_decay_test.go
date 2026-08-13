package device

import (
	"testing"
	"time"
)

// TestRealtimeWriteErrorsExpired 覆盖实时转速写入失败计数的有效期判定。
func TestRealtimeWriteErrorsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		lastErrorAt time.Time
		want        bool
	}{
		{"本次连接尚无失败记录", time.Time{}, false},
		{"刚刚失败过", now.Add(-time.Second), false},
		{"临界点内", now.Add(-realtimeWriteErrorDecay + time.Second), false},
		{"已超过有效期", now.Add(-realtimeWriteErrorDecay - time.Second), true},
		{"隔了几小时", now.Add(-3 * time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := realtimeWriteErrorsExpired(tt.lastErrorAt, now); got != tt.want {
				t.Fatalf("realtimeWriteErrorsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRealtimeWriteErrorsDecayAcrossIdleGaps 验证相隔很久的失败不会攒成"连续失败"。
//
// 稳态智能控温下转速不变就不下发，相邻两次写入可能相隔数分钟乃至数小时。
// 修复前计数只在写入成功时归零，于是三次互不相关的蓝牙瞬时抖动会被判成连续失败，
// 触发一次毫无必要的主动断开重连——用户看到的就是"偶尔莫名断连"。
func TestRealtimeWriteErrorsDecayAcrossIdleGaps(t *testing.T) {
	m := NewManager(testLogger{})

	// 三次相隔一小时的孤立失败：每次都应当重新起算，绝不触发恢复重连。
	for i := range 3 {
		m.lastRealtimeWriteErrorAt = time.Now().Add(-time.Hour)
		m.noteRealtimeWriteResultLocked(false)

		if m.consecutiveRealtimeWriteErrors != 1 {
			t.Fatalf("第 %d 次孤立失败后计数 = %d, want 1", i+1, m.consecutiveRealtimeWriteErrors)
		}
		if m.realtimeWriteRecoveryScheduled {
			t.Fatalf("第 %d 次孤立失败后不应排定断开重连", i+1)
		}
	}
}

// TestRealtimeWriteErrorsStillTripOnRealBurst 确认衰减没有削弱真正的故障检测：
// 短时间内连续失败达到阈值仍会排定断开重连。
func TestRealtimeWriteErrorsStillTripOnRealBurst(t *testing.T) {
	m := NewManager(testLogger{})

	for i := range maxConsecutiveRealtimeWriteErrors {
		m.noteRealtimeWriteResultLocked(false)
		if want := i + 1; m.consecutiveRealtimeWriteErrors != want {
			t.Fatalf("第 %d 次连续失败后计数 = %d, want %d", i+1, m.consecutiveRealtimeWriteErrors, want)
		}
	}

	if !m.realtimeWriteRecoveryScheduled {
		t.Fatalf("连续失败 %d 次后应排定断开重连", maxConsecutiveRealtimeWriteErrors)
	}
}

// TestRealtimeWriteSuccessClearsErrorTimestamp 写入成功必须同时清掉时间戳，
// 否则下一次孤立失败会拿一个陈旧的时间戳去判定衰减。
func TestRealtimeWriteSuccessClearsErrorTimestamp(t *testing.T) {
	m := NewManager(testLogger{})

	m.noteRealtimeWriteResultLocked(false)
	if m.lastRealtimeWriteErrorAt.IsZero() {
		t.Fatal("失败后应记录失败时间")
	}

	m.noteRealtimeWriteResultLocked(true)
	if !m.lastRealtimeWriteErrorAt.IsZero() {
		t.Fatal("写入成功后应清空失败时间")
	}
	if m.consecutiveRealtimeWriteErrors != 0 {
		t.Fatalf("写入成功后计数 = %d, want 0", m.consecutiveRealtimeWriteErrors)
	}
}

// TestResetRealtimeControlStateClearsErrorTimestamp 重连会重置实时控制状态，
// 失败时间戳也必须一起清掉，避免跨连接携带旧状态。
func TestResetRealtimeControlStateClearsErrorTimestamp(t *testing.T) {
	m := NewManager(testLogger{})

	m.noteRealtimeWriteResultLocked(false)
	m.resetRealtimeControlStateLocked()

	if !m.lastRealtimeWriteErrorAt.IsZero() {
		t.Fatal("重置实时控制状态后应清空失败时间")
	}
	if m.consecutiveRealtimeWriteErrors != 0 {
		t.Fatalf("重置后计数 = %d, want 0", m.consecutiveRealtimeWriteErrors)
	}
}

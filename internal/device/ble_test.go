package device

import (
	"errors"
	"sync"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

type blockingBLEAdapter struct {
	stop     chan struct{}
	stopOnce sync.Once
}

func newBlockingBLEAdapter() *blockingBLEAdapter {
	return &blockingBLEAdapter{stop: make(chan struct{})}
}

func (a *blockingBLEAdapter) Enable() error { return nil }

func (a *blockingBLEAdapter) Scan(func(*bluetooth.Adapter, bluetooth.ScanResult)) error {
	<-a.stop
	return nil
}

func (a *blockingBLEAdapter) StopScan() error {
	a.stopOnce.Do(func() { close(a.stop) })
	return nil
}

func (a *blockingBLEAdapter) Connect(bluetooth.Address, bluetooth.ConnectionParams) (bluetooth.Device, error) {
	return bluetooth.Device{}, errors.New("unexpected connect")
}

func TestBLEConnectStopsBlockingScanOnTimeout(t *testing.T) {
	adapter := newBlockingBLEAdapter()
	manager := NewBLEManager(nil)
	manager.adapter = adapter
	manager.scanTimeout = 20 * time.Millisecond

	done := make(chan bool, 1)
	go func() {
		connected, _ := manager.Connect()
		done <- connected
	}()

	select {
	case connected := <-done:
		if connected {
			t.Fatal("Connect reported success after a scan timeout")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Connect remained blocked after the BLE scan timeout")
	}

	select {
	case <-adapter.stop:
	default:
		t.Fatal("BLE scan timeout did not call StopScan")
	}
}

func TestShouldSkipBLEFallback(t *testing.T) {
	tests := []struct {
		name          string
		preferLast    bool
		lastType      string
		sinceLastScan time.Duration
		want          bool
	}{
		{name: "automatic HID reconnect", preferLast: true, lastType: "hid", sinceLastScan: -1, want: true},
		{name: "manual full discovery", lastType: "hid", sinceLastScan: -1, want: false},
		{name: "automatic BLE reconnect", preferLast: true, lastType: "ble", sinceLastScan: time.Second, want: false},
		{name: "first automatic connection scans once", preferLast: true, sinceLastScan: -1, want: false},
		{name: "never-connected reconnect inside cooldown", preferLast: true, sinceLastScan: 30 * time.Second, want: true},
		{name: "never-connected reconnect after cooldown", preferLast: true, sinceLastScan: idleBLEScanCooldown, want: false},
		{name: "manual connect ignores cooldown", lastType: "", sinceLastScan: 30 * time.Second, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSkipBLEFallback(test.preferLast, test.lastType, test.sinceLastScan); got != test.want {
				t.Fatalf("shouldSkipBLEFallback(%v, %q, %v) = %v, want %v", test.preferLast, test.lastType, test.sinceLastScan, got, test.want)
			}
		})
	}
}

// BS1 静置一段时间后广播里只剩服务 UUID 0xFFF0、不带名字，只认名字会一直扫不到（issue #46）。
func TestBS1ScanMatcher(t *testing.T) {
	start := time.Now()
	bs1 := mustBLEAddress(t, "50:54:7B:D6:8E:82")
	other := mustBLEAddress(t, "11:22:33:44:55:66")

	t.Run("带名字的广播立即采用", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		if match, _ := m.offer(bs1, "Flydigi BS1", false, start); !match {
			t.Fatal("名字含 BS1 的广播没有被采用")
		}
		if _, name, byService := m.target(); name != "Flydigi BS1" || byService {
			t.Fatalf("target = %q byService=%v", name, byService)
		}
	})

	t.Run("无名称的 0xFFF0 先等带名字的广播", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		if match, _ := m.offer(other, "", true, start.Add(time.Second)); match {
			t.Fatal("宽限期内就凭 UUID 认定了设备")
		}
		if match, _ := m.offer(bs1, "Flydigi BS1", false, start.Add(2*time.Second)); !match {
			t.Fatal("宽限期内带名字的广播没有被采用")
		}
		if addr, _, _ := m.target(); addr != bs1 {
			t.Fatalf("target = %s, want %s", addr, bs1)
		}
	})

	t.Run("宽限期过后凭 UUID 采用", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		m.offer(bs1, "", true, start)
		if match, _ := m.offer(bs1, "", true, start.Add(bs1ServiceFallbackDelay)); !match {
			t.Fatal("宽限期过后仍未凭 UUID 采用")
		}
		if addr, _, byService := m.target(); addr != bs1 || !byService {
			t.Fatalf("target = %s byService=%v", addr, byService)
		}
	})

	t.Run("连过的地址凭 UUID 立即采用", func(t *testing.T) {
		m := newBS1ScanMatcher(bs1.String(), start)
		if match, _ := m.offer(bs1, "", true, start); !match {
			t.Fatal("已知 BS1 地址的无名称广播没有立即采用")
		}
		if match, _ := newBS1ScanMatcher(bs1.String(), start).offer(other, "", true, start); match {
			t.Fatal("陌生地址也被当成已知 BS1 立即采用")
		}
	})

	t.Run("带别的名字的 0xFFF0 设备不算", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		if match, _ := m.offer(other, "Mi Band", true, start.Add(time.Minute)); match {
			t.Fatal("名字不是 BS1 的设备被当成 BS1")
		}
		if m.acceptFallback() {
			t.Fatal("名字不是 BS1 的设备成了超时后备")
		}
	})

	t.Run("超时后采用最早的候选", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		m.offer(bs1, "", true, start)
		m.offer(other, "", true, start.Add(time.Second))
		if !m.acceptFallback() {
			t.Fatal("超时后没有采用 UUID 候选")
		}
		if addr, _, _ := m.target(); addr != bs1 {
			t.Fatalf("target = %s, want %s", addr, bs1)
		}
	})

	t.Run("没有 UUID 的无名广播忽略", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		m.offer(other, "", false, start.Add(time.Minute))
		if m.acceptFallback() {
			t.Fatal("不带 0xFFF0 的无名设备成了候选")
		}
	})

	t.Run("同一地址只报告一次首次发现", func(t *testing.T) {
		m := newBS1ScanMatcher("", start)
		if _, first := m.offer(other, "", true, start); !first {
			t.Fatal("首次见到地址时 firstSeen=false")
		}
		if _, first := m.offer(other, "", true, start); first {
			t.Fatal("重复广播又报告了 firstSeen")
		}
	})
}

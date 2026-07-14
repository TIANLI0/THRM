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
		name       string
		preferLast bool
		lastType   string
		want       bool
	}{
		{name: "automatic HID reconnect", preferLast: true, lastType: "hid", want: true},
		{name: "manual full discovery", lastType: "hid", want: false},
		{name: "automatic BLE reconnect", preferLast: true, lastType: "ble", want: false},
		{name: "first automatic connection", preferLast: true, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSkipBLEFallback(test.preferLast, test.lastType); got != test.want {
				t.Fatalf("shouldSkipBLEFallback(%v, %q) = %v, want %v", test.preferLast, test.lastType, got, test.want)
			}
		})
	}
}

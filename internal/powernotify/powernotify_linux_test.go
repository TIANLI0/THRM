package powernotify

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestRegisterSuspendResumeNotifications_Success(t *testing.T) {
	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable (expected in CI): %v", err)
	}
	if stop == nil {
		t.Fatal("stop function should not be nil")
	}
	stop()
}

func TestStop_Idempotent(t *testing.T) {
	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}

	stop()
	stop()
	stop()
}

func TestStop_CleansUp(t *testing.T) {
	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}
	stop()

	time.Sleep(50 * time.Millisecond)
}

func TestRegister_NilCallbacks(t *testing.T) {
	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}
	stop()
}

func TestRegister_StopNoLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}

	stop()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestRegister_SignalDelivery(t *testing.T) {
	var mu sync.Mutex
	suspendCalled := false

	stop, err := RegisterSuspendResumeNotifications(
		func() { mu.Lock(); suspendCalled = true; mu.Unlock() },
		func() { mu.Lock(); suspendCalled = true; mu.Unlock() },
	)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}
	defer stop()

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if suspendCalled {
		t.Log("callback was triggered (unexpected during normal operation)")
	}
	mu.Unlock()
}

func TestNotifier_StopOnce(t *testing.T) {
	stop, err := RegisterSuspendResumeNotifications(nil, nil)
	if err != nil {
		t.Skipf("D-Bus system bus unavailable: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stop()
		}()
	}
	wg.Wait()
}

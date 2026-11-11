package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestFanCallbackCannotBlockHIDReaderOnCoreLock(t *testing.T) {
	a := &CoreApp{}
	a.mutex.Lock()
	done := make(chan struct{})
	go func() {
		a.onFanDataUpdate(&types.FanData{CurrentRPM: 2000})
		close(done)
	}()
	select {
	case <-done:
		a.mutex.Unlock()
	case <-time.After(time.Second):
		a.mutex.Unlock()
		<-done
		t.Fatal("Core lock stalled the HID reader/ACK dispatch")
	}
}

package coreapp

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestShutdownCallbackIsBoundedAndCancelledQueuedWorkCannotStopFan(t *testing.T) {
	a := &CoreApp{}
	a.deviceControlMutex.Lock() // a slow HID transaction owns the barrier
	var writes atomic.Int32
	start := time.Now()
	a.startShutdownWork(20*time.Millisecond, func(current func() bool) { writes.Add(1) })
	if time.Since(start) > time.Second {
		a.deviceControlMutex.Unlock()
		t.Fatal("shutdown callback blocked on HID lock")
	}
	restored := make(chan struct{})
	a.cancelShutdownWork(func() { close(restored) })
	a.deviceControlMutex.Unlock()
	select {
	case <-restored:
	case <-time.After(time.Second):
		t.Fatal("cancel did not restore after pending transaction released")
	}
	if writes.Load() != 0 {
		t.Fatal("queued shutdown stopped the fan after cancellation")
	}
}

func TestNewShutdownInvalidatesOldCancellationAndQueuedCleanup(t *testing.T) {
	a := &CoreApp{}
	a.deviceControlMutex.Lock()
	var stale atomic.Int32
	a.startShutdownWork(time.Millisecond, func(func() bool) { stale.Add(1) })
	a.cancelShutdownWork(func() { stale.Add(1) })
	latest := make(chan struct{})
	a.startShutdownWork(time.Millisecond, func(func() bool) { close(latest) })
	a.deviceControlMutex.Unlock()
	select {
	case <-latest:
	case <-time.After(time.Second):
		t.Fatal("latest shutdown did not run")
	}
	if stale.Load() != 0 {
		t.Fatal("old cancellation/cleanup affected a newer shutdown")
	}
}

type sessionDevice struct {
	bs1             bool
	rpm             int
	gear            string
	gearLight       bool
	rgb             bool
	gearLightWrites int
}

func (d *sessionDevice) IsBS1() bool                    { return d.bs1 }
func (d *sessionDevice) SetCustomFanSpeed(rpm int) bool { d.rpm = rpm; return true }
func (d *sessionDevice) SetManualGearRPM(gear, level string, rpm int) bool {
	d.gear, d.rpm = gear, rpm
	return true
}
func (d *sessionDevice) SetGearLight(on bool) bool {
	d.gearLight, d.gearLightWrites = on, d.gearLightWrites+1
	return true
}
func (d *sessionDevice) SetRGBOn() bool  { d.rgb = true; return true }
func (d *sessionDevice) SetRGBOff() bool { d.rgb = false; return true }

func TestShutdownCancelRestoresAllModesAndOutputSwitches(t *testing.T) {
	for _, mode := range []string{"automatic", "custom", "manual"} {
		for _, bs1 := range []bool{false, true} {
			cfg := types.GetDefaultConfig(false)
			cfg.AutoControl = mode == "automatic"
			cfg.CustomSpeedEnabled, cfg.CustomSpeedRPM = mode == "custom", 2750
			cfg.GearLight, cfg.ManualGear = true, "标准"
			cfg.LightStrip.Mode = "static_single"
			d := &sessionDevice{bs1: bs1} // shutdown left all outputs off
			if err := restoreSessionOutputs(d, cfg, 3100, "中"); err != nil {
				t.Fatal(err)
			}
			want := cfg.ResolveGearRPM("标准", "中")
			if mode == "automatic" {
				want = 3100
			}
			if mode == "custom" {
				want = 2750
			}
			if d.rpm != want || !d.rgb || (!bs1 && !d.gearLight) || (bs1 && d.gearLightWrites != 0) {
				t.Fatalf("mode=%s bs1=%t incomplete restore: %+v", mode, bs1, d)
			}
			cfg.LightStrip.Mode, cfg.GearLight = "off", false
			if err := restoreSessionOutputs(d, cfg, 3100, "中"); err != nil {
				t.Fatal(err)
			}
			if d.rgb || d.gearLight {
				t.Fatal("restore enabled user-disabled lights")
			}
		}
	}
}

func TestShutdownRejectsNewAndStaleConnections(t *testing.T) {
	a := &CoreApp{}
	a.connectGeneration.Store(5)
	a.shutdownInProgress.Store(true)
	if a.isConnectionAttemptCurrent(5) {
		t.Fatal("shutdown accepted an in-flight connection")
	}
	if a.connectDevice(true) {
		t.Fatal("shutdown opened a new connection")
	}
	a.requestReconnectWithLogging("test", nil, false)
	if a.reconnectInProgress.Load() {
		t.Fatal("shutdown queued a reconnect")
	}
}

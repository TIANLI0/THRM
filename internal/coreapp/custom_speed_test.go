package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
)

type customSpeedFirmware struct {
	realtime       bool
	target         int
	gear           string
	level          string
	realtimeWrites int
	gearWrites     int
	rejectGear     bool
}

func (f *customSpeedFirmware) SetCustomFanSpeed(rpm int) bool {
	f.realtimeWrites++
	f.realtime, f.target = true, rpm
	return true
}

func (f *customSpeedFirmware) SetManualGearRPM(gear, level string, rpm int) bool {
	f.gearWrites++
	if f.rejectGear {
		return false
	}
	f.realtime, f.gear, f.level, f.target = false, gear, level, rpm
	return true
}

func TestDisableCustomSpeedLeavesRealtimeWithoutZeroTarget(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl, cfg.CustomSpeedEnabled = false, true
	cfg.ManualGear = "标准"
	fw := &customSpeedFirmware{realtime: true, target: 3500}
	if err := applyCustomSpeedMode(fw, cfg, false, -1, "高"); err != nil {
		t.Fatal(err)
	}
	if fw.realtime || fw.gear != "标准" || fw.level != "高" || fw.target != cfg.ResolveGearRPM("标准", "高") {
		t.Fatalf("custom mode did not restore the chosen gear: %+v", fw)
	}
	if fw.realtimeWrites != 0 || fw.gearWrites != 1 {
		t.Fatalf("disable must not depend on a zero-RPM realtime ACK: %+v", fw)
	}
}

func TestRepeatedDisableDoesNotInterruptAutomaticControl(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl, cfg.CustomSpeedEnabled = true, false
	fw := &customSpeedFirmware{realtime: true, target: 2800}
	if err := applyCustomSpeedMode(fw, cfg, false, 0, "中"); err != nil {
		t.Fatal(err)
	}
	if !fw.realtime || fw.target != 2800 || fw.gearWrites != 0 || fw.realtimeWrites != 0 {
		t.Fatalf("idempotent disable changed automatic control: %+v", fw)
	}
}

func TestDisableReportsGearFailureAndCanBeRetried(t *testing.T) {
	cfg := types.GetDefaultConfig(false)
	cfg.CustomSpeedEnabled = true
	fw := &customSpeedFirmware{realtime: true, target: 3000, rejectGear: true}
	if err := applyCustomSpeedMode(fw, cfg, false, 0, "中"); err == nil {
		t.Fatal("gear restore failure was hidden")
	}
	fw.rejectGear = false
	if err := applyCustomSpeedMode(fw, cfg, false, 0, "中"); err != nil || fw.realtime {
		t.Fatalf("disable could not recover on retry: %v", err)
	}
}

func TestOfflineDisablePersistsWithoutDeviceAndPreservesRememberedRPM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)
	mgr := config.NewManager(dir, nil)
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl, cfg.CustomSpeedEnabled, cfg.CustomSpeedRPM = false, true, 2750
	mgr.Set(cfg)
	a := &CoreApp{configManager: mgr}
	// No device manager: closing an offline custom mode must need no device I/O.
	if err := a.SetCustomSpeed(false, 0); err != nil {
		t.Fatal(err)
	}
	got := mgr.Get()
	if got.CustomSpeedEnabled || got.CustomSpeedRPM != 2750 {
		t.Fatalf("offline disable lost user intent or remembered RPM: %+v", got)
	}
	reloaded := config.NewManager(dir, nil)
	reloaded.Load(false)
	if reloaded.Get().CustomSpeedEnabled {
		t.Fatal("disabled mode returned after reloading saved config")
	}
}

package coreapp

import (
	"errors"
	"testing"

	"github.com/TIANLI0/THRM/internal/ipc"
)

func TestFirmwareUpdateDisabledWithoutDebugBeforeAnyIO(t *testing.T) {
	// Deliberately omit the device manager and configuration: none of these
	// entry points may query hardware, download files or launch flash work.
	a := &CoreApp{}
	if status := a.GetFirmwareUpdateStatus(); status != (FirmwareUpdateStatus{}) {
		t.Fatalf("firmware update exposed outside debug mode: %+v", status)
	}
	if list, err := a.ListFirmwareUpdates(); !errors.Is(err, errFirmwareUpdateDebugOnly) || len(list) != 0 {
		t.Fatalf("firmware list bypassed debug gate: %v", err)
	}
	for name, call := range map[string]func() error{
		"install driver":   a.InstallFirmwareDriver,
		"uninstall driver": a.UninstallFirmwareDriver,
		"start flash":      func() error { return a.FlashFirmware(ipc.FlashFirmwareParams{}) },
		"queued flash":     func() error { return a.runFlash(ipc.FlashFirmwareParams{}) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, errFirmwareUpdateDebugOnly) {
				t.Fatalf("operation bypassed debug gate: %v", err)
			}
		})
	}
}

func TestFirmwareUpdateGateFollowsCurrentDebugMode(t *testing.T) {
	a := &CoreApp{}
	for _, enabled := range []bool{true, false, true, false} {
		a.mutex.Lock()
		a.debugMode = enabled
		a.mutex.Unlock()
		err := a.requireFirmwareUpdateDebugMode()
		if enabled && err != nil {
			t.Fatal("debug mode did not unlock firmware update")
		}
		if !enabled && !errors.Is(err, errFirmwareUpdateDebugOnly) {
			t.Fatal("disabling debug mode did not revoke access")
		}
	}
}

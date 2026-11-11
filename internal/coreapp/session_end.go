package coreapp

import (
	"errors"
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/temperature"
	"github.com/TIANLI0/THRM/internal/types"
)

const shutdownOutputGrace = 1500 * time.Millisecond

// Bound time spent in the Windows window callback, including time waiting for
// an in-flight HID transaction. Cancellation invalidates queued work before it
// can touch the device, even if that work only acquires the lock much later.
func (a *CoreApp) startShutdownWork(grace time.Duration, work func(current func() bool)) {
	if a.stopping.Load() || !a.shutdownInProgress.CompareAndSwap(false, true) {
		return
	}
	generation := a.shutdownGeneration.Add(1)
	a.connectGeneration.Add(1)
	a.cancelReconnect()
	done := make(chan struct{})
	a.safeGo("session-shutdown", func() {
		defer close(done)
		a.deviceControlMutex.Lock()
		defer a.deviceControlMutex.Unlock()
		current := func() bool {
			return a.shutdownInProgress.Load() && a.shutdownGeneration.Load() == generation && !a.stopping.Load()
		}
		if current() {
			work(current)
		}
	})
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		a.logInfo("关机输出清理超过 %s，后台继续处理，不阻塞系统关机", grace)
	}
}

func (a *CoreApp) cancelShutdownWork(restore func()) {
	if !a.shutdownInProgress.Swap(false) {
		return
	}
	generation := a.shutdownGeneration.Add(1)
	a.safeGo("session-shutdown-cancelled", func() {
		a.deviceControlMutex.Lock()
		defer a.deviceControlMutex.Unlock()
		if a.shutdownGeneration.Load() != generation || a.shutdownInProgress.Load() || a.stopping.Load() || a.systemSuspended.Load() {
			return
		}
		restore()
	})
}

func (a *CoreApp) handleSystemShutdown() {
	a.startShutdownWork(shutdownOutputGrace, func(current func() bool) {
		if !a.deviceManager.IsConnected() {
			return
		}
		a.logInfo("系统即将关机：停转并关闭灯光输出")
		if !a.deviceManager.SetCustomFanSpeed(0) {
			a.logError("系统即将关机：下发 0 转速失败")
		}
		if !current() {
			return
		}
		if a.configManager.Get().LightStrip.Mode != "off" && !a.deviceManager.SetRGBOff() {
			a.logError("系统即将关机：关闭灯光输出失败")
		}
		if !current() {
			return
		}
		if !a.deviceManager.IsBS1() && !a.deviceManager.SetGearLight(false) {
			a.logError("系统即将关机：关闭挡位灯失败")
		}
	})
}

func (a *CoreApp) handleShutdownCancelled() {
	a.cancelShutdownWork(func() {
		if !a.deviceManager.IsConnected() {
			// Resume an interrupted connection attempt only if the Core had
			// intended to keep a device connected before shutdown.
			a.mutex.RLock()
			connected := a.isConnected
			a.mutex.RUnlock()
			if connected {
				a.requestReconnect("shutdown-cancelled", []time.Duration{0})
			}
			return
		}
		a.logInfo("系统取消关机：恢复转速模式、挡位灯及灯效")
		cfg := a.configManager.Get()
		controlTemp := a.latestControlTemp()
		autoRPM := temperature.CalculateTargetRPM(controlTemp, cfg.FanCurve)
		if controlTemp <= 0 {
			curve := cfg.FanCurve
			if len(curve) == 0 {
				curve = types.GetDefaultFanCurve()
			}
			autoRPM = curve[0].RPM
		}
		level := a.getRememberedManualLevel(cfg.ManualGear, cfg.ManualLevel)
		if err := restoreSessionOutputs(a.deviceManager, cfg, autoRPM, level); err != nil {
			a.logError("取消关机后恢复配置失败: %v", err)
		}
	})
}

type sessionOutputDevice interface {
	IsBS1() bool
	SetCustomFanSpeed(int) bool
	SetManualGearRPM(string, string, int) bool
	SetGearLight(bool) bool
	SetRGBOn() bool
	SetRGBOff() bool
}

// Shutdown only changes the live speed and output switches. Restore all of
// those for every mode, without re-uploading a program or rewriting settings
// such as startup behavior that shutdown never changed.
func restoreSessionOutputs(device sessionOutputDevice, cfg types.AppConfig, autoRPM int, level string) error {
	var failures []error
	if cfg.AutoControl || cfg.CustomSpeedEnabled {
		rpm := autoRPM
		if !cfg.AutoControl {
			rpm = cfg.CustomSpeedRPM
		}
		if !device.SetCustomFanSpeed(rpm) {
			failures = append(failures, fmt.Errorf("恢复实时转速失败"))
		}
	} else if !device.SetManualGearRPM(cfg.ManualGear, level, cfg.ResolveGearRPM(cfg.ManualGear, level)) {
		failures = append(failures, fmt.Errorf("恢复手动挡位失败"))
	}
	if !device.IsBS1() && !device.SetGearLight(cfg.GearLight) {
		failures = append(failures, fmt.Errorf("恢复挡位灯失败"))
	}
	ok := false
	if cfg.LightStrip.Mode == "off" {
		ok = device.SetRGBOff()
	} else {
		ok = device.SetRGBOn()
	}
	if !ok {
		failures = append(failures, fmt.Errorf("恢复灯光输出失败"))
	}
	return errors.Join(failures...)
}

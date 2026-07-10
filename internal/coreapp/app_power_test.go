package coreapp

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestSystemResumeDetectionThreshold(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{name: "uses floor for fast polling", interval: time.Second, want: 20 * time.Second},
		{name: "scales with normal polling", interval: 5 * time.Second, want: 30 * time.Second},
		{name: "caps long polling threshold", interval: 20 * time.Second, want: 45 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := systemResumeDetectionThreshold(test.interval); got != test.want {
				t.Fatalf("systemResumeDetectionThreshold(%v) = %v, want %v", test.interval, got, test.want)
			}
		})
	}
}

func TestShouldRecoverFromSystemResumeGap(t *testing.T) {
	tests := []struct {
		name     string
		gap      time.Duration
		interval time.Duration
		want     bool
	}{
		{name: "ignores normal short gap", gap: 10 * time.Second, interval: time.Second, want: false},
		{name: "detects floor threshold", gap: 20 * time.Second, interval: time.Second, want: true},
		{name: "requires scaled threshold", gap: 40 * time.Second, interval: 10 * time.Second, want: false},
		{name: "detects capped threshold", gap: 45 * time.Second, interval: 10 * time.Second, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldRecoverFromSystemResumeGap(test.gap, test.interval); got != test.want {
				t.Fatalf("shouldRecoverFromSystemResumeGap(%v, %v) = %v, want %v", test.gap, test.interval, got, test.want)
			}
		})
	}
}

func TestShouldReconnectAfterResume(t *testing.T) {
	tests := []struct {
		name                    string
		proactivelySuspended    bool
		resumeReconnectWanted   bool
		autoReconnectSuppressed bool
		forceReconnect          bool
		want                    bool
	}{
		{
			name:                  "reconnects a device that was connected before suspend",
			proactivelySuspended:  true,
			resumeReconnectWanted: true,
			want:                  true,
		},
		{
			name:                    "does not reconnect a manually disconnected device",
			proactivelySuspended:    true,
			autoReconnectSuppressed: true,
			forceReconnect:          true,
			want:                    false,
		},
		{
			name:           "recovers an unexpected resume without a suspend event",
			forceReconnect: true,
			want:           true,
		},
		{
			name:                    "preserves a manual disconnect without a suspend event",
			autoReconnectSuppressed: true,
			forceReconnect:          true,
			want:                    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldReconnectAfterResume(
				test.proactivelySuspended,
				test.resumeReconnectWanted,
				test.autoReconnectSuppressed,
				test.forceReconnect,
			); got != test.want {
				t.Fatalf("shouldReconnectAfterResume() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestConnectionAttemptCurrent(t *testing.T) {
	tests := []struct {
		name                    string
		stopping                bool
		suspended               bool
		autoReconnectSuppressed bool
		generation              uint64
		currentGeneration       uint64
		want                    bool
	}{
		{name: "accepts current active connection", generation: 4, currentGeneration: 4, want: true},
		{name: "rejects stale generation", generation: 4, currentGeneration: 5, want: false},
		{name: "rejects suspended system", suspended: true, generation: 4, currentGeneration: 4, want: false},
		{name: "rejects explicit disconnect", autoReconnectSuppressed: true, generation: 4, currentGeneration: 4, want: false},
		{name: "rejects shutdown", stopping: true, generation: 4, currentGeneration: 4, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isConnectionAttemptCurrent(
				test.stopping,
				test.suspended,
				test.autoReconnectSuppressed,
				test.generation,
				test.currentGeneration,
			); got != test.want {
				t.Fatalf("isConnectionAttemptCurrent() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAutomaticControlAllowed(t *testing.T) {
	tests := []struct {
		name            string
		stopping        bool
		suspended       bool
		coreConnected   bool
		deviceConnected bool
		want            bool
	}{
		{name: "allows a ready active connection", coreConnected: true, deviceConnected: true, want: true},
		{name: "blocks an opened but unready handle", deviceConnected: true, want: false},
		{name: "blocks suspend even when connected", suspended: true, coreConnected: true, deviceConnected: true, want: false},
		{name: "blocks shutdown even when connected", stopping: true, coreConnected: true, deviceConnected: true, want: false},
		{name: "blocks disconnected device", coreConnected: true, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := automaticControlAllowed(test.stopping, test.suspended, test.coreConnected, test.deviceConnected); got != test.want {
				t.Fatalf("automaticControlAllowed() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReconnectDelayElapsed(t *testing.T) {
	t.Run("cancellation interrupts backoff", func(t *testing.T) {
		cancel := make(chan struct{})
		result := make(chan bool, 1)
		go func() {
			result <- reconnectDelayElapsed(time.Second, cancel)
		}()

		close(cancel)
		select {
		case elapsed := <-result:
			if elapsed {
				t.Fatal("cancelled reconnect delay reported as elapsed")
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("cancelled reconnect delay did not return promptly")
		}
	})

	t.Run("zero delay runs immediately", func(t *testing.T) {
		if !reconnectDelayElapsed(0, make(chan struct{})) {
			t.Fatal("zero reconnect delay did not elapse immediately")
		}
	})
}

func TestStopTemperatureMonitoringSignalsOnlyOnce(t *testing.T) {
	app := &CoreApp{
		monitorStop: make(chan struct{}),
		monitorDone: make(chan struct{}),
	}
	app.monitoringTemp.Store(true)

	firstDone := app.stopTemperatureMonitoring()
	secondDone := app.stopTemperatureMonitoring()
	if firstDone == nil || secondDone == nil || firstDone != secondDone {
		t.Fatal("stopTemperatureMonitoring did not return the active monitor session")
	}

	select {
	case <-app.monitorStop:
	default:
		t.Fatal("stopTemperatureMonitoring did not signal the monitor")
	}
}

func TestEffectiveTemperatureMonitorInterval(t *testing.T) {
	tests := []struct {
		name        string
		updateRate  int
		hasClients  bool
		autoControl bool
		want        time.Duration
	}{
		{name: "idle core backs off short interval", updateRate: 2, want: idleTemperatureMonitorInterval},
		{name: "idle core keeps slower configured interval", updateRate: 15, want: 15 * time.Second},
		{name: "gui keeps configured interval", updateRate: 2, hasClients: true, want: 2 * time.Second},
		{name: "automatic control keeps configured interval", updateRate: 2, autoControl: true, want: 2 * time.Second},
		{name: "invalid interval is normalized before idle backoff", updateRate: 0, want: idleTemperatureMonitorInterval},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := effectiveTemperatureMonitorInterval(test.updateRate, test.hasClients, test.autoControl); got != test.want {
				t.Fatalf("effectiveTemperatureMonitorInterval(%d, %v, %v) = %v, want %v", test.updateRate, test.hasClients, test.autoControl, got, test.want)
			}
		})
	}
}

func TestShouldSendTargetRPM(t *testing.T) {
	tests := []struct {
		name          string
		targetRPM     int
		prevTargetRPM int
		minRPMChange  int
		fanData       *types.FanData
		want          bool
	}{
		{name: "sends initial zero target", targetRPM: 0, prevTargetRPM: -1, minRPMChange: 50, want: true},
		{name: "rejects negative target", targetRPM: -1, prevTargetRPM: -1, minRPMChange: 50, want: false},
		{name: "sends initial positive target", targetRPM: 1800, prevTargetRPM: -1, minRPMChange: 50, want: true},
		{name: "sends significant target change", targetRPM: 1900, prevTargetRPM: 1800, minRPMChange: 50, want: true},
		{name: "skips small target change", targetRPM: 1820, prevTargetRPM: 1800, minRPMChange: 50, want: false},
		{name: "resends when device reports zero target", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 0}, want: true},
		{name: "resends when device still stopped", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{CurrentRPM: 0, TargetRPM: 1800}, want: true},
		{name: "resends when device target drifts", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{TargetRPM: 1700}, want: true},
		{name: "keeps small device target drift", targetRPM: 1800, prevTargetRPM: 1800, minRPMChange: 50, fanData: &types.FanData{CurrentRPM: 1750, TargetRPM: 1775}, want: false},
		{name: "skips repeated zero target", targetRPM: 0, prevTargetRPM: 0, minRPMChange: 50, fanData: &types.FanData{CurrentRPM: 0, TargetRPM: 0}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldSendTargetRPM(test.targetRPM, test.prevTargetRPM, test.minRPMChange, test.fanData); got != test.want {
				t.Fatalf("shouldSendTargetRPM() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestShouldApplyRampLimit(t *testing.T) {
	tests := []struct {
		name          string
		targetRPM     int
		prevTargetRPM int
		want          bool
	}{
		{name: "bypasses wake from zero", targetRPM: 1800, prevTargetRPM: 0, want: false},
		{name: "limits normal positive change", targetRPM: 1800, prevTargetRPM: 1000, want: true},
		{name: "limits positive to zero shutdown", targetRPM: 0, prevTargetRPM: 1800, want: true},
		{name: "keeps initial positive unlimited", targetRPM: 1800, prevTargetRPM: -1, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldApplyRampLimit(test.targetRPM, test.prevTargetRPM); got != test.want {
				t.Fatalf("shouldApplyRampLimit() = %v, want %v", got, test.want)
			}
		})
	}
}

package device

import (
	"errors"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

func smartLightTestManager() *Manager {
	m := NewManager(testLogger{})
	m.hasLightConfig = true
	m.lightConfig = types.LightStripConfig{Mode: "smart_temp", SmartTempHysteresis: 1,
		SmartTempBands: []types.SmartTempLightBand{{MinTemp: 0, Preset: 1}, {MinTemp: 60, Preset: 2}, {MinTemp: 75, Preset: 3}}}
	m.hasSmartLightPreset, m.smartLightPreset, m.smartLightBandIndex = true, 1, 0
	return m
}

func TestSmartLightFiltersLongTemperatureOscillationThenAppliesStableBand(t *testing.T) {
	m := smartLightTestManager()
	start := time.Unix(100, 0)
	var sent []byte
	send := func(p byte) error { sent = append(sent, p); return nil }
	// Connection-time synchronization must still catch up to the actual CPU
	// temperature immediately, rather than sitting at the cold preset.
	if err := m.updateSmartTemperatureLightLocked(80, start, send); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 600; i++ {
		temp := 80
		if i%2 == 1 {
			temp = 50
		}
		if err := m.updateSmartTemperatureLightLocked(temp, start.Add(time.Duration(i)*2*time.Second), send); err != nil {
			t.Fatal(err)
		}
	}
	if len(sent) != 1 || sent[0] != 3 {
		t.Fatalf("20 minutes of brief spikes rebuilt the animation: %v", sent)
	}
	for _, seconds := range []int{1202, 1204, 1206} {
		if err := m.updateSmartTemperatureLightLocked(50, start.Add(time.Duration(seconds)*time.Second), send); err != nil {
			t.Fatal(err)
		}
	}
	if len(sent) != 2 || sent[1] != 1 || m.smartLightBandIndex != 0 {
		t.Fatalf("stable cool temperature did not apply: %v", sent)
	}
}

func TestSmartLightFailureDoesNotRetryEveryMonitorTickOrTrustOldPreset(t *testing.T) {
	m := smartLightTestManager()
	start := time.Unix(100, 0)
	writes := 0
	send := func(byte) error { writes++; return errors.New("lost ACK") }
	if err := m.updateSmartTemperatureLightLocked(80, start, send); err == nil {
		t.Fatal("expected first write failure")
	}
	if m.hasSmartLightPreset {
		t.Fatal("preset cache remains trusted after lost ACK")
	}
	for _, seconds := range []int{1, 2, 3, 4} {
		// Even returning to the old preset must be repaired; the failed write
		// could have changed the device despite its missing response.
		_ = m.updateSmartTemperatureLightLocked(50, start.Add(time.Duration(seconds)*time.Second), send)
	}
	if writes != 1 {
		t.Fatalf("failed command retried too quickly: %d", writes)
	}
	_ = m.updateSmartTemperatureLightLocked(50, start.Add(5*time.Second), send)
	if writes != 2 {
		t.Fatalf("stable old preset was not repaired after backoff: %d", writes)
	}
}

func TestSmartLightSamePresetBandAndReplayDoNotRestartAnimation(t *testing.T) {
	m := smartLightTestManager()
	m.lightConfig.SmartTempBands[1].Preset = 1
	start := time.Unix(100, 0)
	send := func(byte) error { t.Fatal("same-preset band sent a command"); return nil }
	if err := m.updateSmartTemperatureLightLocked(65, start, send); err != nil {
		t.Fatal(err)
	}
	if m.smartLightBandIndex != 1 {
		t.Fatal("displayed band did not follow temperature")
	}
	if err := m.setLightSmartTempLocked(m.lightConfig); err != nil {
		t.Fatal(err)
	}
	if m.smartLightBandIndex != 1 || m.smartLightLastAttempt != start {
		t.Fatal("replay reset active temperature/hysteresis state")
	}
	m.resetLightStateCacheLocked()
	if !m.smartLightLastAttempt.IsZero() || !m.smartLightPendingSince.IsZero() {
		t.Fatal("timing state leaked across reconnect")
	}
}

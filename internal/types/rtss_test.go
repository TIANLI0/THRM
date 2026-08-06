package types

import "testing"

func TestDefaultRTSSConfig(t *testing.T) {
	cfg := GetDefaultConfig(false).RTSS
	if cfg.Enabled {
		t.Fatal("RTSS output must be disabled by default")
	}
	if cfg.UpdateIntervalMS != DefaultRTSSUpdateIntervalMS {
		t.Fatalf("default interval = %d, want %d", cfg.UpdateIntervalMS, DefaultRTSSUpdateIntervalMS)
	}
	if cfg.PositionMode != RTSSPositionModeAnchor {
		t.Fatalf("default position mode = %q, want %q", cfg.PositionMode, RTSSPositionModeAnchor)
	}
}

func TestNormalizeRTSSConfig(t *testing.T) {
	for _, interval := range []int{250, 500, 1000, 2000} {
		cfg, changed := NormalizeRTSSConfig(RTSSConfig{Enabled: true, UpdateIntervalMS: interval, PositionMode: RTSSPositionModeAnchor})
		if changed || cfg.UpdateIntervalMS != interval || !cfg.Enabled {
			t.Fatalf("valid interval %d normalized to %+v (changed=%v)", interval, cfg, changed)
		}
	}

	cfg, changed := NormalizeRTSSConfig(RTSSConfig{Enabled: true, UpdateIntervalMS: 750, PositionMode: RTSSPositionModeAnchor})
	if !changed || cfg.UpdateIntervalMS != DefaultRTSSUpdateIntervalMS || !cfg.Enabled {
		t.Fatalf("invalid interval normalized to %+v (changed=%v)", cfg, changed)
	}

	cfg, changed = NormalizeRTSSConfig(RTSSConfig{Enabled: true, UpdateIntervalMS: 500, PositionMode: RTSSPositionModeCustom, PositionX: -5000, PositionY: 5000})
	if !changed || cfg.PositionMode != RTSSPositionModeCustom || cfg.PositionX != -1000 || cfg.PositionY != 1000 {
		t.Fatalf("custom position was not bounded: %+v (changed=%v)", cfg, changed)
	}
}

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
}

func TestNormalizeRTSSConfig(t *testing.T) {
	for _, interval := range []int{250, 500, 1000, 2000} {
		cfg, changed := NormalizeRTSSConfig(RTSSConfig{Enabled: true, UpdateIntervalMS: interval})
		if changed || cfg.UpdateIntervalMS != interval || !cfg.Enabled {
			t.Fatalf("valid interval %d normalized to %+v (changed=%v)", interval, cfg, changed)
		}
	}

	cfg, changed := NormalizeRTSSConfig(RTSSConfig{Enabled: true, UpdateIntervalMS: 750})
	if !changed || cfg.UpdateIntervalMS != DefaultRTSSUpdateIntervalMS || !cfg.Enabled {
		t.Fatalf("invalid interval normalized to %+v (changed=%v)", cfg, changed)
	}
}

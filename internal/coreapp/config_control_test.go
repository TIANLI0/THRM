package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeLightStripConfig_FillsDefaults(t *testing.T) {
	emptyCfg := types.LightStripConfig{}
	normalized, changed := normalizeLightStripConfig(emptyCfg)
	if !changed {
		t.Fatal("should report changed when filling empty light strip config")
	}
	if normalized.Mode == "" {
		t.Fatal("mode should be filled with default")
	}
	if normalized.Speed == "" {
		t.Fatal("speed should be filled with default")
	}
	if len(normalized.Colors) == 0 {
		t.Fatal("colors should be filled with default")
	}
}

func TestNormalizeLightStripConfig_InvalidBrightness(t *testing.T) {
	cfg := types.LightStripConfig{Brightness: 150}
	normalized, changed := normalizeLightStripConfig(cfg)
	if !changed {
		t.Fatal("should report changed when brightness is out of range")
	}
	if normalized.Brightness == 150 {
		t.Fatal("brightness should be clamped/replaced with default")
	}
}

func TestNormalizeLightStripConfig_ValidNoChange(t *testing.T) {
	defaults := types.GetDefaultLightStripConfig()
	_, changed := normalizeLightStripConfig(defaults)
	if changed {
		t.Fatal("should not report changed for already-valid defaults")
	}
}

func TestCloneManualGearLevels_FromNil(t *testing.T) {
	result := cloneManualGearLevels(nil)
	if len(result) != 4 {
		t.Fatalf("expected 4 gears, got %d", len(result))
	}
	for _, gear := range []string{"静音", "标准", "强劲", "超频"} {
		if result[gear] != "中" {
			t.Fatalf("gear %q should default to '中', got %q", gear, result[gear])
		}
	}
}

func TestCloneManualGearLevels_PreservesValid(t *testing.T) {
	source := map[string]string{"静音": "低", "标准": "高", "强劲": "低", "超频": "高"}
	result := cloneManualGearLevels(source)
	if result["静音"] != "低" || result["超频"] != "高" {
		t.Fatalf("should preserve valid levels: %v", result)
	}
}

func TestCloneManualGearLevels_NormalizesInvalid(t *testing.T) {
	source := map[string]string{"静音": "invalid", "标准": "超高"}
	result := cloneManualGearLevels(source)
	if result["静音"] != "中" {
		t.Fatalf("invalid level should be normalized to '中', got %q", result["静音"])
	}
}

func TestRuntimeDebugInfo_ContainsKeys(t *testing.T) {
	info := runtimeDebugInfo()
	requiredKeys := []string{"goroutines", "allocMB", "heapAllocMB", "heapInUseMB", "numGC"}
	for _, key := range requiredKeys {
		if _, ok := info[key]; !ok {
			t.Fatalf("runtimeDebugInfo should contain key %q", key)
		}
	}
}

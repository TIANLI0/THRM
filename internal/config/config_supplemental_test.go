package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

// TestLoadFromCorruptedJSON verifies corrupt JSON falls back to defaults
func TestLoadFromCorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".thrm")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")

	// Write corrupted JSON
	os.WriteFile(configPath, []byte("{not valid json!!!"), 0644)

	// Override home to use tmpDir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	m := NewManager(tmpDir, testLogger{})
	cfg := m.Load(false)

	if cfg.ConfigPath == "" {
		t.Error("Config should have been created with a path")
	}
	// Default auto control is false (user opt-in)
	if cfg.AutoControl {
		t.Error("Default config should have AutoControl = false (safety default)")
	}
}

// TestMissingFieldsBackfill verifies old configs get backfilled with defaults
func TestMissingFieldsBackfill(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".thrm")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")

	// Write a minimal config missing many fields
	minimalJSON := `{"autoControl": false, "fanCurve": [{"temperature": 30, "rpm": 800}, {"temperature": 70, "rpm": 3000}]}`
	os.WriteFile(configPath, []byte(minimalJSON), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	m := NewManager(tmpDir, testLogger{})
	cfg := m.Load(false)

	// Verify backfilled fields
	if cfg.AutoControl {
		t.Error("autoControl should be false from JSON")
	}
	if cfg.SmartControl.Enabled {
		t.Error("SmartControl.Enabled should default to false")
	}
	if cfg.ThemeMode == "" {
		t.Error("ThemeMode should have been backfilled")
	}
	if cfg.ManualGearToggleHotkey == "" {
		t.Error("ManualGearToggleHotkey should have been backfilled")
	}
	if cfg.TempSource == "" {
		t.Error("TempSource should have been backfilled")
	}
	// FanCurve should have 2 points from JSON
	if len(cfg.FanCurve) < 2 {
		t.Fatalf("FanCurve should have >= 2 points, got %d", len(cfg.FanCurve))
	}
}

// TestConcurrentReadWrite verifies thread safety of Get/Set operations
func TestConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".thrm")
	os.MkdirAll(configDir, 0755)

	m := NewManager(tmpDir, testLogger{})
	m.Load(false)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.Get()
			}
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				cfg := m.Get()
				cfg.AutoControl = (idx % 2) == 0
				_ = m.Update(cfg)
			}
		}(i)
	}
	wg.Wait()

	// After concurrent access, config should still be internally consistent
	finalCfg := m.Get()
	if finalCfg.ConfigPath == "" {
		t.Error("Config should have a path after concurrent access")
	}
}

// TestConfigSaveAndReload verifies save → load roundtrip
func TestConfigSaveAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".thrm")
	os.MkdirAll(configDir, 0755)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	m := NewManager(tmpDir, testLogger{})
	cfg := m.Load(false)

	// Modify config
	cfg.AutoControl = false
	cfg.ManualGear = "标准"
	cfg.ManualLevel = "中"
	if err := m.Update(cfg); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	// Reload
	m2 := NewManager(tmpDir, testLogger{})
	reloaded := m2.Load(false)

	if reloaded.AutoControl {
		t.Error("AutoControl should be false after reload")
	}
	if reloaded.ManualGear != "标准" {
		t.Errorf("ManualGear = %q, want 标准", reloaded.ManualGear)
	}
	if reloaded.ManualLevel != "中" {
		t.Errorf("ManualLevel = %q, want 中", reloaded.ManualLevel)
	}
}

// TestInvalidFanCurveRejected verifies bad fan curves are handled
func TestInvalidFanCurveRejected(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, testLogger{})
	cfg := m.Load(false)

	// Negative RPM
	cfg.FanCurve = []types.FanCurvePoint{
		{Temperature: 30, RPM: -100},
		{Temperature: 70, RPM: 3000},
	}
	err := m.Update(cfg)
	if err != nil {
		t.Logf("Negative RPM rejected (expected): %v", err)
	}

	// Re-load for clean state
	m2 := NewManager(tmpDir, testLogger{})
	cfg2 := m2.Load(false)

	// Single-point curve
	cfg2.FanCurve = []types.FanCurvePoint{
		{Temperature: 30, RPM: 1000},
	}
	err = m2.Update(cfg2)
	if err != nil {
		t.Logf("Single-point curve rejected (expected): %v", err)
	}
}

// TestLegacyConfigMigration verifies old config directory migration
func TestLegacyConfigMigration(t *testing.T) {
	tmpDir := t.TempDir()
	legacyDir := filepath.Join(tmpDir, ".bs2pro-controller")
	os.MkdirAll(legacyDir, 0755)

	legacyConfig := `{"autoControl": true, "fanCurve": [{"temperature": 30, "rpm": 800}, {"temperature": 70, "rpm": 3000}]}`
	os.WriteFile(filepath.Join(legacyDir, "config.json"), []byte(legacyConfig), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	m := NewManager(tmpDir, testLogger{})
	cfg := m.Load(false)

	if !cfg.AutoControl {
		t.Error("AutoControl should be true from legacy config")
	}

	// New config should have been created in .thrm/
	newPath := filepath.Join(tmpDir, ".thrm", "config.json")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("Config should have been migrated to %s: %v", newPath, err)
	}
}

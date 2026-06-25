package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

type testLogger struct{}

func (l testLogger) Info(format string, v ...any)    {}
func (l testLogger) Error(format string, v ...any)   {}
func (l testLogger) Warn(format string, v ...any)    {}
func (l testLogger) Debug(format string, v ...any)   {}
func (l testLogger) Close()                          {}
func (l testLogger) CleanOldLogs()                   {}
func (l testLogger) SetDebugMode(enabled bool)       {}
func (l testLogger) GetLogDir() string               { return "" }

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.GetDefaultConfigDir() == "" {
		t.Error("GetDefaultConfigDir returned empty")
	}
}

func TestGetDefaultConfigDir(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	dir := m.GetDefaultConfigDir()
	if dir == "" {
		t.Error("GetDefaultConfigDir should not be empty")
	}
}

func TestSetAndGet(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	cfg := types.GetDefaultConfig(false)
	m.Set(cfg)
	got := m.Get()
	if got.AutoControl != cfg.AutoControl {
		t.Error("Get() should return the set config")
	}
}

func TestGetWithRevision(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	cfg := types.GetDefaultConfig(false)
	m.Set(cfg)
	_, rev1 := m.GetWithRevision()
	m.Set(cfg)
	_, rev2 := m.GetWithRevision()
	if rev2 <= rev1 {
		t.Errorf("revision should increment: %d <= %d", rev2, rev1)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	homeOrig := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", homeOrig)

	m := NewManager("/tmp", testLogger{})
	cfg := types.GetDefaultConfig(false)
	cfg.AutoControl = true
	m.Set(cfg)

	if err := m.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	m2 := NewManager("/tmp", testLogger{})
	loaded := m2.Load(false)
	if loaded.AutoControl != true {
		t.Error("Loaded config should have AutoControl = true")
	}
}

func TestUpdate(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	cfg := types.GetDefaultConfig(false)
	m.Set(cfg)

	cfg.AutoControl = true
	if err := m.Update(cfg); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	got := m.Get()
	if got.AutoControl != true {
		t.Error("Update should persist changes")
	}
}

func TestGetInstallDir(t *testing.T) {
	dir := GetInstallDir()
	if dir == "" {
		t.Error("GetInstallDir should not be empty")
	}
}

func TestGetCurrentWorkingDir(t *testing.T) {
	dir := GetCurrentWorkingDir()
	if dir == "" {
		t.Error("GetCurrentWorkingDir should not be empty")
	}
}

func TestValidateFanCurve_Valid(t *testing.T) {
	curve := types.GetDefaultFanCurve()
	if err := ValidateFanCurve(curve); err != nil {
		t.Errorf("ValidateFanCurve should pass for default curve: %v", err)
	}
}

func TestValidateFanCurve_TooFewPoints(t *testing.T) {
	curve := []types.FanCurvePoint{{Temperature: 30, RPM: 800}}
	if err := ValidateFanCurve(curve); err == nil {
		t.Error("ValidateFanCurve should fail for < 2 points")
	}
}

func TestValidateFanCurve_NonIncreasingTemp(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 50, RPM: 1500},
		{Temperature: 30, RPM: 2000},
	}
	if err := ValidateFanCurve(curve); err == nil {
		t.Error("ValidateFanCurve should fail for decreasing temperature")
	}
}

func TestValidateFanCurve_DecreasingRPM(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 2000},
		{Temperature: 50, RPM: 1000},
	}
	if err := ValidateFanCurve(curve); err == nil {
		t.Error("ValidateFanCurve should fail for decreasing RPM")
	}
}

func TestValidateFanCurve_OutOfRange(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 5000},
		{Temperature: 50, RPM: 5000},
	}
	if err := ValidateFanCurve(curve); err == nil {
		t.Error("ValidateFanCurve should fail for RPM out of range")
	}
}

func TestConfigDirConsistency(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	if m.GetConfigDir() == "" {
		t.Error("GetConfigDir should not be empty")
	}
	if m.GetConfigDir() != m.GetDefaultConfigDir() {
		t.Error("GetConfigDir should equal GetDefaultConfigDir")
	}
}

func TestLegacyConfigDir(t *testing.T) {
	m := NewManager("/tmp", testLogger{})
	legacy := m.GetLegacyConfigDir()
	if legacy == "" {
		t.Error("GetLegacyConfigDir should not be empty")
	}
	if legacy == m.GetDefaultConfigDir() {
		t.Error("Legacy and default config dirs should differ")
	}
}

func TestLoad_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cfgDir := filepath.Join(tmpDir, ".thrm")
	os.RemoveAll(cfgDir)
	defer os.RemoveAll(cfgDir)

	m := NewManager("/tmp", testLogger{})
	cfg := m.Load(false)
	if len(cfg.FanCurve) == 0 {
		t.Error("Load with no config should return default config with FanCurve")
	}
}

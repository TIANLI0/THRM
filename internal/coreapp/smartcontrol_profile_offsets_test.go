package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func makeTestConfig(activeID string, curveLen int, offsets []int) *types.AppConfig {
	cfg := &types.AppConfig{
		ActiveFanCurveProfileID: activeID,
		FanCurve:                make([]types.FanCurvePoint, curveLen),
		FanCurveProfiles: []types.FanCurveProfile{
			{ID: "profile1", Name: "Profile 1", Curve: make([]types.FanCurvePoint, curveLen)},
		},
		SmartControl: types.SmartControlConfig{
			LearnedOffsets:          offsets,
			LearnedOffsetsByProfile: map[string][]int{},
			LearningBias:            "balanced",
			TargetTemp:              68,
		},
	}
	return cfg
}

func TestSyncSmartControlOffsets_NilConfig(t *testing.T) {
	if syncSmartControlOffsetsForActiveProfile(nil) {
		t.Fatal("syncSmartControlOffsetsForActiveProfile should return false for nil config")
	}
}

func TestSyncSmartControlOffsets_NoActiveProfile(t *testing.T) {
	cfg := &types.AppConfig{
		ActiveFanCurveProfileID: "",
		SmartControl: types.SmartControlConfig{
			LearnedOffsets:          []int{1, 2, 3},
			LearnedOffsetsByProfile: map[string][]int{},
		},
	}
	if syncSmartControlOffsetsForActiveProfile(cfg) {
		t.Fatal("should return false when no active profile")
	}
}

func TestSyncSmartControlOffsets_TransferToEmptyProfile(t *testing.T) {
	cfg := makeTestConfig("profile1", 3, []int{100, 200, 300})
	changed := syncSmartControlOffsetsForActiveProfile(cfg)
	if !changed {
		t.Fatal("should report changed when transferring offsets to empty profile")
	}
	profileOffsets := cfg.SmartControl.LearnedOffsetsByProfile["profile1"]
	if len(profileOffsets) != 3 {
		t.Fatalf("expected 3 offsets, got %d", len(profileOffsets))
	}
	if profileOffsets[0] != 100 || profileOffsets[1] != 200 || profileOffsets[2] != 300 {
		t.Fatalf("offsets not copied correctly: %v", profileOffsets)
	}
}

func TestSyncSmartControlOffsets_NoChangeOnExisting(t *testing.T) {
	cfg := makeTestConfig("profile1", 3, []int{100, 200, 300})
	cfg.SmartControl.LearnedOffsetsByProfile["profile1"] = []int{100, 200, 300}
	changed := syncSmartControlOffsetsForActiveProfile(cfg)
	if changed {
		t.Fatal("should not report changed when offsets already match")
	}
}

func TestSyncSmartControlOffsets_DetectedChange(t *testing.T) {
	cfg := makeTestConfig("profile1", 3, []int{50, 50, 50})
	cfg.SmartControl.LearnedOffsetsByProfile["profile1"] = []int{100, 200, 300}
	changed := syncSmartControlOffsetsForActiveProfile(cfg)
	if !changed {
		t.Fatal("should report changed when offsets differ from stored profile")
	}
}

func TestStoreSmartControlOffsets_NilConfig(t *testing.T) {
	if storeSmartControlOffsetsForActiveProfile(nil) {
		t.Fatal("should return false for nil config")
	}
}

func TestStoreSmartControlOffsets_NoActiveProfile(t *testing.T) {
	cfg := &types.AppConfig{
		ActiveFanCurveProfileID: "",
		SmartControl: types.SmartControlConfig{
			LearnedOffsets: []int{1, 2, 3},
		},
	}
	if storeSmartControlOffsetsForActiveProfile(cfg) {
		t.Fatal("should return false when no active profile")
	}
}

func TestSyncThenStore_RoundTrip(t *testing.T) {
	cfg := makeTestConfig("profile1", 4, []int{10, 20, 30, 40})

	syncSmartControlOffsetsForActiveProfile(cfg)
	stored := cfg.SmartControl.LearnedOffsetsByProfile["profile1"]
	if len(stored) != 4 {
		t.Fatalf("sync should store 4 offsets, got %d", len(stored))
	}

	cfg.SmartControl.LearnedOffsets = []int{99, 99, 99, 99}
	storeSmartControlOffsetsForActiveProfile(cfg)
	storedAfterStore := cfg.SmartControl.LearnedOffsetsByProfile["profile1"]
	if storedAfterStore[0] != 99 {
		t.Fatalf("store should update profile offsets: %v", storedAfterStore)
	}

	cfg.SmartControl.LearnedOffsets = []int{0, 0, 0, 0}
	changed := syncSmartControlOffsetsForActiveProfile(cfg)
	if !changed {
		t.Fatal("resync should detect change after store updated profile offsets")
	}
	if cfg.SmartControl.LearnedOffsets[0] != 99 {
		t.Fatalf("resync should restore offsets from profile: %v", cfg.SmartControl.LearnedOffsets)
	}
}

func TestSyncSmartControlOffsets_NilLearnedOffsets(t *testing.T) {
	cfg := makeTestConfig("profile1", 3, nil)
	cfg.SmartControl.LearnedOffsets = nil
	changed := syncSmartControlOffsetsForActiveProfile(cfg)
	if !changed {
		t.Fatal("should report changed when creating offset array from scratch")
	}
	if len(cfg.SmartControl.LearnedOffsets) != 3 {
		t.Fatalf("should create offset array of curve length 3, got %d", len(cfg.SmartControl.LearnedOffsets))
	}
}

func TestCloneIntSlice_Zero(t *testing.T) {
	result := cloneIntSlice([]int{})
	if result != nil {
		t.Fatal("cloneIntSlice of empty should return nil")
	}
}

func TestCloneIntSlice_Nil(t *testing.T) {
	result := cloneIntSlice(nil)
	if result != nil {
		t.Fatal("cloneIntSlice of nil should return nil")
	}
}

func TestCloneIntSlice_Independent(t *testing.T) {
	original := []int{1, 2, 3}
	clone := cloneIntSlice(original)
	original[0] = 99
	if clone[0] != 1 {
		t.Fatal("clone should be independent of original")
	}
}

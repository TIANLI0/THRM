package coreapp

import "github.com/TIANLI0/THRM/internal/types"

func cloneIntSlice(input []int) []int {
	if len(input) == 0 {
		return nil
	}
	out := make([]int, len(input))
	copy(out, input)
	return out
}

func syncSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
	}
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return false
	}

	changed := false
	if _, ok := cfg.SmartControl.LearnedOffsetsByProfile[activeID]; !ok {
		if cfg.SmartControl.LearnedOffsets != nil {
			cfg.SmartControl.LearnedOffsetsByProfile[activeID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
			changed = true
		} else {
			cfg.SmartControl.LearnedOffsetsByProfile[activeID] = make([]int, len(cfg.FanCurve))
			changed = true
		}
	}

	loaded := cfg.SmartControl.LearnedOffsetsByProfile[activeID]
	if loaded == nil {
		loaded = make([]int, len(cfg.FanCurve))
		cfg.SmartControl.LearnedOffsetsByProfile[activeID] = loaded
		changed = true
	}
	if cfg.SmartControl.LearnedOffsets == nil || len(cfg.SmartControl.LearnedOffsets) != len(loaded) {
		cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
		changed = true
	} else {
		for i := range loaded {
			if cfg.SmartControl.LearnedOffsets[i] != loaded[i] {
				cfg.SmartControl.LearnedOffsets = cloneIntSlice(loaded)
				changed = true
				break
			}
		}
	}
	return changed
}

func storeSmartControlOffsetsForActiveProfile(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}
	activeID := cfg.ActiveFanCurveProfileID
	if activeID == "" {
		return false
	}
	if cfg.SmartControl.LearnedOffsetsByProfile == nil {
		cfg.SmartControl.LearnedOffsetsByProfile = map[string][]int{}
	}
	cfg.SmartControl.LearnedOffsetsByProfile[activeID] = cloneIntSlice(cfg.SmartControl.LearnedOffsets)
	return true
}

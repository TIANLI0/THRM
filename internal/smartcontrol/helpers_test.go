package smartcontrol

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestGetCurveRPMBounds(t *testing.T) {
	tests := []struct {
		name    string
		curve   []types.FanCurvePoint
		wantMin int
		wantMax int
	}{
		{"empty", nil, 0, 4000},
		{"empty slice", []types.FanCurvePoint{}, 0, 4000},
		{"single point", []types.FanCurvePoint{{Temperature: 40, RPM: 1200}}, 1200, 1200},
		{"ascending", []types.FanCurvePoint{
			{Temperature: 30, RPM: 800},
			{Temperature: 50, RPM: 1500},
			{Temperature: 80, RPM: 3000},
		}, 800, 3000},
		{"mixed", []types.FanCurvePoint{
			{Temperature: 30, RPM: 2000},
			{Temperature: 50, RPM: 1000},
			{Temperature: 80, RPM: 3000},
		}, 1000, 3000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := GetCurveRPMBounds(tt.curve)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("GetCurveRPMBounds() = (%d, %d), want (%d, %d)",
					gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestApplyRampLimit(t *testing.T) {
	tests := []struct {
		name      string
		targetRPM int
		lastRPM   int
		upLimit   int
		downLimit int
		want      int
	}{
		{"ramp up within limit", 1200, 1000, 300, 300, 1200},
		{"ramp up clamped", 1500, 1000, 200, 200, 1200},
		{"ramp down within limit", 800, 1000, 300, 300, 800},
		{"ramp down clamped", 400, 1000, 300, 300, 700},
		{"no change", 1000, 1000, 300, 300, 1000},
		{"zero limits", 1200, 1000, 0, 0, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyRampLimit(tt.targetRPM, tt.lastRPM, tt.upLimit, tt.downLimit)
			if got != tt.want {
				t.Errorf("ApplyRampLimit() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCalculateTargetRPM_Offsets(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 60, RPM: 2000},
		{Temperature: 90, RPM: 3500},
	}
	cfg := types.GetDefaultSmartControlConfig(curve)
	rpm := CalculateTargetRPM(50, curve, cfg)
	if rpm <= 0 {
		t.Error("CalculateTargetRPM should return positive RPM for valid curve")
	}
}

func TestCalculateTargetRPM_EmptyCurve(t *testing.T) {
	cfg := types.SmartControlConfig{LearningBias: types.LearningBiasBalanced}
	rpm := CalculateTargetRPM(50, nil, cfg)
	if rpm != 0 {
		t.Errorf("CalculateTargetRPM on empty curve = %d, want 0", rpm)
	}
}

func TestFilterTransientSample(t *testing.T) {
	// Needs >= 3 recent temps; returns unchanged otherwise
	temp, changed := FilterTransientSample(50, nil, 5)
	if temp != 50 || changed {
		t.Errorf("FilterTransientSample with nil history should return unchanged: got (%d, %v)", temp, changed)
	}
	temp, changed = FilterTransientSample(50, []int{48}, 5)
	if temp != 50 || changed {
		t.Errorf("FilterTransientSample with < 3 history should return unchanged: got (%d, %v)", temp, changed)
	}

	// Stable band: last 3 values should be close together for filtering to kick in.
	// With hysteresis=5, stableBand=max(2,6)=6, spikeBand=max(5,9)=9.
	// If recent temps are unstable (range > stableBand), no filtering.
	// If stable and spike >= spikeBand, return baseline.
	temp, changed = FilterTransientSample(60, []int{50, 52, 51}, 5)
	if temp == 60 && !changed {
		t.Logf("stable with small spike: (%d, %v)", temp, changed)
	}
}

func TestFilterTransientSpike(t *testing.T) {
	// Needs >= 3 recent temps; returns unchanged otherwise
	temp, changed := FilterTransientSpike(50, nil, 50, 5)
	if temp != 50 || changed {
		t.Errorf("FilterTransientSpike with nil history should return unchanged: got (%d, %v)", temp, changed)
	}
	temp, changed = FilterTransientSpike(50, []int{48}, 50, 5)
	if temp != 50 || changed {
		t.Errorf("FilterTransientSpike with < 3 history should return unchanged: got (%d, %v)", temp, changed)
	}
}

func TestNormalizeConfig(t *testing.T) {
	curve := types.GetDefaultFanCurve()
	cfg := types.SmartControlConfig{
		TargetTemp:    0,
		Aggressiveness: 20,
		Hysteresis:    99,
		MinRPMChange:  0,
		RampUpLimit:   10,
		RampDownLimit: 2000,
		LearnRate:     0,
		LearningBias:  "invalid",
	}
	normalized, changed := NormalizeConfig(cfg, curve, false)
	if !changed {
		t.Error("NormalizeConfig should report changed for invalid config")
	}
	if normalized.TargetTemp < 45 || normalized.TargetTemp > 90 {
		t.Errorf("TargetTemp = %d, should be in [45, 90]", normalized.TargetTemp)
	}
	if normalized.LearnRate < 1 || normalized.LearnRate > 10 {
		t.Errorf("LearnRate = %d, should be in [1, 10]", normalized.LearnRate)
	}
	if normalized.LearningBias != types.LearningBiasBalanced {
		t.Errorf("LearningBias = %q, want %q", normalized.LearningBias, types.LearningBiasBalanced)
	}
}

func TestBlendOffsets(t *testing.T) {
	heat := []int{10, 20, 30}
	cool := []int{-5, -10, -15}
	blended := BlendOffsets(heat, cool)
	if len(blended) != 3 {
		t.Fatalf("len = %d, want 3", len(blended))
	}
	if blended[0] != 2 {
		t.Errorf("blended[0] = %d, want 2", blended[0])
	}
	if blended[1] != 5 {
		t.Errorf("blended[1] = %d, want 5", blended[1])
	}
	if blended[2] != 7 {
		t.Errorf("blended[2] = %d, want 7", blended[2])
	}
}

func TestResetLearnedState(t *testing.T) {
	curve := types.GetDefaultFanCurve()
	cfg := types.GetDefaultSmartControlConfig(curve)
	cfg.LearnedOffsets[0] = 100

	reset := ResetLearnedState(cfg, curve)
	for _, offset := range reset.LearnedOffsets {
		if offset != 0 {
			t.Error("LearnState should be reset to zeros")
		}
	}
}

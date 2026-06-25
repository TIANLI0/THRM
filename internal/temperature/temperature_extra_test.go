package temperature

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestCalculateTargetRPM_Simple(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 50, RPM: 1500},
	}
	rpm := CalculateTargetRPM(40, curve)
	if rpm < 800 || rpm > 1500 {
		t.Errorf("CalculateTargetRPM(40) = %d, want [800, 1500]", rpm)
	}
}

func TestCalculateTargetRPM_BelowMin(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 50, RPM: 1500},
	}
	rpm := CalculateTargetRPM(20, curve)
	if rpm != 800 {
		t.Errorf("CalculateTargetRPM(20) = %d, want 800", rpm)
	}
}

func TestCalculateTargetRPM_AboveMax(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 50, RPM: 1500},
	}
	rpm := CalculateTargetRPM(60, curve)
	if rpm != 1500 {
		t.Errorf("CalculateTargetRPM(60) = %d, want 1500", rpm)
	}
}

func TestCalculateTargetRPM_ExactPoint(t *testing.T) {
	curve := []types.FanCurvePoint{
		{Temperature: 30, RPM: 800},
		{Temperature: 50, RPM: 1500},
	}
	rpm := CalculateTargetRPM(30, curve)
	if rpm != 800 {
		t.Errorf("CalculateTargetRPM(30) = %d, want 800", rpm)
	}
	rpm = CalculateTargetRPM(50, curve)
	if rpm != 1500 {
		t.Errorf("CalculateTargetRPM(50) = %d, want 1500", rpm)
	}
}

func TestCalculateTargetRPM_EmptyCurve(t *testing.T) {
	rpm := CalculateTargetRPM(40, nil)
	if rpm != 0 {
		t.Errorf("CalculateTargetRPM on nil curve = %d, want 0", rpm)
	}
	rpm = CalculateTargetRPM(40, []types.FanCurvePoint{})
	if rpm != 0 {
		t.Errorf("CalculateTargetRPM on empty curve = %d, want 0", rpm)
	}
}

func TestCalculateTargetRPM_DefaultCurve(t *testing.T) {
	curve := types.GetDefaultFanCurve()
	if len(curve) < 2 {
		t.Fatal("Default fan curve should have at least 2 points")
	}
	rpm := CalculateTargetRPM(60, curve)
	if rpm <= 0 {
		t.Errorf("CalculateTargetRPM(60) = %d, should be positive", rpm)
	}
}

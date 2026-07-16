package smartcontrol

import "testing"

func TestDecayLaptopFanPeak(t *testing.T) {
	if got := DecayLaptopFanPeak(0, 3000); got != 3000 {
		t.Fatalf("新读数应刷新峰值, got %d", got)
	}
	if got := DecayLaptopFanPeak(3000, 0); got != 3000-laptopFanPeakDecayPerTick {
		t.Fatalf("无读数时峰值应衰减, got %d", got)
	}
	if got := DecayLaptopFanPeak(20, 0); got != 0 {
		t.Fatalf("峰值不应为负, got %d", got)
	}
}

func TestApplyLaptopFanGuard(t *testing.T) {
	// 本机风扇仍接近峰值 → 限制单周期降幅
	rpm, guarded := ApplyLaptopFanGuard(2000, 3000, 4500, 5000)
	if !guarded || rpm != 3000-laptopFanGuardMaxDropPerTick {
		t.Fatalf("应缓降: got rpm=%d guarded=%v", rpm, guarded)
	}
	// 本机风扇已明显回落 → 不干预
	if rpm, guarded = ApplyLaptopFanGuard(2000, 3000, 3000, 5000); guarded || rpm != 2000 {
		t.Fatalf("不应干预: got rpm=%d guarded=%v", rpm, guarded)
	}
	// 升速方向不受影响
	if rpm, guarded = ApplyLaptopFanGuard(3500, 3000, 5000, 5000); guarded || rpm != 3500 {
		t.Fatalf("升速不应受影响: got rpm=%d guarded=%v", rpm, guarded)
	}
	// 不支持读取本机风扇（0 值）→ 不干预
	if rpm, guarded = ApplyLaptopFanGuard(2000, 3000, 0, 0); guarded || rpm != 2000 {
		t.Fatalf("无本机读数不应干预: got rpm=%d guarded=%v", rpm, guarded)
	}
	// 目标降幅本就很小 → 不放大
	if rpm, guarded = ApplyLaptopFanGuard(2980, 3000, 5000, 5000); guarded || rpm != 2980 {
		t.Fatalf("小降幅不应干预: got rpm=%d guarded=%v", rpm, guarded)
	}
}

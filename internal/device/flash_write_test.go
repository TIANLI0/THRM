package device

import (
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

// 固件在 BLE HID 分支上只接受恰好 24 字节的写入（0x2A4D 特征，长度不符回 ATT 0x0D），
// 而 hidapi 在 Windows 上只会把偏小的缓冲区补齐、偏大的原样交给系统拒绝。
// 灯效命令一旦用回 65，蓝牙连接下整组灯效就全发不出去。
func TestLightReportLengthMatchesBLEHIDWindow(t *testing.T) {
	if hidLightReportLen != hidControlReportLen {
		t.Fatalf("hidLightReportLen = %d, must equal hidControlReportLen = %d", hidLightReportLen, hidControlReportLen)
	}
	if hidControlReportLen != 25 {
		t.Fatalf("hidControlReportLen = %d, firmware accepts exactly 24 payload bytes plus one report ID", hidControlReportLen)
	}

	// 0x47 是最长的灯效帧：4 字节帧头 + 帧索引 + 10 字节数据 + 校验和。
	longest := deviceproto.BuildFrame(deviceproto.CmdRGBFrameWrite, make([]byte, 1+lightUploadFrameSize)...)
	if got := len(deviceproto.BuildReport(longest, hidLightReportLen)); got != hidLightReportLen {
		t.Fatalf("longest light report = %d bytes, want %d", got, hidLightReportLen)
	}
}

// 上传帧数必须落在固件流缓冲区容量内，否则 0x47 会越界写坏相邻 RAM。
func TestLightUploadFrameCountFitsFirmwareBuffer(t *testing.T) {
	if lightUploadFrameCount > deviceproto.RGBFrameCount {
		t.Fatalf("lightUploadFrameCount = %d exceeds the safe firmware bound %d", lightUploadFrameCount, deviceproto.RGBFrameCount)
	}
	if lightUploadFrameCount*lightUploadFrameSize < lightProgramSize {
		t.Fatalf("lightUploadFrameCount = %d does not cover the %d-byte program", lightUploadFrameCount, lightProgramSize)
	}
}

func TestAwaitFlashWriteWindowSpacesConsecutiveWrites(t *testing.T) {
	m := &Manager{}

	// 第一次落盘不需要等待。
	start := time.Now()
	m.awaitFlashWriteWindowLocked()
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("first flash write waited %v, want no wait", elapsed)
	}

	m.noteFlashWriteLocked()
	start = time.Now()
	m.awaitFlashWriteWindowLocked()
	if elapsed := time.Since(start); elapsed < flashWriteSpacing/2 {
		t.Fatalf("back-to-back flash writes waited %v, want at least %v", elapsed, flashWriteSpacing/2)
	}

	// 距离足够远时不再等待。
	m.lastFlashWriteAt = time.Now().Add(-2 * flashWriteSpacing)
	start = time.Now()
	m.awaitFlashWriteWindowLocked()
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("spaced flash write waited %v, want no wait", elapsed)
	}
}

func TestGearRPMCacheSeedingAndReset(t *testing.T) {
	m := &Manager{}
	m.NoteGearRPMTableFromDevice([]types.DeviceGearRPM{
		{Gear: 1, RPM: 1700},
		{Gear: 4, RPM: 4000},
		{Gear: 0, RPM: 999}, // 越界，必须忽略
		{Gear: 9, RPM: 999}, // 越界，必须忽略
		{Gear: 2, RPM: 0},   // 无效转速，必须忽略
	})

	if !m.hasDeviceGearRPM[0] || m.deviceGearRPM[0] != 1700 {
		t.Fatalf("gear 1 cache = (%v, %d), want (true, 1700)", m.hasDeviceGearRPM[0], m.deviceGearRPM[0])
	}
	if !m.hasDeviceGearRPM[3] || m.deviceGearRPM[3] != 4000 {
		t.Fatalf("gear 4 cache = (%v, %d), want (true, 4000)", m.hasDeviceGearRPM[3], m.deviceGearRPM[3])
	}
	if m.hasDeviceGearRPM[1] {
		t.Fatal("a zero RPM readback must not seed the cache")
	}

	// 维护命令与断连之后缓存必须作废，否则会用 0x08 换到一个转速对不上的挡位。
	m.resetGearRPMCacheLocked()
	for idx, known := range m.hasDeviceGearRPM {
		if known {
			t.Fatalf("gear slot %d still cached after reset", idx)
		}
	}
}

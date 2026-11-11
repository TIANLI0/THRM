package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

// BS1 的 0xEF 首字节在两条传输上编码不同：
//
//	USB HID：0x30/0x32/0x34 = 静音/标准/强劲（0x30 + 2×(挡位-1)）
//	BLE    ：高 4 位最大挡位、低 4 位当前挡位（8/A/C/E = 静音…超频）
//
// 下面的 payload 全部取自 2026-11-05 的真机抓包，不是构造出来的。
var bs1HIDGearPayloads = []struct {
	name         string
	payload      []byte
	wantSetGear  string
	wantSelected int
}{
	{
		name:         "静音挡 0x30",
		payload:      []byte{0x30, 0x02, 0x01, 0x98, 0x08, 0x14, 0x05, 0x01, 0xFB},
		wantSetGear:  "静音",
		wantSelected: 1,
	},
	{
		name:         "标准挡 0x32",
		payload:      []byte{0x32, 0x02, 0x01, 0x98, 0x08, 0x34, 0x08, 0x04, 0xFB},
		wantSetGear:  "标准",
		wantSelected: 2,
	},
	{
		name:         "强劲挡 0x34",
		payload:      []byte{0x34, 0x02, 0x01, 0x34, 0x08, 0x8C, 0x0A, 0x07, 0xFB},
		wantSetGear:  "强劲",
		wantSelected: 3,
	},
	{
		// 实时模式下挡位字节保持不变，只有工作模式字节从 0x02 变 0x03。
		name:         "标准挡 + 实时模式 0x32/0x03",
		payload:      []byte{0x32, 0x03, 0x01, 0x34, 0x08, 0xD0, 0x07, 0x94, 0xFB},
		wantSetGear:  "标准",
		wantSelected: 2,
	},
}

// HID 路径必须解析出挡位名与归一化挡位号，而不是过去的"未知(0x0)"。
func TestHandleHIDFrameDecodesGear(t *testing.T) {
	for _, tt := range bs1HIDGearPayloads {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBLEManager(nil)
			frame := deviceproto.BuildFrame(deviceproto.CmdStatusNotify, tt.payload...)
			b.handleHIDFrame(makeBS1HIDInputReport(frame))

			data := b.GetCurrentFanData()
			if data == nil {
				t.Fatalf("0xEF 状态帧未被解析成 FanData")
			}
			if data.SetGear != tt.wantSetGear {
				t.Errorf("SetGear = %q，期望 %q（原始字节 0x%02X）",
					data.SetGear, tt.wantSetGear, data.GearSettings)
			}
			if data.SelectedGear != tt.wantSelected {
				t.Errorf("SelectedGear = %d，期望 %d", data.SelectedGear, tt.wantSelected)
			}
			// HID 的单字节不携带最大挡位信息。
			if data.MaxGear != "" {
				t.Errorf("MaxGear = %q，期望空串（HID 无此信息）", data.MaxGear)
			}
		})
	}
}

// 无法识别的 HID 挡位字节返回空串而不是"未知(0xNN)"，界面才会显示 "--"。
func TestBS1HIDGearNameUnknown(t *testing.T) {
	for _, b := range []uint8{0x00, 0x31, 0x33, 0x35, 0x38, 0xFF} {
		if got := bs1HIDGearName(b); got != "" {
			t.Errorf("bs1HIDGearName(0x%02X) = %q，期望空串", b, got)
		}
		if got := decodeBS1SelectedGear(b, bs1GearEncodingHIDASCII); got != 0 {
			t.Errorf("decodeBS1SelectedGear(0x%02X) = %d，期望 0", b, got)
		}
	}
}

// 超频 0x36 是 0x30+2×(挡位-1) 的延伸，未实测（要 PD 供电），但公式必须自洽。
func TestBS1HIDGearNameOverclock(t *testing.T) {
	if got := bs1HIDGearName(0x36); got != "超频" {
		t.Errorf("bs1HIDGearName(0x36) = %q，期望 超频", got)
	}
	if got := decodeBS1SelectedGear(0x36, bs1GearEncodingHIDASCII); got != 4 {
		t.Errorf("decodeBS1SelectedGear(0x36) = %d，期望 4", got)
	}
}

// BLE 位域路径保持原样：0x6A = 最大挡位超频(6) + 当前挡位标准(A)。
func TestParseBS1NotificationBitfieldUnchanged(t *testing.T) {
	b := NewBLEManager(nil)
	payload := []byte{0x6A, 0x02, 0x01, 0x34, 0x08, 0x34, 0x08, 0x4D, 0x16}
	frame := deviceproto.BuildFrame(deviceproto.CmdStatusNotify, payload...)

	data := b.parseBS1Notification(frame, bs1GearEncodingBitfield)
	if data == nil {
		t.Fatalf("BLE 位域帧未被解析")
	}
	if data.SetGear != "标准" || data.MaxGear != "超频" {
		t.Errorf("SetGear/MaxGear = %q/%q，期望 标准/超频", data.SetGear, data.MaxGear)
	}
	if data.SelectedGear != 2 {
		t.Errorf("SelectedGear = %d，期望 2", data.SelectedGear)
	}
}

// 设备状态里也不能把 HID 的挡位字节当位域解：0x34 的低 4 位是 0x4，
// 按位域会变成 "unknown(0x4)"；HID 没有最大挡位信息，也不该硬编出 "unknown(0x3)"。
func TestApplyCurrentStatusHIDGear(t *testing.T) {
	settings := &types.DeviceSettings{}
	applyCurrentStatus(settings, &types.FanData{
		GearSettings: 0x34,
		SelectedGear: 3,
		SetGear:      "强劲",
		CurrentMode:  0x02,
	})

	if settings.SelectedGear != 3 {
		t.Errorf("SelectedGear = %d，期望 3", settings.SelectedGear)
	}
	if settings.Status == nil {
		t.Fatalf("Status 未填充")
	}
	if settings.Status.Selected != "performance" {
		t.Errorf("Status.Selected = %q，期望 performance", settings.Status.Selected)
	}
	if settings.Status.MaxGear != "" {
		t.Errorf("Status.MaxGear = %q，期望空串（HID 无此信息）", settings.Status.MaxGear)
	}
}

// BLE 位域路径必须与原实现逐字节一致：新加的 decodeBS1SelectedGear 和
// deviceproto.DecodeSelectedGear 是同一套映射的两份实现，这里穷举 0..255
// 把两者钉死，避免以后只改一处。
func TestBS1GearDecodersAgreeOnBitfield(t *testing.T) {
	for b := 0; b < 256; b++ {
		want := deviceproto.DecodeSelectedGear(byte(b))
		got := decodeBS1SelectedGear(byte(b), bs1GearEncodingBitfield)
		if got != want {
			t.Errorf("字节 0x%02X：新实现返回 %d，原实现返回 %d", b, got, want)
		}
	}
}

// 两条传输的判定必须自洽：HID 下能给出挡位名的字节，归一化结果也必须非 0。
func TestBS1HIDGearNameAndIndexAgree(t *testing.T) {
	for b := 0; b < 256; b++ {
		name := bs1HIDGearName(byte(b))
		gear := decodeBS1SelectedGear(byte(b), bs1GearEncodingHIDASCII)
		if (name == "") != (gear == 0) {
			t.Errorf("字节 0x%02X：挡位名 %q 与挡位号 %d 不一致", b, name, gear)
		}
	}
}

// 唤醒序列要用归一化挡位：HID 下 0x34 的低 4 位是 0x4，按位域解会退回静音。
func TestFixedGearForWakePrefersSelectedGear(t *testing.T) {
	tests := []struct {
		name string
		data *types.FanData
		want byte
	}{
		{"HID 强劲挡", &types.FanData{GearSettings: 0x34, SelectedGear: 3}, 3},
		{"HID 超频挡", &types.FanData{GearSettings: 0x36, SelectedGear: 4}, 4},
		{"只有位域字节时按位域解", &types.FanData{GearSettings: 0x0C}, 3},
		{"挡位未知时退回静音", &types.FanData{GearSettings: 0x30}, 1},
		{"nil 退回静音", nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fixedGearForWake(tt.data); got != tt.want {
				t.Errorf("fixedGearForWake() = %d，期望 %d", got, tt.want)
			}
		})
	}
}

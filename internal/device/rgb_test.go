package device

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestLightProgramFirmwareLayout(t *testing.T) {
	program := newLightProgram(5, 10, 25)
	if got, want := program[0][:6], []byte{0, 2, 0, 5, 10, 25}; !equalBytes(got, want) {
		t.Fatalf("header = %v, want %v", got, want)
	}

	first := types.RGBColor{R: 255, G: 128, B: 64}
	last := types.RGBColor{R: 1, G: 2, B: 3}
	program.setColor(0, 0, first)
	program.setColor(5, 9, last)

	if got := program[0][6:9]; !equalBytes(got, []byte{255, 128, 64}) {
		t.Fatalf("first LED/keyframe color = %v", got)
	}
	if got := program[18][3:6]; !equalBytes(got, []byte{1, 2, 3}) {
		t.Fatalf("last LED/keyframe color = %v", got)
	}
	if program[0][5] != 25 || program[0][6] != 255 {
		t.Fatal("RGB values must remain unscaled; firmware applies the header brightness")
	}
}

func TestLightProgramUsesLEDMajorColorOrder(t *testing.T) {
	program := newLightProgram(1, 5, 100)
	program.setColor(1, 0, types.RGBColor{R: 10, G: 20, B: 30})
	program.setColor(0, 1, types.RGBColor{R: 40, G: 50, B: 60})

	// Header(6) + LED(1)*10*RGB(3) = byte 36.
	if got := program[3][6:9]; !equalBytes(got, []byte{10, 20, 30}) {
		t.Fatalf("LED-major offset = %v", got)
	}
	// Header(6) + keyframe(1)*RGB(3) = byte 9, crossing a 10-byte upload frame.
	if program[0][9] != 40 || program[1][0] != 50 || program[1][1] != 60 {
		t.Fatalf("cross-frame RGB = [%d %d %d]", program[0][9], program[1][0], program[1][1])
	}
}

// 温度区间现在由用户配置。这里锁住默认区间（<=70 绿 / 71..80 黄 / >80 红）
// 与 2°C 回差下的判定结果：区间下标 i 的下界记作 B，向上进入需要 temp >= B+回差，
// 向下退出需要 temp < B-回差。
func TestSelectSmartTempLightBandWithDefaults(t *testing.T) {
	bands := types.GetDefaultSmartTempLightBands()
	const hysteresis = types.DefaultSmartTempLightHysteresis

	tests := []struct {
		name        string
		temperature int
		current     int
		wantIndex   int
		wantPreset  int
	}{
		{name: "initial green", temperature: 70, current: -1, wantIndex: 0, wantPreset: 1},
		{name: "initial yellow", temperature: 71, current: -1, wantIndex: 1, wantPreset: 2},
		{name: "initial red", temperature: 81, current: -1, wantIndex: 2, wantPreset: 3},
		{name: "green holds inside hysteresis", temperature: 72, current: 0, wantIndex: 0, wantPreset: 1},
		{name: "green to yellow", temperature: 73, current: 0, wantIndex: 1, wantPreset: 2},
		{name: "yellow holds inside hysteresis", temperature: 69, current: 1, wantIndex: 1, wantPreset: 2},
		{name: "yellow to green", temperature: 68, current: 1, wantIndex: 0, wantPreset: 1},
		{name: "yellow to red", temperature: 83, current: 1, wantIndex: 2, wantPreset: 3},
		{name: "red holds inside hysteresis", temperature: 79, current: 2, wantIndex: 2, wantPreset: 3},
		{name: "red to yellow", temperature: 78, current: 2, wantIndex: 1, wantPreset: 2},
		{name: "a large drop falls straight to the bottom band", temperature: 40, current: 2, wantIndex: 0, wantPreset: 1},
		{name: "a large rise jumps straight to the top band", temperature: 95, current: 0, wantIndex: 2, wantPreset: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := types.SelectSmartTempLightBand(bands, hysteresis, test.temperature, test.current)
			if index != test.wantIndex {
				t.Fatalf("band(%d°C, current=%d) = %d, want %d", test.temperature, test.current, index, test.wantIndex)
			}
			if got := bands[index].Preset; got != test.wantPreset {
				t.Fatalf("preset = %d, want %d", got, test.wantPreset)
			}
		})
	}
}

// 用户可以自定义区间数量、边界和预设，判定必须完全跟着配置走，
// 而不是继续用写死的 70/80°C。
func TestSelectSmartTempLightBandFollowsUserConfiguration(t *testing.T) {
	bands := []types.SmartTempLightBand{
		{MinTemp: 0, Preset: 4},
		{MinTemp: 55, Preset: 1},
		{MinTemp: 90, Preset: 5},
	}
	cases := []struct {
		temperature int
		wantPreset  int
	}{
		{temperature: 30, wantPreset: 4},
		{temperature: 55, wantPreset: 1},
		{temperature: 89, wantPreset: 1},
		{temperature: 90, wantPreset: 5},
	}
	for _, test := range cases {
		index := types.SelectSmartTempLightBand(bands, 0, test.temperature, -1)
		if got := bands[index].Preset; got != test.wantPreset {
			t.Fatalf("preset(%d°C) = %d, want %d", test.temperature, got, test.wantPreset)
		}
	}
}

// 切回原生预设时不再无条件先关再开灯效：0x46 每次都会擦写一页固件数据闪存，
// 而这里的开/关本来就是同一个值，纯属浪费一次擦写。
func TestSmartLightActivationSequenceAvoidsRedundantEnableToggle(t *testing.T) {
	steps := smartLightActivationSequence(2)
	want := []struct {
		command byte
		payload byte
	}{
		{command: 0x44, payload: 0x00},
		{command: 0x46, payload: 0x01},
		{command: 0x44, payload: 0x02},
	}
	if len(steps) != len(want) {
		t.Fatalf("activation steps = %d, want %d", len(steps), len(want))
	}
	for i := range want {
		if steps[i].command != want[i].command || len(steps[i].payload) != 1 || steps[i].payload[0] != want[i].payload {
			t.Fatalf("step %d = command %#02x payload %v, want command %#02x payload %#02x",
				i, steps[i].command, steps[i].payload, want[i].command, want[i].payload)
		}
	}
	for _, step := range steps {
		if step.command == 0x46 && step.payload[0] == 0x00 {
			t.Fatal("序列里不应再出现 0x46 00：它会多产生一次固件闪存擦写")
		}
	}
}

func equalBytes(got, want []byte) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// 固件的灯效缓冲区容量在流状态结构里写死为 256 字节，而 0x47 的固件分支完全不做
// 边界检查。按协议文档的 31 帧下发就是 310 字节，会越过缓冲区末尾写进紧随其后的
// 关键帧渲染表。帧数必须刚好覆盖固件会读的 184 字节，且不超出容量。
func TestLightUploadStaysInsideFirmwareBuffer(t *testing.T) {
	const firmwareStreamCapacity = 256

	uploaded := lightUploadFrameCount * lightUploadFrameSize
	if uploaded > firmwareStreamCapacity {
		t.Fatalf("上传 %d 字节超出固件缓冲区容量 %d 字节", uploaded, firmwareStreamCapacity)
	}

	// 固件重排循环读到的最大偏移是 6 + 5*30 + 9*3 = 183。
	highestRead := lightColorDataOffset + (lightLEDCount-1)*lightKeyframeCount*3 + (lightKeyframeCount-1)*3 + 2
	if uploaded <= highestRead {
		t.Fatalf("上传 %d 字节覆盖不到固件会读的最高偏移 %d", uploaded, highestRead)
	}
	if uploaded-highestRead > lightUploadFrameSize {
		t.Fatalf("上传 %d 字节比固件需要的多出超过一帧（最高读取偏移 %d）", uploaded, highestRead)
	}
}

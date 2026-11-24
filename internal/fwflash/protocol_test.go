package fwflash

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestBuildErase(t *testing.T) {
	// 170 = 0xAA 扇区，小端 4 字节。
	got := buildErase(170)
	want := []byte{cmdErase, 0x04, 0x00, 0xAA, 0x00, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("buildErase = % X, want % X", got, want)
	}
}

func TestBuildProgramHeader(t *testing.T) {
	// program addr=0x38, padding=0x1c, 2 字节数据 => size = 4+1+2 = 7 = 0x07
	got := buildProgramLike(cmdProgram, 0x38, 0x1c, []byte{0xAA, 0xBB})
	want := []byte{cmdProgram, 0x07, 0x00, 0x38, 0x00, 0x00, 0x00, 0x1c, 0xAA, 0xBB}
	if !bytes.Equal(got, want) {
		t.Fatalf("buildProgramLike = % X, want % X", got, want)
	}
}

func TestBuildIdentify(t *testing.T) {
	got := buildIdentify(0, 0)
	want := append([]byte{cmdIdentify, 0x12, 0x00, 0x00, 0x00}, []byte("MCU ISP & WCH.CN")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("buildIdentify = % X, want % X", got, want)
	}
	if len(got) != 21 {
		t.Fatalf("identify len = %d, want 21", len(got))
	}
}

func TestParseResponse(t *testing.T) {
	// cmd=0xA1, state=0x00, len=0x02, payload=[0x91,0x22]
	raw := []byte{0xA1, 0x00, 0x02, 0x00, 0x91, 0x22}
	p, err := parseResponse(0xA1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p, []byte{0x91, 0x22}) {
		t.Fatalf("payload = % X", p)
	}
	if _, err := parseResponse(0xA2, raw); err != errCmdMismatch {
		t.Fatalf("want errCmdMismatch, got %v", err)
	}
}

// 真机向量：用户 BS3PRO 的 CH591 芯片 UID（来自 wchisp 的 chip_info dump），
// chipID=0x91。锁死 XOR 密钥算法——刷写时 isp_key 回包首字节必须等于 keyChecksum。
func TestXorKeyRealDevice(t *testing.T) {
	uid := []byte{0x29, 0x6C, 0x2D, 0x64, 0x7F, 0x9C, 0xD5, 0x6C}
	key := xorKey(uid, 0x91)
	want := [8]byte{0x82, 0x82, 0x82, 0x82, 0x82, 0x82, 0x82, 0x13}
	if key != want {
		t.Fatalf("xorKey = % X, want % X", key, want)
	}
	if cs := keyChecksum(key); cs != 0xA1 {
		t.Fatalf("keyChecksum = 0x%02X, want 0xA1", cs)
	}
}

func TestUIDChecksumRealDevice(t *testing.T) {
	// CH59x 的 8 字节 UID 满足: u16[0]+u16[2]+u16[4] == u16[6]（低 16 位）。
	uid := []byte{0x29, 0x6C, 0x2D, 0x64, 0x7F, 0x9C, 0xD5, 0x6C}
	if !uidChecksumValid(uid) {
		t.Fatal("真机 UID 校验应通过")
	}
	bad := []byte{0x00, 0x6C, 0x2D, 0x64, 0x7F, 0x9C, 0xD5, 0x6C}
	if uidChecksumValid(bad) {
		t.Fatal("被改坏的 UID 不应通过校验")
	}
}

func TestIntelHexRoundTrip(t *testing.T) {
	// 一段简单 HEX：地址 0 写 4 字节 01 02 03 04。
	hexText := ":0400000001020304F2\n:00000001FF\n"
	got, err := parseIntelHex([]byte(hexText))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatalf("parseIntelHex = % X", got)
	}
}

func TestPadToSectorBoundary(t *testing.T) {
	p := padToSectorBoundary(make([]byte, 100))
	if len(p) != 1024 {
		t.Fatalf("pad len = %d, want 1024", len(p))
	}
	p2 := padToSectorBoundary(make([]byte, 2048))
	if len(p2) != 2048 {
		t.Fatalf("pad len = %d, want 2048 (已对齐不应变)", len(p2))
	}
}

func TestParseIntelHexRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		// 声明 0x10 字节数据，实际只有 1 字节（校验和按实际内容算对，专门测长度检查）。
		"声明长度大于实际": ":10000000AA46\n:00000001FF\n",
		// 扩展线性地址记录的数据必须正好 2 字节。
		"扩展地址长度错误": ":0100000400FB\n:00000001FF\n",
		// 两条数据相距约 4GB，展开会要求分配巨大镜像。
		"地址跨度过大": ":0100000000FF\n:02000004FFFFFC\n:01FFFF00AA57\n:00000001FF\n",
	}
	for name, src := range cases {
		if _, err := parseIntelHex([]byte(src)); err == nil {
			t.Errorf("%s: 应解析失败", name)
		}
	}
}

func TestCheckImageFits(t *testing.T) {
	chip := Chip{Name: "CH591", FlashSize: 196608}
	ok := make([]byte, 1024)
	ok[0] = 0x6F
	if err := checkImageFits(ok, chip); err != nil {
		t.Errorf("正常镜像应通过: %v", err)
	}
	full := make([]byte, chip.FlashSize)
	full[0] = 0x6F
	if err := checkImageFits(full, chip); err != nil {
		t.Errorf("正好填满代码区应通过: %v", err)
	}
	if err := checkImageFits(append(full, 0x01), chip); err == nil {
		t.Error("超出代码区容量应被拒")
	}
	if err := checkImageFits(nil, chip); err == nil {
		t.Error("空镜像应被拒")
	}
	blank := make([]byte, 2048)
	for i := range blank[:1024] {
		blank[i] = 0xFF
	}
	if err := checkImageFits(blank, chip); err == nil {
		t.Error("全空白镜像应被拒")
	}
}

// 仓库里随清单分发的固件必须能被解析并放进 CH591。
func TestRepositoryFirmwareFits(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "firmware", "*.hex"))
	if len(files) == 0 {
		t.Fatal("没有找到仓库固件")
	}
	for _, f := range files {
		img, err := LoadFirmware(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if err := checkImageFits(img, Chip{Name: "CH591", FlashSize: 196608}); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

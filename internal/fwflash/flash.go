package fwflash

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

var (
	errShortResponse = errors.New("fwflash: 回包过短")
	errCmdMismatch   = errors.New("fwflash: 回包命令码不匹配")
	errBadLength     = errors.New("fwflash: 回包长度字段越界")
)

// Transport 是 ISP 底层字节通道（USB bulk）。实现见 transport_windows.go。
type Transport interface {
	SendRaw(b []byte) error
	RecvRaw(timeout time.Duration) ([]byte, error)
	Close() error
}

// Logger 用于把刷写进度回传给调用方（CLI/GUI）。全部可选。
type Logger interface {
	Infof(format string, args ...any)
	// Progress 在写入/校验过程中回报已处理字节 done / 总字节 total。
	Progress(phase string, done, total int)
}

// nopLogger 是空实现。
type nopLogger struct{}

func (nopLogger) Infof(string, ...any)      {}
func (nopLogger) Progress(string, int, int) {}

const (
	defaultTimeout = 500 * time.Millisecond
	eraseTimeout   = 5 * time.Second
	programTimeout = 300 * time.Millisecond
)

// Flashing 持有一次刷写会话的传输通道与芯片信息。
type Flashing struct {
	t         Transport
	log       Logger
	Chip      Chip
	uid       []byte // uidSize 字节
	btver     [4]byte
	Protected bool // 代码区是否开启读保护
}

// transfer 发送一条命令并读回 payload，等待 timeout。
func (f *Flashing) transfer(cmd []byte, timeout time.Duration) ([]byte, error) {
	if err := f.t.SendRaw(cmd); err != nil {
		return nil, fmt.Errorf("发送命令 0x%02X 失败: %w", cmd[0], err)
	}
	raw, err := f.t.RecvRaw(timeout)
	if err != nil {
		return nil, fmt.Errorf("读命令 0x%02X 回包失败: %w", cmd[0], err)
	}
	return parseResponse(cmd[0], raw)
}

// Open 在给定传输通道上建立刷写会话：识别芯片、读配置与 UID。
func Open(t Transport, log Logger) (*Flashing, error) {
	if log == nil {
		log = nopLogger{}
	}
	f := &Flashing{t: t, log: log}

	id, err := f.transfer(buildIdentify(0, 0), defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("识别芯片失败: %w", err)
	}
	if len(id) < 2 {
		return nil, fmt.Errorf("识别回包过短")
	}
	chip, ok := findChip(id[0], id[1])
	if !ok {
		return nil, fmt.Errorf("不支持的芯片: chipID=0x%02X deviceType=0x%02X（THRM 目前仅支持 CH591/CH592）", id[0], id[1])
	}
	f.Chip = chip

	cfg, err := f.transfer(buildReadConfig(cfgMaskAll), defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("读配置失败: %w", err)
	}
	// payload: [2 字节头][RDPR...][btver@14..18][uid@18..]
	if len(cfg) < 18+chip.uidSize() {
		return nil, fmt.Errorf("读配置回包过短: %d 字节", len(cfg))
	}
	copy(f.btver[:], cfg[14:18])
	f.uid = append([]byte(nil), cfg[18:18+chip.uidSize()]...)
	// cfg[2] 是 RDPR 字节：非 0xA5 表示代码区读保护开启。
	f.Protected = chip.supportCodeFlashProtect() && len(cfg) > 2 && cfg[2] != 0xA5
	if f.Protected {
		return nil, fmt.Errorf("芯片已开启代码区读保护，当前引擎不支持自动解保护")
	}
	if !uidChecksumValid(f.uid) {
		return nil, fmt.Errorf("芯片 UID 校验失败，疑似通信异常，已中止以免误刷")
	}
	return f, nil
}

// Info 返回可读的芯片信息串。
func (f *Flashing) Info() string {
	return fmt.Sprintf("Chip: %s[0x%02X%02X] (Code Flash: %dKiB, Data EEPROM: %dKiB)  UID: %X  BTVER: %x%x.%x%x",
		f.Chip.Name, f.Chip.ChipID, f.Chip.DeviceType,
		f.Chip.FlashSize/1024, f.Chip.EEPROMSize/1024,
		f.uid, f.btver[0], f.btver[1], f.btver[2], f.btver[3])
}

// eraseCode 擦除 sectors 个 1KiB 扇区（不足最小值则取最小值）。
func (f *Flashing) eraseCode(sectors uint32) error {
	if min := f.Chip.minEraseSectors(); sectors < min {
		sectors = min
	}
	if _, err := f.transfer(buildErase(sectors), eraseTimeout); err != nil {
		return fmt.Errorf("擦除失败: %w", err)
	}
	f.log.Infof("已擦除 %d 个代码区扇区", sectors)
	return nil
}

// sendISPKey 下发全 0 密钥种子并校验回包。
func (f *Flashing) sendISPKey(key [8]byte) error {
	resp, err := f.transfer(buildISPKey(make([]byte, ispKeySeed)), defaultTimeout)
	if err != nil {
		return fmt.Errorf("isp_key 失败: %w", err)
	}
	if len(resp) < 1 || resp[0] != keyChecksum(key) {
		return fmt.Errorf("isp_key 校验失败")
	}
	return nil
}

// uidChecksumValid 校验 CH59x 的 8 字节 UID：u16[0]+u16[2]+u16[4] == u16[6]（低 16 位）。
func uidChecksumValid(uid []byte) bool {
	if len(uid) != 8 {
		return true // 非 8 字节 UID 不做此校验
	}
	u := func(i int) uint16 { return uint16(uid[i]) | uint16(uid[i+1])<<8 }
	return u(0)+u(2)+u(4) == u(6)
}

func randByte() byte {
	var b [1]byte
	_, _ = rand.Read(b[:])
	return b[0]
}

// Flash 擦除并写入整片代码区镜像（会先补齐到扇区边界），带写后校验与复位。
// 完整流程：擦除 → isp_key → 分块 program → 空块 → isp_key → 分块 verify → 复位。
func (f *Flashing) Flash(raw []byte, verify, reset bool) error {
	// 先确认镜像合理再动设备：擦除之后才发现写不进去，设备就只剩下载模式可用了。
	if err := checkImageFits(raw, f.Chip); err != nil {
		return err
	}
	bin := padToSectorBoundary(raw)
	sectors := uint32(len(bin)/sectorSize) + 1

	f.log.Infof("擦除中……（固件 %d 字节，%d 扇区）", len(bin), sectors)
	if err := f.eraseCode(sectors); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	key := xorKey(f.uid, f.Chip.ChipID)

	f.log.Infof("写入代码区……")
	if err := f.sendISPKey(key); err != nil {
		return err
	}
	var addr uint32
	for off := 0; off < len(bin); off += programChunk {
		end := off + programChunk
		if end > len(bin) {
			end = len(bin)
		}
		chunk := bin[off:end]
		cmd := buildProgramLike(cmdProgram, addr, randByte(), xorInto(chunk, key))
		if _, err := f.transfer(cmd, programTimeout); err != nil {
			return fmt.Errorf("写入 0x%08X 失败: %w", addr, err)
		}
		addr += uint32(len(chunk))
		f.log.Progress("write", int(addr), len(bin))
	}
	// 固件要求以一个空数据写入收尾才算成功。
	if _, err := f.transfer(buildProgramLike(cmdProgram, addr, randByte(), nil), programTimeout); err != nil {
		return fmt.Errorf("写入收尾失败: %w", err)
	}
	f.log.Infof("已写入 %d 字节", addr)
	time.Sleep(500 * time.Millisecond)

	if verify {
		f.log.Infof("校验中……")
		if err := f.Verify(bin); err != nil {
			return err
		}
		f.log.Infof("校验通过")
	}

	if reset {
		f.log.Infof("复位设备")
		if err := f.Reset(); err != nil {
			return err
		}
	}
	return nil
}

// checkImageFits 确认镜像非空、不是全空白，且放得进芯片的代码区。
func checkImageFits(raw []byte, chip Chip) error {
	if len(raw) == 0 {
		return fmt.Errorf("固件为空")
	}
	if chip.FlashSize > 0 && uint32(len(raw)) > chip.FlashSize {
		return fmt.Errorf("固件 %d 字节超出 %s 代码区容量 %d 字节，拒绝刷写", len(raw), chip.Name, chip.FlashSize)
	}
	for _, b := range raw {
		if b != 0xFF && b != 0x00 {
			return nil
		}
	}
	return fmt.Errorf("固件内容全为空白，拒绝刷写")
}

// Verify 逐块比对代码区内容与 raw（raw 应已补齐到扇区边界）。
func (f *Flashing) Verify(raw []byte) error {
	key := xorKey(f.uid, f.Chip.ChipID)
	if err := f.sendISPKey(key); err != nil {
		return err
	}
	var addr uint32
	for off := 0; off < len(raw); off += programChunk {
		end := off + programChunk
		if end > len(raw) {
			end = len(raw)
		}
		chunk := raw[off:end]
		cmd := buildProgramLike(cmdVerify, addr, randByte(), xorInto(chunk, key))
		resp, err := f.transfer(cmd, defaultTimeout)
		if err != nil {
			return fmt.Errorf("校验 0x%08X 失败: %w", addr, err)
		}
		if len(resp) < 1 || resp[0] != 0x00 {
			return fmt.Errorf("校验不一致 @0x%08X", addr)
		}
		addr += uint32(len(chunk))
		f.log.Progress("verify", int(addr), len(raw))
	}
	return nil
}

// Reset 结束 ISP 会话并复位设备（连接随后断开）。
func (f *Flashing) Reset() error {
	if _, err := f.transfer(buildISPEnd(1), defaultTimeout); err != nil {
		return fmt.Errorf("复位失败: %w", err)
	}
	return nil
}

// Close 关闭底层传输。
func (f *Flashing) Close() error { return f.t.Close() }

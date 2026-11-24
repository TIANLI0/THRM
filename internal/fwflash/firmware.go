package fwflash

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

// LoadFirmware 从 .hex 或 .bin 文件读出扁平固件镜像（基址 0）。
func LoadFirmware(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFirmwareBytes(path, data)
}

// ParseFirmwareBytes 从内存字节解析固件镜像（下载场景用，不落盘）。
// 以 ':' 开头按 Intel HEX 解析，否则按裸二进制。nameHint 仅用于错误提示。
func ParseFirmwareBytes(nameHint string, data []byte) ([]byte, error) {
	if looksLikeIntelHex(data) {
		return parseIntelHex(data)
	}
	return data, nil
}

func looksLikeIntelHex(data []byte) bool {
	for _, b := range data {
		if b == ' ' || b == '\r' || b == '\n' || b == '\t' {
			continue
		}
		return b == ':'
	}
	return false
}

// maxImageSpan 是 Intel HEX 允许覆盖的最大地址跨度。远大于任何受支持芯片的代码区
// （见 chip.go），只用来挡住地址字段异常的文件：否则一条高地址记录就能让镜像膨胀到数 GB。
const maxImageSpan = 1024 * 1024

// parseIntelHex 解析 Intel HEX，返回从最小地址到最大地址的扁平镜像（空洞填 0xFF）。
func parseIntelHex(data []byte) ([]byte, error) {
	mem := map[uint32]byte{}
	var base uint32
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] != ':' {
			continue
		}
		b, err := hex.DecodeString(line[1:])
		if err != nil || len(b) < 5 {
			return nil, fmt.Errorf("Intel HEX 行格式错误: %q", line)
		}
		var sum byte
		for _, x := range b {
			sum += x
		}
		if sum != 0 {
			return nil, fmt.Errorf("Intel HEX 校验失败: %q", line)
		}
		n := b[0]
		// 行长必须正好是 长度+地址+类型+数据+校验，否则下面取 payload 会越界或读到校验字节。
		if len(b) != int(n)+5 {
			return nil, fmt.Errorf("Intel HEX 行长度与声明不符: %q", line)
		}
		addr := uint32(b[1])<<8 | uint32(b[2])
		rtype := b[3]
		payload := b[4 : 4+int(n)]
		if (rtype == 0x02 || rtype == 0x04) && n != 2 {
			return nil, fmt.Errorf("Intel HEX 扩展地址记录长度错误: %q", line)
		}
		switch rtype {
		case 0x00: // data
			for i, v := range payload {
				mem[base+addr+uint32(i)] = v
			}
		case 0x01: // EOF
			goto done
		case 0x02: // extended segment address
			base = (uint32(payload[0])<<8 | uint32(payload[1])) << 4
		case 0x04: // extended linear address
			base = (uint32(payload[0])<<8 | uint32(payload[1])) << 16
		}
	}
done:
	if len(mem) == 0 {
		return nil, fmt.Errorf("Intel HEX 无数据")
	}
	addrs := make([]uint32, 0, len(mem))
	for a := range mem {
		addrs = append(addrs, a)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })
	lo, hi := addrs[0], addrs[len(addrs)-1]
	if hi-lo >= maxImageSpan {
		return nil, fmt.Errorf("Intel HEX 地址跨度过大（0x%08X–0x%08X），不是有效的固件", lo, hi)
	}
	out := make([]byte, hi-lo+1)
	for i := range out {
		out[i] = 0xFF
	}
	for a, v := range mem {
		out[a-lo] = v
	}
	return out, nil
}

// padToSectorBoundary 把镜像用 0 补齐到 1KiB 边界（与 wchisp 一致）。
func padToSectorBoundary(buf []byte) []byte {
	if len(buf)%sectorSize == 0 {
		return buf
	}
	remain := sectorSize - (len(buf) % sectorSize)
	return append(buf, make([]byte, remain)...)
}

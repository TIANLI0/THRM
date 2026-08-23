package deviceproto

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const (
	Magic0   byte = 0x5A
	Magic1   byte = 0xA5
	ReportID byte = 0x02
)

type Frame struct {
	ReportID   byte
	Offset     int
	Command    byte
	Length     byte
	Payload    []byte
	Checksum   byte
	ChecksumOK bool
	Frame      []byte
}

func Checksum(cmd byte, payload ...byte) byte {
	sum := uint16(cmd) + uint16(2+len(payload))
	for _, b := range payload {
		sum += uint16(b)
	}
	return byte(sum & 0xFF)
}

func BuildFrame(cmd byte, payload ...byte) []byte {
	frame := make([]byte, 0, 5+len(payload))
	frame = append(frame, Magic0, Magic1, cmd, byte(2+len(payload)))
	frame = append(frame, payload...)
	frame = append(frame, Checksum(cmd, payload...))
	return frame
}

func BuildReport(frame []byte, reportLen int) []byte {
	if reportLen <= 0 || reportLen < len(frame)+1 {
		reportLen = len(frame) + 1
	}
	report := make([]byte, reportLen)
	report[0] = ReportID
	copy(report[1:], frame)
	return report
}

func ParseFrame(data []byte) (Frame, bool) {
	offset := -1
	reportID := byte(0)
	switch {
	case len(data) >= 2 && data[0] == Magic0 && data[1] == Magic1:
		offset = 0
	case len(data) >= 3 && data[1] == Magic0 && data[2] == Magic1:
		offset = 1
		reportID = data[0]
	default:
		return Frame{}, false
	}

	if len(data) < offset+5 {
		return Frame{}, false
	}

	length := int(data[offset+3])
	if length < 2 {
		return Frame{}, false
	}

	frameLen := 2 + length + 1
	if len(data) < offset+frameLen {
		return Frame{}, false
	}

	frame := data[offset : offset+frameLen]
	checksumIndex := offset + 2 + length
	var sum uint16
	for _, b := range data[offset+2 : checksumIndex] {
		sum += uint16(b)
	}

	payloadLen := length - 2
	payload := make([]byte, payloadLen)
	copy(payload, data[offset+4:offset+4+payloadLen])

	copiedFrame := make([]byte, len(frame))
	copy(copiedFrame, frame)

	return Frame{
		ReportID:   reportID,
		Offset:     offset,
		Command:    data[offset+2],
		Length:     byte(length),
		Payload:    payload,
		Checksum:   data[checksumIndex],
		ChecksumOK: byte(sum&0xFF) == data[checksumIndex],
		Frame:      copiedFrame,
	}, true
}

func NormalizeDebugInput(input string) ([]byte, error) {
	data, err := ParseHex(input)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	if len(data) >= 3 && data[0] == ReportID && data[1] == Magic0 && data[2] == Magic1 {
		parsed, ok := ParseFrame(data)
		if !ok {
			return nil, fmt.Errorf("invalid HID report frame")
		}
		if err := ValidateOutboundFrame(parsed.Frame); err != nil {
			return nil, err
		}
		return parsed.Frame, nil
	}
	if len(data) >= 2 && data[0] == Magic0 && data[1] == Magic1 {
		parsed, ok := ParseFrame(data)
		if !ok {
			return nil, fmt.Errorf("invalid protocol frame")
		}
		if err := ValidateOutboundFrame(parsed.Frame); err != nil {
			return nil, err
		}
		return parsed.Frame, nil
	}

	cmd := data[0]
	payload := data[1:]
	frame := BuildFrame(cmd, payload...)
	if err := ValidateOutboundFrame(frame); err != nil {
		return nil, err
	}
	return frame, nil
}

// RGBFrameCount 是固件灯效帧缓冲区能容纳的帧数（索引 0..30）。
const RGBFrameCount = 31

// ValidateOutboundFrame 拦截会让固件写越界的命令。
//
// 固件的 0x47 分支是 copy(stream_buffer + payload[0]*10, payload+1, 10)：既不检查
// 帧索引上界，也不检查缓冲区指针是否已分配（只有 0x41 检查后者）。索引是一个完整
// 字节，所以 0x47 FF ... 会往缓冲区后面 2550 字节处写数据，直接把固件打挂。
// 正常控制路径固定使用 0..30，但调试控制台可以发任意帧，必须在这里挡住。
func ValidateOutboundFrame(frame []byte) error {
	parsed, ok := ParseFrame(frame)
	if !ok {
		return nil
	}
	switch parsed.Command {
	case CmdRGBFrameWrite:
		if len(parsed.Payload) < 1 {
			return fmt.Errorf("命令 0x47 需要帧索引")
		}
		if int(parsed.Payload[0]) >= RGBFrameCount {
			return fmt.Errorf("命令 0x47 的帧索引 %d 超出固件缓冲区上限 %d，固件不做越界检查，发送会破坏其内存",
				parsed.Payload[0], RGBFrameCount-1)
		}
	case CmdSetGearRPM:
		if len(parsed.Payload) >= 1 && parsed.Payload[0] > 3 {
			return fmt.Errorf("命令 0x26 的挡位索引 %d 超出 0..3", parsed.Payload[0])
		}
	}
	return nil
}

func ParseHex(input string) ([]byte, error) {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == ',' || r == ';' || r == ':' || r == '-' {
			return ' '
		}
		return r
	}, strings.TrimSpace(input))

	fields := strings.Fields(normalized)
	var cleaned string
	if len(fields) <= 1 {
		cleaned = normalized
	} else {
		var b strings.Builder
		for _, field := range fields {
			b.WriteString(strings.TrimPrefix(strings.TrimPrefix(field, "0x"), "0X"))
		}
		cleaned = b.String()
	}
	cleaned = strings.ReplaceAll(cleaned, "0x", "")
	cleaned = strings.ReplaceAll(cleaned, "0X", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if len(cleaned)%2 != 0 {
		cleaned = "0" + cleaned
	}
	data, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid hex command: %w", err)
	}
	return data, nil
}

func Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	encoded := strings.ToUpper(hex.EncodeToString(data))
	var b strings.Builder
	b.Grow(len(encoded) + len(encoded)/2)
	for i := 0; i < len(encoded); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(encoded[i : i+2])
	}
	return b.String()
}

//go:build linux

package logger

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.uber.org/zap/zapcore"
)

const (
	systemdJournalSocket = "/run/systemd/journal/socket"
	journalWriteTimeout  = 100 * time.Millisecond
)

// journalSink 是 journalCore 与传输层之间的窄接口，也让级别映射和降级逻辑可以
// 在没有运行 systemd 的测试环境里完整验证。
type journalSink interface {
	io.Closer
	Send(entry zapcore.Entry, message string) error
}

var openJournalSink = func(identifier string) (journalSink, error) {
	return dialNativeJournalSink(systemdJournalSocket, identifier)
}

type nativeJournalSink struct {
	mu         sync.Mutex
	conn       *net.UnixConn
	socketPath string
	identifier string
}

func dialNativeJournalSink(socketPath, identifier string) (*nativeJournalSink, error) {
	conn, err := dialJournalSocket(socketPath)
	if err != nil {
		return nil, err
	}
	return &nativeJournalSink{
		conn:       conn,
		socketPath: socketPath,
		identifier: sanitizeJournalIdentifier(identifier),
	}, nil
}

func dialJournalSocket(socketPath string) (*net.UnixConn, error) {
	address := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	return net.DialUnix("unixgram", nil, address)
}

func (s *nativeJournalSink) Send(entry zapcore.Entry, message string) error {
	payload := journalPayload(s.identifier, entry, message)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		if err := writeJournalDatagram(s.conn, payload); err == nil {
			return nil
		}
		_ = s.conn.Close()
		s.conn = nil
	}

	// journald 重启时原来的 Unix datagram 连接会失效，重连一次后重试当前消息。
	conn, err := dialJournalSocket(s.socketPath)
	if err != nil {
		return err
	}
	s.conn = conn
	return writeJournalDatagram(s.conn, payload)
}

func writeJournalDatagram(conn *net.UnixConn, payload []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(journalWriteTimeout)); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func (s *nativeJournalSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func journalPayload(identifier string, entry zapcore.Entry, message string) []byte {
	var payload bytes.Buffer
	appendJournalField(&payload, "MESSAGE", message)
	appendJournalField(&payload, "PRIORITY", strconv.Itoa(journalPriority(entry.Level)))
	appendJournalField(&payload, "SYSLOG_IDENTIFIER", identifier)
	appendJournalField(&payload, "SYSLOG_FACILITY", "1") // user-level messages
	if entry.Caller.Defined {
		appendJournalField(&payload, "CODE_FILE", entry.Caller.File)
		appendJournalField(&payload, "CODE_LINE", strconv.Itoa(entry.Caller.Line))
		if entry.Caller.Function != "" {
			appendJournalField(&payload, "CODE_FUNC", entry.Caller.Function)
		}
	}
	return payload.Bytes()
}

// appendJournalField 实现 Journal Native Protocol。包含换行或 NUL 的字段必须使用
// 二进制字段形式，否则多行 panic 堆栈会被 journald 错拆成无效字段。
func appendJournalField(dst *bytes.Buffer, name, value string) {
	if !strings.ContainsAny(value, "\n\x00") {
		dst.WriteString(name)
		dst.WriteByte('=')
		dst.WriteString(value)
		dst.WriteByte('\n')
		return
	}

	dst.WriteString(name)
	dst.WriteByte('\n')
	_ = binary.Write(dst, binary.LittleEndian, uint64(len(value)))
	dst.WriteString(value)
	dst.WriteByte('\n')
}

func sanitizeJournalIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "thrm"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, identifier)
}

func journalPriority(level zapcore.Level) int {
	switch {
	case level <= zapcore.DebugLevel:
		return 7 // LOG_DEBUG
	case level == zapcore.InfoLevel:
		return 6 // LOG_INFO
	case level == zapcore.WarnLevel:
		return 4 // LOG_WARNING
	case level == zapcore.ErrorLevel:
		return 3 // LOG_ERR
	default:
		return 2 // LOG_CRIT (DPanic/Panic/Fatal)
	}
}

type journalCore struct {
	level   zapcore.LevelEnabler
	encoder zapcore.Encoder
	sink    journalSink
}

func newJournalCore(level zapcore.LevelEnabler, sink journalSink) zapcore.Core {
	config := logEncoderConfig()
	// journal 自己保存时间、级别和调用位置。MESSAGE 只保留正文及结构化字段，
	// journalctl 的默认输出因而不会重复出现两套时间戳和级别。
	config.TimeKey = zapcore.OmitKey
	config.LevelKey = zapcore.OmitKey
	config.CallerKey = zapcore.OmitKey
	config.NameKey = zapcore.OmitKey
	config.StacktraceKey = zapcore.OmitKey
	config.LineEnding = ""
	return &journalCore{
		level:   level,
		encoder: zapcore.NewConsoleEncoder(config),
		sink:    sink,
	}
}

func (c *journalCore) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level)
}

func (c *journalCore) With(fields []zapcore.Field) zapcore.Core {
	clone := &journalCore{
		level:   c.level,
		encoder: c.encoder.Clone(),
		sink:    c.sink,
	}
	for i := range fields {
		fields[i].AddTo(clone.encoder)
	}
	return clone
}

func (c *journalCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *journalCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	buffer, err := c.encoder.Clone().EncodeEntry(entry, fields)
	if err != nil {
		return err
	}
	message := strings.TrimSuffix(buffer.String(), zapcore.DefaultLineEnding)
	buffer.Free()

	if err := c.sink.Send(entry, message); err != nil {
		// journal 在启动后失效时不能静默吞日志。降级内容保持单行头 + 原始正文，
		// 多行堆栈仍会原样输出到 stderr。
		_, fallbackErr := fmt.Fprintf(
			os.Stderr,
			"%s\t%s\t%s\n",
			entry.Time.Format("2006-01-02T15:04:05.000Z07:00"),
			strings.ToUpper(entry.Level.String()),
			message,
		)
		return fallbackErr
	}
	return nil
}

func (c *journalCore) Sync() error {
	return nil
}

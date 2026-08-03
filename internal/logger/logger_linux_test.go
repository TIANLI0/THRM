//go:build linux

package logger

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

type recordedJournalEntry struct {
	entry   zapcore.Entry
	message string
}

type recordingJournalSink struct {
	mu      sync.Mutex
	entries []recordedJournalEntry
	closed  bool
	err     error
}

func (s *recordingJournalSink) Send(entry zapcore.Entry, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, recordedJournalEntry{entry: entry, message: message})
	return s.err
}

func (s *recordingJournalSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func useRecordingJournal(t *testing.T) *recordingJournalSink {
	t.Helper()
	sink := &recordingJournalSink{}
	original := openJournalSink
	openJournalSink = func(string) (journalSink, error) { return sink, nil }
	t.Cleanup(func() { openJournalSink = original })
	return sink
}

func TestLinuxLoggerUsesJournalWithoutCreatingInstallLogs(t *testing.T) {
	sink := useRecordingJournal(t)
	installDir := t.TempDir()

	log, err := NewCustomLogger(false, installDir)
	if err != nil {
		t.Fatalf("NewCustomLogger() error = %v", err)
	}
	log.Info("core ready: %d", 42)
	log.Debug("suppressed")
	log.Close()

	if log.GetLogDir() != "" {
		t.Fatalf("GetLogDir() = %q, want empty when journald is used", log.GetLogDir())
	}
	if _, err := os.Stat(filepath.Join(installDir, "logs")); !os.IsNotExist(err) {
		t.Fatalf("Linux logger created install-dir logs unexpectedly: %v", err)
	}
	if len(sink.entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(sink.entries))
	}
	if sink.entries[0].entry.Level != zapcore.InfoLevel || !strings.Contains(sink.entries[0].message, "core ready: 42") {
		t.Fatalf("unexpected journal entry: %+v", sink.entries[0])
	}
	if !sink.closed {
		t.Fatal("Close() did not close the journal sink")
	}
}

func TestDefaultLogDirLinuxIsEmpty(t *testing.T) {
	if got := defaultLogDir("/usr/bin"); got != "" {
		t.Fatalf("defaultLogDir() = %q, want empty for journal logging", got)
	}
}

func TestLinuxLoggerDebugModeCanBeToggled(t *testing.T) {
	sink := useRecordingJournal(t)
	log, err := NewCustomLogger(false, t.TempDir())
	if err != nil {
		t.Fatalf("NewCustomLogger() error = %v", err)
	}
	defer log.Close()

	log.Debug("before")
	log.SetDebugMode(true)
	log.Debug("after")

	if len(sink.entries) != 1 {
		t.Fatalf("journal entries = %d, want 1", len(sink.entries))
	}
	if sink.entries[0].entry.Level != zapcore.DebugLevel || sink.entries[0].message != "after" {
		t.Fatalf("unexpected debug entry: %+v", sink.entries[0])
	}
}

func TestLinuxLoggerFallsBackWhenJournalIsUnavailable(t *testing.T) {
	original := openJournalSink
	openJournalSink = func(string) (journalSink, error) {
		return nil, errors.New("journal unavailable")
	}
	t.Cleanup(func() { openJournalSink = original })

	log, err := NewCustomLogger(false, t.TempDir())
	if err != nil {
		t.Fatalf("journal failure must not prevent startup: %v", err)
	}
	defer log.Close()
	if log.GetLogDir() != "" {
		t.Fatalf("fallback logger must not use /var/log or install dir: %q", log.GetLogDir())
	}
}

func TestJournalPriorityMapping(t *testing.T) {
	tests := []struct {
		level zapcore.Level
		want  int
	}{
		{zapcore.DebugLevel, 7},
		{zapcore.InfoLevel, 6},
		{zapcore.WarnLevel, 4},
		{zapcore.ErrorLevel, 3},
		{zapcore.DPanicLevel, 2},
		{zapcore.PanicLevel, 2},
		{zapcore.FatalLevel, 2},
	}
	for _, test := range tests {
		if got := journalPriority(test.level); got != test.want {
			t.Errorf("journalPriority(%s) = %d, want %d", test.level, got, test.want)
		}
	}
}

func TestJournalPayloadEncodesMultilineMessageAsBinaryField(t *testing.T) {
	message := "panic\nstack line"
	payload := journalPayload("thrm-core", zapcore.Entry{Level: zapcore.ErrorLevel}, message)
	prefix := []byte("MESSAGE\n")
	if !bytes.HasPrefix(payload, prefix) {
		t.Fatalf("payload prefix = %q, want binary MESSAGE field", payload)
	}
	lengthOffset := len(prefix)
	length := binary.LittleEndian.Uint64(payload[lengthOffset : lengthOffset+8])
	if length != uint64(len(message)) {
		t.Fatalf("binary MESSAGE length = %d, want %d", length, len(message))
	}
	if !bytes.Contains(payload, []byte("\nPRIORITY=3\nSYSLOG_IDENTIFIER=thrm-core\n")) {
		t.Fatalf("payload is missing priority or identifier: %q", payload)
	}
}

func TestNativeJournalSinkWritesDatagram(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "journal.sock")
	listener, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socketPath, Net: "unixgram"})
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skip("sandbox does not allow Unix datagram sockets")
		}
		t.Fatalf("ListenUnixgram() error = %v", err)
	}
	defer listener.Close()

	sink, err := dialNativeJournalSink(socketPath, "thrm core")
	if err != nil {
		t.Fatalf("dialNativeJournalSink() error = %v", err)
	}
	defer sink.Close()
	if err := sink.Send(zapcore.Entry{Level: zapcore.WarnLevel}, "fan disconnected"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	payload := make([]byte, 4096)
	count, _, err := listener.ReadFromUnix(payload)
	if err != nil {
		t.Fatalf("ReadFromUnix() error = %v", err)
	}
	payload = payload[:count]
	for _, field := range [][]byte{
		[]byte("MESSAGE=fan disconnected\n"),
		[]byte("PRIORITY=4\n"),
		[]byte("SYSLOG_IDENTIFIER=thrm-core\n"),
	} {
		if !bytes.Contains(payload, field) {
			t.Errorf("journal datagram does not contain %q: %q", field, payload)
		}
	}
}

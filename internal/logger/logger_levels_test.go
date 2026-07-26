package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readLogFile(t *testing.T, logDir, prefix string) string {
	t.Helper()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("读取日志目录失败: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, entry.Name()))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", entry.Name(), err)
		}
		return string(data)
	}
	return ""
}

// TestDebugFileOnlyReceivesDebugLevel 是 #12 的回归测试。
//
// debug 文件此前用的门槛是 atom（非调试模式下等于 Info），与 app 文件完全相同，
// 于是每条 Info 日志都被写进两个文件，磁盘写量凭空翻倍。
func TestDebugFileOnlyReceivesDebugLevel(t *testing.T) {
	installDir := t.TempDir()

	l, err := NewCustomLogger(false, installDir)
	if err != nil {
		t.Fatalf("NewCustomLogger error: %v", err)
	}

	const marker = "MARKER_INFO_SHOULD_NOT_DUPLICATE"
	l.Info("%s", marker)
	l.Error("MARKER_ERROR")
	l.Debug("MARKER_DEBUG_SUPPRESSED")
	l.Close()
	// lumberjack 是同步写入，这里只为稳妥留一点时间给文件系统。
	time.Sleep(50 * time.Millisecond)

	logDir := defaultLogDir(installDir)
	appLog := readLogFile(t, logDir, "app_")
	debugLog := readLogFile(t, logDir, "debug_")

	if !strings.Contains(appLog, marker) {
		t.Fatal("app 日志里没有 Info 记录")
	}
	if strings.Contains(debugLog, marker) {
		t.Fatal("非调试模式下 Info 日志仍被重复写入 debug 文件，磁盘写量翻倍")
	}
	if strings.Contains(debugLog, "MARKER_ERROR") {
		t.Fatal("Error 日志被重复写入 debug 文件")
	}
	t.Logf("app 日志 %d 字节，debug 日志 %d 字节（修复前两者内容相同）", len(appLog), len(debugLog))
}

// TestDebugFileReceivesDebugWhenEnabled 验证调试模式下 debug 文件仍然收 Debug 日志。
func TestDebugFileReceivesDebugWhenEnabled(t *testing.T) {
	installDir := t.TempDir()

	l, err := NewCustomLogger(true, installDir)
	if err != nil {
		t.Fatalf("NewCustomLogger error: %v", err)
	}

	const marker = "MARKER_DEBUG_VISIBLE"
	l.Debug("%s", marker)
	l.Close()
	time.Sleep(50 * time.Millisecond)

	debugLog := readLogFile(t, defaultLogDir(installDir), "debug_")
	if !strings.Contains(debugLog, marker) {
		t.Fatal("调试模式下 Debug 日志未写入 debug 文件")
	}
}

// TestSetDebugModeTogglesDebugFile 验证运行期切换调试模式后 Debug 日志开始落盘。
func TestSetDebugModeTogglesDebugFile(t *testing.T) {
	installDir := t.TempDir()

	l, err := NewCustomLogger(false, installDir)
	if err != nil {
		t.Fatalf("NewCustomLogger error: %v", err)
	}

	l.Debug("MARKER_BEFORE_ENABLE")
	l.SetDebugMode(true)
	const marker = "MARKER_AFTER_ENABLE"
	l.Debug("%s", marker)
	l.Close()
	time.Sleep(50 * time.Millisecond)

	debugLog := readLogFile(t, defaultLogDir(installDir), "debug_")
	if strings.Contains(debugLog, "MARKER_BEFORE_ENABLE") {
		t.Fatal("开启调试模式前的 Debug 日志不应落盘")
	}
	if !strings.Contains(debugLog, marker) {
		t.Fatal("开启调试模式后的 Debug 日志未落盘")
	}
}

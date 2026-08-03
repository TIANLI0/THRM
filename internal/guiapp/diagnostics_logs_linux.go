//go:build linux

package guiapp

import (
	"archive/zip"
	"bytes"
	"context"
	"os/exec"
	"time"
)

const (
	diagnosticJournalTimeout = 5 * time.Second
	diagnosticJournalMaxSize = 4 << 20
)

// cappedBuffer 保留前 limit 字节但始终向调用方报告完整写入，避免 journalctl 因
// 诊断包的体积上限收到 short write 并以失败退出。
type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (w *cappedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := w.limit - w.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = w.buffer.Write(data)
	}
	return written, nil
}

func addPlatformDiagnosticLogs(archive *zip.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticJournalTimeout)
	defer cancel()

	// 两个 --identifier 条件在 journalctl 中按 OR 匹配。限制到最近七天、最新
	// 4000 条，并在内存侧再限制 4 MiB，防止高频调试日志撑大诊断包。
	command := exec.CommandContext(
		ctx,
		"journalctl",
		"--no-pager",
		"--quiet",
		"--output=short-iso-precise",
		"--since=7 days ago",
		"--lines=4000",
		"--reverse",
		"--identifier=thrm",
		"--identifier=thrm-core",
	)
	var output cappedBuffer
	output.limit = diagnosticJournalMaxSize
	command.Stdout = &output
	if err := command.Run(); err != nil || output.buffer.Len() == 0 {
		// journalctl 不存在、用户无读取权限或本机没有匹配日志，都不应让整个诊断包
		// 导出失败；diagnostics.json 及其他可用日志仍然有价值。
		return nil
	}

	entry, err := archive.Create("logs/journal.log")
	if err != nil {
		return err
	}
	_, err = entry.Write(output.buffer.Bytes())
	return err
}

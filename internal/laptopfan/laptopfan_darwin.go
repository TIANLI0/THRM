//go:build darwin

// macOS 的笔记本内置风扇读取。
//
// 数据来源是 FanProbe.kext：它每 2 秒调用 ACPI 的 ECRR 方法读 EC 扩展窗口
// （0x0464 = CPU 风扇、0x046C = GPU 风扇，和 Windows 侧 Uniwill 后端读的
// 是同一对寄存器），然后用 IOLog 打一行：
//
//	FanProbe: CPU raw 0A F0 -> 2800 RPM
//	FanProbe: GPU raw 0A 55 -> 2645 RPM
//
// 这里起一个 log stream 子进程把这些行读进来缓存。读不到就返回 false，
// 调用方照常走「风扇不可用」的分支，不影响控温。

package laptopfan

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	fanProbeMarker   = "FanProbe:"
	fanProbeMaxAge   = 10 * time.Second
	fanProbeMinRPM   = 100
	fanProbeMaxValid = 20000
)

type darwinReader struct {
	logger types.Logger

	mu      sync.Mutex
	cpuRPM  int
	gpuRPM  int
	updated time.Time
}

func newPlatformReader(logger types.Logger) readerImpl {
	r := &darwinReader{logger: logger}
	go r.consume()
	return r
}

func (r *darwinReader) consume() {
	cmd := exec.Command("/usr/bin/log", "stream", "--style", "compact",
		"--predicate", `eventMessage CONTAINS "`+fanProbeMarker+`"`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.logInfo("笔记本风扇: 创建日志管道失败: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		r.logInfo("笔记本风扇: 启动 log stream 失败: %v", err)
		return
	}
	r.logInfo("笔记本风扇: 开始监听 FanProbe 日志")

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		r.parseLine(scanner.Text())
	}
	_ = cmd.Wait()
	r.logInfo("笔记本风扇: 日志监听结束")
}

func (r *darwinReader) parseLine(line string) {
	if !strings.Contains(line, fanProbeMarker) {
		return
	}
	arrow := strings.Index(line, "-> ")
	if arrow < 0 {
		return
	}
	rest := line[arrow+3:]

	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return
	}
	rpm, err := strconv.Atoi(rest[:end])
	if err != nil || rpm < fanProbeMinRPM || rpm > fanProbeMaxValid {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case strings.Contains(line, "CPU"):
		r.cpuRPM = rpm
	case strings.Contains(line, "GPU"):
		r.gpuRPM = rpm
	default:
		return
	}
	r.updated = time.Now()
}

func (r *darwinReader) read() (FanSpeeds, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.updated.IsZero() || time.Since(r.updated) > fanProbeMaxAge {
		return FanSpeeds{}, false
	}
	return FanSpeeds{CPUFanRPM: r.cpuRPM, GPUFanRPM: r.gpuRPM}, true
}

func (r *darwinReader) logInfo(format string, v ...any) {
	if r.logger != nil {
		r.logger.Info(format, v...)
	}
}

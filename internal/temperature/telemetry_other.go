//go:build !linux

package temperature

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	helperCommandTimeout = 1200 * time.Millisecond
	gpuVendorCacheTTL    = 30 * time.Second

	platformTelemetryIsExpensive = true
)

func readPlatformTelemetry(r *Reader, disableGPU bool) telemetryReading {
	if disableGPU {
		return telemetryReading{}
	}
	temp, power := r.readGPUTempAndPower()
	return telemetryReading{gpuTemp: temp, gpuPower: power}
}

// readGPUTempAndPower 一次调用同时取回 GPU 温度与功耗。
func (r *Reader) readGPUTempAndPower() (int, float64) {
	if r.detectGPUVendor() != "nvidia" {
		return 0, 0
	}

	output, err := execHelperCommand(helperCommandTimeout,
		"nvidia-smi", "--query-gpu=temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	if err != nil {
		r.logger.Debug("读取 NVIDIA GPU 温度/功耗失败: %v", err)
		return 0, 0
	}
	return parseNvidiaTempAndPower(string(output))
}

// parseNvidiaTempAndPower 解析 "62, 35.12" 形式的输出。
func parseNvidiaTempAndPower(output string) (int, float64) {
	line := strings.TrimSpace(output)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if line == "" {
		return 0, 0
	}

	fields := strings.Split(line, ",")
	temp := 0
	if value, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil && value > 0 && value < 150 {
		temp = value
	}

	power := 0.0
	if len(fields) > 1 {
		if watts, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64); err == nil && watts > 0 && watts <= 2000 {
			power = watts
		}
	}
	return temp, power
}

// detectGPUVendor 检测 NVIDIA GPU 厂商。
func (r *Reader) detectGPUVendor() string {
	now := readTimeNow()
	r.cacheMutex.RLock()
	if cached := r.cachedGPUVendor; cached != "" && now.Sub(r.cachedVendorAt) < gpuVendorCacheTTL {
		r.cacheMutex.RUnlock()
		return cached
	}
	r.cacheMutex.RUnlock()

	vendor := "unknown"
	if _, err := execHelperCommand(helperCommandTimeout, "nvidia-smi", "--version"); err == nil {
		vendor = "nvidia"
	} else if !errors.Is(err, context.DeadlineExceeded) {
		r.logger.Debug("检测GPU厂商失败: %v", err)
	} else {
		r.logger.Debug("检测GPU厂商超时: %v", err)
	}

	r.cacheMutex.Lock()
	r.cachedGPUVendor = vendor
	r.cachedVendorAt = now
	r.cacheMutex.Unlock()
	return vendor
}

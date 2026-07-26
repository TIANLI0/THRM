//go:build windows

package temperature

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
// Windows 走 wmic，单次最坏耗时接近 helperCommandTimeout，必须纳入降级节流。
const platformCPUTempIsExpensive = true

func (r *Reader) readPlatformCPUTemp() int {
	output, err := execHelperCommand(helperCommandTimeout, "wmic", "/namespace:\\\\root\\wmi", "PATH", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Debug("读取Windows CPU温度超时: %v", err)
		} else {
			r.logger.Debug("读取Windows CPU温度失败: %v", err)
		}
		return 0
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "CurrentTemperature="); ok {
			tempStr := strings.TrimSpace(after)
			if tempStr == "" {
				continue
			}

			temp, err := strconv.Atoi(tempStr)
			if err != nil {
				continue
			}

			celsius := (temp - 2732) / 10
			if celsius > 0 && celsius < 150 {
				return celsius
			}
		}
	}

	return 0
}


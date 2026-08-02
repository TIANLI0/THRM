//go:build !windows && !linux

package temperature

// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
const platformCPUTempIsExpensive = false

// platformHasCPUTempFallback 报告本平台是否存在可信的进程内 CPU 温度来源。
// 其它平台没有实现，gopsutil 仍可能给出可用读数，因此保持放行。
const platformHasCPUTempFallback = true

func (r *Reader) readPlatformCPUTemp() int {
	return 0
}


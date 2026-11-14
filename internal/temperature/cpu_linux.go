//go:build linux

package temperature

// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
// Linux 直接读 sysfs，开销可忽略。
const platformCPUTempIsExpensive = false

// platformHasCPUTempFallback 报告本平台是否存在可信的进程内 CPU 温度来源。
const platformHasCPUTempFallback = true

func (r *Reader) readPlatformCPUTemp() int {
	return readLinuxCPUSysfsFromRoot("/sys").TemperatureC
}

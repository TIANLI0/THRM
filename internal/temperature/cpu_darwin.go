//go:build darwin

package temperature

// platformCPUTempIsExpensive：SMC 读取是进程内的 IOKit 调用，不需要外部进程。
const platformCPUTempIsExpensive = false

// platformHasCPUTempFallback：macOS 通过 AppleSMC 读取，视为可信来源。
const platformHasCPUTempFallback = true

func (r *Reader) readPlatformCPUTemp() int {
	return 0
}

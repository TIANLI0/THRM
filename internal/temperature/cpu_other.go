//go:build !windows && !linux

package temperature

// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
const platformCPUTempIsExpensive = false

func (r *Reader) readPlatformCPUTemp() int {
	return 0
}


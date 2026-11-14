//go:build linux

package temperature

// platformTelemetryIsExpensive 表示平台遥测是否需要创建外部进程。
const platformTelemetryIsExpensive = false

func readPlatformTelemetry(r *Reader, disableGPU bool) telemetryReading {
	return readLinuxHardwareTelemetry(r, disableGPU)
}

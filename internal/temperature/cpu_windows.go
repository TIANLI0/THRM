//go:build windows

package temperature

// Windows 上没有可信的 CPU 温度兜底来源。
//
// 唯一不依赖驱动的候选是 ACPI 温区（MSAcpi_ThermalZoneTemperature、"Thermal Zone
// Information" 性能计数器、gopsutil 在 Windows 上的实现——三者读的是同一份数据），
// 而它报的是主板/外壳温区而不是 CPU 核心温度：多数笔记本上它要么恒定不变，要么比
// 真实核心温度低十几度。拿它驱动风扇曲线只会得到错误的转速；更糟的是会把"PawnIO
// 没装好"这个真正的问题伪装成一次正常读数，用户因此永远看不到可以一键修复的提示。
//
// 因此 Windows 的 CPU 温度一律由 TempBridge（LibreHardwareMonitor + PawnIO）提供，
// 读不到就如实上报 CPUTempError，由界面引导用户重装 PawnIO 或重启应用。
// 顺带也消除了原先每次降级读取都要创建一个 wmic 进程的后台开销。
const (
	// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
	platformCPUTempIsExpensive = false

	// platformHasCPUTempFallback 报告本平台是否存在可信的进程内 CPU 温度兜底来源。
	platformHasCPUTempFallback = false
)

func (r *Reader) readPlatformCPUTemp() int {
	return 0
}

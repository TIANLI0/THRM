//go:build darwin

package temperature

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

static io_service_t gGpuService = 0;

// gpu_find_service 找 NVAccel 服务并缓存句柄。
// 优先 NVAccel（NullMoth 驱动），找不到再退回通用的 IOAccelerator。
static int gpu_find_service(void) {
    if (gGpuService) return 0;

    const char *classes[] = {"NVAccel", "IOAccelerator", NULL};
    for (int i = 0; classes[i]; i++) {
        io_iterator_t iter = 0;
        CFMutableDictionaryRef m = IOServiceMatching(classes[i]);
        if (!m) continue;
        if (IOServiceGetMatchingServices(kIOMainPortDefault, m, &iter) != KERN_SUCCESS) continue;
        io_service_t svc = IOIteratorNext(iter);
        IOObjectRelease(iter);
        if (svc) {
            gGpuService = svc;
            return 0;
        }
    }
    return -1;
}

// gpu_read_stat 从 PerformanceStatistics 字典里取一个整数。
static int gpu_read_stat(const char *key) {
    if (gpu_find_service() != 0) return -1;

    CFMutableDictionaryRef props = NULL;
    if (IORegistryEntryCreateCFProperties(gGpuService, &props, kCFAllocatorDefault, 0) != KERN_SUCCESS || !props) {
        return -1;
    }

    int result = -1;
    CFTypeRef stats = CFDictionaryGetValue(props, CFSTR("PerformanceStatistics"));
    if (stats && CFGetTypeID(stats) == CFDictionaryGetTypeID()) {
        CFStringRef k = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
        if (k) {
            CFTypeRef v = CFDictionaryGetValue((CFDictionaryRef)stats, k);
            if (v && CFGetTypeID(v) == CFNumberGetTypeID()) {
                int n = 0;
                if (CFNumberGetValue((CFNumberRef)v, kCFNumberIntType, &n)) result = n;
            }
            CFRelease(k);
        }
    }
    CFRelease(props);
    return result;
}

// 温度：优先 "Temperature(C)"，回退 "GPU Temperature(C)"。
static int gpu_temperature(void) {
    int t = gpu_read_stat("Temperature(C)");
    if (t <= 0) t = gpu_read_stat("GPU Temperature(C)");
    return t;
}

// 功耗：优先 "GPU Power(W)"，回退 "Total Power(W)"；单位瓦。
static int gpu_power_watts(void) {
    int p = gpu_read_stat("GPU Power(W)");
    if (p < 0) p = gpu_read_stat("Total Power(W)");
    return p;
}

// ---- CPU 功耗：走 AMDRyzenCPUPowerManagement 的 IOUserClient ----
//
// 该 kext 暴露 selector 4：scalarOut[0] = 核心数，structOut 是 Float 数组，
// [0] = 整包功耗(W)，[1] = 温度(°C)，[3..] = 每核频率。
// 参考 brorbw/amd-cpu-data（对 trulyspinach/SMCAMDProcessor 的封装）。

static io_connect_t gCpuConn = 0;

static int cpu_open(void) {
    if (gCpuConn) return 0;
    io_service_t svc = IOServiceGetMatchingService(kIOMainPortDefault,
                                                  IOServiceMatching("AMDRyzenCPUPowerManagement"));
    if (!svc) return -1;
    kern_return_t kr = IOServiceOpen(svc, mach_task_self(), 0, &gCpuConn);
    IOObjectRelease(svc);
    if (kr != KERN_SUCCESS) { gCpuConn = 0; return -1; }
    return 0;
}

// 返回整包功耗(W)；失败返回 -1。
static double cpu_power_watts(void) {
    if (cpu_open() != 0) return -1.0;

    uint64_t scalarOut = 0;
    uint32_t outputCount = 1;
    float output[67];
    memset(output, 0, sizeof(output));
    size_t outputSize = sizeof(output);

    kern_return_t kr = IOConnectCallMethod(gCpuConn, 4, NULL, 0, NULL, 0,
                                           &scalarOut, &outputCount,
                                           output, &outputSize);
    if (kr != KERN_SUCCESS) return -1.0;
    return (double)output[0];
}
*/
import "C"

import (
	"strings"
	"syscall"
)

// platformTelemetryIsExpensive：IOKit 属性读取是进程内调用，不创建外部进程。
const platformTelemetryIsExpensive = false

func readPlatformTelemetry(r *Reader, disableGPU bool) telemetryReading {
	reading := telemetryReading{}

	// CPU 型号：Windows 由 TempBridge 提供，macOS 直接问 sysctl。
	if name, err := syscall.Sysctl("machdep.cpu.brand_string"); err == nil {
		reading.cpuModel = strings.TrimSpace(name)
	}
	if r.logger != nil {
		r.logger.Debug("CPU 型号读取: %q", reading.cpuModel)
	}

	if !disableGPU {
		temp, power := r.readGPUTempAndPower()
		reading.gpuTemp = temp
		reading.gpuPower = power
	}

	// CPU 整包功耗来自 AMD 驱动的 user client；读不到就保持 0（界面显示 "-"）。
	if watts := float64(C.cpu_power_watts()); watts > 0 && watts <= 500 {
		reading.cpuPower = watts
		if r.logger != nil {
			r.logger.Debug("CPU 功耗: %.1f W（AMDRyzenCPUPowerManagement）", watts)
		}
	}

	return reading
}

// readGPUTempAndPower 读取 GPU 温度与功耗。
//
// macOS 下不再依赖 nvidia-smi（不存在），也不再依赖 SMCNvidiaGPU 的 SMC 键。
// NullMoth 驱动把 GPU 遥测发布在 IOAccelerator（NVAccel）的 PerformanceStatistics
// 字典里，用户态直接读即可：
//
//	Temperature(C)   = 46
//	GPU Power(W)     = 21
//	Power Limit(W)   = 50
//	GPU Core Clock(MHz) = 2220
func (r *Reader) readGPUTempAndPower() (int, float64) {
	temp := int(C.gpu_temperature())
	watts := int(C.gpu_power_watts())

	if temp <= 0 || temp > 150 {
		temp = 0
	}
	power := 0.0
	if watts > 0 && watts <= 2000 {
		power = float64(watts)
	}

	if r.logger != nil && (temp > 0 || power > 0) {
		r.logger.Debug("GPU 遥测: 温度 %d°C, 功耗 %.0f W", temp, power)
	}
	return temp, power
}

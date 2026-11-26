//go:build darwin

package temperature

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

static int thrmStatInt(CFDictionaryRef stats, const char *key, int *out) {
	CFStringRef k = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
	if (!k) return 0;
	CFTypeRef v = CFDictionaryGetValue(stats, k);
	CFRelease(k);
	if (!v || CFGetTypeID(v) != CFNumberGetTypeID()) return 0;
	return CFNumberGetValue((CFNumberRef)v, kCFNumberIntType, out) ? 1 : 0;
}

// thrmReadGPUStats 读取第一个 IOAccelerator 服务的 PerformanceStatistics。
// 温度/功耗读不到时对应输出为 -1。每次调用都重新匹配并释放全部 IOKit 对象，
// 显卡驱动重载后不会拿着失效的句柄。
static void thrmReadGPUStats(int *temp, int *power) {
	*temp = -1;
	*power = -1;

	io_iterator_t iter = 0;
	if (IOServiceGetMatchingServices(kIOMainPortDefault, IOServiceMatching("IOAccelerator"), &iter) != KERN_SUCCESS) {
		return;
	}
	io_service_t svc;
	while ((svc = IOIteratorNext(iter)) != 0) {
		CFTypeRef stats = IORegistryEntryCreateCFProperty(svc, CFSTR("PerformanceStatistics"), kCFAllocatorDefault, 0);
		IOObjectRelease(svc);
		if (!stats) continue;
		if (CFGetTypeID(stats) == CFDictionaryGetTypeID()) {
			CFDictionaryRef d = (CFDictionaryRef)stats;
			if (!thrmStatInt(d, "Temperature(C)", temp)) thrmStatInt(d, "GPU Temperature(C)", temp);
			if (!thrmStatInt(d, "GPU Power(W)", power)) thrmStatInt(d, "Total Power(W)", power);
		}
		CFRelease(stats);
		if (*temp > 0 || *power > 0) break;
	}
	IOObjectRelease(iter);
}
*/
import "C"

// platformTelemetryIsExpensive：IORegistry 读取是进程内调用，不创建外部进程。
const platformTelemetryIsExpensive = false

// readPlatformTelemetry 从 IOAccelerator 的 PerformanceStatistics 读取 GPU 温度与功耗。
//
// macOS 没有 nvidia-smi；显卡驱动把遥测发布在 IORegistry 里。是否提供温度/功耗
// 取决于驱动：独立显卡驱动（含黑苹果的 NVIDIA/AMD 驱动）通常有，Apple Silicon
// 的 AGX 只给利用率，此时两项都保持 0，界面显示“无数据”。
func readPlatformTelemetry(_ *Reader, disableGPU bool) telemetryReading {
	if disableGPU {
		return telemetryReading{}
	}
	var temp, power C.int
	C.thrmReadGPUStats(&temp, &power)
	return telemetryReading{gpuTemp: sanitizeGPUTemp(int(temp)), gpuPower: sanitizeGPUPower(int(power))}
}

func sanitizeGPUTemp(temp int) int {
	if temp <= 0 || temp >= 150 {
		return 0
	}
	return temp
}

func sanitizeGPUPower(watts int) float64 {
	if watts <= 0 || watts > 2000 {
		return 0
	}
	return float64(watts)
}

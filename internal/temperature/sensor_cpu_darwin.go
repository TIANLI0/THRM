//go:build darwin

package temperature

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <string.h>

#define KERNEL_INDEX_SMC 2
#define SMC_CMD_READ_BYTES 5
#define SMC_CMD_READ_KEYINFO 9

typedef struct { char major, minor, build, reserved[1]; unsigned short release; } SMCVers;
typedef struct { unsigned short version, length; unsigned int cpuPLimit, gpuPLimit, memPLimit; } SMCPLimit;
typedef struct { unsigned int dataSize, dataType; char dataAttributes; } SMCKeyInfo;

typedef struct {
    unsigned int key;
    SMCVers vers;
    SMCPLimit pLimitData;
    SMCKeyInfo keyInfo;
    char result, status, data8;
    unsigned int data32;
    char bytes[32];
} SMCKeyData;

static io_connect_t gConn = 0;

static int smc_call(SMCKeyData *in, SMCKeyData *out) {
    size_t outSize = sizeof(*out);
    memset(out, 0, sizeof(*out));
    kern_return_t kr = IOConnectCallStructMethod(gConn, KERNEL_INDEX_SMC,
                                                 in, sizeof(*in), out, &outSize);
    return (kr == KERN_SUCCESS) ? 0 : -1;
}

static int smc_open(void) {
    if (gConn) return 0;
    io_service_t svc = IOServiceGetMatchingService(kIOMainPortDefault,
                                                   IOServiceMatching("AppleSMC"));
    if (!svc) return -1;
    kern_return_t kr = IOServiceOpen(svc, mach_task_self(), 0, &gConn);
    IOObjectRelease(svc);
    if (kr != KERN_SUCCESS) { gConn = 0; return -1; }
    IOConnectCallScalarMethod(gConn, 0, NULL, 0, NULL, NULL);
    return 0;
}

static unsigned int fourcc(const char *s) {
    unsigned int r = 0;
    for (int i = 0; i < 4; i++) r = (r << 8) | (unsigned char)s[i];
    return r;
}

// 返回 sp78 温度；失败返回 -999
static double smc_read_temp(const char *key) {
    if (!key) return -999;
    if (!gConn && smc_open() != 0) return -999;

    unsigned int k = fourcc(key);
    SMCKeyData in, out;

    memset(&in, 0, sizeof(in));
    in.key = k;
    in.data8 = SMC_CMD_READ_KEYINFO;
    if (smc_call(&in, &out) != 0) return -999;
    unsigned int size = out.keyInfo.dataSize;
    if (size != 2) return -999;

    memset(&in, 0, sizeof(in));
    in.key = k;
    in.keyInfo.dataSize = size;
    in.data8 = SMC_CMD_READ_BYTES;
    if (smc_call(&in, &out) != 0) return -999;

    // sp78 是大端序
    short raw = (short)(((unsigned char)out.bytes[0] << 8) | (unsigned char)out.bytes[1]);
    double c = (double)raw / 256.0;
    if (c < -20.0 || c > 130.0) return -999;
    return c;
}

// 依次尝试多个键，返回第一个有效值
static double smc_cpu_temp(void) {
    const char *keys[] = {"TC0P", "TC0E", "TC0F", "TC0T", "TC0p"};
    for (int i = 0; i < 5; i++) {
        double t = smc_read_temp(keys[i]);
        if (t > -100) return t;
    }
    return -999;
}
*/
import "C"

// readSensorCPUTemp 通过 AppleSMC 读取 CPU 温度（macOS）。
func (r *Reader) readSensorCPUTemp() int {
	t := float64(C.smc_cpu_temp())
	if t <= -100 {
		return 0
	}
	return int(t)
}

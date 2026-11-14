//go:build linux

package temperature

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

typedef int thrm_nvmlReturn_t;
typedef void *thrm_nvmlDevice_t;

typedef thrm_nvmlReturn_t (*thrm_nvmlInit_t)(void);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetCount_t)(unsigned int *);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetHandleByIndex_t)(unsigned int, thrm_nvmlDevice_t *);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetName_t)(thrm_nvmlDevice_t, char *, unsigned int);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetTemperature_t)(thrm_nvmlDevice_t, unsigned int, unsigned int *);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetPowerUsage_t)(thrm_nvmlDevice_t, unsigned int *);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetEnforcedPowerLimit_t)(thrm_nvmlDevice_t, unsigned int *);
typedef thrm_nvmlReturn_t (*thrm_nvmlDeviceGetPciInfo_t)(thrm_nvmlDevice_t, void *);

typedef struct thrm_nvmlPciInfo_st {
char busIdLegacy[16];
unsigned int domain;
unsigned int bus;
unsigned int device;
unsigned int pciDeviceId;
unsigned int pciSubSystemId;
char busId[32];
} thrm_nvmlPciInfo_t;

static void *thrm_nvml_handle = NULL;
static int thrm_nvml_ready = 0;
static int thrm_nvml_init_rc = -1;
static thrm_nvmlInit_t thrm_nvml_init = NULL;
static thrm_nvmlDeviceGetCount_t thrm_nvml_get_count = NULL;
static thrm_nvmlDeviceGetHandleByIndex_t thrm_nvml_get_handle = NULL;
static thrm_nvmlDeviceGetName_t thrm_nvml_get_name = NULL;
static thrm_nvmlDeviceGetTemperature_t thrm_nvml_get_temperature = NULL;
static thrm_nvmlDeviceGetPowerUsage_t thrm_nvml_get_power = NULL;
static thrm_nvmlDeviceGetEnforcedPowerLimit_t thrm_nvml_get_power_limit = NULL;
static thrm_nvmlDeviceGetPciInfo_t thrm_nvml_get_pci_info = NULL;

static int thrm_nvml_load(void) {
if (thrm_nvml_ready) {
return 0;
}
if (thrm_nvml_handle == NULL) {
const char *names[] = {"libnvidia-ml.so.1", "libnvidia-ml.so", NULL};
for (int i = 0; names[i] != NULL && thrm_nvml_handle == NULL; i++) {
thrm_nvml_handle = dlopen(names[i], RTLD_NOW | RTLD_LOCAL);
}
if (thrm_nvml_handle == NULL) {
thrm_nvml_init_rc = -1;
return -1;
}
}

thrm_nvml_init = (thrm_nvmlInit_t)dlsym(thrm_nvml_handle, "nvmlInit_v2");
if (thrm_nvml_init == NULL) {
thrm_nvml_init = (thrm_nvmlInit_t)dlsym(thrm_nvml_handle, "nvmlInit");
}
thrm_nvml_get_count = (thrm_nvmlDeviceGetCount_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetCount_v2");
if (thrm_nvml_get_count == NULL) {
thrm_nvml_get_count = (thrm_nvmlDeviceGetCount_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetCount");
}
thrm_nvml_get_handle = (thrm_nvmlDeviceGetHandleByIndex_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetHandleByIndex_v2");
if (thrm_nvml_get_handle == NULL) {
thrm_nvml_get_handle = (thrm_nvmlDeviceGetHandleByIndex_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetHandleByIndex");
}
thrm_nvml_get_name = (thrm_nvmlDeviceGetName_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetName");
thrm_nvml_get_temperature = (thrm_nvmlDeviceGetTemperature_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetTemperature");
thrm_nvml_get_power = (thrm_nvmlDeviceGetPowerUsage_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetPowerUsage");
thrm_nvml_get_power_limit = (thrm_nvmlDeviceGetEnforcedPowerLimit_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetEnforcedPowerLimit");
thrm_nvml_get_pci_info = (thrm_nvmlDeviceGetPciInfo_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetPciInfo_v3");
if (thrm_nvml_get_pci_info == NULL) {
thrm_nvml_get_pci_info = (thrm_nvmlDeviceGetPciInfo_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetPciInfo_v2");
}
if (thrm_nvml_get_pci_info == NULL) {
thrm_nvml_get_pci_info = (thrm_nvmlDeviceGetPciInfo_t)dlsym(thrm_nvml_handle, "nvmlDeviceGetPciInfo");
}

if (thrm_nvml_init == NULL || thrm_nvml_get_count == NULL || thrm_nvml_get_handle == NULL ||
    thrm_nvml_get_name == NULL || thrm_nvml_get_temperature == NULL || thrm_nvml_get_power == NULL) {
thrm_nvml_init_rc = -2;
return -2;
}

thrm_nvml_init_rc = thrm_nvml_init();
if (thrm_nvml_init_rc != 0) {
return thrm_nvml_init_rc;
}
thrm_nvml_ready = 1;
return 0;
}

int thrm_nvml_device_count(unsigned int *count) {
int rc = thrm_nvml_load();
if (rc != 0) {
return rc;
}
return thrm_nvml_get_count(count);
}

int thrm_nvml_device_info(
unsigned int index,
char *name, unsigned int nameLen,
char *busId, unsigned int busIdLen,
unsigned int *temperature, int *hasTemperature,
unsigned int *power, int *hasPower,
unsigned int *powerLimit, int *hasPowerLimit) {
int rc = thrm_nvml_load();
if (rc != 0) {
return rc;
}

thrm_nvmlDevice_t device = NULL;
rc = thrm_nvml_get_handle(index, &device);
if (rc != 0 || device == NULL) {
return rc != 0 ? rc : -3;
}

if (name != NULL && nameLen > 0) {
name[0] = '\0';
if (thrm_nvml_get_name(device, name, nameLen) != 0) {
name[0] = '\0';
} else {
name[nameLen - 1] = '\0';
}
}

if (busId != NULL && busIdLen > 0) {
busId[0] = '\0';
if (thrm_nvml_get_pci_info != NULL) {
thrm_nvmlPciInfo_t info;
memset(&info, 0, sizeof(info));
if (thrm_nvml_get_pci_info(device, &info) == 0) {
const char *src = info.busId[0] != '\0' ? info.busId : info.busIdLegacy;
if (src[0] != '\0') {
snprintf(busId, busIdLen, "%s", src);
}
}
}
}

if (temperature != NULL && hasTemperature != NULL) {
*temperature = 0;
*hasTemperature = 0;
if (thrm_nvml_get_temperature(device, 0, temperature) == 0) {
*hasTemperature = 1;
}
}
if (power != NULL && hasPower != NULL) {
*power = 0;
*hasPower = 0;
if (thrm_nvml_get_power(device, power) == 0) {
*hasPower = 1;
}
}
if (powerLimit != NULL && hasPowerLimit != NULL) {
*powerLimit = 0;
*hasPowerLimit = 0;
if (thrm_nvml_get_power_limit != NULL && thrm_nvml_get_power_limit(device, powerLimit) == 0) {
*hasPowerLimit = 1;
}
}
return 0;
}


*/
import "C"

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	nvmlNameBufferSize  = 96
	nvmlBusIDBufferSize = 32
)

type nvmlProvider struct {
	mutex sync.Mutex
}

type nvmlReading struct {
	Name           string
	BusID          string
	TemperatureC   int
	PowerW         float64
	PowerLimitW    float64
	HasTemperature bool
	HasPower       bool
	HasPowerLimit  bool
}

func newNVMLProvider() *nvmlProvider { return &nvmlProvider{} }

func (p *nvmlProvider) Name() string             { return "nvml" }
func (p *nvmlProvider) Kind() hardwareDeviceKind { return hardwareDeviceGPU }

func (p *nvmlProvider) Read() ([]hardwareReading, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	count, err := nvmlDeviceCount()
	if err != nil {
		return nil, err
	}
	readings := make([]hardwareReading, 0, count)
	for i := 0; i < count; i++ {
		device, err := nvmlReadDevice(i)
		if err != nil {
			continue
		}
		bdf := normalizePCIBDF(device.BusID)
		key := fmt.Sprintf("nvml:%d", i)
		if bdf != "" {
			key = "pci:" + bdf
		}
		reading := hardwareReading{
			Kind:    hardwareDeviceGPU,
			Key:     key,
			Vendor:  "nvidia",
			Model:   device.Name,
			Sources: []string{"nvml"},
		}
		if device.HasTemperature {
			reading.TempC = intPointer(device.TemperatureC)
			reading.Sensors = []types.TemperatureSensor{{
				Key:   key + "/nvml/temp",
				Name:  "GPU Temperature",
				Value: device.TemperatureC,
			}}
		}
		if device.HasPower {
			reading.PowerW = floatPointer(device.PowerW)
			reading.PowerSensors = []types.PowerSensor{{
				Key:   key + "/nvml/power",
				Name:  "GPU Power",
				Value: device.PowerW,
			}}
		}
		if device.HasPowerLimit {
			reading.LimitW = floatPointer(device.PowerLimitW)
		}
		readings = append(readings, reading)
	}
	if len(readings) == 0 {
		return nil, fmt.Errorf("NVML 未发现可用 GPU")
	}
	return readings, nil
}

func nvmlDeviceCount() (int, error) {
	var count C.uint
	rc := C.thrm_nvml_device_count(&count)
	if rc != 0 {
		return 0, fmt.Errorf("NVML 初始化失败: %d", int(rc))
	}
	return int(count), nil
}

func nvmlReadDevice(index int) (nvmlReading, error) {
	nameBuf := make([]byte, nvmlNameBufferSize)
	busBuf := make([]byte, nvmlBusIDBufferSize)
	var temperature C.uint
	var power C.uint
	var powerLimit C.uint
	var hasTemperature C.int
	var hasPower C.int
	var hasPowerLimit C.int

	rc := C.thrm_nvml_device_info(
		C.uint(index),
		(*C.char)(unsafe.Pointer(&nameBuf[0])), C.uint(len(nameBuf)),
		(*C.char)(unsafe.Pointer(&busBuf[0])), C.uint(len(busBuf)),
		&temperature, &hasTemperature,
		&power, &hasPower,
		&powerLimit, &hasPowerLimit,
	)
	if rc != 0 {
		return nvmlReading{}, fmt.Errorf("NVML 读取设备 %d 失败: %d", index, int(rc))
	}

	reading := nvmlReading{
		Name:           strings.TrimSpace(C.GoString((*C.char)(unsafe.Pointer(&nameBuf[0])))),
		BusID:          strings.TrimSpace(C.GoString((*C.char)(unsafe.Pointer(&busBuf[0])))),
		TemperatureC:   int(temperature),
		PowerW:         float64(power) / 1000,
		PowerLimitW:    float64(powerLimit) / 1000,
		HasTemperature: hasTemperature != 0,
		HasPower:       hasPower != 0,
		HasPowerLimit:  hasPowerLimit != 0,
	}
	if reading.Name == "" {
		reading.Name = fmt.Sprintf("NVIDIA GPU %d", index)
	}
	return reading, nil
}

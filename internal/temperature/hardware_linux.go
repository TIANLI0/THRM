//go:build linux

package temperature

import (
	"sort"
	"sync"

	"github.com/TIANLI0/THRM/internal/types"
)

type hardwareDeviceKind string

const (
	hardwareDeviceCPU hardwareDeviceKind = "cpu"
	hardwareDeviceGPU hardwareDeviceKind = "gpu"
)

// hardwareReading 是统一 provider 输出。optional 数值用指针区分“0”和“不可用”。
type hardwareReading struct {
	Kind         hardwareDeviceKind
	Key          string
	Vendor       string
	Model        string
	TempC        *int
	PowerW       *float64
	LimitW       *float64
	Sensors      []types.TemperatureSensor
	PowerSensors []types.PowerSensor
	Sources      []string
}

type hardwareProvider interface {
	Name() string
	Kind() hardwareDeviceKind
	Read() ([]hardwareReading, error)
}

type hardwareManager struct {
	mutex     sync.Mutex
	providers []hardwareProvider
}

func newLinuxHardwareManager() *hardwareManager {
	manager := &hardwareManager{}
	manager.Register(newNVMLProvider())
	manager.Register(&sysfsGPUProvider{root: "/sys"})
	manager.Register(newCPUProvider())
	return manager
}

func newHardwareManagerWithProviders(providers ...hardwareProvider) *hardwareManager {
	manager := &hardwareManager{}
	for _, provider := range providers {
		manager.Register(provider)
	}
	return manager
}

func (m *hardwareManager) Register(provider hardwareProvider) {
	m.providers = append(m.providers, provider)
}

// Read 收集所有 provider 的读数，并按设备 key 合并。
// 注册顺序就是字段优先级：先注册的 provider 提供的数据优先。
func (m *hardwareManager) Read(disableGPU bool) []hardwareReading {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	merged := make(map[string]*hardwareReading)
	for _, provider := range m.providers {
		if disableGPU && provider.Kind() == hardwareDeviceGPU {
			continue
		}
		readings, err := provider.Read()
		if err != nil {
			continue
		}
		for _, reading := range readings {
			if disableGPU && reading.Kind == hardwareDeviceGPU {
				continue
			}
			mergeHardwareReading(merged, reading)
		}
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]hardwareReading, 0, len(keys))
	for _, key := range keys {
		result = append(result, *merged[key])
	}
	return result
}

func mergeHardwareReading(merged map[string]*hardwareReading, incoming hardwareReading) {
	if incoming.Key == "" {
		return
	}
	existing := merged[incoming.Key]
	if existing == nil {
		reading := incoming
		reading.Sources = uniqueStrings(incoming.Sources)
		merged[incoming.Key] = &reading
		return
	}

	if existing.Vendor == "" {
		existing.Vendor = incoming.Vendor
	}
	if existing.Model == "" {
		existing.Model = incoming.Model
	}
	if existing.TempC == nil && incoming.TempC != nil {
		value := *incoming.TempC
		existing.TempC = &value
	}
	if existing.PowerW == nil && incoming.PowerW != nil {
		value := *incoming.PowerW
		existing.PowerW = &value
	}
	if existing.LimitW == nil && incoming.LimitW != nil {
		value := *incoming.LimitW
		existing.LimitW = &value
	}
	existing.Sensors = mergeTemperatureSensors(existing.Sensors, incoming.Sensors)
	existing.PowerSensors = mergePowerSensors(existing.PowerSensors, incoming.PowerSensors)
	existing.Sources = uniqueStrings(append(existing.Sources, incoming.Sources...))
}

func mergeTemperatureSensors(dst, src []types.TemperatureSensor) []types.TemperatureSensor {
	seen := make(map[string]struct{}, len(dst))
	for _, sensor := range dst {
		seen[sensor.Key] = struct{}{}
	}
	for _, sensor := range src {
		if _, ok := seen[sensor.Key]; ok {
			continue
		}
		dst = append(dst, sensor)
		seen[sensor.Key] = struct{}{}
	}
	return dst
}

func mergePowerSensors(dst, src []types.PowerSensor) []types.PowerSensor {
	seen := make(map[string]struct{}, len(dst))
	for _, sensor := range dst {
		seen[sensor.Key] = struct{}{}
	}
	for _, sensor := range src {
		if _, ok := seen[sensor.Key]; ok {
			continue
		}
		dst = append(dst, sensor)
		seen[sensor.Key] = struct{}{}
	}
	return dst
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || containsString(result, value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var linuxHardwareManager = newLinuxHardwareManager()

// readLinuxHardwareTelemetry 把统一 provider 读数转换为 THRM 的上报结构。
func readLinuxHardwareTelemetry(r *Reader, disableGPU bool) telemetryReading {
	readings := linuxHardwareManager.Read(disableGPU)
	var result telemetryReading
	var selectedGPU *hardwareReading
	for i := range readings {
		reading := &readings[i]
		switch reading.Kind {
		case hardwareDeviceCPU:
			result.cpuModel = reading.Model
			if reading.TempC != nil {
				result.cpuTemp = *reading.TempC
			}
			if reading.PowerW != nil {
				result.cpuPower = *reading.PowerW
			}
			result.cpuSensors = reading.Sensors
		case hardwareDeviceGPU:
			result.gpuDevices = append(result.gpuDevices, types.TemperatureGPUDevice{
				Key:          reading.Key,
				Name:         reading.Model,
				Vendor:       reading.Vendor,
				Sensors:      reading.Sensors,
				PowerSensors: reading.PowerSensors,
			})
			if selectedGPU == nil && reading.TempC != nil {
				selectedGPU = reading
			}
		}
	}
	if selectedGPU == nil {
		for i := range readings {
			if readings[i].Kind == hardwareDeviceGPU && readings[i].PowerW != nil {
				selectedGPU = &readings[i]
				break
			}
		}
	}
	if selectedGPU == nil {
		for i := range readings {
			if readings[i].Kind == hardwareDeviceGPU {
				selectedGPU = &readings[i]
				break
			}
		}
	}

	if selectedGPU != nil {
		result.selectedGpuDevice = selectedGPU.Key
		result.gpuModel = selectedGPU.Model
		if selectedGPU.TempC != nil {
			result.gpuTemp = *selectedGPU.TempC
		}
		if selectedGPU.PowerW != nil {
			result.gpuPower = *selectedGPU.PowerW
		}
		result.gpuSensors = selectedGPU.Sensors
		result.gpuPowerSensors = selectedGPU.PowerSensors
	}

	if r != nil && r.logger != nil {
		r.logger.Debug("Linux provider 返回 %d 个设备，CPU 型号=%q GPU 温度=%d GPU 功耗=%.2f",
			len(readings), result.cpuModel, result.gpuTemp, result.gpuPower)
	}
	return result
}

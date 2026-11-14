//go:build linux

package temperature

import (
	"fmt"
	"testing"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

type fakeHardwareProvider struct {
	name     string
	kind     hardwareDeviceKind
	readings []hardwareReading
	err      error
	reads    int
}

func (p *fakeHardwareProvider) Name() string             { return p.name }
func (p *fakeHardwareProvider) Kind() hardwareDeviceKind { return p.kind }
func (p *fakeHardwareProvider) Read() ([]hardwareReading, error) {
	p.reads++
	if p.err != nil {
		return nil, p.err
	}
	return p.readings, nil
}

func TestHardwareManagerMergesReadingsByKey(t *testing.T) {
	high := &fakeHardwareProvider{
		name: "nvml",
		kind: hardwareDeviceGPU,
		readings: []hardwareReading{{
			Kind:    hardwareDeviceGPU,
			Key:     "pci:0000:01:00.0",
			Vendor:  "nvidia",
			Model:   "High model",
			TempC:   intPointer(65),
			Sources: []string{"nvml"},
		}},
	}
	low := &fakeHardwareProvider{
		name: "sysfs",
		kind: hardwareDeviceGPU,
		readings: []hardwareReading{{
			Kind:    hardwareDeviceGPU,
			Key:     "pci:0000:01:00.0",
			Vendor:  "nvidia",
			Model:   "Low model",
			PowerW:  floatPointer(42.5),
			Sources: []string{"sysfs"},
		}},
	}
	manager := newHardwareManagerWithProviders(high, low)
	readings := manager.Read(false)
	if len(readings) != 1 {
		t.Fatalf("readings = %d, want 1", len(readings))
	}
	got := readings[0]
	if got.Model != "High model" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.TempC == nil || *got.TempC != 65 {
		t.Fatalf("temp = %+v", got.TempC)
	}
	if got.PowerW == nil || *got.PowerW != 42.5 {
		t.Fatalf("power = %+v", got.PowerW)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("sources = %v", got.Sources)
	}
}

func TestHardwareManagerSkipsFailedAndDisabledProviders(t *testing.T) {
	bad := &fakeHardwareProvider{name: "bad", kind: hardwareDeviceGPU, err: fmt.Errorf("boom")}
	gpu := &fakeHardwareProvider{
		name: "gpu",
		kind: hardwareDeviceGPU,
		readings: []hardwareReading{{
			Kind: hardwareDeviceGPU, Key: "gpu:0", TempC: intPointer(70),
		}},
	}
	cpu := &fakeHardwareProvider{
		name: "cpu",
		kind: hardwareDeviceCPU,
		readings: []hardwareReading{{
			Kind: hardwareDeviceCPU, Key: "cpu:0", TempC: intPointer(50),
		}},
	}
	manager := newHardwareManagerWithProviders(bad, gpu, cpu)

	readings := manager.Read(true)
	if len(readings) != 1 || readings[0].Key != "cpu:0" {
		t.Fatalf("readings = %+v", readings)
	}
	if gpu.reads != 0 {
		t.Fatalf("disabled GPU provider read %d times", gpu.reads)
	}
	if bad.reads != 0 {
		t.Fatalf("disabled GPU provider read %d times", bad.reads)
	}
}

func TestReadLinuxHardwareTelemetryReportsCPUAndGPU(t *testing.T) {
	oldManager := linuxHardwareManager
	linuxHardwareManager = newHardwareManagerWithProviders(
		&fakeHardwareProvider{
			name: "cpu-test",
			kind: hardwareDeviceCPU,
			readings: []hardwareReading{{
				Kind:    hardwareDeviceCPU,
				Key:     "cpu:package:0",
				Model:   "Test CPU",
				TempC:   intPointer(52),
				PowerW:  floatPointer(35.5),
				Sensors: []types.TemperatureSensor{{Key: "cpu/temp", Name: "Package", Value: 52}},
			}},
		},
		&fakeHardwareProvider{
			name: "gpu-test",
			kind: hardwareDeviceGPU,
			readings: []hardwareReading{{
				Kind:    hardwareDeviceGPU,
				Key:     "pci:0000:01:00.0",
				Vendor:  "nvidia",
				Model:   "Test GPU",
				TempC:   intPointer(61),
				PowerW:  floatPointer(88.5),
				Sensors: []types.TemperatureSensor{{Key: "gpu/temp", Name: "GPU", Value: 61}},
			}},
		},
	)
	t.Cleanup(func() { linuxHardwareManager = oldManager })

	reading := readLinuxHardwareTelemetry(nil, false)
	if reading.cpuTemp != 52 || reading.cpuPower != 35.5 || reading.cpuModel != "Test CPU" {
		t.Fatalf("cpu reading = %+v", reading)
	}
	if reading.gpuTemp != 61 || reading.gpuPower != 88.5 || reading.gpuModel != "Test GPU" {
		t.Fatalf("gpu reading = %+v", reading)
	}
	if len(reading.gpuDevices) != 1 || reading.selectedGpuDevice != "pci:0000:01:00.0" {
		t.Fatalf("gpu devices = %+v", reading.gpuDevices)
	}
}

func TestDisableGPUStillReadsCPUProvider(t *testing.T) {
	oldManager := linuxHardwareManager
	cpuProvider := &fakeHardwareProvider{
		name: "cpu",
		kind: hardwareDeviceCPU,
		readings: []hardwareReading{{
			Kind: hardwareDeviceCPU, Key: "cpu:package:0", TempC: intPointer(49),
		}},
	}
	gpuProvider := &fakeHardwareProvider{
		name: "gpu",
		kind: hardwareDeviceGPU,
		readings: []hardwareReading{{
			Kind: hardwareDeviceGPU, Key: "pci:0000:01:00.0", TempC: intPointer(70),
		}},
	}
	linuxHardwareManager = newHardwareManagerWithProviders(cpuProvider, gpuProvider)
	t.Cleanup(func() { linuxHardwareManager = oldManager })

	reading := readLinuxHardwareTelemetry(nil, true)
	if reading.cpuTemp != 49 {
		t.Fatalf("cpu temp = %d, want 49", reading.cpuTemp)
	}
	if reading.gpuTemp != 0 || len(reading.gpuDevices) != 0 || gpuProvider.reads != 0 {
		t.Fatalf("gpu should be skipped: reading=%+v reads=%d", reading, gpuProvider.reads)
	}
}

func TestLinuxFallbackDoesNotForkNvidiaSmi(t *testing.T) {
	oldExec := execHelperCommand
	execHelperCommand = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
		t.Fatalf("Linux provider 路径不应启动外部命令: %s %v", name, args)
		return nil, nil
	}
	t.Cleanup(func() { execHelperCommand = oldExec })

	oldManager := linuxHardwareManager
	linuxHardwareManager = newHardwareManagerWithProviders(&fakeHardwareProvider{
		name: "empty", kind: hardwareDeviceCPU,
	})
	t.Cleanup(func() { linuxHardwareManager = oldManager })

	reader := NewReader(nil, testLogger{})
	reader.readNativeTelemetry(false)
}

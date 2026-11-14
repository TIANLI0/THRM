//go:build linux

package temperature

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func TestCPUProviderUsesGopsutilOnlyForModel(t *testing.T) {
	oldInfo := readGopsutilCPUInfo
	linuxCPUModelOnce = sync.Once{}
	readGopsutilCPUInfo = func() ([]cpu.InfoStat, error) {
		return []cpu.InfoStat{{ModelName: "Test CPU"}}, nil
	}
	t.Cleanup(func() {
		readGopsutilCPUInfo = oldInfo
		linuxCPUModelOnce = sync.Once{}
	})

	readings, err := newCPUProvider().Read()
	if err != nil || len(readings) != 1 {
		t.Fatalf("readings = %+v, err=%v", readings, err)
	}
	if readings[0].Model != "Test CPU" {
		t.Fatalf("cpu model = %q", readings[0].Model)
	}
}

func TestCPUProviderReadsBtopStyleHwmonSensors(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "class", "hwmon", "hwmon0")
	mustMkdirHardware(t, hwmon)
	mustWriteHardware(t, filepath.Join(hwmon, "name"), "coretemp\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_input"), "55000\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_label"), "Package id 0\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp2_input"), "60000\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp2_label"), "Core 0\n")

	oldPower := linuxCPUPower
	linuxCPUPower = &linuxCPUPowerProvider{root: root}
	t.Cleanup(func() { linuxCPUPower = oldPower })

	readings, err := (&cpuProvider{sysfsRoot: root}).Read()
	if err != nil || len(readings) != 1 {
		t.Fatalf("readings = %+v, err=%v", readings, err)
	}
	reading := readings[0]
	if reading.TempC == nil || *reading.TempC != 55 {
		t.Fatalf("temperature = %+v, want package 55", reading.TempC)
	}
	if reading.PowerW != nil {
		t.Fatalf("CPU hwmon power fallback should be removed, got %+v", reading.PowerW)
	}
	if len(reading.Sensors) != 2 {
		t.Fatalf("sensors = %+v", reading.Sensors)
	}
}

func TestLinuxCPUSysfsThermalZoneFallback(t *testing.T) {
	root := t.TempDir()
	zone := filepath.Join(root, "class", "thermal", "thermal_zone0")
	mustMkdirHardware(t, zone)
	mustWriteHardware(t, filepath.Join(zone, "type"), "x86_pkg_temp\n")
	mustWriteHardware(t, filepath.Join(zone, "temp"), "44000\n")

	reading := readLinuxCPUSysfsFromRoot(root)
	if reading.TemperatureC != 44 {
		t.Fatalf("temperature = %d, want 44", reading.TemperatureC)
	}
	if len(reading.Sensors) != 1 || reading.Sensors[0].Value != 44 {
		t.Fatalf("sensors = %+v", reading.Sensors)
	}
}

func TestLinuxCPUSysfsExcludesGPU(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "class", "hwmon", "hwmon0")
	mustMkdirHardware(t, hwmon)
	mustWriteHardware(t, filepath.Join(hwmon, "name"), "amdgpu\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_input"), "70000\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_label"), "edge\n")

	reading := readLinuxCPUSysfsFromRoot(root)
	if reading.TemperatureC != 0 || len(reading.Sensors) != 0 {
		t.Fatalf("reading = %+v, want empty", reading)
	}
}

func TestLinuxCPUPowerRAPL(t *testing.T) {
	provider := &linuxCPUPowerProvider{}
	now := time.Unix(100, 0)
	if got := provider.energyDeltaWatts("rapl", 1_000_000, now); got != 0 {
		t.Fatalf("first = %f", got)
	}
	if got := provider.energyDeltaWatts("rapl", 11_000_000, now.Add(time.Second)); got != 10 {
		t.Fatalf("second = %f, want 10", got)
	}
	if got := provider.energyDeltaWatts("rapl", 1, now.Add(2*time.Second)); got != 0 {
		t.Fatalf("wrap = %f, want 0", got)
	}
}

func TestSysfsGPUProviderReadsUnknownVendor(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "class", "drm", "card7", "device")
	mustMkdirHardware(t, device)
	mustWriteHardware(t, filepath.Join(device, "class"), "0x030000\n")
	mustWriteHardware(t, filepath.Join(device, "vendor"), "0x1234\n")
	mustWriteHardware(t, filepath.Join(device, "device"), "0xabcd\n")

	readings, err := (&sysfsGPUProvider{root: root}).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1 {
		t.Fatalf("readings = %d, want 1", len(readings))
	}
	reading := readings[0]
	if reading.Kind != hardwareDeviceGPU || reading.Vendor != "pci:1234" {
		t.Fatalf("reading = %+v", reading)
	}
	if reading.Model != "GPU 1234:ABCD" {
		t.Fatalf("model = %q", reading.Model)
	}
}

func TestSysfsGPUProviderReadsTemperatureAndPower(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "class", "drm", "card0", "device")
	hwmon := filepath.Join(device, "hwmon", "hwmon0")
	mustMkdirHardware(t, hwmon)
	mustWriteHardware(t, filepath.Join(device, "class"), "0x030000\n")
	mustWriteHardware(t, filepath.Join(device, "vendor"), "0x1002\n")
	mustWriteHardware(t, filepath.Join(device, "device"), "0x73bf\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_input"), "47000\n")
	mustWriteHardware(t, filepath.Join(hwmon, "temp1_label"), "edge\n")
	mustWriteHardware(t, filepath.Join(hwmon, "power1_average"), "95000000\n")

	readings, err := (&sysfsGPUProvider{root: root}).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 1 {
		t.Fatalf("readings = %d, want 1", len(readings))
	}
	reading := readings[0]
	if reading.TempC == nil || *reading.TempC != 47 {
		t.Fatalf("temperature = %+v", reading.TempC)
	}
	if reading.PowerW == nil || *reading.PowerW != 95 {
		t.Fatalf("power = %+v", reading.PowerW)
	}
	if len(reading.Sensors) != 1 || len(reading.PowerSensors) != 1 {
		t.Fatalf("sensors = %+v", reading)
	}
}

func TestSysfsGPUProviderEnergyDeltaWatts(t *testing.T) {
	provider := &sysfsGPUProvider{}
	now := time.Unix(100, 0)
	if got := provider.energyDeltaWatts("energy", 1_000_000, now); got != 0 {
		t.Fatalf("first sample = %f", got)
	}
	if got := provider.energyDeltaWatts("energy", 11_000_000, now.Add(time.Second)); got != 10 {
		t.Fatalf("second sample = %f, want 10", got)
	}
	if got := provider.energyDeltaWatts("energy", 1, now.Add(2*time.Second)); got != 0 {
		t.Fatalf("wrapped sample = %f, want 0", got)
	}
}

func TestSysfsGPUProviderSkipsNonDisplayDevice(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "class", "drm", "card3", "device")
	mustMkdirHardware(t, device)
	mustWriteHardware(t, filepath.Join(device, "class"), "0x020000\n")
	mustWriteHardware(t, filepath.Join(device, "vendor"), "0x8086\n")
	mustWriteHardware(t, filepath.Join(device, "device"), "0x1234\n")

	readings, err := (&sysfsGPUProvider{root: root}).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 0 {
		t.Fatalf("readings = %+v, want none", readings)
	}
}

func TestPCINameDatabaseParser(t *testing.T) {
	content := "1002  Advanced Micro Devices, Inc. [AMD/ATI]\n" +
		"\t73bf  Navi 21 [Radeon RX 6800/6800 XT / 6900 XT]\n" +
		"10de  NVIDIA Corporation\n" +
		"\t2684  AD102 [GeForce RTX 4090]\n"
	path := filepath.Join(t.TempDir(), "pci.ids")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	names := make(map[uint32]string)
	parsePCINameDatabase(file, names)
	if got := names[0x1002<<16|0x73bf]; got != "Radeon RX 6800/6800 XT / 6900 XT" {
		t.Fatalf("amd name = %q", got)
	}
	if got := names[0x10de<<16|0x2684]; got != "GeForce RTX 4090" {
		t.Fatalf("nvidia name = %q", got)
	}
}

func mustMkdirHardware(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteHardware(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRAPLHelperFallback(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "thrm-rapl-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '12.5\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THRM_RAPL_HELPER", helper)

	provider := &linuxCPUPowerProvider{root: t.TempDir()}
	if got := provider.Read(); got != 0 {
		t.Fatalf("first helper read = %f, want 0 before first sample", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := provider.Read(); got == 12.5 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("helper fallback did not return 12.5W")
}

func TestParseRAPLHelperPower(t *testing.T) {
	cases := map[string]struct {
		want  float64
		valid bool
	}{
		"12.5":    {12.5, true},
		"0":       {0, true},
		"-1":      {0, false},
		"2001":    {0, false},
		"NaN":     {0, false},
		"garbage": {0, false},
	}
	for input, tc := range cases {
		got, ok := parseRAPLHelperPower(input)
		if ok != tc.valid || got != tc.want {
			t.Fatalf("parseRAPLHelperPower(%q) = (%f,%v), want (%f,%v)", input, got, ok, tc.want, tc.valid)
		}
	}
}

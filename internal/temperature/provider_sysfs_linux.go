//go:build linux

package temperature

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

// sysfsGPUProvider 通过标准 DRM + hwmon 接口读取所有厂商 GPU。
// 未知厂商只要暴露标准 sysfs ABI，也能获得基础温度和功耗。
type sysfsGPUProvider struct {
	root string

	energyMutex sync.Mutex
	energyState map[string]hardwareEnergyState
}

type hardwareEnergyState struct {
	lastUJ uint64
	at     time.Time
	valid  bool
}

type hardwareHwmonQuirk struct {
	Driver      string
	TempFiles   []string
	PowerFiles  []string
	EnergyFiles []string
}

var hardwareHwmonQuirks = []hardwareHwmonQuirk{
	{Driver: "amdgpu", TempFiles: []string{"temp1_input"}, PowerFiles: []string{"power1_average", "power1_input"}},
	{Driver: "radeon", TempFiles: []string{"temp1_input"}, PowerFiles: []string{"power1_average", "power1_input"}},
	{Driver: "i915", TempFiles: []string{"temp1_input"}},
	{Driver: "xe", TempFiles: []string{"temp2_input", "temp1_input"}},
	{Driver: "nouveau", TempFiles: []string{"temp1_input"}},
}

func (p *sysfsGPUProvider) Name() string             { return "sysfs" }
func (p *sysfsGPUProvider) Kind() hardwareDeviceKind { return hardwareDeviceGPU }

func (p *sysfsGPUProvider) Read() ([]hardwareReading, error) {
	root := p.root
	if root == "" {
		root = "/sys"
	}
	drmRoot := filepath.Join(root, "class", "drm")
	entries, err := os.ReadDir(drmRoot)
	if err != nil {
		return nil, err
	}

	var readings []hardwareReading
	for _, entry := range entries {
		cardName := entry.Name()
		if !isHardwareDRMCardName(cardName) {
			continue
		}
		devicePath := filepath.Join(drmRoot, cardName, "device")
		if _, err := os.Stat(devicePath); err != nil {
			continue
		}
		class := readHardwareClass(devicePath)
		if class != 0 && ((class>>16)&0xff) != 0x03 {
			continue
		}
		vendorID, _ := readHardwareHexUint16(filepath.Join(devicePath, "vendor"))
		deviceID, _ := readHardwareHexUint16(filepath.Join(devicePath, "device"))
		vendor := gpuVendorName(vendorID)
		if class == 0 && vendorID == 0 {
			continue
		}

		driver := readHardwareLinkBase(filepath.Join(devicePath, "driver"))
		bdf := hardwarePCIBDFFromDevicePath(devicePath)
		key := "drm:" + cardName
		if bdf != "" {
			key = "pci:" + bdf
		}
		reading := hardwareReading{
			Kind:    hardwareDeviceGPU,
			Key:     key,
			Vendor:  vendor,
			Model:   resolveGPUModel(devicePath, vendor, vendorID, deviceID),
			Sources: []string{"sysfs"},
		}

		hwmonPath := findGPUHardwareHwmon(devicePath)
		if hwmonPath != "" {
			quirk, _ := hardwareQuirkForDriver(driver)
			tempC, tempSensors := readGPUHwmonTemperatures(hwmonPath, key, quirk)
			if tempC > 0 {
				reading.TempC = intPointer(tempC)
			}
			reading.Sensors = tempSensors
			powerW, limitW, powerSensors := p.readGPUHwmonPower(hwmonPath, key, quirk)
			if powerW > 0 {
				reading.PowerW = floatPointer(powerW)
			}
			if limitW > 0 {
				reading.LimitW = floatPointer(limitW)
			}
			reading.PowerSensors = powerSensors
		}
		readings = append(readings, reading)
	}
	return readings, nil
}

func resolveGPUModel(devicePath, vendor string, vendorID, deviceID uint16) string {
	if resolved, ok := linuxPCIIDResolver.Resolve(vendorID, deviceID); ok {
		return resolved
	}
	if productName := readHardwareString(filepath.Join(devicePath, "product_name")); productName != "" {
		return productName
	}
	return fallbackPCIGPUModel(vendor, vendorID, deviceID)
}

func readHardwareClass(devicePath string) uint64 {
	raw := strings.TrimPrefix(strings.ToLower(readHardwareString(filepath.Join(devicePath, "class"))), "0x")
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return 0
	}
	return value
}

func findGPUHardwareHwmon(devicePath string) string {
	for _, dir := range findHardwareHwmonDirs(devicePath) {
		if hasHardwareHwmonData(dir) {
			return dir
		}
	}
	return ""
}

func hardwareQuirkForDriver(driver string) (hardwareHwmonQuirk, bool) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	for _, quirk := range hardwareHwmonQuirks {
		if quirk.Driver == driver {
			return quirk, true
		}
	}
	return hardwareHwmonQuirk{}, false
}

func readGPUHwmonTemperatures(hwmonPath, key string, quirk hardwareHwmonQuirk) (int, []types.TemperatureSensor) {
	type candidate struct {
		value int
		score int
	}
	var best candidate
	var sensors []types.TemperatureSensor

	inputs, _ := filepath.Glob(filepath.Join(hwmonPath, "temp*_input"))
	sort.Strings(inputs)
	for _, path := range inputs {
		value := readHardwareMilliCelsius(path)
		if value <= 0 {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(path), "_input")
		label := readHardwareString(filepath.Join(hwmonPath, base+"_label"))
		name := label
		if name == "" {
			name = base
		}
		sensors = append(sensors, types.TemperatureSensor{
			Key:   key + "/hwmon/" + base,
			Name:  name,
			Value: value,
		})
		score := scoreGPUHwmonTemperature(base, label, quirk)
		if score > best.score || (score == best.score && value > best.value) {
			best = candidate{value: value, score: score}
		}
	}
	return best.value, sensors
}

func scoreGPUHwmonTemperature(base, label string, quirk hardwareHwmonQuirk) int {
	for rank, candidate := range quirk.TempFiles {
		if strings.EqualFold(base+"_input", candidate) {
			return 1000 - rank
		}
	}
	label = strings.ToLower(label)
	switch {
	case strings.Contains(label, "edge"):
		return 900
	case strings.Contains(label, "junction"):
		return 880
	case strings.Contains(label, "gpu"):
		return 860
	case strings.Contains(label, "die"):
		return 840
	case strings.Contains(label, "package"):
		return 820
	case strings.Contains(label, "temp"):
		return 700
	case label != "":
		return 500
	default:
		return 100
	}
}

func (p *sysfsGPUProvider) readGPUHwmonPower(hwmonPath, key string, quirk hardwareHwmonQuirk) (float64, float64, []types.PowerSensor) {
	var sensors []types.PowerSensor
	for _, name := range firstNonEmptyPaths(quirk.PowerFiles, []string{"power1_average", "power1_input", "power2_average", "power2_input"}) {
		raw, err := readHardwareUint(filepath.Join(hwmonPath, name))
		if err != nil || raw == 0 || raw > 2_000_000_000 {
			continue
		}
		sensors = append(sensors, types.PowerSensor{
			Key:   key + "/hwmon/" + strings.TrimSuffix(name, "_input") + "_watts",
			Name:  name,
			Value: float64(raw) / 1_000_000,
		})
	}
	powerW := 0.0
	if len(sensors) > 0 {
		powerW = sensors[0].Value
	}
	for _, name := range firstNonEmptyPaths(quirk.EnergyFiles, []string{"energy1_input", "energy2_input"}) {
		path := filepath.Join(hwmonPath, name)
		raw, err := readHardwareUint(path)
		if err != nil || raw == 0 {
			continue
		}
		if watts := p.energyDeltaWatts(path, raw, time.Now()); watts > 0 {
			sensors = append(sensors, types.PowerSensor{
				Key:   key + "/hwmon/" + strings.TrimSuffix(name, "_input") + "_watts",
				Name:  name,
				Value: watts,
			})
			if powerW == 0 {
				powerW = watts
			}
			break
		}
	}
	limitW := 0.0
	for _, name := range []string{"power1_cap", "power2_cap", "power1_max", "power2_max"} {
		raw, err := readHardwareUint(filepath.Join(hwmonPath, name))
		if err != nil || raw == 0 || raw > 2_000_000_000 {
			continue
		}
		limitW = float64(raw) / 1_000_000
		break
	}
	return powerW, limitW, sensors
}

func firstNonEmptyPaths(preferred, fallback []string) []string {
	if len(preferred) > 0 {
		return preferred
	}
	return fallback
}

func (p *sysfsGPUProvider) energyDeltaWatts(key string, currentUJ uint64, now time.Time) float64 {
	p.energyMutex.Lock()
	defer p.energyMutex.Unlock()
	if p.energyState == nil {
		p.energyState = make(map[string]hardwareEnergyState)
	}
	previous, ok := p.energyState[key]
	p.energyState[key] = hardwareEnergyState{lastUJ: currentUJ, at: now, valid: true}
	if !ok || !previous.valid || currentUJ < previous.lastUJ {
		return 0
	}
	elapsed := now.Sub(previous.at).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(currentUJ-previous.lastUJ) / 1_000_000 / elapsed
}

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }

func (p *sysfsGPUProvider) String() string {
	return fmt.Sprintf("sysfs(root=%s)", p.root)
}

func readHardwareString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readHardwareUint(path string) (uint64, error) {
	raw := readHardwareString(path)
	if raw == "" {
		return 0, fmt.Errorf("空文件: %s", path)
	}
	return strconv.ParseUint(raw, 10, 64)
}

func readHardwareHexUint16(path string) (uint16, error) {
	raw := strings.TrimPrefix(strings.ToLower(readHardwareString(path)), "0x")
	if raw == "" {
		return 0, fmt.Errorf("空文件: %s", path)
	}
	value, err := strconv.ParseUint(raw, 16, 16)
	if err != nil {
		return 0, err
	}
	return uint16(value), nil
}

func readHardwareLinkBase(path string) string {
	target, err := os.Readlink(path)
	if err != nil || target == "" {
		return ""
	}
	return filepath.Base(target)
}

func readHardwareMilliCelsius(path string) int {
	value, err := readHardwareUint(path)
	if err != nil || value == 0 || value >= 150_000 {
		return 0
	}
	return int((value + 500) / 1000)
}

func isHardwareDRMCardName(name string) bool {
	if !strings.HasPrefix(name, "card") || len(name) <= len("card") {
		return false
	}
	for _, char := range name[len("card"):] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// normalizePCIBDF 将不同 provider 的 BDF 统一成 "0000:bb:dd.f"。
func normalizePCIBDF(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	parts := strings.Split(value, ":")
	if len(parts) == 2 {
		value = "0000:" + value
		parts = strings.Split(value, ":")
	}
	if len(parts) != 3 {
		return ""
	}
	domain, err := strconv.ParseUint(parts[0], 16, 16)
	if err != nil {
		return ""
	}
	bus, err := strconv.ParseUint(parts[1], 16, 8)
	if err != nil {
		return ""
	}
	if !isPCIDeviceFunction(parts[2]) {
		return ""
	}
	return fmt.Sprintf("%04x:%02x:%s", domain, bus, parts[2])
}

func isPCIDeviceFunction(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	if _, err := strconv.ParseUint(parts[0], 16, 8); err != nil {
		return false
	}
	if _, err := strconv.ParseUint(parts[1], 16, 8); err != nil {
		return false
	}
	return true
}

func hardwarePCIBDFFromDevicePath(devicePath string) string {
	resolved, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		resolved = devicePath
	}
	return normalizePCIBDF(filepath.Base(resolved))
}

func findHardwareHwmonDirs(devicePath string) []string {
	dirs, _ := filepath.Glob(filepath.Join(devicePath, "hwmon", "hwmon*"))
	sort.Strings(dirs)
	return dirs
}

func hasHardwareHwmonData(dir string) bool {
	patterns := []string{"temp*_input", "power*_average", "power*_input", "energy*_input"}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

// pciIDResolver 是可选名称增强。文件不存在时不影响任何采集功能。
type pciIDResolver struct {
	once  sync.Once
	names map[uint32]string
}

var linuxPCIIDResolver pciIDResolver

func (r *pciIDResolver) Resolve(vendorID, deviceID uint16) (string, bool) {
	r.once.Do(func() {
		r.names = loadPCINameDatabase()
	})
	name, ok := r.names[uint32(vendorID)<<16|uint32(deviceID)]
	return name, ok
}

func loadPCINameDatabase() map[uint32]string {
	names := make(map[uint32]string)
	paths := []string{
		"/usr/share/hwdata/pci.ids",
		"/usr/share/misc/pci.ids",
		"/usr/local/share/hwdata/pci.ids",
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		parsePCINameDatabase(file, names)
		file.Close()
		if len(names) > 0 {
			break
		}
	}
	return names
}

func parsePCINameDatabase(file *os.File, names map[uint32]string) {
	scanner := bufio.NewScanner(file)
	var vendor uint16
	haveVendor := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line[0] != '\t' {
			if len(line) < 4 {
				haveVendor = false
				continue
			}
			value, err := strconv.ParseUint(line[:4], 16, 16)
			if err != nil {
				haveVendor = false
				continue
			}
			vendor = uint16(value)
			haveVendor = true
			continue
		}
		if !haveVendor || strings.HasPrefix(line, "\t\t") || len(line) < 6 {
			continue
		}
		value, err := strconv.ParseUint(strings.TrimSpace(line[1:5]), 16, 16)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(line[6:])
		if idx := strings.IndexByte(name, '['); idx >= 0 {
			if end := strings.IndexByte(name[idx+1:], ']'); end >= 0 {
				name = name[idx+1 : idx+1+end]
			}
		}
		if name != "" {
			names[uint32(vendor)<<16|uint32(value)] = name
		}
	}
}

func gpuVendorName(vendorID uint16) string {
	switch vendorID {
	case 0x10de:
		return "nvidia"
	case 0x1002:
		return "amd"
	case 0x8086:
		return "intel"
	case 0x106b:
		return "apple"
	case 0x13b5:
		return "arm"
	case 0x5143:
		return "qualcomm"
	default:
		return fmt.Sprintf("pci:%04x", vendorID)
	}
}

func fallbackPCIGPUModel(vendor string, vendorID, deviceID uint16) string {
	switch vendor {
	case "nvidia":
		return fmt.Sprintf("NVIDIA GPU %04X:%04X", vendorID, deviceID)
	case "amd":
		return fmt.Sprintf("AMD GPU %04X:%04X", vendorID, deviceID)
	case "intel":
		return fmt.Sprintf("Intel GPU %04X:%04X", vendorID, deviceID)
	case "apple":
		return fmt.Sprintf("Apple GPU %04X:%04X", vendorID, deviceID)
	default:
		return fmt.Sprintf("GPU %04X:%04X", vendorID, deviceID)
	}
}

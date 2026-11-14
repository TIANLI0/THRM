//go:build linux

package temperature

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
	"github.com/shirou/gopsutil/v4/cpu"
)

var (
	linuxCPUModelOnce sync.Once
	linuxCPUModel     string

	readGopsutilCPUInfo = cpu.Info
)

type linuxCPUPowerProvider struct {
	root string

	mutex  sync.Mutex
	lastUJ uint64
	lastAt time.Time
	valid  bool

	helperOnce sync.Once
	helper     *raplHelperClient
}

var linuxCPUPower = &linuxCPUPowerProvider{root: "/sys"}

const (
	raplHelperName     = "thrm-rapl-helper"
	raplHelperMaxAge   = 5 * time.Second
	raplHelperMaxWatts = 2000.0
)

type raplHelperClient struct {
	mutex   sync.Mutex
	latestW float64
	lastAt  time.Time
}

func (c *raplHelperClient) setPower(watts float64) {
	c.mutex.Lock()
	c.latestW = watts
	c.lastAt = time.Now()
	c.mutex.Unlock()
}

func (c *raplHelperClient) power() float64 {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.lastAt.IsZero() || time.Since(c.lastAt) > raplHelperMaxAge {
		return 0
	}
	return c.latestW
}

func startRAPLHelper() *raplHelperClient {
	path := findRAPLHelper()
	if path == "" {
		return nil
	}
	cmd := exec.Command(path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil
	}

	client := &raplHelperClient{}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if watts, ok := parseRAPLHelperPower(scanner.Text()); ok {
				client.setPower(watts)
			}
		}
	}()
	go func() { _ = cmd.Wait() }()
	return client
}

func findRAPLHelper() string {
	if configured := strings.TrimSpace(os.Getenv("THRM_RAPL_HELPER")); configured != "" {
		if isExecutableFile(configured) {
			return configured
		}
		return ""
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(dir, raplHelperName),
			filepath.Join(dir, "..", "lib", "thrm", raplHelperName),
		} {
			if isExecutableFile(candidate) {
				return candidate
			}
		}
	}
	if path, err := exec.LookPath(raplHelperName); err == nil {
		return path
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func parseRAPLHelperPower(line string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > raplHelperMaxWatts {
		return 0, false
	}
	return value, true
}

type cpuProvider struct {
	sysfsRoot string
}

func newCPUProvider() *cpuProvider {
	return &cpuProvider{sysfsRoot: "/sys"}
}

func (p *cpuProvider) Name() string             { return "cpu" }
func (p *cpuProvider) Kind() hardwareDeviceKind { return hardwareDeviceCPU }

func (p *cpuProvider) Read() ([]hardwareReading, error) {
	reading := hardwareReading{
		Kind:    hardwareDeviceCPU,
		Key:     "cpu:package:0",
		Vendor:  "cpu",
		Model:   readLinuxCPUModel(),
		Sources: []string{"cpu"},
	}
	reading.TempC, reading.Sensors = p.readCPUTemperature()
	if power := linuxCPUPower.Read(); power > 0 {
		reading.PowerW = floatPointer(power)
	}
	return []hardwareReading{reading}, nil
}

func (p *cpuProvider) readCPUTemperature() (*int, []types.TemperatureSensor) {
	reading := readLinuxCPUSysfsFromRoot(p.sysfsRoot)
	if reading.TemperatureC <= 0 {
		return nil, reading.Sensors
	}
	return intPointer(reading.TemperatureC), reading.Sensors
}

func readLinuxCPUModel() string {
	linuxCPUModelOnce.Do(func() {
		infos, err := readGopsutilCPUInfo()
		if err != nil || len(infos) == 0 {
			return
		}
		info := infos[0]
		linuxCPUModel = strings.TrimSpace(info.ModelName)
		if linuxCPUModel == "" && info.VendorID != "" {
			linuxCPUModel = strings.TrimSpace(strings.Join([]string{info.VendorID, info.Family, info.Model}, " "))
		}
	})
	return linuxCPUModel
}

// Read 优先直接读取 Intel RAPL 的累计能量；权限不足时按需启动
// thrm-rapl-helper。helper 固定只读 powercap energy_uj，不接收任意路径。
func (p *linuxCPUPowerProvider) Read() float64 {
	root := p.root
	if root == "" {
		root = "/sys"
	}
	raplPath := filepath.Join(root, "class", "powercap", "intel-rapl:0", "energy_uj")
	if raw, err := readHardwareUint(raplPath); err == nil && raw > 0 {
		return p.energyDeltaWatts(raplPath, raw, time.Now())
	}

	p.helperOnce.Do(func() {
		p.helper = startRAPLHelper()
	})
	if p.helper != nil {
		return p.helper.power()
	}
	return 0
}

func (p *linuxCPUPowerProvider) energyDeltaWatts(key string, currentUJ uint64, now time.Time) float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if !p.valid {
		p.lastUJ = currentUJ
		p.lastAt = now
		p.valid = true
		return 0
	}
	if currentUJ < p.lastUJ {
		p.lastUJ = currentUJ
		p.lastAt = now
		return 0
	}
	elapsed := now.Sub(p.lastAt).Seconds()
	previousUJ := p.lastUJ
	p.lastUJ = currentUJ
	p.lastAt = now
	if elapsed <= 0 {
		return 0
	}
	return float64(currentUJ-previousUJ) / 1_000_000 / elapsed
}

func (p *linuxCPUPowerProvider) String() string {
	return fmt.Sprintf("cpu-power(root=%s)", p.root)
}

type linuxCPUSysfsReading struct {
	TemperatureC int
	Sensors      []types.TemperatureSensor
}

// readLinuxCPUSysfsFromRoot 以 btop 的方式扫描 hwmon；
// hwmon 找不到主 CPU 温度时回退 thermal_zone。
func readLinuxCPUSysfsFromRoot(root string) linuxCPUSysfsReading {
	if root == "" {
		root = "/sys"
	}

	var result linuxCPUSysfsReading
	bestScore := -1
	hwmonDirs, _ := filepath.Glob(filepath.Join(root, "class", "hwmon", "hwmon*"))
	for _, dir := range hwmonDirs {
		driver := strings.ToLower(readHardwareString(filepath.Join(dir, "name")))
		if isExcludedCPUTempSource(driver) {
			continue
		}
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		sort.Strings(inputs)
		for _, input := range inputs {
			value := readHardwareMilliCelsius(input)
			if value <= 0 {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(input), "_input")
			label := readHardwareString(filepath.Join(dir, base+"_label"))
			score := scoreLinuxCPUTempSource(driver, label)
			if score <= 0 {
				continue
			}
			name := label
			if name == "" {
				name = base
			}
			result.Sensors = append(result.Sensors, types.TemperatureSensor{
				Key:   "cpu/hwmon/" + driver + "/" + base,
				Name:  name,
				Value: value,
			})
			if score > bestScore || (score == bestScore && value > result.TemperatureC) {
				bestScore = score
				result.TemperatureC = value
			}
		}
	}

	// btop 在找不到可靠 hwmon CPU 温度时扫描 thermal_zone。
	zones, _ := filepath.Glob(filepath.Join(root, "class", "thermal", "thermal_zone*"))
	sort.Strings(zones)
	for i, zone := range zones {
		zoneType := strings.ToLower(readHardwareString(filepath.Join(zone, "type")))
		if isExcludedCPUTempSource(zoneType) {
			continue
		}
		value := readHardwareMilliCelsius(filepath.Join(zone, "temp"))
		if value <= 0 {
			continue
		}
		score := scoreLinuxCPUThermalZone(zoneType)
		if score <= 0 {
			continue
		}
		result.Sensors = append(result.Sensors, types.TemperatureSensor{
			Key:   "cpu/thermal/" + strconv.Itoa(i) + "/" + zoneType,
			Name:  zoneType,
			Value: value,
		})
		if score > bestScore || (score == bestScore && value > result.TemperatureC) {
			bestScore = score
			result.TemperatureC = value
		}
	}

	sort.SliceStable(result.Sensors, func(i, j int) bool {
		return result.Sensors[i].Key < result.Sensors[j].Key
	})
	return result
}

func isExcludedCPUTempSource(name string) bool {
	for _, marker := range []string{"gpu", "nvidia", "amdgpu", "radeon", "nouveau", "i915", "xe_", "nvme", "battery"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func scoreLinuxCPUTempSource(driver, label string) int {
	driver = strings.ToLower(driver)
	label = strings.ToLower(label)

	base := 0
	switch driver {
	case "coretemp", "k10temp", "zenpower", "fam15h_power":
		base = 300
	default:
		if strings.Contains(driver, "cpu") || strings.Contains(driver, "soc") || strings.Contains(driver, "core") {
			base = 100
		}
	}

	labelScore := 0
	switch {
	case strings.Contains(label, "package"):
		labelScore = 1000
	case strings.Contains(label, "tdie"):
		labelScore = 950
	case strings.Contains(label, "tctl"):
		labelScore = 900
	case strings.Contains(label, "soc temperature"):
		labelScore = 850
	case strings.Contains(label, "tccd"):
		labelScore = 700
	case strings.Contains(label, "core"):
		labelScore = 600
	case strings.Contains(label, "cpu"):
		labelScore = 500
	default:
		if base > 0 {
			labelScore = 200
		}
	}
	return base + labelScore
}

func scoreLinuxCPUThermalZone(zoneType string) int {
	switch {
	case strings.Contains(zoneType, "x86_pkg_temp"):
		return 1000
	case strings.Contains(zoneType, "pkg"):
		return 900
	case strings.Contains(zoneType, "cpu"):
		return 800
	case strings.Contains(zoneType, "soc"):
		return 700
	default:
		return 0
	}
}

// Linux 的 CPU 温度统一走 sysfs 扫描，gopsutil sensors 不再参与。
func (r *Reader) readSensorCPUTemp() int {
	return 0
}

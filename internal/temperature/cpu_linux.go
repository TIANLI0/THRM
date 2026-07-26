//go:build linux

package temperature

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// platformCPUTempIsExpensive 报告平台 CPU 温度读取是否需要创建外部进程。
// Linux 直接读 sysfs，开销可忽略，不应被降级节流缓存——在没有桥接的平台上
// 这就是主读取路径，缓存会让控温依据变旧。
const platformCPUTempIsExpensive = false

func (r *Reader) readPlatformCPUTemp() int {
	if temp := readLinuxCPUTempFromHwmon(); temp > 0 {
		return temp
	}
	return readLinuxCPUTempFromThermalZone()
}

func readLinuxCPUTempFromHwmon() int {
	hwmonDirs, err := filepath.Glob("/sys/class/hwmon/hwmon*")
	if err != nil {
		return 0
	}

	allowed := map[string]struct{}{
		"coretemp": {},
		"k10temp":  {},
		"zenpower": {},
	}

	bestTemp := 0
	bestScore := -1

	for _, hwmonDir := range hwmonDirs {
		name := strings.ToLower(readFirstLine(filepath.Join(hwmonDir, "name")))
		if _, ok := allowed[name]; !ok {
			continue
		}

		inputs, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
		if err != nil {
			continue
		}

		for _, input := range inputs {
			temp := parseMilliCFileToC(input)
			if temp <= 0 || temp >= 150 {
				continue
			}

			idx := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(input), "temp"), "_input")
			label := strings.ToLower(readFirstLine(filepath.Join(hwmonDir, "temp"+idx+"_label")))
			score := scoreCPULabel(label)
			if score > bestScore || (score == bestScore && temp > bestTemp) {
				bestScore = score
				bestTemp = temp
			}
		}
	}

	return bestTemp
}

func readLinuxCPUTempFromThermalZone() int {
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil {
		return 0
	}

	bestTemp := 0
	bestScore := -1

	for _, zone := range zones {
		zoneType := strings.ToLower(readFirstLine(filepath.Join(zone, "type")))
		temp := parseMilliCFileToC(filepath.Join(zone, "temp"))
		if temp <= 0 || temp >= 150 {
			continue
		}

		score := scoreCPUZoneType(zoneType)
		if score > bestScore || (score == bestScore && temp > bestTemp) {
			bestScore = score
			bestTemp = temp
		}
	}

	return bestTemp
}

func readFirstLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func parseMilliCFileToC(path string) int {
	raw := readFirstLine(path)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	if v > 1000 {
		return (v + 500) / 1000
	}
	return v
}

func scoreCPULabel(label string) int {
	switch {
	case strings.Contains(label, "package"):
		return 100
	case strings.Contains(label, "tdie"):
		return 90
	case strings.Contains(label, "tctl"):
		return 80
	case strings.Contains(label, "cpu"):
		return 70
	case strings.Contains(label, "core"):
		return 60
	case label != "":
		return 20
	default:
		return 10
	}
}

func scoreCPUZoneType(zoneType string) int {
	switch {
	case strings.Contains(zoneType, "x86_pkg_temp"):
		return 100
	case strings.Contains(zoneType, "pkg"):
		return 90
	case strings.Contains(zoneType, "cpu"):
		return 80
	default:
		return 10
	}
}


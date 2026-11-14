//go:build linux

// thrm-rapl-helper 是只读 Intel RAPL 能量计数的小型辅助程序。
//
// 它固定读取 /sys/class/powercap/intel-rapl:0/energy_uj，不接受外部路径参数，
// 避免把需要 CAP_DAC_READ_SEARCH 的进程扩大成任意文件读取器。
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	raplPath         = "/sys/class/powercap/intel-rapl:0/energy_uj"
	sampleInterval   = time.Second
	energyMicroJoule = 1_000_000.0
)

func main() {
	file, err := os.Open(raplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", raplPath, err)
		os.Exit(1)
	}
	defer file.Close()

	var previousUJ uint64
	var previousAt time.Time
	havePrevious := false
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		currentUJ, err := readEnergyUJ(file)
		now := time.Now()
		if err != nil {
			if _, writeErr := fmt.Fprintln(os.Stdout, "0"); writeErr != nil {
				os.Exit(0)
			}
		} else {
			watts := 0.0
			if havePrevious {
				watts = energyDeltaWatts(previousUJ, currentUJ, now.Sub(previousAt))
			}
			if _, writeErr := fmt.Fprintf(os.Stdout, "%.6f\n", watts); writeErr != nil {
				os.Exit(0)
			}
			previousUJ = currentUJ
			previousAt = now
			havePrevious = true
		}
		<-ticker.C
	}
}

func energyDeltaWatts(previousUJ, currentUJ uint64, elapsed time.Duration) float64 {
	if elapsed <= 0 || currentUJ < previousUJ {
		return 0
	}
	return float64(currentUJ-previousUJ) / energyMicroJoule / elapsed.Seconds()
}

func readEnergyUJ(file *os.File) (uint64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	var buffer [64]byte
	n, err := file.Read(buffer[:])
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(buffer[:n]))
	energy, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return energy, nil
}

//go:build ignore

// 独立调试脚本，不属于 THRM 构建：ignore 约束让 go build/vet/test ./... 跳过它。
// 运行方式：go run scripts/dev/sensors_probe.go
package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/sensors"
)

func main() {
	// 获取CPU信息
	cpus, err := cpu.Info()
	if err != nil {
		fmt.Println("Error getting CPU info:", err)
		return
	}

	// 打印CPU信息
	for _, cpu := range cpus {
		fmt.Printf("CPU: %s\n", cpu.ModelName)
		fmt.Printf("Core Count: %d\n", cpu.Cores)
		fmt.Printf("MHz: %f\n", cpu.Mhz)
	}

	// 获取CPU使用率
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		fmt.Printf("CPU Usage: %.2f%%\n", cpuPercent[0])
	}

	// 获取主机信息（包括温度信息，如果可用）
	hostInfo, err := host.Info()
	if err == nil {
		fmt.Printf("Host: %s\n", hostInfo.Hostname)
		fmt.Printf("系统: %s\n", hostInfo.Platform)
		fmt.Printf("系统版本: %s\n", hostInfo.PlatformVersion)
	}

	// 尝试获取传感器信息（可能需要管理员权限）
	fmt.Println("\n--- Sensor Information ---")
	sensors, err := sensors.SensorsTemperatures()
	if err != nil {
		fmt.Printf("获取传感器数据时出错: %v\n", err)
	} else {
		// 打印传感器信息
		for _, sensor := range sensors {
			fmt.Printf("Sensor: %s\n", sensor.SensorKey)
			fmt.Printf("Temperature: %.2f°C\n", sensor.Temperature)
		}
	}

	// 尝试获取GPU信息
	fmt.Println("\n--- GPU Information ---")
	gpus, err := GetNvidiaGPUInfo()
	if err != nil {
		fmt.Printf("获取GPU信息时出错: %v\n", err)
	} else {
		// 打印GPU信息
		for _, gpu := range gpus {
			fmt.Printf("GPU: %s\n", gpu.Name)
			fmt.Printf("Temperature: %d°C\n", gpu.Temperature)
		}
	}

}

// GPUInfo 表示单个GPU的信息
type GPUInfo struct {
	Name        string `json:"name"`
	Temperature int    `json:"temperature"` // 单位: °C
}

// GetNvidiaGPUInfo 使用 nvidia-smi 获取 GPU 名称和温度
// 返回 GPUInfo 切片，每个元素对应一个GPU
func GetNvidiaGPUInfo() ([]GPUInfo, error) {
	// 使用 nvidia-smi 查询 GPU 名称和温度，CSV 格式输出
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=name,temperature.gpu",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []GPUInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 格式: "GPU Name", temperature
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected nvidia-smi output format: %s", line)
		}

		name := strings.TrimSpace(parts[0])
		tempStr := strings.TrimSpace(parts[1])

		temp, err := strconv.Atoi(tempStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse temperature '%s': %w", tempStr, err)
		}

		gpus = append(gpus, GPUInfo{
			Name:        name,
			Temperature: temp,
		})
	}

	return gpus, nil
}

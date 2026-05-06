// Package temperature 提供温度读取功能
package temperature

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/bridge"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Reader 温度读取器
type Reader struct {
	bridgeManager *bridge.Manager
	logger        types.Logger
}

// NewReader 创建新的温度读取器
func NewReader(bridgeManager *bridge.Manager, logger types.Logger) *Reader {
	return &Reader{
		bridgeManager: bridgeManager,
		logger:        logger,
	}
}

// Read 读取温度
func (r *Reader) Read(selection types.TemperatureSelection) types.TemperatureData {
	selection = types.NormalizeTemperatureSelection(selection)
	temp := types.TemperatureData{
		UpdateTime:    time.Now().Unix(),
		BridgeOk:      true,
		ControlSource: selection.TempSource,
	}

	// 优先使用桥接程序读取温度
	bridgeTemp := r.bridgeManager.GetTemperature(selection)
	copyBridgeTemperatureMetadata(&temp, bridgeTemp, selection)
	if bridgeTemp.Success {
		if bridgeTemp.CpuTemp == 0 && bridgeTemp.GpuTemp == 0 {
			temp.BridgeOk = false
			temp.BridgeMsg = "桥接程序返回空温度（CPU/GPU 均为 0），请重启软件或重新安装 PawnIO 驱动。"
			r.logger.Warn("桥接程序返回空温度数据，使用备用方法")

			temp.CPUTemp = r.readCPUTemperature()
			temp.GPUTemp = r.readGPUTemperature()
			temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
			temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)
			return temp
		}

		temp.BridgeOk = true
		temp.BridgeMsg = ""
		return temp
	}

	// 如果桥接程序失败，使用备用方法
	r.logger.Warn("桥接程序读取温度失败: %s, 使用备用方法", bridgeTemp.Error)
	temp.BridgeOk = false
	temp.BridgeMsg = bridgeTemp.Error
	if strings.TrimSpace(temp.BridgeMsg) == "" {
		temp.BridgeMsg = "CPU/GPU 温度读取失败，请检查 PawnIO 是否安装成功，或升级最新版。"
	}

	// 读取CPU温度
	temp.CPUTemp = r.readCPUTemperature()

	// 读取GPU温度
	temp.GPUTemp = r.readGPUTemperature()

	// 计算最高温度
	temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
	temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)

	return temp
}

func copyBridgeTemperatureMetadata(temp *types.TemperatureData, bridgeTemp types.BridgeTemperatureData, selection types.TemperatureSelection) {
	if temp == nil {
		return
	}

	temp.CPUTemp = bridgeTemp.CpuTemp
	temp.GPUTemp = bridgeTemp.GpuTemp
	temp.MaxTemp = bridgeTemp.MaxTemp
	temp.ControlTemp = bridgeTemp.ControlTemp
	temp.ControlSource = bridgeTemp.ControlSource
	temp.SelectedGpuDevice = bridgeTemp.SelectedGpuDevice
	if temp.ControlSource == "" {
		temp.ControlSource = selection.TempSource
	}
	if temp.ControlTemp == 0 {
		temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, temp.ControlSource)
	}
	temp.CpuModel = bridgeTemp.CpuModel
	temp.GpuModel = bridgeTemp.GpuModel
	temp.CpuSensors = bridgeTemp.CpuSensors
	temp.GpuSensors = bridgeTemp.GpuSensors
	temp.GpuDevices = bridgeTemp.GpuDevices
	if bridgeTemp.UpdateTime > 0 {
		temp.UpdateTime = bridgeTemp.UpdateTime
	}
}

func resolveControlTemp(cpuTemp, gpuTemp int, source string) int {
	switch types.NormalizeTempSource(source) {
	case types.TempSourceCPU:
		return cpuTemp
	case types.TempSourceGPU:
		return gpuTemp
	default:
		return max(cpuTemp, gpuTemp)
	}
}

// readCPUTemperature 读取CPU温度
func (r *Reader) readCPUTemperature() int {
	sensorTemps, err := sensors.SensorsTemperatures()
	if err == nil {
		for _, sensor := range sensorTemps {
			// 查找ACPI ThermalZone TZ00_0或类似的CPU温度传感器
			if strings.Contains(strings.ToLower(sensor.SensorKey), "tz00") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "cpu") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "core") {
				return int(sensor.Temperature)
			}
		}
	}

	// 如果传感器方式失败，尝试通过WMI (Windows)
	return r.readWindowsCPUTemp()
}

// readGPUTemperature 读取GPU温度
func (r *Reader) readGPUTemperature() int {
	vendor := r.detectGPUVendor()
	return r.readGPUTempByVendor(vendor)
}

// readWindowsCPUTemp 通过WMI读取Windows CPU温度
func (r *Reader) readWindowsCPUTemp() int {
	output, err := execCommandHidden("wmic", "/namespace:\\\\root\\wmi", "PATH", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value")
	if err != nil {
		r.logger.Debug("读取Windows CPU温度失败: %v", err)
		return 0
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "CurrentTemperature="); ok {
			tempStr := after
			tempStr = strings.TrimSpace(tempStr)
			if tempStr != "" {
				if temp, err := strconv.Atoi(tempStr); err == nil {
					celsius := (temp - 2732) / 10
					if celsius > 0 && celsius < 150 {
						return celsius
					}
				}
			}
		}
	}

	return 0
}

// detectGPUVendor 检测GPU厂商
func (r *Reader) detectGPUVendor() string {
	// 尝试NVIDIA
	if _, err := execCommandHidden("nvidia-smi", "--version"); err == nil {
		return "nvidia"
	}

	return "unknown"
}

// readGPUTempByVendor 根据厂商读取GPU温度
func (r *Reader) readGPUTempByVendor(vendor string) int {
	switch vendor {
	case "nvidia":
		return r.readNvidiaGPUTemp()
	case "amd":
		return 0
	default:
		return 0
	}
}

// readNvidiaGPUTemp 安全读取NVIDIA GPU温度
func (r *Reader) readNvidiaGPUTemp() int {
	output, err := execCommandHidden("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		r.logger.Debug("读取NVIDIA GPU温度失败: %v", err)
		return 0
	}

	tempStr := strings.TrimSpace(string(output))
	lines := strings.Split(tempStr, "\n")

	if len(lines) > 0 && lines[0] != "" {
		if temp, err := strconv.Atoi(lines[0]); err == nil {
			return temp
		}
	}

	return 0
}

// execCommandHidden 执行命令并隐藏窗口
func execCommandHidden(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	return cmd.Output()
}

// CalculateTargetRPM 根据温度计算目标转速
func CalculateTargetRPM(temperature int, fanCurve []types.FanCurvePoint) int {
	if len(fanCurve) < 2 {
		return 0
	}

	if temperature <= fanCurve[0].Temperature {
		return fanCurve[0].RPM
	}

	lastPoint := fanCurve[len(fanCurve)-1]
	if temperature >= lastPoint.Temperature {
		return lastPoint.RPM
	}

	// 线性插值计算转速
	for i := 0; i < len(fanCurve)-1; i++ {
		p1 := fanCurve[i]
		p2 := fanCurve[i+1]

		if temperature >= p1.Temperature && temperature <= p2.Temperature {
			// 线性插值
			ratio := float64(temperature-p1.Temperature) / float64(p2.Temperature-p1.Temperature)
			rpm := float64(p1.RPM) + ratio*float64(p2.RPM-p1.RPM)
			return int(rpm)
		}
	}

	return 0
}

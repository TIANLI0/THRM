// Package temperature 提供温度读取功能
package temperature

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/bridge"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/shirou/gopsutil/v4/sensors"
)

const (
	helperCommandTimeout = 1200 * time.Millisecond
	gpuVendorCacheTTL    = 30 * time.Second
)

var (
	execHelperCommand = execCommandHiddenWithTimeout
	readTimeNow       = time.Now
)

// Reader 温度读取器
type Reader struct {
	bridgeManager *bridge.Manager
	logger        types.Logger

	cacheMutex      sync.RWMutex
	cachedGPUVendor string
	cachedVendorAt  time.Time

	// 桥接不可用时的降级读取节流状态，见 fallback.go。
	fallback fallbackState
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
		UpdateTime:    time.Now().UnixMilli(),
		BridgeOk:      true,
		ControlSource: selection.TempSource,
	}

	if r.bridgeManager != nil && r.bridgeManager.IsSupported() {
		bridgeTemp := r.bridgeManager.GetTemperature(selection)
		copyBridgeTemperatureMetadata(&temp, bridgeTemp, selection)
		if selection.DisableGpu {
			stripGpuReadings(&temp, selection.TempSource)
		}
		// 桥接正在后台完成启动握手：这是过渡态，既不该走开销高昂的降级读取，
		// 也不该被上层当成故障触发自愈重启。原样上报让上层识别。
		if !bridgeTemp.Success && bridge.IsStarting(bridgeTemp.Error) {
			temp.BridgeOk = false
			temp.BridgeMsg = bridgeTemp.Error
			return temp
		}
		if bridgeTemp.Success {
			if bridgeTemp.CpuTemp == 0 && (selection.DisableGpu || bridgeTemp.GpuTemp == 0) {
				temp.BridgeOk = false
				temp.BridgeMsg = "桥接程序返回空温度（CPU/GPU 均为 0），已尝试备用读取；可重新初始化温度监控或检查 PawnIO/其它硬件监控工具。"
				r.logger.Warn("桥接程序返回空温度数据，使用备用方法")

				fallback := r.readFallback(selection.DisableGpu)
				temp.CPUTemp = fallback.cpuTemp
				temp.GPUTemp = fallback.gpuTemp
				temp.GPUPower = fallback.gpuPower
				temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
				temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)
				return temp
			}

			// 用户多选 CPU 传感器时，按所选传感器温度的算术平均覆盖 CPU 基准温度。
			applyMultiSensorCpuAverage(&temp, selection.CpuSensors)

			temp.BridgeOk = true
			temp.BridgeMsg = ""
			return temp
		}

		r.logger.Warn("桥接程序读取温度失败: %s, 使用备用方法", bridgeTemp.Error)
		temp.BridgeOk = false
		temp.BridgeMsg = bridgeTemp.Error
		if strings.TrimSpace(temp.BridgeMsg) == "" {
			temp.BridgeMsg = "CPU/GPU 温度读取失败，可重新初始化温度监控；若 CPU 仍为空，请安装/更新 PawnIO 或关闭其它硬件监控工具。"
		}
	}
	// 桥接程序为 Windows 专用（LibreHardwareMonitor/PawnIO）。在不支持的平台（如 Linux）
	// 上，内置传感器读取才是正常路径，不应作为错误提示，因此保持 BridgeOk 默认的 true。

	// 读取内置传感器（停用 GPU 监测时完全跳过 GPU，避免 nvidia-smi 等轮询唤醒独显）。
	// readFallback 内部带刷新节流与失败退避，桥接长期不可用时不会每个采样周期都起进程。
	fallback := r.readFallback(selection.DisableGpu)
	temp.CPUTemp = fallback.cpuTemp
	temp.GPUTemp = fallback.gpuTemp
	temp.GPUPower = fallback.gpuPower

	// 计算最高温度
	temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
	temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, selection.TempSource)

	return temp
}

// stripGpuReadings 清空 GPU 相关读数与元数据。停用 GPU 监测时即便桥接程序
// （如旧版本）仍返回 GPU 数据，也保证对外一致为“无 GPU 读数”，控温基准回退 CPU。
func stripGpuReadings(temp *types.TemperatureData, tempSource string) {
	temp.GPUTemp = 0
	temp.GPUPower = 0
	temp.GpuModel = ""
	temp.GpuSensors = nil
	temp.GpuPowerSensors = nil
	temp.GpuDevices = nil
	temp.SelectedGpuDevice = ""
	temp.MaxTemp = temp.CPUTemp
	temp.ControlTemp = resolveControlTemp(temp.CPUTemp, 0, tempSource)
}

func copyBridgeTemperatureMetadata(temp *types.TemperatureData, bridgeTemp types.BridgeTemperatureData, selection types.TemperatureSelection) {
	if temp == nil {
		return
	}

	temp.CPUTemp = bridgeTemp.CpuTemp
	temp.GPUTemp = bridgeTemp.GpuTemp
	temp.CPUPower = bridgeTemp.CpuPower
	temp.GPUPower = bridgeTemp.GpuPower
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
	temp.CpuPowerSensors = bridgeTemp.CpuPowerSensors
	temp.GpuPowerSensors = bridgeTemp.GpuPowerSensors
	temp.GpuDevices = bridgeTemp.GpuDevices
	if bridgeTemp.UpdateTime > 0 {
		temp.UpdateTime = bridgeTemp.UpdateTime
		if temp.UpdateTime < 1_000_000_000_000 {
			temp.UpdateTime *= 1000
		}
	}
}

// applyMultiSensorCpuAverage 当用户多选 CPU 传感器时，用所选传感器温度的算术平均
// 覆盖 CPU 基准温度，并同步刷新 MaxTemp 与 ControlTemp。未命中任何传感器时不改动。
func applyMultiSensorCpuAverage(temp *types.TemperatureData, keys []string) {
	if temp == nil {
		return
	}
	avg, ok := averageSelectedCpuTemp(temp.CpuSensors, keys)
	if !ok {
		return
	}
	temp.CPUTemp = avg
	temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
	temp.ControlTemp = resolveControlTemp(temp.CPUTemp, temp.GPUTemp, temp.ControlSource)
}

// averageSelectedCpuTemp 从已采集的传感器列表中按 key 匹配并计算算术平均(四舍五入)。
// keys 为空或未命中任何传感器时返回 ok=false。
func averageSelectedCpuTemp(sensors []types.TemperatureSensor, keys []string) (int, bool) {
	if len(keys) == 0 || len(sensors) == 0 {
		return 0, false
	}
	sum, n := 0, 0
	for _, key := range keys {
		for i := range sensors {
			if strings.EqualFold(sensors[i].Key, key) {
				sum += sensors[i].Value
				n++
				break
			}
		}
	}
	if n == 0 {
		return 0, false
	}
	return (sum + n/2) / n, true
}

func resolveControlTemp(cpuTemp, gpuTemp int, source string) int {
	switch types.NormalizeTempSource(source) {
	case types.TempSourceCPU:
		if cpuTemp <= 0 && gpuTemp > 0 {
			return gpuTemp
		}
		return cpuTemp
	case types.TempSourceGPU:
		if gpuTemp <= 0 && cpuTemp > 0 {
			return cpuTemp
		}
		return gpuTemp
	default:
		return max(cpuTemp, gpuTemp)
	}
}

// readSensorCPUTemp 通过 gopsutil 读取 CPU 温度。纯进程内操作，开销可忽略，
// 因此不参与降级路径的节流缓存。读不到返回 0。
func (r *Reader) readSensorCPUTemp() int {
	sensorTemps, err := sensors.SensorsTemperatures()
	if err != nil {
		return 0
	}
	for _, sensor := range sensorTemps {
		// 查找ACPI ThermalZone TZ00_0或类似的CPU温度传感器
		if strings.Contains(strings.ToLower(sensor.SensorKey), "tz00") ||
			strings.Contains(strings.ToLower(sensor.SensorKey), "cpu") ||
			strings.Contains(strings.ToLower(sensor.SensorKey), "core") {
			return int(sensor.Temperature)
		}
	}
	return 0
}

// readCPUTemperature 读取CPU温度：先试廉价的进程内传感器，再退到平台实现。
func (r *Reader) readCPUTemperature() int {
	if temp := r.readSensorCPUTemp(); temp > 0 {
		return temp
	}
	return r.readPlatformCPUTemp()
}

// readWindowsCPUTemp 通过WMI读取Windows CPU温度
func (r *Reader) readWindowsCPUTemp() int {
	output, err := execHelperCommand(helperCommandTimeout, "wmic", "/namespace:\\\\root\\wmi", "PATH", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Debug("读取Windows CPU温度超时: %v", err)
		} else {
			r.logger.Debug("读取Windows CPU温度失败: %v", err)
		}
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
	now := readTimeNow()
	r.cacheMutex.RLock()
	if cached := r.cachedGPUVendor; cached != "" && now.Sub(r.cachedVendorAt) < gpuVendorCacheTTL {
		r.cacheMutex.RUnlock()
		return cached
	}
	r.cacheMutex.RUnlock()

	vendor := "unknown"
	// 尝试NVIDIA
	if _, err := execHelperCommand(helperCommandTimeout, "nvidia-smi", "--version"); err == nil {
		vendor = "nvidia"
	} else if !errors.Is(err, context.DeadlineExceeded) {
		r.logger.Debug("检测GPU厂商失败: %v", err)
	} else {
		r.logger.Debug("检测GPU厂商超时: %v", err)
	}

	r.cacheMutex.Lock()
	r.cachedGPUVendor = vendor
	r.cachedVendorAt = now
	r.cacheMutex.Unlock()

	return vendor
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

package temperature

import (
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

// 桥接不可用时的降级读取路径依赖外部命令（wmic / nvidia-smi），每次调用都要创建
// 进程，单次最坏耗时接近 helperCommandTimeout。按温度采样频率（默认 2s）无节制地
// 调用，会在桥接故障期间造成每秒约 1.5 次进程创建的持续后台负载，同时把单个监控
// 周期拖到数秒。这里给降级路径加独立的刷新节流与失败退避：
//
//   - 读到有效数据时按 externalFreshInterval 复用缓存，控温精度损失有限；
//   - 连续读不到数据时逐级退避到 externalMaxInterval，避免为了拿回 0 反复起进程；
//   - 桥接处于启动过渡态时完全跳过，交由上层识别为过渡态。
const (
	externalFreshInterval = 5 * time.Second
	externalBackoffStart  = 15 * time.Second
	externalMaxInterval   = 60 * time.Second
)

// telemetryReading 是一次平台遥测读取结果。
type telemetryReading struct {
	cpuTemp  int
	gpuTemp  int
	gpuPower float64

	// Linux 原生 provider 额外返回的上报元数据。
	cpuModel          string
	cpuPower          float64
	cpuSensors        []types.TemperatureSensor
	gpuModel          string
	gpuSensors        []types.TemperatureSensor
	gpuPowerSensors   []types.PowerSensor
	gpuDevices        []types.TemperatureGPUDevice
	selectedGpuDevice string
}

func (f telemetryReading) usable() bool {
	return f.cpuTemp > 0 || f.gpuTemp > 0 || f.gpuPower > 0
}

type platformTelemetryReader func(*Reader, bool) telemetryReading

func (f *telemetryReading) mergeCPU(other telemetryReading) {
	if other.cpuTemp > 0 {
		f.cpuTemp = other.cpuTemp
	}
	if other.cpuModel != "" {
		f.cpuModel = other.cpuModel
	}
	if other.cpuPower > 0 {
		f.cpuPower = other.cpuPower
	}
	if len(other.cpuSensors) > 0 {
		f.cpuSensors = other.cpuSensors
	}
}

func (f *telemetryReading) mergeGPU(other telemetryReading) {
	f.gpuTemp = other.gpuTemp
	f.gpuPower = other.gpuPower
	f.gpuModel = other.gpuModel
	f.gpuSensors = other.gpuSensors
	f.gpuPowerSensors = other.gpuPowerSensors
	f.gpuDevices = other.gpuDevices
	f.selectedGpuDevice = other.selectedGpuDevice
}

type externalThrottleState struct {
	mutex    sync.Mutex
	reading  telemetryReading
	at       time.Time
	interval time.Duration
	// 上一次读取时是否停用了 GPU：选项切换后必须强制重新读取，
	// 否则会把"停用 GPU"期间缓存的空 GPU 读数一直沿用下去。
	disabledGpu bool
	primed      bool
}

// next 计算下一次允许真实读取的间隔。有效读数用较短的刷新间隔，
// 无效读数逐级翻倍退避。
func (s *externalThrottleState) next(usable bool) time.Duration {
	if usable {
		return externalFreshInterval
	}
	if s.interval < externalBackoffStart {
		return externalBackoffStart
	}
	if doubled := s.interval * 2; doubled < externalMaxInterval {
		return doubled
	}
	return externalMaxInterval
}

// readNativeTelemetry 从当前平台实现读取 CPU/GPU 遥测。
func (r *Reader) readNativeTelemetry(disableGpu bool) telemetryReading {
	return r.readNativeTelemetryWith(disableGpu, readPlatformTelemetry, platformTelemetryIsExpensive)
}

// readNativeTelemetryWith 显式接收平台实现，便于测试注入，不修改包级状态。
func (r *Reader) readNativeTelemetryWith(
	disableGpu bool,
	readTelemetry platformTelemetryReader,
	expensive bool,
) telemetryReading {
	if !expensive {
		telemetry := readTelemetry(r, disableGpu)
		reading := telemetryReading{}
		reading.mergeCPU(telemetry)
		if !disableGpu {
			reading.mergeGPU(telemetry)
		}
		if reading.cpuTemp <= 0 {
			reading.cpuTemp = r.readCheapCPUTemp()
		}
		return reading
	}

	reading := telemetryReading{cpuTemp: r.readCheapCPUTemp()}
	needExternalCPU := reading.cpuTemp <= 0 && platformCPUTempIsExpensive
	needExternalGPU := !disableGpu && expensive
	if !needExternalCPU && !needExternalGPU {
		return reading
	}
	external := r.readExternalTelemetryThrottled(needExternalCPU, needExternalGPU, disableGpu, readTelemetry)
	if reading.cpuTemp <= 0 {
		reading.cpuTemp = external.cpuTemp
	}
	if needExternalGPU {
		reading.mergeGPU(external)
	}
	return reading
}

// readCheapCPUTemp 只做进程内的廉价 CPU 温度读取。
//
// 平台没有可信的 CPU 温度来源时（Windows，见 cpu_windows.go）直接返回 0：
// gopsutil 在 Windows 上读的同样是 ACPI 温区，不是 CPU 核心温度，
// 用它填补空缺只会掩盖 PawnIO 故障并给出错误的控温依据。
func (r *Reader) readCheapCPUTemp() int {
	if !platformHasCPUTempFallback {
		return 0
	}
	if temp := r.readSensorCPUTemp(); temp > 0 {
		return temp
	}
	if !platformCPUTempIsExpensive {
		return r.readPlatformCPUTemp()
	}
	return 0
}

// readExternalTelemetryThrottled 返回需要创建外部进程的读数，带刷新节流与失败退避。
func (r *Reader) readExternalTelemetryThrottled(needCPU, needGPU, disableGpu bool, readTelemetry platformTelemetryReader) telemetryReading {
	now := readTimeNow()

	r.externalThrottle.mutex.Lock()
	defer r.externalThrottle.mutex.Unlock()

	// 切换"停用 GPU"后必须强制重新采样，否则会一直沿用停用期间缓存的空 GPU 读数。
	selectionChanged := r.externalThrottle.primed && r.externalThrottle.disabledGpu != disableGpu
	if r.externalThrottle.primed && !selectionChanged {
		if now.Sub(r.externalThrottle.at) < r.externalThrottle.interval {
			return r.externalThrottle.reading
		}
	}

	reading := r.sampleExternalTelemetry(needCPU, needGPU, readTelemetry)

	r.externalThrottle.reading = reading
	r.externalThrottle.at = now
	r.externalThrottle.interval = r.externalThrottle.next(reading.usable())
	r.externalThrottle.disabledGpu = disableGpu
	r.externalThrottle.primed = true

	if !reading.usable() {
		r.logger.Debug("降级温度读取未获得有效数据，下次重试间隔 %s", r.externalThrottle.interval)
	}

	return reading
}

// sampleExternalTelemetry 真正执行平台读取。CPU 与 GPU 两条链路并发执行，
// 把最坏阻塞时间从"逐条相加"压回单条命令的超时上限。
func (r *Reader) sampleExternalTelemetry(needCPU, needGPU bool, readTelemetry platformTelemetryReader) telemetryReading {
	var reading telemetryReading

	var wg sync.WaitGroup
	if needCPU {
		wg.Go(func() {
			reading.cpuTemp = r.readPlatformCPUTemp()
		})
	}
	if needGPU {
		wg.Go(func() {
			reading.mergeGPU(readTelemetry(r, false))
		})
	}
	wg.Wait()

	return reading
}

package temperature

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// 桥接不可用时的降级读取路径依赖外部命令（wmic / nvidia-smi），每次调用都要创建
// 进程，单次最坏耗时接近 helperCommandTimeout。按温度采样频率（默认 2s）无节制地
// 调用，会在桥接故障期间造成每秒约 1.5 次进程创建的持续后台负载，同时把单个监控
// 周期拖到数秒。这里给降级路径加独立的刷新节流与失败退避：
//
//   - 读到有效数据时按 fallbackFreshInterval 复用缓存，控温精度损失有限；
//   - 连续读不到数据时逐级退避到 fallbackMaxInterval，避免为了拿回 0 反复起进程；
//   - 桥接处于启动过渡态时完全跳过，交由上层识别为过渡态。
const (
	fallbackFreshInterval = 5 * time.Second
	fallbackBackoffStart  = 15 * time.Second
	fallbackMaxInterval   = 60 * time.Second
)

// fallbackReading 降级路径的一次读取结果。
type fallbackReading struct {
	cpuTemp  int
	gpuTemp  int
	gpuPower float64
}

func (f fallbackReading) usable() bool {
	return f.cpuTemp > 0 || f.gpuTemp > 0
}

type fallbackState struct {
	mutex    sync.Mutex
	reading  fallbackReading
	at       time.Time
	interval time.Duration
	// 上一次读取时是否停用了 GPU：选项切换后必须强制重新读取，
	// 否则会把"停用 GPU"期间缓存的空 GPU 读数一直沿用下去。
	disabledGpu bool
	primed      bool
}

// next 计算下一次允许真实读取的间隔。有效读数用较短的刷新间隔，
// 无效读数逐级翻倍退避。
func (s *fallbackState) next(usable bool) time.Duration {
	if usable {
		return fallbackFreshInterval
	}
	if s.interval < fallbackBackoffStart {
		return fallbackBackoffStart
	}
	if doubled := s.interval * 2; doubled < fallbackMaxInterval {
		return doubled
	}
	return fallbackMaxInterval
}

// readFallback 返回降级路径的温度读数。
//
// 只有需要创建外部进程的那部分才走节流缓存；进程内的廉价读取（gopsutil 传感器、
// Linux sysfs）每次都取新值——在没有硬件桥接的平台上它就是主读取路径，
// 缓存会让控温依据变旧。
func (r *Reader) readFallback(disableGpu bool) fallbackReading {
	reading := fallbackReading{cpuTemp: r.readCheapCPUTemp()}

	needExternalCPU := reading.cpuTemp <= 0 && platformCPUTempIsExpensive
	needGPU := !disableGpu
	if !needExternalCPU && !needGPU {
		return reading
	}

	external := r.readThrottledExternal(needExternalCPU, needGPU, disableGpu)
	if reading.cpuTemp <= 0 {
		reading.cpuTemp = external.cpuTemp
	}
	reading.gpuTemp = external.gpuTemp
	reading.gpuPower = external.gpuPower
	return reading
}

// readCheapCPUTemp 只做进程内的廉价 CPU 温度读取。
func (r *Reader) readCheapCPUTemp() int {
	if temp := r.readSensorCPUTemp(); temp > 0 {
		return temp
	}
	if !platformCPUTempIsExpensive {
		return r.readPlatformCPUTemp()
	}
	return 0
}

// readThrottledExternal 返回需要创建外部进程的读数，带刷新节流与失败退避。
func (r *Reader) readThrottledExternal(needCPU, needGPU, disableGpu bool) fallbackReading {
	now := readTimeNow()

	r.fallback.mutex.Lock()
	defer r.fallback.mutex.Unlock()

	// 切换"停用 GPU"后必须强制重新采样，否则会一直沿用停用期间缓存的空 GPU 读数。
	selectionChanged := r.fallback.primed && r.fallback.disabledGpu != disableGpu
	if r.fallback.primed && !selectionChanged {
		if now.Sub(r.fallback.at) < r.fallback.interval {
			return r.fallback.reading
		}
	}

	reading := r.sampleExternal(needCPU, needGPU)

	r.fallback.reading = reading
	r.fallback.at = now
	r.fallback.interval = r.fallback.next(reading.usable())
	r.fallback.disabledGpu = disableGpu
	r.fallback.primed = true

	if !reading.usable() {
		r.logger.Debug("降级温度读取未获得有效数据，下次重试间隔 %s", r.fallback.interval)
	}

	return reading
}

// sampleExternal 真正执行外部命令。CPU 与 GPU 两条链路并发执行，
// 把最坏阻塞时间从"逐条相加"压回单条命令的超时上限。
func (r *Reader) sampleExternal(needCPU, needGPU bool) fallbackReading {
	var reading fallbackReading

	var wg sync.WaitGroup
	if needCPU {
		wg.Go(func() {
			reading.cpuTemp = r.readPlatformCPUTemp()
		})
	}
	if needGPU {
		wg.Go(func() {
			// GPU 温度与功耗合并成一次 nvidia-smi 调用：两者本来各起一个进程，
			// 而 --query-gpu 支持一次查询多个字段。
			reading.gpuTemp, reading.gpuPower = r.readGPUTempAndPower()
		})
	}
	wg.Wait()

	return reading
}

// readGPUTempAndPower 一次调用同时取回 GPU 温度与功耗。
func (r *Reader) readGPUTempAndPower() (int, float64) {
	if r.detectGPUVendor() != "nvidia" {
		return 0, 0
	}

	output, err := execHelperCommand(helperCommandTimeout,
		"nvidia-smi", "--query-gpu=temperature.gpu,power.draw", "--format=csv,noheader,nounits")
	if err != nil {
		r.logger.Debug("读取 NVIDIA GPU 温度/功耗失败: %v", err)
		return 0, 0
	}

	return parseNvidiaTempAndPower(string(output))
}

// parseNvidiaTempAndPower 解析 "62, 35.12" 形式的输出。
func parseNvidiaTempAndPower(output string) (int, float64) {
	line := strings.TrimSpace(output)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if line == "" {
		return 0, 0
	}

	fields := strings.Split(line, ",")
	temp := 0
	if value, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil && value > 0 && value < 150 {
		temp = value
	}

	power := 0.0
	if len(fields) > 1 {
		if watts, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64); err == nil && watts > 0 && watts <= 2000 {
			power = watts
		}
	}

	return temp, power
}

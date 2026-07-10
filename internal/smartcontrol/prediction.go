package smartcontrol

import (
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	predictionHistoryLength = 6
	predictionMinSamples    = 3
	predictionHorizon       = 6 * time.Second
	maxPredictedRise        = 6.0
	maxPowerLead            = 3.0
	maxTemperatureSlope     = 2.0
)

type thermalSample struct {
	at       time.Time
	cpuTemp  int
	gpuTemp  int
	cpuPower float64
	gpuPower float64
}

// ThermalPrediction contains the bounded short-term temperature estimate used
// for feed-forward fan control. It is deliberately not persisted: learned curve
// offsets must only be derived from measured, stable thermal equilibrium.
type ThermalPrediction struct {
	CPUTemp     int
	GPUTemp     int
	ControlTemp int
	CPURise     float64
	GPURise     float64
}

// ThermalPredictor estimates a near-future thermal rise from recent CPU/GPU
// temperatures and a sudden increase in package/board power. It is local to a
// monitoring session so suspend/resume or a bridge restart cannot reuse stale
// samples.
type ThermalPredictor struct {
	samples []thermalSample
}

func NewThermalPredictor() *ThermalPredictor {
	return &ThermalPredictor{samples: make([]thermalSample, 0, predictionHistoryLength)}
}

func (p *ThermalPredictor) Reset() {
	if p == nil {
		return
	}
	p.samples = p.samples[:0]
}

// Observe stores the current sample and returns an estimate up to six seconds
// ahead. TrendGain controls the strength of the feed-forward path (1..12).
// Cooling is never predicted below the current reading; normal ramp-down and
// hysteresis continue to make that decision from measured temperature.
func (p *ThermalPredictor) Observe(temp types.TemperatureData, at time.Time, source string, trendGain int) ThermalPrediction {
	if p == nil {
		return predictionFromMeasured(temp, source)
	}
	if at.IsZero() {
		at = time.Now()
	}

	sample := thermalSample{
		at:       at,
		cpuTemp:  temp.CPUTemp,
		gpuTemp:  temp.GPUTemp,
		cpuPower: temp.CPUPower,
		gpuPower: temp.GPUPower,
	}
	p.samples = append(p.samples, sample)
	if len(p.samples) > predictionHistoryLength {
		p.samples = p.samples[len(p.samples)-predictionHistoryLength:]
	}

	gain := normalizedTrendGain(trendGain)
	cpuRise := predictedRise(p.samples, func(s thermalSample) int { return s.cpuTemp }, func(s thermalSample) float64 { return s.cpuPower }, gain)
	gpuRise := predictedRise(p.samples, func(s thermalSample) int { return s.gpuTemp }, func(s thermalSample) float64 { return s.gpuPower }, gain)

	prediction := ThermalPrediction{
		CPUTemp: temp.CPUTemp,
		GPUTemp: temp.GPUTemp,
		CPURise: cpuRise,
		GPURise: gpuRise,
	}
	if prediction.CPUTemp > 0 {
		prediction.CPUTemp += roundFloat(cpuRise)
	}
	if prediction.GPUTemp > 0 {
		prediction.GPUTemp += roundFloat(gpuRise)
	}
	prediction.ControlTemp = resolvePredictedControlTemp(prediction.CPUTemp, prediction.GPUTemp, source)
	return prediction
}

func predictionFromMeasured(temp types.TemperatureData, source string) ThermalPrediction {
	return ThermalPrediction{
		CPUTemp:     temp.CPUTemp,
		GPUTemp:     temp.GPUTemp,
		ControlTemp: resolvePredictedControlTemp(temp.CPUTemp, temp.GPUTemp, source),
	}
}

func normalizedTrendGain(value int) float64 {
	if value < 1 {
		value = 1
	}
	if value > 12 {
		value = 12
	}
	// Keep the default (6) close to a direct six-second slope projection while
	// allowing the existing UI control to tune sensitivity conservatively.
	return 0.45 + float64(value)*0.09
}

func predictedRise(samples []thermalSample, temperature func(thermalSample) int, power func(thermalSample) float64, gain float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	currentTemp := temperature(samples[len(samples)-1])
	if currentTemp <= 0 {
		return 0
	}

	rise := positiveTemperatureSlope(samples, temperature) * predictionHorizon.Seconds() * gain
	rise += powerStepLead(samples, power, gain)
	if rise < 0 {
		return 0
	}
	if rise > maxPredictedRise {
		return maxPredictedRise
	}
	return rise
}

func positiveTemperatureSlope(samples []thermalSample, temperature func(thermalSample) int) float64 {
	if len(samples) < predictionMinSamples {
		return 0
	}

	start := len(samples) - predictionHistoryLength
	if start < 0 {
		start = 0
	}
	base := samples[start].at
	var sumX, sumY, sumXX, sumXY float64
	count := 0
	for _, sample := range samples[start:] {
		value := temperature(sample)
		if value <= 0 || sample.at.Before(base) {
			continue
		}
		x := sample.at.Sub(base).Seconds()
		if x < 0 {
			continue
		}
		y := float64(value)
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
		count++
	}
	if count < predictionMinSamples {
		return 0
	}
	denominator := float64(count)*sumXX - sumX*sumX
	if denominator <= 0 {
		return 0
	}
	slope := (float64(count)*sumXY - sumX*sumY) / denominator
	if slope <= 0 {
		return 0
	}
	if slope > maxTemperatureSlope {
		return maxTemperatureSlope
	}
	return slope
}

func powerStepLead(samples []thermalSample, power func(thermalSample) float64, gain float64) float64 {
	if len(samples) < predictionMinSamples {
		return 0
	}
	current := power(samples[len(samples)-1])
	if current <= 0 {
		return 0
	}

	var total float64
	count := 0
	for _, sample := range samples[:len(samples)-1] {
		value := power(sample)
		if value <= 0 {
			continue
		}
		total += value
		count++
	}
	if count < 2 {
		return 0
	}

	surge := current - total/float64(count)
	if surge <= 5 {
		return 0
	}
	// The conversion is intentionally bounded and modest: power is a leading
	// indicator, not a substitute for temperature. At default gain a 100 W step
	// contributes about 1.8°C of advance control, capped at 3°C.
	lead := surge * 0.018 * gain
	if lead > maxPowerLead {
		return maxPowerLead
	}
	return lead
}

func resolvePredictedControlTemp(cpuTemp, gpuTemp int, source string) int {
	switch types.NormalizeTempSource(source) {
	case types.TempSourceCPU:
		return cpuTemp
	case types.TempSourceGPU:
		return gpuTemp
	default:
		return max(cpuTemp, gpuTemp)
	}
}

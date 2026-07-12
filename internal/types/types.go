// Package types 定义了 BS2PRO 控制器应用中使用的所有共享类型
package types

import (
	"maps"
	"strings"

	"github.com/TIANLI0/THRM/internal/deviceproto"
)

// FanCurvePoint 风扇曲线点
type FanCurvePoint struct {
	Temperature int `json:"temperature"` // 温度 °C
	RPM         int `json:"rpm"`         // 转速 RPM
}

const (
	FanCurveMaxTemperature = 110
	ThemeModeSystem        = "system"
	ThemeModeLight         = "light"
	ThemeModeDark          = "dark"
	ThemeModeTHRM          = "thrm"
	TempSourceMax          = "max"
	TempSourceCPU          = "cpu"
	TempSourceGPU          = "gpu"
	TempDeviceAuto         = "auto"
	TempSensorAuto         = "auto"
	LearningBiasBalanced   = "balanced"
	LearningBiasCooling    = "cooling"
	LearningBiasQuiet      = "quiet"
	// WindowBlurAuto 根据系统版本自动决定窗口模糊效果(Win11 开启, Win10 关闭)。
	WindowBlurAuto = "auto"
	// WindowBlurOn 强制开启窗口模糊效果。
	WindowBlurOn = "on"
	// WindowBlurAcrylic 使用亚克力窗口材质。
	WindowBlurAcrylic = "acrylic"
	// WindowBlurMica 使用云母窗口材质。
	WindowBlurMica = "mica"
	// WindowBlurTabbed 使用云母 Alt（Tabbed）窗口材质。
	WindowBlurTabbed = "tabbed"
	// WindowBlurOff 强制关闭窗口模糊效果。
	WindowBlurOff = "off"
)

// NormalizeWindowBlur 归一化窗口模糊效果设置，非法值回退为 auto。
func NormalizeWindowBlur(mode string) string {
	switch mode {
	case WindowBlurOn, WindowBlurAcrylic, WindowBlurMica, WindowBlurTabbed:
		return mode
	case WindowBlurOff:
		return WindowBlurOff
	default:
		return WindowBlurAuto
	}
}

// NormalizeThemeMode 归一化主题模式。
//
// 取值说明：
//   - system/light/dark：内置基础主题。
//   - 其它合法 id（小写字母/数字/-/_）：视为自定义主题 id（如 "thrm"），原样透传，
//     由前端按安装目录/用户目录下发现的主题加载对应 CSS。
//   - 空值或非法字符：回退为 system。
func NormalizeThemeMode(mode string) string {
	switch mode {
	case ThemeModeLight:
		return ThemeModeLight
	case ThemeModeDark:
		return ThemeModeDark
	case ThemeModeSystem:
		return ThemeModeSystem
	}
	if isValidThemeID(mode) {
		return mode
	}
	return ThemeModeSystem
}

// isValidThemeID 校验自定义主题 id：仅允许小写字母、数字、连字符、下划线。
// 与 internal/theme 包的校验保持一致，避免非法值被写入配置或用作 CSS 选择器。
func isValidThemeID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// NormalizeTempSource 归一化控温温度来源，非法值回退为 max。
func NormalizeTempSource(source string) string {
	switch source {
	case TempSourceCPU:
		return TempSourceCPU
	case TempSourceGPU:
		return TempSourceGPU
	default:
		return TempSourceMax
	}
}

// NormalizeSensorSelection 归一化传感器选择，空值回退为 auto。
func NormalizeSensorSelection(selection string) string {
	if selection == "" {
		return TempSensorAuto
	}
	return selection
}

// NormalizeSensorSelections 归一化多选传感器列表：去除空白与重复(忽略大小写)。
// 列表为空、或包含 "auto" 时返回 nil，表示自动选择(不做多传感器平均)。
func NormalizeSensorSelections(selections []string) []string {
	if len(selections) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(selections))
	out := make([]string, 0, len(selections))
	for _, s := range selections {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.EqualFold(s, TempSensorAuto) {
			return nil
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NormalizeDeviceSelection 归一化设备选择，空值回退为 auto。
func NormalizeDeviceSelection(selection string) string {
	if selection == "" {
		return TempDeviceAuto
	}
	return selection
}

// NormalizeLearningBias 归一化学习倾向，非法值回退为 balanced。
func NormalizeLearningBias(bias string) string {
	switch bias {
	case LearningBiasCooling:
		return LearningBiasCooling
	case LearningBiasQuiet:
		return LearningBiasQuiet
	default:
		return LearningBiasBalanced
	}
}

// TemperatureSelection 温度读取选择配置。
type TemperatureSelection struct {
	TempSource string `json:"tempSource"`
	GpuDevice  string `json:"gpuDevice"`
	CpuSensor  string `json:"cpuSensor"`
	// CpuSensors 多选 CPU 传感器(用于多核平均)。非空时优先于 CpuSensor，
	// 取所选传感器温度的算术平均作为 CPU 控温基准。
	CpuSensors []string `json:"cpuSensors"`
	GpuSensor  string   `json:"gpuSensor"`
}

// NormalizeTemperatureSelection 归一化温度选择配置。
func NormalizeTemperatureSelection(selection TemperatureSelection) TemperatureSelection {
	selection.TempSource = NormalizeTempSource(selection.TempSource)
	selection.GpuDevice = NormalizeDeviceSelection(selection.GpuDevice)
	selection.CpuSensor = NormalizeSensorSelection(selection.CpuSensor)
	selection.CpuSensors = NormalizeSensorSelections(selection.CpuSensors)
	selection.GpuSensor = NormalizeSensorSelection(selection.GpuSensor)
	return selection
}

// GetDefaultTemperatureSelection 获取默认温度选择配置。
func GetDefaultTemperatureSelection() TemperatureSelection {
	return TemperatureSelection{
		TempSource: TempSourceMax,
		GpuDevice:  TempDeviceAuto,
		CpuSensor:  TempSensorAuto,
		GpuSensor:  TempSensorAuto,
	}
}

// TemperatureSensor 可选温度传感器信息。
type TemperatureSensor struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// TemperatureGPUDevice 可选 GPU 设备信息。
type TemperatureGPUDevice struct {
	Key          string              `json:"key"`
	Name         string              `json:"name"`
	Vendor       string              `json:"vendor"`
	Sensors      []TemperatureSensor `json:"sensors"`
	PowerSensors []PowerSensor       `json:"powerSensors"`
}

// PowerSensor is a hardware-monitoring power sensor in watts. A zero value
// means the source has no current reading; it does not represent zero draw.
type PowerSensor struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// FanCurveProfile 温控曲线方案
type FanCurveProfile struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Curve []FanCurvePoint `json:"curve"`
}

// FanCurveProfilesPayload 风扇曲线方案返回载荷
type FanCurveProfilesPayload struct {
	Profiles []FanCurveProfile `json:"profiles"`
	ActiveID string            `json:"activeId"`
}

// FanData 风扇数据结构
type FanData struct {
	ReportID     uint8  `json:"reportId"`
	MagicSync    uint16 `json:"magicSync"`
	Command      uint8  `json:"command"`
	Status       uint8  `json:"status"`
	GearSettings uint8  `json:"gearSettings"`
	CurrentMode  uint8  `json:"currentMode"`
	Reserved1    uint8  `json:"reserved1"`
	CurrentRPM   uint16 `json:"currentRpm"`
	TargetRPM    uint16 `json:"targetRpm"`
	MaxGear      string `json:"maxGear"`
	SetGear      string `json:"setGear"`
	WorkMode     string `json:"workMode"`
}

// DeviceDebugFrame is a captured low-level device protocol frame.
type DeviceDebugFrame struct {
	ID          uint64 `json:"id"`
	Direction   string `json:"direction"`
	Transport   string `json:"transport"`
	Timestamp   string `json:"timestamp"`
	RawHex      string `json:"rawHex"`
	FrameHex    string `json:"frameHex"`
	Command     string `json:"command"`
	Length      int    `json:"length"`
	PayloadHex  string `json:"payloadHex"`
	ChecksumOK  bool   `json:"checksumOk"`
	Description string `json:"description"`
	Decoded     string `json:"decoded,omitempty"`
	Parsed      any    `json:"parsed,omitempty"`
}

// DeviceSettings contains settings read back from the device firmware.
type DeviceSettings struct {
	Available    bool               `json:"available"`
	Source       string             `json:"source"`
	ReadAt       string             `json:"readAt"`
	Model        string             `json:"model,omitempty"`
	GearRPMTable []DeviceGearRPM    `json:"gearRpmTable,omitempty"`
	WorkMode     string             `json:"workMode,omitempty"`
	WorkModeName string             `json:"workModeName,omitempty"`
	RGBState     string             `json:"rgbState,omitempty"`
	RGBStateName string             `json:"rgbStateName,omitempty"`
	Status       *DeviceStatusRead  `json:"status,omitempty"`
	RawFrames    []DeviceDebugFrame `json:"rawFrames,omitempty"`
}

type DeviceGearRPM struct {
	Gear  int    `json:"gear"`
	Label string `json:"label"`
	RPM   int    `json:"rpm"`
}

type DeviceStatusRead struct {
	GearSetting        string `json:"gearSetting,omitempty"`
	MaxGear            string `json:"maxGear,omitempty"`
	Selected           string `json:"selected,omitempty"`
	Mode               string `json:"mode,omitempty"`
	ModeName           string `json:"modeName,omitempty"`
	SmartStartStop     string `json:"smartStartStop,omitempty"`
	SmartStartStopName string `json:"smartStartStopName,omitempty"`
	CurrentRPM         int    `json:"currentRpm,omitempty"`
	TargetRPM          int    `json:"targetRpm,omitempty"`
}

// DeviceDebugCommandPreset describes a safe command that can be sent from the debug panel.
type DeviceDebugCommandPreset struct {
	Name        string `json:"name"`
	CommandHex  string `json:"commandHex"`
	Description string `json:"description"`
}

// DeviceDebugCommandResult is returned after sending a debug command.
type DeviceDebugCommandResult struct {
	Transport string             `json:"transport"`
	InputHex  string             `json:"inputHex"`
	FrameHex  string             `json:"frameHex"`
	RawHex    string             `json:"rawHex"`
	WaitMs    int                `json:"waitMs"`
	Frames    []DeviceDebugFrame `json:"frames"`
}

// GearCommand 挡位命令结构
type GearCommand struct {
	Name    string `json:"name"`    // 挡位名称
	Command []byte `json:"command"` // 命令字节
	RPM     int    `json:"rpm"`     // 对应转速
}

// TemperatureData 温度数据
type TemperatureData struct {
	CPUTemp           int                    `json:"cpuTemp"`           // CPU温度
	GPUTemp           int                    `json:"gpuTemp"`           // GPU温度
	CPUPower          float64                `json:"cpuPower"`          // CPU package power (W), 0 when unavailable
	GPUPower          float64                `json:"gpuPower"`          // selected GPU power (W), 0 when unavailable
	MaxTemp           int                    `json:"maxTemp"`           // 最高温度
	ControlTemp       int                    `json:"controlTemp"`       // 当前控温基准温度
	ControlSource     string                 `json:"controlSource"`     // 当前控温基准来源
	SelectedGpuDevice string                 `json:"selectedGpuDevice"` // 当前选中的 GPU 设备 key
	CpuModel          string                 `json:"cpuModel"`          // 当前识别的 CPU 型号
	GpuModel          string                 `json:"gpuModel"`          // 当前识别的 GPU 型号
	CpuSensors        []TemperatureSensor    `json:"cpuSensors"`        // 当前识别到的 CPU 温度传感器
	GpuSensors        []TemperatureSensor    `json:"gpuSensors"`        // 当前识别到的 GPU 温度传感器
	CpuPowerSensors   []PowerSensor          `json:"cpuPowerSensors"`   // 当前识别到的 CPU 功耗传感器
	GpuPowerSensors   []PowerSensor          `json:"gpuPowerSensors"`   // 当前选中 GPU 的功耗传感器
	GpuDevices        []TemperatureGPUDevice `json:"gpuDevices"`        // 当前识别到的 GPU 设备列表
	UpdateTime        int64                  `json:"updateTime"`        // 更新时间戳
	BridgeOk          bool                   `json:"bridgeOk"`          // 桥接程序是否正常
	BridgeMsg         string                 `json:"bridgeMessage"`     // 桥接故障提示
}

// TemperatureHistoryPoint CPU/GPU 温度历史点。
type TemperatureHistoryPoint struct {
	Timestamp int64   `json:"timestamp"`
	CPUTemp   int     `json:"cpuTemp"`
	GPUTemp   int     `json:"gpuTemp"`
	CPUPower  float64 `json:"cpuPower"`
	GPUPower  float64 `json:"gpuPower"`
	FanRPM    int     `json:"fanRpm"`
}

// TemperatureHistoryPayload 温度历史返回载荷。
type TemperatureHistoryPayload struct {
	Enabled               bool                      `json:"enabled"`
	SampleIntervalSeconds int                       `json:"sampleIntervalSeconds"`
	Points                []TemperatureHistoryPoint `json:"points"`
}

// BridgeTemperatureData 桥接程序返回的温度数据
type BridgeTemperatureData struct {
	CpuTemp           int                    `json:"cpuTemp"`
	GpuTemp           int                    `json:"gpuTemp"`
	CpuPower          float64                `json:"cpuPower"`
	GpuPower          float64                `json:"gpuPower"`
	MaxTemp           int                    `json:"maxTemp"`
	ControlTemp       int                    `json:"controlTemp"`
	ControlSource     string                 `json:"controlSource"`
	SelectedGpuDevice string                 `json:"selectedGpuDevice"`
	CpuModel          string                 `json:"cpuModel"`
	GpuModel          string                 `json:"gpuModel"`
	CpuSensors        []TemperatureSensor    `json:"cpuSensors"`
	GpuSensors        []TemperatureSensor    `json:"gpuSensors"`
	CpuPowerSensors   []PowerSensor          `json:"cpuPowerSensors"`
	GpuPowerSensors   []PowerSensor          `json:"gpuPowerSensors"`
	GpuDevices        []TemperatureGPUDevice `json:"gpuDevices"`
	UpdateTime        int64                  `json:"updateTime"`
	Success           bool                   `json:"success"`
	Error             string                 `json:"error"`
}

// BridgeCommand 桥接程序命令
type BridgeCommand struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// BridgeResponse 桥接程序响应
type BridgeResponse struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error"`
	Data    *BridgeTemperatureData `json:"data"`
}

// RGBColor RGB 颜色
type RGBColor struct {
	R byte `json:"r"`
	G byte `json:"g"`
	B byte `json:"b"`
}

// LightStripConfig 灯带配置
type LightStripConfig struct {
	Mode       string     `json:"mode"`       // off/smart_temp/static_single/static_multi/rotation/flowing/breathing
	Speed      string     `json:"speed"`      // fast/medium/slow
	Brightness int        `json:"brightness"` // 0-100
	Colors     []RGBColor `json:"colors"`     // 颜色列表
}

// SmartControlConfig 智能控温配置
type FanGearTarget struct {
	Gear  string `json:"gear"`
	Level string `json:"level"`
}

type LegionFnQConfig struct {
	Enabled     bool                     `json:"enabled"`
	TakeOverFan bool                     `json:"takeOverFan"`
	ModeMapping map[string]FanGearTarget `json:"modeMapping"`
}

type LegionFnQSupportCache struct {
	Checked   bool `json:"checked"`
	Supported bool `json:"supported"`
}

// NoiseProfilePoint 一次噪音测试中某个转速的实测噪音水平。
// DB 为相对量（以测试中最安静点为 0 dB 的 A 计权相对噪音），不是绝对声压级。
type NoiseProfilePoint struct {
	RPM int     `json:"rpm"` // 实测时的目标转速
	DB  float64 `json:"db"`  // 相对噪音水平 (dB)
}

type SmartControlConfig struct {
	Enabled                 bool              `json:"enabled"`                         // 智能耦合控制开关
	Learning                bool              `json:"learning"`                        // 学习开关
	PredictiveBoost         bool              `json:"predictiveBoost"`                 // 功耗预测前馈开关(独立于学习)
	LearningBias            string            `json:"learningBias"`                    // 学习倾向: balanced/cooling/quiet
	FilterTransientSpike    bool              `json:"filterTransientSpike"`            // 是否过滤孤立温度尖峰
	TargetTemp              int               `json:"targetTemp"`                      // 目标温度(°C)
	Aggressiveness          int               `json:"aggressiveness"`                  // 响应激进度(1-10)
	Hysteresis              int               `json:"hysteresis"`                      // 滞回温差(°C)
	MinRPMChange            int               `json:"minRpmChange"`                    // 最小生效转速变化(RPM)
	RampUpLimit             int               `json:"rampUpLimit"`                     // 每次更新最大升速(RPM)
	RampDownLimit           int               `json:"rampDownLimit"`                   // 每次更新最大降速(RPM)
	LearnRate               int               `json:"learnRate"`                       // 学习速度(1-10)
	LearnWindow             int               `json:"learnWindow"`                     // 稳态学习窗口(采样点)
	LearnDelay              int               `json:"learnDelay"`                      // 学习延迟步数(处理热惯性)
	OverheatWeight          int               `json:"overheatWeight"`                  // 过热惩罚权重
	RPMDeltaWeight          int               `json:"rpmDeltaWeight"`                  // 转速变化惩罚权重
	NoiseWeight             int               `json:"noiseWeight"`                     // 高转速噪音惩罚权重
	TrendGain               int               `json:"trendGain"`                       // 温升趋势前馈增益
	MaxLearnOffset          int               `json:"maxLearnOffset"`                  // 学习偏移上限(RPM)
	LearnedOffsets          []int             `json:"learnedOffsets"`                  // 每个曲线点的学习偏移(RPM)
	LearnedOffsetsByProfile map[string][]int  `json:"learnedOffsetsByProfile"`         // 每个曲线方案的学习偏移(RPM)
	TargetTempByProfile     map[string]int    `json:"targetTempByProfile,omitempty"`   // 每个曲线方案的目标温度(°C)
	LearningBiasByProfile   map[string]string `json:"learningBiasByProfile,omitempty"` // 每个曲线方案的学习倾向
	LearnedOffsetsHeat      []int             `json:"learnedOffsetsHeat"`              // 升温工况学习偏移(RPM)
	LearnedOffsetsCool      []int             `json:"learnedOffsetsCool"`              // 降温工况学习偏移(RPM)
	LearnedRateHeat         []int             `json:"learnedRateHeat"`                 // 升温变化率学习偏置(分桶RPM)
	LearnedRateCool         []int             `json:"learnedRateCool"`                 // 降温变化率学习偏置(分桶RPM)

	NoiseProfile          []NoiseProfilePoint `json:"noiseProfile"`          // 实测转速-噪音曲线(麦克风噪音测试结果)
	NoiseProfileUpdatedAt int64               `json:"noiseProfileUpdatedAt"` // 噪音测试完成时间(Unix 秒)
}

// AppConfig 应用配置
type AppConfig struct {
	LegionFnQ                LegionFnQConfig           `json:"legionFnQ"`
	LegionFnQSupport         LegionFnQSupportCache     `json:"legionFnQSupport"`
	AutoControl              bool                      `json:"autoControl"`              // 智能变频开关
	ManualGearToggleHotkey   string                    `json:"manualGearToggleHotkey"`   // 切换手动挡位快捷键
	AutoControlToggleHotkey  string                    `json:"autoControlToggleHotkey"`  // 开关智能变频快捷键
	CurveProfileToggleHotkey string                    `json:"curveProfileToggleHotkey"` // 切换温控曲线方案快捷键
	ManualGearLevels         map[string]string         `json:"manualGearLevels"`         // 每个大挡位记忆的小挡位(低中高)
	ManualGearRPM            map[string]map[string]int `json:"manualGearRpm"`            // 每个大挡位低/中/高的自定义转速
	FanCurve                 []FanCurvePoint           `json:"fanCurve"`                 // 风扇曲线
	FanCurveProfiles         []FanCurveProfile         `json:"fanCurveProfiles"`         // 风扇曲线方案列表
	ActiveFanCurveProfileID  string                    `json:"activeFanCurveProfileId"`  // 当前激活曲线方案ID
	GearLight                bool                      `json:"gearLight"`                // 挡位灯
	PowerOnStart             bool                      `json:"powerOnStart"`             // 通电自启动
	WindowsAutoStart         bool                      `json:"windowsAutoStart"`         // Windows开机自启动
	ThemeMode                string                    `json:"themeMode"`                // 主题模式: system/light/dark/thrm
	SmartStartStop           string                    `json:"smartStartStop"`           // 智能启停
	Brightness               int                       `json:"brightness"`               // 亮度
	TempUpdateRate           int                       `json:"tempUpdateRate"`           // 温度更新频率(秒)
	TempSampleCount          int                       `json:"tempSampleCount"`          // 温度采样次数(用于平均)
	TempSource               string                    `json:"tempSource"`               // 控温温度来源: max/cpu/gpu
	GpuDevice                string                    `json:"gpuDevice"`                // GPU 设备选择: auto 或设备 key
	CpuSensor                string                    `json:"cpuSensor"`                // CPU 传感器选择: auto 或传感器 key
	CpuSensors               []string                  `json:"cpuSensors"`               // CPU 多传感器选择(多核平均): 为空则按 cpuSensor 单选/自动
	GpuSensor                string                    `json:"gpuSensor"`                // GPU 传感器选择: auto 或传感器 key
	WindowBlur               string                    `json:"windowBlur"`               // 窗口材质: auto/acrylic/mica/tabbed/off；兼容旧值 on
	SuspendFanOff            bool                      `json:"suspendFanOff"`            // 系统休眠/睡眠时自动归零转速并关闭挡位灯与 RGB
	ConfigPath               string                    `json:"configPath"`               // 配置文件路径
	ManualGear               string                    `json:"manualGear"`               // 手动挡位设置
	ManualLevel              string                    `json:"manualLevel"`              // 手动挡位级别(低中高)
	DebugMode                bool                      `json:"debugMode"`                // 调试模式
	GuiMonitoring            bool                      `json:"guiMonitoring"`            // GUI监控开关
	CustomSpeedEnabled       bool                      `json:"customSpeedEnabled"`       // 自定义转速开关
	CustomSpeedRPM           int                       `json:"customSpeedRPM"`           // 自定义转速值(无上下限)
	IgnoreDeviceOnReconnect  bool                      `json:"ignoreDeviceOnReconnect"`  // 断连后忽略设备状态(保持APP配置)
	SpeedAvoidance           SpeedAvoidanceConfig      `json:"speedAvoidance"`           // 智能控温转速避让
	TimeCurveSchedule        TimeCurveScheduleConfig   `json:"timeCurveSchedule"`        // 分时曲线计划
	SmartControl             SmartControlConfig        `json:"smartControl"`             // 学习型智能控温配置
	LightStrip               LightStripConfig          `json:"lightStrip"`               // 灯带配置
}

// GetDefaultLightStripConfig 获取默认灯带配置
func GetDefaultLightStripConfig() LightStripConfig {
	return LightStripConfig{
		Mode:       "smart_temp",
		Speed:      "medium",
		Brightness: 100,
		Colors: []RGBColor{
			{R: 255, G: 0, B: 0},
			{R: 0, G: 255, B: 0},
			{R: 0, G: 128, B: 255},
		},
	}
}

// GetDefaultSmartControlConfig 获取默认智能控温配置
func GetDefaultSmartControlConfig(curve []FanCurvePoint) SmartControlConfig {
	offsets := make([]int, len(curve))
	heatOffsets := make([]int, len(curve))
	coolOffsets := make([]int, len(curve))
	heatRate := make([]int, 7)
	coolRate := make([]int, 7)

	return SmartControlConfig{
		Enabled:              true,
		Learning:             true,
		PredictiveBoost:      true,
		LearningBias:         LearningBiasBalanced,
		FilterTransientSpike: true,
		TargetTemp:           68,
		Aggressiveness:       5,
		Hysteresis:           2,
		MinRPMChange:         50,
		RampUpLimit:          220,
		RampDownLimit:        160,
		LearnRate:            3,
		LearnWindow:          8,
		LearnDelay:           3,
		OverheatWeight:       8,
		RPMDeltaWeight:       5,
		NoiseWeight:          4,
		TrendGain:            5,
		MaxLearnOffset:       300,
		LearnedOffsets:       offsets,
		LearnedOffsetsHeat:   heatOffsets,
		LearnedOffsetsCool:   coolOffsets,
		LearnedRateHeat:      heatRate,
		LearnedRateCool:      coolRate,
	}
}

// Logger 日志记录器接口
func GetDefaultLegionFnQConfig() LegionFnQConfig {
	return LegionFnQConfig{
		Enabled:     false,
		TakeOverFan: false,
		ModeMapping: map[string]FanGearTarget{
			"Quiet":       {Gear: "静音", Level: "中"},
			"Balance":     {Gear: "标准", Level: "中"},
			"Performance": {Gear: "强劲", Level: "中"},
			"Extreme":     {Gear: "超频", Level: "中"},
			"GodMode":     {Gear: "超频", Level: "高"},
		},
	}
}

func NormalizeLegionFnQConfig(cfg LegionFnQConfig) LegionFnQConfig {
	defaults := GetDefaultLegionFnQConfig()
	if cfg.ModeMapping == nil {
		cfg.ModeMapping = map[string]FanGearTarget{}
	}

	for mode, target := range defaults.ModeMapping {
		existing, ok := cfg.ModeMapping[mode]
		if !ok {
			cfg.ModeMapping[mode] = target
			continue
		}
		cfg.ModeMapping[mode] = normalizeFanGearTarget(existing, target)
	}

	for mode, target := range cfg.ModeMapping {
		defaultTarget, ok := defaults.ModeMapping[mode]
		if !ok {
			delete(cfg.ModeMapping, mode)
			continue
		}
		cfg.ModeMapping[mode] = normalizeFanGearTarget(target, defaultTarget)
	}

	return cfg
}

func normalizeFanGearTarget(target, fallback FanGearTarget) FanGearTarget {
	if _, ok := GearCommands[target.Gear]; !ok {
		target.Gear = fallback.Gear
	}
	if target.Level != "低" && target.Level != "中" && target.Level != "高" {
		target.Level = fallback.Level
	}
	return target
}

type Logger interface {
	Info(format string, v ...any)
	Error(format string, v ...any)
	Warn(format string, v ...any)
	Debug(format string, v ...any)
	Close()
	CleanOldLogs()
	SetDebugMode(enabled bool)
	GetLogDir() string
}

// DeviceType 设备类型
const (
	DeviceTypeHID = "hid" // BS2/BS2PRO (HID 通信)
	DeviceTypeBLE = "ble" // BS1 (BLE 通信)
)

// BS1GearCommands BS1 挡位命令（无子级别，只有4个固定挡位）
// 命令格式: 5AA5 08 03 <gear_number> <checksum>
var BS1GearCommands = map[string]GearCommand{
	"静音": {"静音", deviceproto.BuildFrame(0x08, 0x01), 1300},
	"标准": {"标准", deviceproto.BuildFrame(0x08, 0x02), 2100},
	"强劲": {"强劲", deviceproto.BuildFrame(0x08, 0x03), 2800},
	"超频": {"超频", deviceproto.BuildFrame(0x08, 0x04), 3500},
}

// BS1 BLE 命令常量
var (
	// BS1CmdEnterDynamic 进入动态转速模式
	BS1CmdEnterDynamic = deviceproto.BuildFrame(deviceproto.CmdRGBEnable, 0x01)
	// BS1CmdPowerOnStartEnable 开启通电自启动
	BS1CmdPowerOnStartEnable = deviceproto.BuildFrame(deviceproto.CmdSetPowerOnStart, 0x01)
	// BS1CmdPowerOnStartDisable 关闭通电自启动
	BS1CmdPowerOnStartDisable = deviceproto.BuildFrame(deviceproto.CmdSetPowerOnStart, 0x02)
	// BS1CmdHeartbeat1 动态模式心跳包1
	BS1CmdHeartbeat1 = deviceproto.BuildFrame(deviceproto.CmdEnterRealtimeRPM)
	// BS1CmdHeartbeat2 动态模式心跳包2
	BS1CmdHeartbeat2 = deviceproto.BuildFrame(deviceproto.CmdRGBStatus)
)

// BS1DeviceName BS1 蓝牙设备名称
const BS1DeviceName = "Flydigi BS1"

// GearCommands 预设挡位命令
var GearCommands = map[string][]GearCommand{
	"静音": {
		{"1挡低", buildGearRPMCommand(0, 1300), 1300},
		{"1挡中", buildGearRPMCommand(0, 1700), 1700},
		{"1挡高", buildGearRPMCommand(0, 1900), 1900},
	},
	"标准": {
		{"2挡低", buildGearRPMCommand(1, 2100), 2100},
		{"2挡中", buildGearRPMCommand(1, 2400), 2400},
		{"2挡高", buildGearRPMCommand(1, 2700), 2700},
	},
	"强劲": {
		{"3挡低", buildGearRPMCommand(2, 2800), 2800},
		{"3挡中", buildGearRPMCommand(2, 3000), 3000},
		{"3挡高", buildGearRPMCommand(2, 3300), 3300},
	},
	"超频": {
		{"4挡低", buildGearRPMCommand(3, 3500), 3500},
		{"4挡中", buildGearRPMCommand(3, 3700), 3700},
		{"4挡高", buildGearRPMCommand(3, 4000), 4000},
	},
}

func buildGearRPMCommand(gear int, rpm int) []byte {
	return deviceproto.BuildFrame(deviceproto.CmdSetGearRPM, byte(gear), byte(rpm), byte(rpm>>8))
}

// 手动挡位转速约束（固件不上报最高转速, 也不做上限裁剪, 由 App 约束）
const (
	ManualGearMinRPM = 800  // 自定义挡位转速下限
	ManualGearMaxRPM = 4500 // 自定义挡位转速上限
)

// ManualGearOrder 四个大挡位从低到高顺序
var ManualGearOrder = []string{"静音", "标准", "强劲", "超频"}

// ManualLevelOrder 每个大挡位的小挡位从低到高顺序
var ManualLevelOrder = []string{"低", "中", "高"}

// DefaultManualGearRPM 出厂默认的 12 个挡位转速 (gear -> level -> rpm)
var DefaultManualGearRPM = map[string]map[string]int{
	"静音": {"低": 1300, "中": 1700, "高": 1900},
	"标准": {"低": 2100, "中": 2400, "高": 2700},
	"强劲": {"低": 2800, "中": 3000, "高": 3300},
	"超频": {"低": 3500, "中": 3700, "高": 4000},
}

// CloneDefaultManualGearRPM 返回默认挡位转速表的深拷贝
func CloneDefaultManualGearRPM() map[string]map[string]int {
	out := make(map[string]map[string]int, len(DefaultManualGearRPM))
	for gear, levels := range DefaultManualGearRPM {
		inner := make(map[string]int, len(levels))
		maps.Copy(inner, levels)
		out[gear] = inner
	}
	return out
}

// GearIndex 返回大挡位对应的设备挡位索引(0-3)
func GearIndex(gear string) (int, bool) {
	for i, g := range ManualGearOrder {
		if g == gear {
			return i, true
		}
	}
	return 0, false
}

// BuildGearRPMCommand 构建 0x26 挡位转速设置命令(可下发任意 16 位转速)
func BuildGearRPMCommand(gear int, rpm int) []byte {
	return buildGearRPMCommand(gear, rpm)
}

// DefaultGearRPM 返回某挡位某级别的出厂默认转速
func DefaultGearRPM(gear, level string) int {
	if levels, ok := DefaultManualGearRPM[gear]; ok {
		if rpm, ok := levels[level]; ok {
			return rpm
		}
	}
	return 0
}

// ResolveGearRPM 返回配置中某挡位某级别的转速(优先自定义, 回退默认)
func (c *AppConfig) ResolveGearRPM(gear, level string) int {
	if c != nil && c.ManualGearRPM != nil {
		if levels, ok := c.ManualGearRPM[gear]; ok {
			if rpm, ok := levels[level]; ok && rpm > 0 {
				return rpm
			}
		}
	}
	return DefaultGearRPM(gear, level)
}

func clampManualGearRPM(rpm int) int {
	if rpm < ManualGearMinRPM {
		return ManualGearMinRPM
	}
	if rpm > ManualGearMaxRPM {
		return ManualGearMaxRPM
	}
	return rpm
}

// NormalizeManualGearRPM 校验并补全 12 个自定义挡位转速:
// 缺失项用默认值补全; 限制在 [ManualGearMinRPM, ManualGearMaxRPM];
// 按从低到高(静音低 -> 超频高)强制非递减。返回是否发生修改。
func NormalizeManualGearRPM(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	changed := false
	if cfg.ManualGearRPM == nil {
		cfg.ManualGearRPM = map[string]map[string]int{}
		changed = true
	}
	prev := 0
	for _, gear := range ManualGearOrder {
		levels, ok := cfg.ManualGearRPM[gear]
		if !ok || levels == nil {
			levels = map[string]int{}
			cfg.ManualGearRPM[gear] = levels
			changed = true
		}
		for _, level := range ManualLevelOrder {
			rpm, ok := levels[level]
			if !ok || rpm <= 0 {
				rpm = DefaultGearRPM(gear, level)
			}
			rpm = max(clampManualGearRPM(rpm), prev)
			if levels[level] != rpm {
				levels[level] = rpm
				changed = true
			}
			prev = rpm
		}
	}
	return changed
}

// BS1Checksum 计算 BS1 命令校验和: (sum of all bytes + 1) & 0xFF
func BS1Checksum(data []byte) byte {
	var sum int
	for _, b := range data {
		sum += int(b)
	}
	return byte((sum + 1) & 0xFF)
}

// BuildBS1RPMCommand 构建 BS1 动态转速设置命令
// 格式: 5AA5 21 04 <rpm_lo> <rpm_hi> <checksum>
func BuildBS1RPMCommand(rpm int) []byte {
	lo := byte(rpm & 0xFF)
	hi := byte((rpm >> 8) & 0xFF)
	return deviceproto.BuildFrame(deviceproto.CmdSetRealtimeRPM, lo, hi)
}

// GetDefaultFanCurve 获取默认风扇曲线
func GetDefaultFanCurve() []FanCurvePoint {
	return []FanCurvePoint{
		{Temperature: 30, RPM: 1000},
		{Temperature: 35, RPM: 1200},
		{Temperature: 40, RPM: 1400},
		{Temperature: 45, RPM: 1600},
		{Temperature: 50, RPM: 1800},
		{Temperature: 55, RPM: 2000},
		{Temperature: 60, RPM: 2300},
		{Temperature: 65, RPM: 2600},
		{Temperature: 70, RPM: 2900},
		{Temperature: 75, RPM: 3200},
		{Temperature: 80, RPM: 3500},
		{Temperature: 85, RPM: 3800},
		{Temperature: 90, RPM: 4000},
		{Temperature: 95, RPM: 4000},
		{Temperature: 100, RPM: 4000},
		{Temperature: 105, RPM: 4000},
		{Temperature: 110, RPM: 4000},
	}
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig(isAutoStart bool) AppConfig {
	defaultCurve := GetDefaultFanCurve()
	defaultTempSelection := GetDefaultTemperatureSelection()

	return AppConfig{
		AutoControl:              false,
		ManualGearToggleHotkey:   "Ctrl+Alt+Shift+M",
		AutoControlToggleHotkey:  "Ctrl+Alt+Shift+A",
		CurveProfileToggleHotkey: "Ctrl+Alt+Shift+C",
		ManualGearLevels: map[string]string{
			"静音": "中",
			"标准": "中",
			"强劲": "中",
			"超频": "中",
		},
		ManualGearRPM: CloneDefaultManualGearRPM(),
		FanCurve:      defaultCurve,
		FanCurveProfiles: []FanCurveProfile{
			{ID: "default", Name: "默认", Curve: defaultCurve},
		},
		ActiveFanCurveProfileID: "default",
		GearLight:               true,
		PowerOnStart:            false,
		WindowsAutoStart:        false,
		ThemeMode:               ThemeModeSystem,
		SmartStartStop:          "off",
		Brightness:              100,
		TempUpdateRate:          2,
		TempSampleCount:         1,
		TempSource:              defaultTempSelection.TempSource,
		GpuDevice:               defaultTempSelection.GpuDevice,
		CpuSensor:               defaultTempSelection.CpuSensor,
		CpuSensors:              nil,
		GpuSensor:               defaultTempSelection.GpuSensor,
		WindowBlur:              WindowBlurAuto,
		SuspendFanOff:           false,
		ConfigPath:              "",
		ManualGear:              "标准",
		ManualLevel:             "中",
		DebugMode:               false,
		GuiMonitoring:           true,
		CustomSpeedEnabled:      false,
		CustomSpeedRPM:          2000,
		IgnoreDeviceOnReconnect: true, // 默认开启，防止断连后误判用户手动切换
		SpeedAvoidance:          GetDefaultSpeedAvoidanceConfig(),
		TimeCurveSchedule:       GetDefaultTimeCurveScheduleConfig(),
		SmartControl:            GetDefaultSmartControlConfig(defaultCurve),
		LightStrip:              GetDefaultLightStripConfig(),
		LegionFnQ:               GetDefaultLegionFnQConfig(),
		LegionFnQSupport:        LegionFnQSupportCache{},
	}
}

// Package types 定义了 BS2PRO 控制器应用中使用的所有共享类型
package types

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
	TempSourceMax          = "max"
	TempSourceCPU          = "cpu"
	TempSourceGPU          = "gpu"
	TempDeviceAuto         = "auto"
	TempSensorAuto         = "auto"
)

// NormalizeThemeMode 归一化主题模式，非法值回退为 system。
func NormalizeThemeMode(mode string) string {
	switch mode {
	case ThemeModeLight:
		return ThemeModeLight
	case ThemeModeDark:
		return ThemeModeDark
	default:
		return ThemeModeSystem
	}
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

// NormalizeDeviceSelection 归一化设备选择，空值回退为 auto。
func NormalizeDeviceSelection(selection string) string {
	if selection == "" {
		return TempDeviceAuto
	}
	return selection
}

// TemperatureSelection 温度读取选择配置。
type TemperatureSelection struct {
	TempSource string `json:"tempSource"`
	GpuDevice  string `json:"gpuDevice"`
	CpuSensor  string `json:"cpuSensor"`
	GpuSensor  string `json:"gpuSensor"`
}

// NormalizeTemperatureSelection 归一化温度选择配置。
func NormalizeTemperatureSelection(selection TemperatureSelection) TemperatureSelection {
	selection.TempSource = NormalizeTempSource(selection.TempSource)
	selection.GpuDevice = NormalizeDeviceSelection(selection.GpuDevice)
	selection.CpuSensor = NormalizeSensorSelection(selection.CpuSensor)
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
	Key     string              `json:"key"`
	Name    string              `json:"name"`
	Vendor  string              `json:"vendor"`
	Sensors []TemperatureSensor `json:"sensors"`
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
	MaxTemp           int                    `json:"maxTemp"`           // 最高温度
	ControlTemp       int                    `json:"controlTemp"`       // 当前控温基准温度
	ControlSource     string                 `json:"controlSource"`     // 当前控温基准来源
	SelectedGpuDevice string                 `json:"selectedGpuDevice"` // 当前选中的 GPU 设备 key
	CpuModel          string                 `json:"cpuModel"`          // 当前识别的 CPU 型号
	GpuModel          string                 `json:"gpuModel"`          // 当前识别的 GPU 型号
	CpuSensors        []TemperatureSensor    `json:"cpuSensors"`        // 当前识别到的 CPU 温度传感器
	GpuSensors        []TemperatureSensor    `json:"gpuSensors"`        // 当前识别到的 GPU 温度传感器
	GpuDevices        []TemperatureGPUDevice `json:"gpuDevices"`        // 当前识别到的 GPU 设备列表
	UpdateTime        int64                  `json:"updateTime"`        // 更新时间戳
	BridgeOk          bool                   `json:"bridgeOk"`          // 桥接程序是否正常
	BridgeMsg         string                 `json:"bridgeMessage"`     // 桥接故障提示
}

// BridgeTemperatureData 桥接程序返回的温度数据
type BridgeTemperatureData struct {
	CpuTemp           int                    `json:"cpuTemp"`
	GpuTemp           int                    `json:"gpuTemp"`
	MaxTemp           int                    `json:"maxTemp"`
	ControlTemp       int                    `json:"controlTemp"`
	ControlSource     string                 `json:"controlSource"`
	SelectedGpuDevice string                 `json:"selectedGpuDevice"`
	CpuModel          string                 `json:"cpuModel"`
	GpuModel          string                 `json:"gpuModel"`
	CpuSensors        []TemperatureSensor    `json:"cpuSensors"`
	GpuSensors        []TemperatureSensor    `json:"gpuSensors"`
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

type SmartControlConfig struct {
	Enabled              bool  `json:"enabled"`              // 智能耦合控制开关
	Learning             bool  `json:"learning"`             // 学习开关
	FilterTransientSpike bool  `json:"filterTransientSpike"` // 是否过滤孤立温度尖峰
	TargetTemp           int   `json:"targetTemp"`           // 目标温度(°C)
	Aggressiveness       int   `json:"aggressiveness"`       // 响应激进度(1-10)
	Hysteresis           int   `json:"hysteresis"`           // 滞回温差(°C)
	MinRPMChange         int   `json:"minRpmChange"`         // 最小生效转速变化(RPM)
	RampUpLimit          int   `json:"rampUpLimit"`          // 每次更新最大升速(RPM)
	RampDownLimit        int   `json:"rampDownLimit"`        // 每次更新最大降速(RPM)
	LearnRate            int   `json:"learnRate"`            // 学习速度(1-10)
	LearnWindow          int   `json:"learnWindow"`          // 稳态学习窗口(采样点)
	LearnDelay           int   `json:"learnDelay"`           // 学习延迟步数(处理热惯性)
	OverheatWeight       int   `json:"overheatWeight"`       // 过热惩罚权重
	RPMDeltaWeight       int   `json:"rpmDeltaWeight"`       // 转速变化惩罚权重
	NoiseWeight          int   `json:"noiseWeight"`          // 高转速噪音惩罚权重
	TrendGain            int   `json:"trendGain"`            // 温升趋势前馈增益
	MaxLearnOffset       int   `json:"maxLearnOffset"`       // 学习偏移上限(RPM)
	LearnedOffsets       []int `json:"learnedOffsets"`       // 每个曲线点的学习偏移(RPM)
	LearnedOffsetsHeat   []int `json:"learnedOffsetsHeat"`   // 升温工况学习偏移(RPM)
	LearnedOffsetsCool   []int `json:"learnedOffsetsCool"`   // 降温工况学习偏移(RPM)
	LearnedRateHeat      []int `json:"learnedRateHeat"`      // 升温变化率学习偏置(分桶RPM)
	LearnedRateCool      []int `json:"learnedRateCool"`      // 降温变化率学习偏置(分桶RPM)
}

// AppConfig 应用配置
type AppConfig struct {
	LegionFnQ                LegionFnQConfig    `json:"legionFnQ"`
	AutoControl              bool               `json:"autoControl"`              // 智能变频开关
	ManualGearToggleHotkey   string             `json:"manualGearToggleHotkey"`   // 切换手动挡位快捷键
	AutoControlToggleHotkey  string             `json:"autoControlToggleHotkey"`  // 开关智能变频快捷键
	CurveProfileToggleHotkey string             `json:"curveProfileToggleHotkey"` // 切换温控曲线方案快捷键
	ManualGearLevels         map[string]string  `json:"manualGearLevels"`         // 每个大挡位记忆的小挡位(低中高)
	FanCurve                 []FanCurvePoint    `json:"fanCurve"`                 // 风扇曲线
	FanCurveProfiles         []FanCurveProfile  `json:"fanCurveProfiles"`         // 风扇曲线方案列表
	ActiveFanCurveProfileID  string             `json:"activeFanCurveProfileId"`  // 当前激活曲线方案ID
	GearLight                bool               `json:"gearLight"`                // 挡位灯
	PowerOnStart             bool               `json:"powerOnStart"`             // 通电自启动
	WindowsAutoStart         bool               `json:"windowsAutoStart"`         // Windows开机自启动
	ThemeMode                string             `json:"themeMode"`                // 主题模式: system/light/dark
	SmartStartStop           string             `json:"smartStartStop"`           // 智能启停
	Brightness               int                `json:"brightness"`               // 亮度
	TempUpdateRate           int                `json:"tempUpdateRate"`           // 温度更新频率(秒)
	TempSampleCount          int                `json:"tempSampleCount"`          // 温度采样次数(用于平均)
	TempSource               string             `json:"tempSource"`               // 控温温度来源: max/cpu/gpu
	GpuDevice                string             `json:"gpuDevice"`                // GPU 设备选择: auto 或设备 key
	CpuSensor                string             `json:"cpuSensor"`                // CPU 传感器选择: auto 或传感器 key
	GpuSensor                string             `json:"gpuSensor"`                // GPU 传感器选择: auto 或传感器 key
	ConfigPath               string             `json:"configPath"`               // 配置文件路径
	ManualGear               string             `json:"manualGear"`               // 手动挡位设置
	ManualLevel              string             `json:"manualLevel"`              // 手动挡位级别(低中高)
	DebugMode                bool               `json:"debugMode"`                // 调试模式
	GuiMonitoring            bool               `json:"guiMonitoring"`            // GUI监控开关
	CustomSpeedEnabled       bool               `json:"customSpeedEnabled"`       // 自定义转速开关
	CustomSpeedRPM           int                `json:"customSpeedRPM"`           // 自定义转速值(无上下限)
	IgnoreDeviceOnReconnect  bool               `json:"ignoreDeviceOnReconnect"`  // 断连后忽略设备状态(保持APP配置)
	SmartControl             SmartControlConfig `json:"smartControl"`             // 学习型智能控温配置
	LightStrip               LightStripConfig   `json:"lightStrip"`               // 灯带配置
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
		FilterTransientSpike: true,
		TargetTemp:           68,
		Aggressiveness:       5,
		Hysteresis:           2,
		MinRPMChange:         50,
		RampUpLimit:          220,
		RampDownLimit:        160,
		LearnRate:            4,
		LearnWindow:          6,
		LearnDelay:           2,
		OverheatWeight:       8,
		RPMDeltaWeight:       5,
		NoiseWeight:          4,
		TrendGain:            5,
		MaxLearnOffset:       600,
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
	"静音": {"静音", []byte{0x5a, 0xa5, 0x08, 0x03, 0x01, 0x0c}, 1300},
	"标准": {"标准", []byte{0x5a, 0xa5, 0x08, 0x03, 0x02, 0x0d}, 2100},
	"强劲": {"强劲", []byte{0x5a, 0xa5, 0x08, 0x03, 0x03, 0x0e}, 2800},
	"超频": {"超频", []byte{0x5a, 0xa5, 0x08, 0x03, 0x04, 0x0f}, 3500},
}

// BS1 BLE 命令常量
var (
	// BS1CmdEnterDynamic 进入动态转速模式
	BS1CmdEnterDynamic = []byte{0x5a, 0xa5, 0x46, 0x03, 0x01, 0x4a}
	// BS1CmdPowerOnStartEnable 开启通电自启动
	BS1CmdPowerOnStartEnable = []byte{0x5a, 0xa5, 0x0c, 0x03, 0x01, 0x10}
	// BS1CmdPowerOnStartDisable 关闭通电自启动
	BS1CmdPowerOnStartDisable = []byte{0x5a, 0xa5, 0x0c, 0x03, 0x02, 0x11}
	// BS1CmdHeartbeat1 动态模式心跳包1
	BS1CmdHeartbeat1 = []byte{0x5a, 0xa5, 0x23, 0x02, 0x25}
	// BS1CmdHeartbeat2 动态模式心跳包2
	BS1CmdHeartbeat2 = []byte{0x5a, 0xa5, 0x45, 0x02, 0x47}
)

// BS1DeviceName BS1 蓝牙设备名称
const BS1DeviceName = "Flydigi BS1"

// GearCommands 预设挡位命令
var GearCommands = map[string][]GearCommand{
	"静音": {
		{"1挡低", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0x14, 0x05, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1300},
		{"1挡中", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0xa4, 0x06, 0xd5, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1700},
		{"1挡高", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0x6c, 0x07, 0x9e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1900},
	},
	"标准": {
		{"2挡低", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x34, 0x08, 0x68, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2100},
		{"2挡中", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x60, 0x09, 0x95, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2310},
		{"2挡高", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x8c, 0x0a, 0xc2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2760},
	},
	"强劲": {
		{"3挡低", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xf0, 0x0a, 0x27, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2800},
		{"3挡中", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xb8, 0x0b, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3000},
		{"3挡高", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xe4, 0x0c, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3300},
	},
	"超频": {
		{"4挡低", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0xac, 0x0d, 0xe7, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3500},
		{"4挡中", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0x74, 0x0e, 0xb0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3700},
		{"4挡高", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0xa0, 0x0f, 0xdd, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 4000},
	},
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
	payload := []byte{0x5a, 0xa5, 0x21, 0x04, lo, hi}
	checksum := BS1Checksum(payload)
	return append(payload, checksum)
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
		FanCurve: defaultCurve,
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
		GpuSensor:               defaultTempSelection.GpuSensor,
		ConfigPath:              "",
		ManualGear:              "标准",
		ManualLevel:             "中",
		DebugMode:               false,
		GuiMonitoring:           true,
		CustomSpeedEnabled:      false,
		CustomSpeedRPM:          2000,
		IgnoreDeviceOnReconnect: true, // 默认开启，防止断连后误判用户手动切换
		SmartControl:            GetDefaultSmartControlConfig(defaultCurve),
		LightStrip:              GetDefaultLightStripConfig(),
		LegionFnQ:               GetDefaultLegionFnQConfig(),
	}
}

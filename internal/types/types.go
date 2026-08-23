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
	// DisableGpu 完全跳过 GPU 温度/功耗读取。混合显卡笔记本上轮询 GPU 传感器
	// 会持续唤醒独显，开启后控温基准只使用 CPU 温度。
	DisableGpu bool `json:"disableGpu"`
	// ExtendedSensors 额外读取内存、硬盘、主板/EC、电源等温度。它们不参与控温，
	// 只用于展示与散热收益报告；单列开关是因为要为此打开 SMART/SPD/Super I/O 通道，
	// 对常驻后台的核心不是零成本。
	ExtendedSensors bool `json:"extendedSensors"`
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
	FrameLength  uint8  `json:"frameLength"`
	Status       uint8  `json:"status,omitempty"` // BS1 status byte; BS2PRO uses FrameLength instead.
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
	Available                bool     `json:"available"`
	Source                   string   `json:"source"`
	ReadAt                   string   `json:"readAt"`
	ReadErrors               []string `json:"readErrors,omitempty"`
	Model                    string   `json:"model,omitempty"`
	DeviceCPUModel           string   `json:"deviceCpuModel,omitempty"`
	DeviceCPUModelSource     string   `json:"deviceCpuModelSource,omitempty"`
	HIDManufacturer          string   `json:"hidManufacturer,omitempty"`
	HIDProduct               string   `json:"hidProduct,omitempty"`
	HIDSerialNumber          string   `json:"hidSerialNumber,omitempty"`
	HIDReleaseNumber         uint16   `json:"hidReleaseNumber,omitempty"`
	HIDReleaseNumberHex      string   `json:"hidReleaseNumberHex,omitempty"`
	FirmwareVersion          string   `json:"firmwareVersion,omitempty"`
	FirmwareVersionRaw       string   `json:"firmwareVersionRaw,omitempty"`
	FirmwareReadStatus       string   `json:"firmwareReadStatus,omitempty"` // ready/failed/unsupported
	FirmwareReadError        string   `json:"firmwareReadError,omitempty"`
	DeviceIdentifier         string   `json:"deviceIdentifier,omitempty"`
	IdentityMarker           string   `json:"identityMarker,omitempty"`
	IdentityHex              string   `json:"identityHex,omitempty"`
	ConfigState              string   `json:"configState,omitempty"`
	ConfigStateName          string   `json:"configStateName,omitempty"`
	ControllerCapabilityTier *int     `json:"controllerCapabilityTier,omitempty"`
	// MaxSelectableGear 是按能力档位推导出的、设备真的会切换过去的最高挡位。
	// 固件对超出的挡位照常回 ACK=1 但不换挡，所以这个值必须由主机自己算。
	MaxSelectableGear    *int               `json:"maxSelectableGear,omitempty"`
	RuntimeProfileRaw    *int               `json:"runtimeProfileRaw,omitempty"`
	MeasuredRPM          *int               `json:"measuredRpm,omitempty"`
	TargetRPM            *int               `json:"targetRpm,omitempty"`
	GearRPMTable         []DeviceGearRPM    `json:"gearRpmTable,omitempty"`
	QueriedWorkState     string             `json:"queriedWorkState,omitempty"`
	QueriedWorkStateName string             `json:"queriedWorkStateName,omitempty"`
	LiveModeFlags        string             `json:"liveModeFlags,omitempty"`
	LiveModeName         string             `json:"liveModeName,omitempty"`
	ActiveGear           int                `json:"activeGear,omitempty"`
	SelectedGear         int                `json:"selectedGear,omitempty"`
	RealtimeActive       *bool              `json:"realtimeActive,omitempty"`
	RGBState             string             `json:"rgbState,omitempty"`
	RGBStateName         string             `json:"rgbStateName,omitempty"`
	Status               *DeviceStatusRead  `json:"status,omitempty"`
	RawFrames            []DeviceDebugFrame `json:"rawFrames,omitempty"`
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
	// OtherSensors 是 CPU/GPU 之外能读到的全部温度：内存、硬盘、主板、EC、电源、电池。
	// 归属由 Key 前缀标明（memory/ storage/ board/ ec/ psu/ battery/），与 cpu/ gpu/ 同一套约定。
	// 仅在开启扩展传感器时非空，且不参与控温决策。
	OtherSensors []TemperatureSensor `json:"otherSensors,omitempty"`
	UpdateTime   int64               `json:"updateTime"`    // 更新时间戳
	BridgeOk     bool                `json:"bridgeOk"`      // 桥接程序是否正常
	BridgeMsg    string              `json:"bridgeMessage"` // 桥接故障提示
	// CPUTempError 是 CPU 温度专属的故障说明与修复指引。
	// CPU 温度只能由 PawnIO 读取，没有可信的替代来源，因此读不到时必须明确告知用户，
	// 而不是留一个没有解释的空值。GPU 正常时 BridgeOk 仍为 true，该字段独立生效。
	CPUTempError string `json:"cpuTempError"`
	CPUFanRPM    int    `json:"cpuFanRpm"` // 笔记本内置 CPU 风扇转速（0=不可用）
	GPUFanRPM    int    `json:"gpuFanRpm"` // 笔记本内置 GPU 风扇转速（0=不可用）
}

// TemperatureHistoryPoint CPU/GPU 温度历史点。
type TemperatureHistoryPoint struct {
	Timestamp int64   `json:"timestamp"`
	CPUTemp   int     `json:"cpuTemp"`
	GPUTemp   int     `json:"gpuTemp"`
	CPUPower  float64 `json:"cpuPower"`
	GPUPower  float64 `json:"gpuPower"`
	FanRPM    int     `json:"fanRpm"`
	CPUFanRPM int     `json:"cpuFanRpm"` // 笔记本内置 CPU 风扇转速（0=不可用）
	GPUFanRPM int     `json:"gpuFanRpm"` // 笔记本内置 GPU 风扇转速（0=不可用）
}

// TimelineEvent 温度趋势图上标注的一次状态变化（设备断连、切换控温模式、睡眠唤醒等）。
//
// LabelKey 存的是前端 i18n 键而不是本地化文案：核心服务常驻后台、不感知 GUI 当前
// 语言，且事件会跨进程重启持久化，存成中文会让日后切换语言的用户看到混排文案。
type TimelineEvent struct {
	Timestamp int64  `json:"timestamp"` // Unix 毫秒
	Type      string `json:"type"`      // 决定标记颜色，见 TimelineEventType* 常量
	LabelKey  string `json:"labelKey"`  // 前端 i18n 键
}

// 时间轴事件分类。前端按此决定参考线颜色。
const (
	TimelineEventTypeMode       = "mode"       // 连接、控温模式等常规状态变化
	TimelineEventTypeDisconnect = "disconnect" // 设备断连
	TimelineEventTypeResume     = "resume"     // 系统睡眠唤醒
	TimelineEventTypeProfile    = "profile"    // 曲线方案切换
)

// 时间轴事件的前端 i18n 键，对应 locales 里的 fanCurve.history.timeline.*。
const (
	TimelineKeyDeviceConnected    = "fanCurve.history.timeline.deviceConnected"
	TimelineKeyDeviceDisconnected = "fanCurve.history.timeline.deviceDisconnected"
	TimelineKeySmartControlOn     = "fanCurve.history.timeline.smartControlOn"
	TimelineKeySmartControlOff    = "fanCurve.history.timeline.smartControlOff"
	TimelineKeyCurveSwitched      = "fanCurve.history.timeline.curveSwitched"
	TimelineKeyResumeFromSleep    = "fanCurve.history.timeline.resumeFromSleep"
	TimelineKeySystemSuspended    = "fanCurve.history.timeline.systemSuspended"
	TimelineKeyCoreStarted        = "fanCurve.history.timeline.coreStarted"
)

// 温度历史后台保留时长的取值范围。放在 types 而非 temperature 包：配置默认值与归一化
// 都要用到它，而 temperature 反过来依赖 types。
const (
	DefaultTemperatureHistoryRetentionHours = 1
	MaxTemperatureHistoryRetentionHours     = 24
)

// NormalizeTemperatureHistoryRetentionHours 把保留时长夹到合法区间；
// 0（旧配置文件缺少该字段时的零值）按默认处理。
func NormalizeTemperatureHistoryRetentionHours(hours int) int {
	if hours < 1 {
		return DefaultTemperatureHistoryRetentionHours
	}
	if hours > MaxTemperatureHistoryRetentionHours {
		return MaxTemperatureHistoryRetentionHours
	}
	return hours
}

// TemperatureHistoryPayload 温度历史返回载荷。
type TemperatureHistoryPayload struct {
	Enabled               bool                      `json:"enabled"`
	SampleIntervalSeconds int                       `json:"sampleIntervalSeconds"`
	RetentionHours        int                       `json:"retentionHours"` // 后台保留的历史时长(小时)
	Points                []TemperatureHistoryPoint `json:"points"`
	Events                []TimelineEvent           `json:"events"` // 与 Points 同一保留窗口内的状态变化
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
	OtherSensors      []TemperatureSensor    `json:"otherSensors"`
	UpdateTime        int64                  `json:"updateTime"`
	Success           bool                   `json:"success"`
	Error             string                 `json:"error"`
	// CpuTempError 在 GPU 可读、仅 CPU 读不到时也会填充（此时 Success 仍为 true）。
	CpuTempError string `json:"cpuTempError"`
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

	// SmartTempBands 是智能温控模式下"温度区间 -> 固件原生灯效预设"的映射，
	// 按 MinTemp 升序排列，第一段的 MinTemp 恒为 0。
	SmartTempBands []SmartTempLightBand `json:"smartTempBands"`
	// SmartTempHysteresis 是区间切换的回差（°C），防止临界温度反复抖动。
	SmartTempHysteresis int `json:"smartTempHysteresis"`
}

/* ── 智能温控灯效 ──

固件自己读不到电脑温度，所以温度分区完全由主机驱动：主机按当前 CPU/GPU 最高温
决定该用哪个原生预设，再用 0x44 下发。

这里只使用固件原生预设（0x44），不上传自定义帧。原因是固件的自定义帧提交命令
0x43 会触发一次 256 字节数据闪存的擦写，而温度分区切换是随负载持续发生的：
用自定义颜色实现分区，等于把闪存擦写绑到温度曲线上。0x44 只改运行期状态，
不写闪存，可以任意频繁地切换。
*/

const (
	// SmartTempLightMinPreset/SmartTempLightMaxPreset 是固件 0x44 接受的预设范围。
	// 0 是"停止动画"，业务上不作为区间取值。
	SmartTempLightMinPreset = 1
	SmartTempLightMaxPreset = 5
	// SmartTempLightMaxBands 限制区间数量，避免配置膨胀到无法在界面上展示。
	SmartTempLightMaxBands = 5
	// SmartTempLightMaxTemp 是区间下限允许的最高值。
	SmartTempLightMaxTemp = 110
	// SmartTempLightMaxHysteresis 是回差上限。
	SmartTempLightMaxHysteresis = 10
)

// SmartTempLightBand 是智能温控灯效的一个温度区间。
type SmartTempLightBand struct {
	// MinTemp 是该区间的下限（含）。第一段恒为 0，其余必须严格递增。
	MinTemp int `json:"minTemp"`
	// Preset 是固件原生灯效预设编号，取值 1..5。
	Preset int `json:"preset"`
}

// SmartTempLightStatus 是智能温控灯效当前的运行状态，供界面展示"现在落在哪一段"。
type SmartTempLightStatus struct {
	Active      bool `json:"active"`      // 灯带模式是否为 smart_temp 且已下发过预设
	BandIndex   int  `json:"bandIndex"`   // 当前命中的区间下标；未生效时为 -1
	Preset      int  `json:"preset"`      // 当前已下发的固件预设；未生效时为 0
	Temperature int  `json:"temperature"` // 最近一次用于判定的温度
}

// GetDefaultSmartTempLightBands 返回默认温度分区。它与固件原生预设 1/2/3 在视觉上
// 依次是绿、黄、红，边界沿用历史行为：<=70°C 绿，71..80°C 黄，>80°C 红。
func GetDefaultSmartTempLightBands() []SmartTempLightBand {
	return []SmartTempLightBand{
		{MinTemp: 0, Preset: 1},
		{MinTemp: 71, Preset: 2},
		{MinTemp: 81, Preset: 3},
	}
}

// DefaultSmartTempLightHysteresis 是默认回差，沿用历史的 2°C。
const DefaultSmartTempLightHysteresis = 2

// NormalizeSmartTempLightBands 把任意用户输入整理成一组合法区间：下限升序、
// 首段为 0、预设落在 1..5、数量不超过上限。返回是否发生了修改。
func NormalizeSmartTempLightBands(bands []SmartTempLightBand) ([]SmartTempLightBand, bool) {
	if len(bands) == 0 {
		return GetDefaultSmartTempLightBands(), true
	}

	changed := false
	if len(bands) > SmartTempLightMaxBands {
		bands = bands[:SmartTempLightMaxBands]
		changed = true
	}

	normalized := make([]SmartTempLightBand, 0, len(bands))
	previousMin := -1
	for i, band := range bands {
		preset := band.Preset
		if preset < SmartTempLightMinPreset || preset > SmartTempLightMaxPreset {
			preset = SmartTempLightMinPreset
			changed = true
		}

		minTemp := band.MinTemp
		if i == 0 {
			// 第一段必须覆盖到 0°C，否则低温时没有任何区间命中。
			if minTemp != 0 {
				minTemp = 0
				changed = true
			}
		} else {
			if minTemp > SmartTempLightMaxTemp {
				minTemp = SmartTempLightMaxTemp
				changed = true
			}
			if minTemp <= previousMin {
				minTemp = previousMin + 1
				changed = true
			}
		}
		if minTemp > SmartTempLightMaxTemp {
			// 上一段已经顶到上限，这一段无处安放，直接丢弃后面的区间。
			changed = true
			break
		}

		if minTemp != band.MinTemp || preset != band.Preset {
			changed = true
		}
		normalized = append(normalized, SmartTempLightBand{MinTemp: minTemp, Preset: preset})
		previousMin = minTemp
	}

	if len(normalized) == 0 {
		return GetDefaultSmartTempLightBands(), true
	}
	return normalized, changed
}

// SelectSmartTempLightBand 返回温度应当落入的区间下标。
//
// currentIndex 传入上一次生效的下标可启用回差：只有超过上一段上界 + 回差才向上跳，
// 只有低于本段下界 - 回差才向下退。currentIndex 为负表示首次判定，直接按阈值取值。
func SelectSmartTempLightBand(bands []SmartTempLightBand, hysteresis, temperature, currentIndex int) int {
	if len(bands) == 0 {
		return -1
	}
	if hysteresis < 0 {
		hysteresis = 0
	}
	if hysteresis > SmartTempLightMaxHysteresis {
		hysteresis = SmartTempLightMaxHysteresis
	}

	if currentIndex < 0 || currentIndex >= len(bands) {
		index := 0
		for i := range bands {
			if temperature >= bands[i].MinTemp {
				index = i
			}
		}
		return index
	}

	index := currentIndex
	for index+1 < len(bands) && temperature >= bands[index+1].MinTemp+hysteresis {
		index++
	}
	for index > 0 && temperature < bands[index].MinTemp-hysteresis {
		index--
	}
	return index
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
	PredictiveBoost         bool              `json:"predictiveBoost"`                 // 提前升速：自适应学习的子开关，仅在 Learning 开启时生效
	LearningBias            string            `json:"learningBias"`                    // 学习倾向: balanced/cooling/quiet
	FilterTransientSpike    bool              `json:"filterTransientSpike"`            // 是否过滤孤立温度尖峰
	LaptopFanGuard          bool              `json:"laptopFanGuard"`                  // 笔记本风扇高转时缓慢降速：自适应学习的子开关，仅在 Learning 开启且能读到笔记本风扇转速时生效
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

const DefaultRTSSUpdateIntervalMS = 1000

const (
	RTSSPositionModeAnchor = "anchor"
	RTSSPositionModeCustom = "custom"
	defaultRTSSPositionX   = 0
	defaultRTSSPositionY   = 0
	minRTSSPosition        = -1000
	maxRTSSPosition        = 1000
)

type RTSSConfig struct {
	Enabled          bool   `json:"enabled"`
	UpdateIntervalMS int    `json:"updateIntervalMs"`
	PositionMode     string `json:"positionMode"`
	PositionX        int    `json:"positionX"`
	PositionY        int    `json:"positionY"`
}

func GetDefaultRTSSConfig() RTSSConfig {
	return RTSSConfig{
		Enabled:          false,
		UpdateIntervalMS: DefaultRTSSUpdateIntervalMS,
		PositionMode:     RTSSPositionModeAnchor,
		PositionX:        defaultRTSSPositionX,
		PositionY:        defaultRTSSPositionY,
	}
}

func NormalizeRTSSConfig(cfg RTSSConfig) (RTSSConfig, bool) {
	changed := false
	switch cfg.UpdateIntervalMS {
	case 250, 500, 1000, 2000:
	default:
		cfg.UpdateIntervalMS = DefaultRTSSUpdateIntervalMS
		changed = true
	}
	if cfg.PositionMode == "" {
		// Existing installations used the OverlayEditor anchor implicitly.
		// Keep that behavior when the new setting is absent.
		cfg.PositionMode = RTSSPositionModeAnchor
		changed = true
	}
	if cfg.PositionMode != RTSSPositionModeAnchor && cfg.PositionMode != RTSSPositionModeCustom {
		cfg.PositionMode = RTSSPositionModeAnchor
		changed = true
	}
	if cfg.PositionX < minRTSSPosition {
		cfg.PositionX = minRTSSPosition
		changed = true
	} else if cfg.PositionX > maxRTSSPosition {
		cfg.PositionX = maxRTSSPosition
		changed = true
	}
	if cfg.PositionY < minRTSSPosition {
		cfg.PositionY = minRTSSPosition
		changed = true
	} else if cfg.PositionY > maxRTSSPosition {
		cfg.PositionY = maxRTSSPosition
		changed = true
	}
	return cfg, changed
}

// AppConfig 应用配置
type AppConfig struct {
	LegionFnQ                        LegionFnQConfig           `json:"legionFnQ"`
	LegionFnQSupport                 LegionFnQSupportCache     `json:"legionFnQSupport"`
	AutoControl                      bool                      `json:"autoControl"`                      // 智能变频开关
	ManualGearToggleHotkey           string                    `json:"manualGearToggleHotkey"`           // 切换手动挡位快捷键
	AutoControlToggleHotkey          string                    `json:"autoControlToggleHotkey"`          // 开关智能变频快捷键
	CurveProfileToggleHotkey         string                    `json:"curveProfileToggleHotkey"`         // 切换温控曲线方案快捷键
	ManualGearLevels                 map[string]string         `json:"manualGearLevels"`                 // 每个大挡位记忆的小挡位(低中高)
	ManualGearRPM                    map[string]map[string]int `json:"manualGearRpm"`                    // 每个大挡位低/中/高的自定义转速
	FanCurve                         []FanCurvePoint           `json:"fanCurve"`                         // 风扇曲线
	FanCurveProfiles                 []FanCurveProfile         `json:"fanCurveProfiles"`                 // 风扇曲线方案列表
	ActiveFanCurveProfileID          string                    `json:"activeFanCurveProfileId"`          // 当前激活曲线方案ID
	GearLight                        bool                      `json:"gearLight"`                        // 挡位灯
	PowerOnStart                     bool                      `json:"powerOnStart"`                     // 通电自启动
	WindowsAutoStart                 bool                      `json:"windowsAutoStart"`                 // Windows开机自启动
	DisableSystemTray                bool                      `json:"disableSystemTray"`                // 关闭系统托盘图标(取反语义：旧配置缺字段时默认显示托盘)
	ThemeMode                        string                    `json:"themeMode"`                        // 主题模式: system/light/dark/thrm
	SmartStartStop                   string                    `json:"smartStartStop"`                   // 智能启停
	Brightness                       int                       `json:"brightness"`                       // 亮度
	TempUpdateRate                   int                       `json:"tempUpdateRate"`                   // 温度更新频率(秒)
	TempSampleCount                  int                       `json:"tempSampleCount"`                  // 温度采样次数(用于平均)
	TempSource                       string                    `json:"tempSource"`                       // 控温温度来源: max/cpu/gpu
	TemperatureHistoryRetentionHours int                       `json:"temperatureHistoryRetentionHours"` // 温度历史后台保留时长(小时)，默认1
	DisableGpuMonitoring             bool                      `json:"disableGpuMonitoring"`             // 停用 GPU 温度监测(混合显卡防止独显被轮询唤醒)
	GpuDevice                        string                    `json:"gpuDevice"`                        // GPU 设备选择: auto 或设备 key
	CpuSensor                        string                    `json:"cpuSensor"`                        // CPU 传感器选择: auto 或传感器 key
	CpuSensors                       []string                  `json:"cpuSensors"`                       // CPU 多传感器选择(多核平均): 为空则按 cpuSensor 单选/自动
	GpuSensor                        string                    `json:"gpuSensor"`                        // GPU 传感器选择: auto 或传感器 key
	WindowBlur                       string                    `json:"windowBlur"`                       // 窗口材质: auto/acrylic/mica/tabbed/off；兼容旧值 on
	ConfigPath                       string                    `json:"configPath"`                       // 配置文件路径
	ManualGear                       string                    `json:"manualGear"`                       // 手动挡位设置
	ManualLevel                      string                    `json:"manualLevel"`                      // 手动挡位级别(低中高)
	DebugMode                        bool                      `json:"debugMode"`                        // 调试模式
	GuiMonitoring                    bool                      `json:"guiMonitoring"`                    // 已废弃：健康监控/自愈现在无条件运行，保留字段仅为兼容旧配置文件
	CustomSpeedEnabled               bool                      `json:"customSpeedEnabled"`               // 自定义转速开关
	CustomSpeedRPM                   int                       `json:"customSpeedRPM"`                   // 自定义转速值(无上下限)
	IgnoreDeviceOnReconnect          bool                      `json:"ignoreDeviceOnReconnect"`          // 断连后忽略设备状态(保持APP配置)
	LastDeviceTransport              string                    `json:"lastDeviceTransport"`              // 上次成功连接的传输方式("hid"/"ble")，用于重启后恢复重连偏好
	FlydigiCompat                    bool                      `json:"flydigiCompat"`                    // 飞智空间站兼容：阻止其后台服务(LocalSystem)接管散热器
	RTSS                             RTSSConfig                `json:"rtss"`                             // RTSS 游戏内叠加层转速输出

	SpeedAvoidance    SpeedAvoidanceConfig    `json:"speedAvoidance"`    // 智能控温转速避让
	TimeCurveSchedule TimeCurveScheduleConfig `json:"timeCurveSchedule"` // 分时曲线计划
	SmartControl      SmartControlConfig      `json:"smartControl"`      // 学习型智能控温配置
	CoolingBenefit    CoolingBenefitState     `json:"coolingBenefit"`    // 散热收益实测与日常统计（与学习模式无关）
	LightStrip        LightStripConfig        `json:"lightStrip"`        // 灯带配置
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
		SmartTempBands:      GetDefaultSmartTempLightBands(),
		SmartTempHysteresis: DefaultSmartTempLightHysteresis,
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
		LaptopFanGuard:       true,
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

// GearCommands 是 App 的 12 个虚拟预设；固件只有 4 个硬件槽位。
// 选择低/中/高时，App 会把对应 RPM 写入该大挡位的同一个硬件槽位并立即切换。
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

// 手动挡位转速约束。协议字段可容纳更高数值，App 开放到 5000 RPM。
const (
	ManualGearMinRPM = 800  // 自定义挡位转速下限
	ManualGearMaxRPM = 5000 // 自定义挡位转速上限
)

// ManualGearOrder 四个大挡位从低到高顺序
var ManualGearOrder = []string{"静音", "标准", "强劲", "超频"}

// ManualLevelOrder 每个大挡位的小挡位从低到高顺序
var ManualLevelOrder = []string{"低", "中", "高"}

// DefaultManualGearRPM 是 App 的 12 个虚拟预设，不是固件中的 12 个槽位。
// 固件 0x06 的四个默认值 1700/2400/3000/4000 分别对应前三挡“中”和超频“高”。
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
		DisableSystemTray:       false,
		ThemeMode:               ThemeModeSystem,
		SmartStartStop:          "off",
		Brightness:              100,
		TempUpdateRate:          2,
		TempSampleCount:         1,
		// 显式写入默认值，避免新装配置文件里出现一个非法的 0。
		TemperatureHistoryRetentionHours: DefaultTemperatureHistoryRetentionHours,
		TempSource:                       defaultTempSelection.TempSource,
		GpuDevice:                        defaultTempSelection.GpuDevice,
		CpuSensor:                        defaultTempSelection.CpuSensor,
		CpuSensors:                       nil,
		GpuSensor:                        defaultTempSelection.GpuSensor,
		WindowBlur:                       WindowBlurAuto,
		ConfigPath:                       "",
		ManualGear:                       "标准",
		ManualLevel:                      "中",
		DebugMode:                        false,
		GuiMonitoring:                    true,
		CustomSpeedEnabled:               false,
		CustomSpeedRPM:                   2000,
		IgnoreDeviceOnReconnect:          true,  // 默认开启，防止断连后误判用户手动切换
		FlydigiCompat:                    false, // 默认关闭：会改写设备安全描述符，必须由用户显式开启
		RTSS:                             GetDefaultRTSSConfig(),
		SpeedAvoidance:                   GetDefaultSpeedAvoidanceConfig(),
		TimeCurveSchedule:                GetDefaultTimeCurveScheduleConfig(),
		SmartControl:                     GetDefaultSmartControlConfig(defaultCurve),
		LightStrip:                       GetDefaultLightStripConfig(),
		LegionFnQ:                        GetDefaultLegionFnQConfig(),
		LegionFnQSupport:                 LegionFnQSupportCache{},
	}
}

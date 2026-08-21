package device

import (
	"fmt"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	lightSpeedFast   byte = 0x05
	lightSpeedMedium byte = 0x0A
	lightSpeedSlow   byte = 0x0F

	lightLEDCount         = 6
	lightKeyframeCount    = 10
	lightUploadFrameCount = 31
	lightUploadFrameSize  = 10
	lightColorDataOffset  = 6

	smartLightWarmTemperature = 70
	smartLightHotTemperature  = 80
	smartLightHysteresis      = 2
)

// lightProgram mirrors the firmware upload buffer. Bytes 0..5 are the header:
// [flags0, flags1, firstKeyframe, lastKeyframe, transitionTicks, brightness].
// The color area is LED-major: 6 LEDs * 10 keyframes * RGB.
type lightProgram [lightUploadFrameCount][lightUploadFrameSize]byte

type lightCommand struct {
	command byte
	payload []byte
}

// SetLightStrip 设置灯带模式
func (m *Manager) SetLightStrip(cfg types.LightStripConfig) error {
	if m.IsBS1() {
		return fmt.Errorf("BS1 不支持灯带设置")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}
	if err := m.setLightStripLocked(cfg); err != nil {
		return err
	}
	m.rememberLightConfigLocked(cfg)
	return nil
}

func (m *Manager) rememberLightConfigLocked(cfg types.LightStripConfig) {
	m.lightConfig = cfg
	m.hasLightConfig = true
	if cfg.Mode == "smart_temp" {
		m.smartLightPreset = 1
		m.hasSmartLightPreset = true
	} else {
		m.hasSmartLightPreset = false
	}
}

func (m *Manager) setLightStripLocked(cfg types.LightStripConfig) error {
	brightness := clampLightBrightness(cfg.Brightness)
	speed := parseLightSpeed(cfg.Speed)

	switch cfg.Mode {
	case "off":
		return m.setRGBOffLocked()
	case "smart_temp":
		return m.setLightSmartTempLocked()
	case "static_single":
		color := firstOrDefaultColor(cfg.Colors)
		return m.setLightStaticSingleLocked(color, brightness)
	case "static_multi":
		colors := toThreeColors(cfg.Colors)
		return m.setLightStaticMultiLocked(colors, brightness)
	case "rotation":
		colors := ensureMinColors(cfg.Colors, 1)
		return m.setLightRotationLocked(colors, speed, brightness)
	case "flowing":
		return m.setLightFlowingLocked(speed, brightness)
	case "breathing":
		colors := ensureMinColors(cfg.Colors, 1)
		return m.setLightBreathingLocked(colors, speed, brightness)
	default:
		return fmt.Errorf("未知灯带模式: %s", cfg.Mode)
	}
}

// SetRGBOff 关闭RGB灯光
func (m *Manager) SetRGBOff() bool {
	if m.IsBS1() {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	err := m.setRGBOffLocked()
	return err == nil
}

func (m *Manager) setRGBOffLocked() error {
	return m.sendLightCommandLocked(deviceproto.CmdRGBEnable, 0x00)
}

func clampLightBrightness(value int) byte {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return byte(value)
}

func parseLightSpeed(speed string) byte {
	switch speed {
	case "fast":
		return lightSpeedFast
	case "slow":
		return lightSpeedSlow
	default:
		return lightSpeedMedium
	}
}

func firstOrDefaultColor(colors []types.RGBColor) types.RGBColor {
	if len(colors) == 0 {
		return types.RGBColor{R: 255, G: 255, B: 255}
	}
	return colors[0]
}

func toThreeColors(colors []types.RGBColor) [3]types.RGBColor {
	base := [3]types.RGBColor{
		{R: 255, G: 0, B: 0},
		{R: 0, G: 255, B: 0},
		{R: 0, G: 128, B: 255},
	}
	for i := 0; i < len(base) && i < len(colors); i++ {
		base[i] = colors[i]
	}
	return base
}

func ensureMinColors(colors []types.RGBColor, min int) []types.RGBColor {
	if len(colors) >= min {
		return colors
	}
	defaults := []types.RGBColor{{R: 255, G: 0, B: 0}, {R: 0, G: 255, B: 0}, {R: 0, G: 128, B: 255}}
	result := make([]types.RGBColor, 0, min)
	result = append(result, colors...)
	for len(result) < min {
		result = append(result, defaults[(len(result))%len(defaults)])
	}
	return result
}

// sendLightCommandLocked sends one RGB write and waits for its firmware ACK.
// BuildFrame derives LEN from payload, so callers cannot accidentally repeat
// the old captures' invalid/redundant length variants.
func (m *Manager) sendLightCommandLocked(command byte, payload ...byte) error {
	frame, err := m.sendHIDCommandAndWaitLocked(command, payload, hidLightReportLen, deviceResponseTimeout)
	if err != nil {
		return err
	}
	return validateACK(frame, 1)
}

func newLightProgram(lastKeyframe, speed, brightness byte) lightProgram {
	var program lightProgram
	program[0] = [lightUploadFrameSize]byte{0x00, 0x02, 0x00, lastKeyframe, speed, brightness}
	return program
}

func (p *lightProgram) setColor(led, keyframe int, color types.RGBColor) {
	if led < 0 || led >= lightLEDCount || keyframe < 0 || keyframe >= lightKeyframeCount {
		return
	}
	offset := lightColorDataOffset + led*lightKeyframeCount*3 + keyframe*3
	for i, value := range [...]byte{color.R, color.G, color.B} {
		position := offset + i
		p[position/lightUploadFrameSize][position%lightUploadFrameSize] = value
	}
}

func (m *Manager) applyLightFramesLocked(program lightProgram) error {
	// Firmware flow: disable/apply the current LED state, enable the uploader,
	// initialize the frame buffer, write frame 0..30, then commit. 0x45 is only
	// a status query and must not be mixed into this write transaction.
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBEnable, 0x00); err != nil {
		return err
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBEnable, 0x01); err != nil {
		return err
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBUploadInit); err != nil {
		return err
	}

	for i := range program {
		framePayload := append([]byte{byte(i)}, program[i][:]...)
		if err := m.sendLightCommandLocked(deviceproto.CmdRGBFrameWrite, framePayload...); err != nil {
			return err
		}
	}

	return m.sendLightCommandLocked(deviceproto.CmdRGBCommit, 0x01)
}

func (m *Manager) setLightStaticSingleLocked(color types.RGBColor, brightness byte) error {
	program := newLightProgram(0, lightSpeedMedium, brightness)
	for led := range lightLEDCount {
		program.setColor(led, 0, color)
	}
	return m.applyLightFramesLocked(program)
}

func (m *Manager) setLightStaticMultiLocked(colors [3]types.RGBColor, brightness byte) error {
	program := newLightProgram(0, lightSpeedMedium, brightness)
	for led := range lightLEDCount {
		program.setColor(led, 0, colors[led%len(colors)])
	}
	return m.applyLightFramesLocked(program)
}

func (m *Manager) setLightRotationLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) < 1 {
		return fmt.Errorf("旋转需要至少 1 个颜色")
	}
	if len(colors) > 6 {
		colors = colors[:6]
	}

	program := newLightProgram(lightLEDCount-1, speed, brightness)
	numColors := len(colors)
	for led := range lightLEDCount {
		for keyframe := range lightLEDCount {
			colorIndex := (led + keyframe) % lightLEDCount
			if colorIndex < numColors {
				program.setColor(led, keyframe, colors[colorIndex])
			}
		}
	}
	return m.applyLightFramesLocked(program)
}

func (m *Manager) setLightFlowingLocked(speed, brightness byte) error {
	palette := [...]types.RGBColor{
		{R: 255, G: 0, B: 0},
		{R: 255, G: 255, B: 0},
		{R: 0, G: 255, B: 0},
		{R: 0, G: 255, B: 255},
		{R: 0, G: 0, B: 255},
		{R: 255, G: 0, B: 255},
	}
	program := newLightProgram(byte(len(palette)-1), speed, brightness)
	for led := range lightLEDCount {
		for keyframe := range len(palette) {
			program.setColor(led, keyframe, palette[(led+keyframe)%len(palette)])
		}
	}
	return m.applyLightFramesLocked(program)
}

func (m *Manager) setLightBreathingLocked(colors []types.RGBColor, speed, brightness byte) error {
	if len(colors) == 0 {
		return fmt.Errorf("颜色列表不能为空")
	}
	if len(colors) > 5 {
		colors = colors[:5]
	}

	lastKeyframe := byte(len(colors)*2 - 1)
	program := newLightProgram(lastKeyframe, speed, brightness)
	for led := range lightLEDCount {
		for i, color := range colors {
			// The following odd keyframe is left black. Firmware interpolation
			// therefore fades each selected color to black before the next color.
			program.setColor(led, i*2, color)
		}
	}
	return m.applyLightFramesLocked(program)
}

func (m *Manager) setLightSmartTempLocked() error {
	// The computer selects firmware presets 1..3 as its temperature changes.
	// A custom upload leaves its animation active. Tear that state down before
	// selecting a native preset; 0x44 applies its generated table immediately,
	// so following it with the custom-buffer commit command 0x43 is incorrect.
	for _, step := range smartLightActivationSequence(1) {
		if err := m.sendLightCommandLocked(step.command, step.payload...); err != nil {
			return err
		}
	}
	return nil
}

func smartLightActivationSequence(preset byte) []lightCommand {
	return []lightCommand{
		{command: deviceproto.CmdRGBEnable, payload: []byte{0x00}},
		{command: deviceproto.CmdRGBDynamicParam, payload: []byte{0x00}},
		{command: deviceproto.CmdRGBEnable, payload: []byte{0x01}},
		{command: deviceproto.CmdRGBDynamicParam, payload: []byte{preset}},
	}
}

// UpdateSmartTemperatureLight switches among the firmware's green, yellow and
// red native animations according to the highest current computer temperature.
// It sends nothing while another light mode is active or while the temperature
// remains in the same hysteresis band.
func (m *Manager) UpdateSmartTemperatureLight(temperature int) error {
	if m.IsBS1() || temperature <= 0 {
		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}
	if !m.hasLightConfig || m.lightConfig.Mode != "smart_temp" {
		return nil
	}

	current := byte(0)
	if m.hasSmartLightPreset {
		current = m.smartLightPreset
	}
	next := smartLightPresetForTemperature(temperature, current)
	if next == 0 || (m.hasSmartLightPreset && next == m.smartLightPreset) {
		return nil
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBDynamicParam, next); err != nil {
		return err
	}
	m.smartLightPreset = next
	m.hasSmartLightPreset = true
	m.logDebug("智能温控灯效切换: %d°C -> 固件预设 %d", temperature, next)
	return nil
}

func smartLightPresetForTemperature(temperature int, current byte) byte {
	if temperature <= 0 {
		return 0
	}

	// The first three 0x44 presets are visibly ordered green, yellow and red in
	// the firmware's generated RGB tables. Keep the UI's existing 70/80°C bands
	// and add 2°C hysteresis so sensor noise cannot toggle animations each tick.
	switch current {
	case 1:
		if temperature > smartLightHotTemperature+smartLightHysteresis {
			return 3
		}
		if temperature > smartLightWarmTemperature+smartLightHysteresis {
			return 2
		}
		return 1
	case 2:
		if temperature > smartLightHotTemperature+smartLightHysteresis {
			return 3
		}
		if temperature < smartLightWarmTemperature-smartLightHysteresis {
			return 1
		}
		return 2
	case 3:
		if temperature < smartLightWarmTemperature-smartLightHysteresis {
			return 1
		}
		if temperature < smartLightHotTemperature-smartLightHysteresis {
			return 2
		}
		return 3
	default:
		if temperature > smartLightHotTemperature {
			return 3
		}
		if temperature > smartLightWarmTemperature {
			return 2
		}
		return 1
	}
}

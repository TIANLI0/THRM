package device

import (
	"fmt"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	lightSpeedFast   byte = 0x05
	lightSpeedMedium byte = 0x0A
	lightSpeedSlow   byte = 0x0F

	lightLEDCount        = 6
	lightKeyframeCount   = 10
	lightUploadFrameSize = 10
	lightColorDataOffset = 6

	// lightProgramSize 是固件真正会读回的程序字节数：6 字节头部，加上
	// 灯珠主序的颜色区（6 颗灯 × 10 关键帧 × RGB）。重排循环取到的最大偏移是
	// 6 + 5*30 + 9*3 = 183，所以 184 字节就覆盖了全部内容。
	lightProgramSize = lightColorDataOffset + lightLEDCount*lightKeyframeCount*3

	// lightUploadFrameCount 是需要下发的 0x47 帧数。
	//
	// 这里必须比协议文档里的 31 帧少。固件的灯效缓冲区容量写在流状态结构
	// （RAM 0x20002604）的第四个字里，实测镜像值为 0x100 = 256 字节；而 0x47 的
	// 固件分支是 copy(*stream + index*10, payload+1, 10)，既不检查索引上界也不检查
	// 容量——只有分块写 0x42 才比较 stream[1] + len <= stream[3]。下发 31 帧就是
	// 310 字节，会越过缓冲区末尾 54 字节，直接写进紧随其后的关键帧渲染表
	// （0x20004AC8）。按 184 字节取整到帧边界只需 19 帧，既落在容量内，
	// 又完整覆盖固件会读的每一个字节，还省掉 12 次命令往返。
	lightUploadFrameCount = (lightProgramSize + lightUploadFrameSize - 1) / lightUploadFrameSize
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
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// BS1 的连接由 bleManager 持有句柄，m.device 始终为 nil，不能用它判连接。
	// 注意这里读 deviceType 而不是调 IsBS1()：后者会拿 m.mutex.RLock，
	// 而本函数已持有写锁，RWMutex 不可重入，调用会直接自死锁。
	if !m.isConnected || (m.device == nil && m.deviceType != types.DeviceTypeBLE) {
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
	if cfg.Mode != "smart_temp" {
		m.hasSmartLightPreset = false
		m.smartLightBandIndex = -1
		m.smartLightTemperature = 0
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
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || (m.device == nil && m.deviceType != types.DeviceTypeBLE) {
		return false
	}

	err := m.setRGBOffLocked()
	return err == nil
}

func (m *Manager) setRGBOffLocked() error {
	return m.setRGBEnableLocked(false)
}

// setRGBEnableLocked 下发 0x46，但在设备侧已知就是目标值时跳过。
//
// 固件的 0x46 分支无条件调用配置落盘（擦除并重写一页 256 字节数据闪存），
// 而 0x0C/0x0D 都会先比较新旧值再落盘——0x46/0x48 少了这个判断。原来每次应用
// 灯效都要先关再开，等于凭空产生两次闪存擦写；重连重放时更是每次都重复一遍。
// 缓存设备侧的当前值并跳过重复写入，是主机端唯一能做的缓解。
func (m *Manager) setRGBEnableLocked(enabled bool) error {
	if m.hasRGBEnabled && m.rgbEnabled == enabled {
		m.logDebug("灯效开关已是 %t，跳过一次固件闪存写入", enabled)
		return nil
	}
	payload := byte(0x00)
	if enabled {
		payload = 0x01
	}
	m.awaitFlashWriteWindowLocked()
	err := m.sendLightCommandLocked(deviceproto.CmdRGBEnable, payload)
	m.noteFlashWriteLocked()
	if err != nil {
		// 写入结果未知，缓存不再可信。
		m.hasRGBEnabled = false
		return err
	}
	m.rgbEnabled = enabled
	m.hasRGBEnabled = true
	return nil
}

// NoteRGBEnabledFromDevice 用 0x45 读回的状态播种缓存，使重连后的首次重放
// 在设备本来就处于目标状态时不必写闪存。
func (m *Manager) NoteRGBEnabledFromDevice(enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.rgbEnabled = enabled
	m.hasRGBEnabled = true
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
// lightFrameRetries 是单条灯效命令的重发次数。
//
// 实测（BS1，2026-11-06）：连续上传时约每两三次就会漏掉某一帧 0x47 的 ACK，
// 单帧超时会让整次上传失败，用户看到的是"设置灯带失败: 等待命令 0x47 响应超时"。
// 帧载荷带序号且内容幂等，重发不会污染缓冲区。
const lightFrameRetries = 2

// lightCommitTimeout 是提交命令 0x43 的专用等待时间。
//
// 0x43 会让固件擦写数据闪存，应答明显慢于普通命令；默认的 900ms 实测经常不够，
// 现象是"设置灯带失败: 等待命令 0x43 响应超时"，但灯效其实已经生效——属于假失败。
// 提交内容幂等（同一份缓冲区），重发一次不会产生副作用。
const lightCommitTimeout = 2500 * time.Millisecond

// sendLightCommandWithTimeoutLocked 按指定超时下发一条灯效命令。
func (m *Manager) sendLightCommandWithTimeoutLocked(timeout time.Duration, command byte, payload ...byte) error {
	// BS1 的 HID 句柄由 bleManager 的传输层持有，命令必须走那条通道。
	// 直接读 deviceType 而不是调 IsBS1()：调用方已持有 m.mutex 写锁。
	if m.deviceType == types.DeviceTypeBLE {
		frame, err := m.bleManager.sendBLECommandAndWait(command, payload, timeout)
		if err != nil {
			return err
		}
		return validateACK(frame, 1)
	}
	frame, err := m.sendHIDCommandAndWaitLocked(command, payload, hidLightReportLen, timeout)
	if err != nil {
		return err
	}
	return validateACK(frame, 1)
}

// sendLightCommandRetryLocked 下发一条灯效命令，失败时按指定超时重发。
func (m *Manager) sendLightCommandRetryLocked(retries int, timeout time.Duration, command byte, payload ...byte) error {
	var err error
	for attempt := 0; attempt <= retries; attempt++ {
		err = m.sendLightCommandWithTimeoutLocked(timeout, command, payload...)
		if err == nil {
			return nil
		}
		if attempt < retries {
			m.logDebug("灯效命令 0x%02X 第 %d 次下发失败，重发: %v", command, attempt+1, err)
		}
	}
	return err
}

func (m *Manager) sendLightCommandLocked(command byte, payload ...byte) error {
	return m.sendLightCommandWithTimeoutLocked(deviceResponseTimeout, command, payload...)
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
	// Firmware flow: make sure the LED output is on, initialize the frame
	// buffer, write frame 0..30, then commit. 0x45 is only a status query and
	// must not be mixed into this write transaction.
	//
	// The old sequence toggled 0x46 off then on before every upload. The
	// firmware persists the enable flag to data flash on each 0x46, so that
	// toggle cost two flash erase/program cycles per light change for no
	// protocol benefit: 0x41 only needs the stream buffer, not a fresh enable.
	defer m.beginTransaction()()

	if err := m.setRGBEnableLocked(true); err != nil {
		return err
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBUploadInit); err != nil {
		return err
	}

	for i := range program {
		framePayload := append([]byte{byte(i)}, program[i][:]...)
		if err := m.sendLightCommandRetryLocked(lightFrameRetries, deviceResponseTimeout,
			deviceproto.CmdRGBFrameWrite, framePayload...); err != nil {
			return err
		}
	}

	// 0x43 makes the firmware queue its own flash write of the frame buffer.
	// That is one erase/program cycle per applied custom effect and is the
	// reason smart-temperature lighting uses native presets instead of
	// re-uploading a program whenever the temperature crosses a band.
	//
	// 它的落盘发生在 ACK 之后（TMOS 事件 0x20），所以既要与前一条落盘命令拉开，
	// 也要记时，好让紧随其后的 0x0C/0x48 不会撞进这次异步擦写。
	m.awaitFlashWriteWindowLocked()
	err := m.sendLightCommandRetryLocked(1, lightCommitTimeout, deviceproto.CmdRGBCommit, 0x01)
	m.noteFlashWriteLocked()
	return err
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
	// The computer selects a firmware preset as its temperature changes.
	// A custom upload leaves its animation active. Tear that state down before
	// selecting a native preset; 0x44 applies its generated table immediately,
	// so following it with the custom-buffer commit command 0x43 is incorrect.
	defer m.beginTransaction()()

	preset := byte(types.SmartTempLightMinPreset)
	if bands, _ := types.NormalizeSmartTempLightBands(m.lightConfig.SmartTempBands); len(bands) > 0 {
		preset = byte(bands[0].Preset)
	}
	for _, step := range smartLightActivationSequence(preset) {
		if step.command == deviceproto.CmdRGBEnable {
			if err := m.setRGBEnableLocked(step.payload[0] != 0); err != nil {
				return err
			}
			continue
		}
		if err := m.sendLightCommandLocked(step.command, step.payload...); err != nil {
			return err
		}
	}
	m.smartLightPreset = preset
	m.hasSmartLightPreset = true
	m.smartLightBandIndex = 0
	return nil
}

// smartLightActivationSequence 是从自定义帧灯效切回原生预设的序列。
//
// 0x46 由 setRGBEnableLocked 消费，已经处于目标状态时会被跳过；0x44 只改运行期
// 状态，不写闪存，可以放心每次都发。先发 0x44 00 是为了停掉上一套自定义动画。
func smartLightActivationSequence(preset byte) []lightCommand {
	return []lightCommand{
		{command: deviceproto.CmdRGBDynamicParam, payload: []byte{0x00}},
		{command: deviceproto.CmdRGBEnable, payload: []byte{0x01}},
		{command: deviceproto.CmdRGBDynamicParam, payload: []byte{preset}},
	}
}

// UpdateSmartTemperatureLight switches among the firmware's native animations
// according to the highest current computer temperature and the user's
// configured temperature bands. It sends nothing while another light mode is
// active or while the temperature stays inside the current band's hysteresis.
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

	bands, _ := types.NormalizeSmartTempLightBands(m.lightConfig.SmartTempBands)
	if len(bands) == 0 {
		return nil
	}
	current := -1
	if m.hasSmartLightPreset {
		current = m.smartLightBandIndex
	}
	index := types.SelectSmartTempLightBand(bands, m.lightConfig.SmartTempHysteresis, temperature, current)
	if index < 0 || index >= len(bands) {
		return nil
	}
	m.smartLightTemperature = temperature

	next := byte(bands[index].Preset)
	if m.hasSmartLightPreset && index == m.smartLightBandIndex && next == m.smartLightPreset {
		return nil
	}
	if err := m.sendLightCommandLocked(deviceproto.CmdRGBDynamicParam, next); err != nil {
		return err
	}
	m.smartLightPreset = next
	m.smartLightBandIndex = index
	m.hasSmartLightPreset = true
	m.logDebug("智能温控灯效切换: %d°C -> 区间 %d (固件预设 %d)", temperature, index+1, next)
	return nil
}

// SmartTempLightStatus 返回智能温控灯效当前落在哪一段，供界面实时展示。
func (m *Manager) SmartTempLightStatus() types.SmartTempLightStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := types.SmartTempLightStatus{BandIndex: -1}
	if !m.hasLightConfig || m.lightConfig.Mode != "smart_temp" || !m.hasSmartLightPreset {
		return status
	}
	status.Active = true
	status.BandIndex = m.smartLightBandIndex
	status.Preset = int(m.smartLightPreset)
	status.Temperature = m.smartLightTemperature
	return status
}

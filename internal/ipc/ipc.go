// Package ipc 提供核心服务与 GUI 之间的进程间通信
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/types"
)

var messageCounter uint64

const (
	// PipeName 命名管道名称
	PipeName = appmeta.IPCPipeName
	// LegacyPipeName 旧版本命名管道名称
	LegacyPipeName = appmeta.LegacyIPCPipeName
	// PipePath 命名管道完整路径
	PipePath = `\\.\pipe\` + PipeName
	// LegacyPipePath 旧版本命名管道完整路径
	LegacyPipePath = `\\.\pipe\` + LegacyPipeName
	// EnvPipeName allows tests and diagnostics to isolate IPC from a running Core.
	EnvPipeName = "THRM_IPC_PIPE_NAME"
)

func activePipeName() string {
	if name := strings.TrimSpace(os.Getenv(EnvPipeName)); name != "" {
		return name
	}
	return PipeName
}

func ipcPipeCandidates() []string {
	if name := strings.TrimSpace(os.Getenv(EnvPipeName)); name != "" {
		return []string{name}
	}
	return appmeta.IPCPipeCandidates()
}

// RequestType 请求类型
type RequestType string

const (
	// 设备相关
	ReqConnect               RequestType = "Connect"
	ReqDisconnect            RequestType = "Disconnect"
	ReqGetDeviceStatus       RequestType = "GetDeviceStatus"
	ReqGetCurrentFanData     RequestType = "GetCurrentFanData"
	ReqRefreshDeviceSettings RequestType = "RefreshDeviceSettings"
	ReqReinitializeFirmware  RequestType = "ReinitializeFirmware"

	// 配置相关
	ReqGetConfig                RequestType = "GetConfig"
	ReqUpdateConfig             RequestType = "UpdateConfig"
	ReqSetFanCurve              RequestType = "SetFanCurve"
	ReqGetFanCurve              RequestType = "GetFanCurve"
	ReqGetFanCurveProfiles      RequestType = "GetFanCurveProfiles"
	ReqSetActiveFanCurveProfile RequestType = "SetActiveFanCurveProfile"
	ReqSaveFanCurveProfile      RequestType = "SaveFanCurveProfile"
	ReqDeleteFanCurveProfile    RequestType = "DeleteFanCurveProfile"
	ReqExportFanCurveProfiles   RequestType = "ExportFanCurveProfiles"
	ReqImportFanCurveProfiles   RequestType = "ImportFanCurveProfiles"
	ReqResetLearnedOffsets      RequestType = "ResetLearnedOffsets"
	ReqGetCoolingBenefit        RequestType = "GetCoolingBenefit"
	ReqSaveCoolingBenefitReport RequestType = "SaveCoolingBenefitReport"
	ReqClearCoolingBenefit      RequestType = "ClearCoolingBenefit"
	ReqSetExtendedSensors       RequestType = "SetExtendedSensors"
	ReqExportCoolingBenefitText RequestType = "ExportCoolingBenefitText"
	ReqPreviewRTSSPosition      RequestType = "PreviewRTSSPosition"
	// ReqSetUILanguage 上报 GUI 当前界面语言（SetStringParams），核心据此本地化托盘与通知。
	ReqSetUILanguage RequestType = "SetUILanguage"

	// 飞智空间站兼容
	ReqGetFlydigiCompatStatus RequestType = "GetFlydigiCompatStatus"
	ReqSetFlydigiCompat       RequestType = "SetFlydigiCompat"

	// 控制相关
	ReqSetAutoControl      RequestType = "SetAutoControl"
	ReqSetManualGear       RequestType = "SetManualGear"
	ReqGetAvailableGears   RequestType = "GetAvailableGears"
	ReqSetCustomSpeed      RequestType = "SetCustomSpeed"
	ReqSetGearLight        RequestType = "SetGearLight"
	ReqSetPowerOnStart     RequestType = "SetPowerOnStart"
	ReqSetSmartStartStop   RequestType = "SetSmartStartStop"
	ReqSetBrightness       RequestType = "SetBrightness"
	ReqSetLightStrip       RequestType = "SetLightStrip"
	ReqGetSmartLightStatus RequestType = "GetSmartLightStatus"

	// 固件更新
	ReqGetFirmwareUpdateStatus RequestType = "GetFirmwareUpdateStatus"
	ReqListFirmwareUpdates     RequestType = "ListFirmwareUpdates"
	ReqInstallFirmwareDriver   RequestType = "InstallFirmwareDriver"
	ReqUninstallFirmwareDriver RequestType = "UninstallFirmwareDriver"
	ReqFlashFirmware           RequestType = "FlashFirmware"

	// 温度相关
	ReqGetTemperature                      RequestType = "GetTemperature"
	ReqGetTemperatureHistory               RequestType = "GetTemperatureHistory"
	ReqSetTemperatureHistoryEnabled        RequestType = "SetTemperatureHistoryEnabled"
	ReqSetTemperatureHistoryRetentionHours RequestType = "SetTemperatureHistoryRetentionHours"
	ReqTestTemperatureReading              RequestType = "TestTemperatureReading"
	ReqTestBridgeProgram                   RequestType = "TestBridgeProgram"
	ReqGetBridgeProgramStatus              RequestType = "GetBridgeProgramStatus"
	ReqRestartPawnIO                       RequestType = "RestartPawnIO"
	ReqReinstallPawnIO                     RequestType = "ReinstallPawnIO"

	// 自启动相关
	ReqSetWindowsAutoStart    RequestType = "SetWindowsAutoStart"
	ReqCheckWindowsAutoStart  RequestType = "CheckWindowsAutoStart"
	ReqIsRunningAsAdmin       RequestType = "IsRunningAsAdmin"
	ReqGetAutoStartMethod     RequestType = "GetAutoStartMethod"
	ReqSetAutoStartWithMethod RequestType = "SetAutoStartWithMethod"

	// 窗口相关
	ReqShowWindow RequestType = "ShowWindow"
	ReqHideWindow RequestType = "HideWindow"
	ReqQuitApp    RequestType = "QuitApp"

	// 调试相关
	ReqGetDebugInfo           RequestType = "GetDebugInfo"
	ReqSetDebugMode           RequestType = "SetDebugMode"
	ReqSendDeviceDebugCommand RequestType = "SendDeviceDebugCommand"
	ReqGetDeviceDebugFrames   RequestType = "GetDeviceDebugFrames"
	ReqUpdateGuiResponseTime  RequestType = "UpdateGuiResponseTime"

	// 系统相关
	ReqPing              RequestType = "Ping"
	ReqIsAutoStartLaunch RequestType = "IsAutoStartLaunch"
	ReqSubscribeEvents   RequestType = "SubscribeEvents"
	ReqUnsubscribeEvents RequestType = "UnsubscribeEvents"
)

// Request IPC 请求
type Request struct {
	ProtocolVersion string          `json:"protocolVersion,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	Timestamp       int64           `json:"timestamp,omitempty"`
	Type            RequestType     `json:"type"`
	Data            json.RawMessage `json:"data,omitempty"`
}

// Response IPC 响应
type Response struct {
	ProtocolVersion string          `json:"protocolVersion,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	Timestamp       int64           `json:"timestamp,omitempty"`
	IsResponse      bool            `json:"isResponse"` // 标识这是响应而非事件
	Success         bool            `json:"success"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	Error           string          `json:"error,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
}

// Event IPC 事件（服务器推送给客户端）
type Event struct {
	SchemaVersion string          `json:"schemaVersion,omitempty"`
	EventID       string          `json:"eventId,omitempty"`
	Timestamp     int64           `json:"timestamp,omitempty"`
	Source        string          `json:"source,omitempty"`
	IsEvent       bool            `json:"isEvent"` // 标识这是事件
	Type          string          `json:"type"`
	Data          json.RawMessage `json:"data,omitempty"`
}

// EventType 事件类型
const (
	EventFanDataUpdate            = "fan-data-update"
	EventTemperatureUpdate        = "temperature-update"
	EventTemperatureHistoryUpdate = "temperature-history-update"
	EventDeviceConnected          = "device-connected"
	EventDeviceDisconnected       = "device-disconnected"
	EventDeviceError              = "device-error"
	EventDeviceSettingsUpdate     = "device-settings-update"
	EventConfigUpdate             = "config-update"
	// EventSmartLightUpdate 在智能温控灯效切换温度区间时下发，让界面能实时
	// 显示"当前落在哪一段、用的是哪个固件预设"。
	EventSmartLightUpdate       = "smart-light-update"
	EventHotkeyTriggered        = "hotkey-triggered"
	EventLegionPowerModeUpdate  = "legion-power-mode-update"
	EventLegionFnQSupportUpdate = "legion-fnq-support-update"
	EventFlydigiCompatUpdate    = "flydigi-compat-update"
	EventHealthPing             = "health-ping"
	EventHeartbeat              = "heartbeat"
	EventTimelineEvent          = "timeline-event"
	EventFirmwareUpdate         = "firmware-update" // 固件刷写进度/日志/完成
)

// Server IPC 服务器
type Server struct {
	listener      net.Listener
	clients       map[net.Conn]*clientState
	mutex         sync.RWMutex
	handler       RequestHandler
	logger        types.Logger
	running       bool
	throttleMutex sync.Mutex
	lastEventEmit map[string]time.Time
	// activeCh 在出现新的前台客户端（新连接或重新订阅）时收到一次信号，
	// 让后台低频采样立即恢复，而不是等完一个空闲采样周期。
	activeCh chan struct{}
}

type clientState struct {
	conn net.Conn
	// writeCh 承载事件，respCh 承载请求响应。两者分开是因为优先级不同：
	// 事件拥塞时可以丢弃，响应一旦丢弃就会让对端等到超时，因此响应必须优先写出
	// 且不参与"队列满即丢"的策略。
	writeCh   chan []byte
	respCh    chan []byte
	sem       chan struct{}
	closeOnce sync.Once
	closed    chan struct{}
	// subscribed 表示该客户端当前需要高频遥测（风扇、温度、温度历史）。连上即为
	// true，兼容从不发订阅请求的旧客户端；GUI 窗口隐藏时退订，核心据此回到后台模式。
	subscribed atomic.Bool
	// features 是客户端在 SubscribeEvents 里声明的协议能力位，见 clientFeatureBits。
	features atomic.Uint32
}

// clientFeatureBits 把订阅请求里的能力名映射为位。未知能力直接忽略，新旧版本互不影响。
var clientFeatureBits = map[string]uint32{
	FeatureTemperatureSensorValues: 1 << 0,
}

// SubscribeEventsParams 是 SubscribeEvents 请求的参数，可省略。
type SubscribeEventsParams struct {
	Features []string `json:"features,omitempty"`
}

const clientWriteQueueSize = 64
const clientResponseQueueSize = 64
const defaultRequestTimeout = 15 * time.Second
const serverCriticalEventEnqueueTimeout = 500 * time.Millisecond

// maxConcurrentRequestsPerClient 限制单个连接上并发执行的请求处理数，
// 既避免慢请求相互阻塞，也避免异常对端灌请求导致协程无上限增长。
const maxConcurrentRequestsPerClient = 16

// clientWriteTimeout 是单次写操作的上限。
//
// 没有这个超时，对端进程假死（webview 卡死、进程被挂起、调试器断住）时
// conn.Write 会永久不返回：写协程再也不会退出，客户端条目永远留在 s.clients 里，
// HasClients() 恒为 true，于是核心永远按"有 GUI 在线"全速采样并广播事件，
// 且再也不会归还空闲内存——这是一个只能靠重启核心才能解除的资源泄漏。
//
// 声明为变量以便测试缩短等待。
var clientWriteTimeout = 10 * time.Second

// maxRequestFrameSize 是服务端单条请求帧（含换行）的上限。
//
// 最大的合法请求是携带完整配置的 UpdateConfig，只有几十 KiB；4 MiB 留足余量。
// 没有上限时，本机任意进程连上管道后只发字节不发换行，就能让以管理员身份运行的
// 核心无限制地累积内存。超限按协议错误处理，直接断开该连接。
const maxRequestFrameSize = 4 << 20

var ErrRequestTimeout = errors.New("等待 IPC 响应超时")

// slowRequestWarnInterval 是慢请求看门狗的告警间隔。客户端 12 秒就会放弃，
// 所以第一次告警必须明显早于它，否则日志还没写出来客户端就已经报超时了。
const slowRequestWarnInterval = 5 * time.Second

// ErrConnectionClosed 表示请求所在的连接在收到响应前已断开，响应不可能再到达。
var ErrConnectionClosed = errors.New("IPC 连接已断开")

var errFrameTooLarge = errors.New("IPC 请求帧超过大小上限")

// RequestHandler 请求处理函数类型
type RequestHandler func(req Request) Response

func newMessageID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), atomic.AddUint64(&messageCounter, 1))
}

// NewServer 创建 IPC 服务器
func NewServer(handler RequestHandler, logger types.Logger) *Server {
	return &Server{
		clients:       make(map[net.Conn]*clientState),
		handler:       handler,
		logger:        logger,
		lastEventEmit: make(map[string]time.Time),
		activeCh:      make(chan struct{}, 1),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	listener, addr, err := listenIPC()
	if err != nil {
		return err
	}

	s.mutex.Lock()
	s.listener = listener
	s.running = true
	s.mutex.Unlock()
	s.logInfo("IPC 服务器已启动: %s", addr)

	// 接受连接
	go s.acceptConnections()

	return nil
}

// acceptConnections 接受客户端连接
func (s *Server) acceptConnections() {
	consecutiveFailures := 0
	for s.isRunning() {
		s.mutex.RLock()
		listener := s.listener
		s.mutex.RUnlock()
		if listener == nil {
			return
		}
		conn, err := listener.Accept()
		if err != nil {
			if !s.isRunning() {
				return
			}
			// 监听器持续故障时退避重试，避免热循环空转占满 CPU 并刷爆日志。
			consecutiveFailures++
			s.logError("接受连接失败（连续第 %d 次）: %v", consecutiveFailures, err)
			backoff := min(time.Duration(consecutiveFailures*100)*time.Millisecond, 3*time.Second)
			time.Sleep(backoff)
			continue
		}
		consecutiveFailures = 0

		state := &clientState{
			conn:    conn,
			writeCh: make(chan []byte, clientWriteQueueSize),
			respCh:  make(chan []byte, clientResponseQueueSize),
			sem:     make(chan struct{}, maxConcurrentRequestsPerClient),
			closed:  make(chan struct{}),
		}
		state.subscribed.Store(true)

		s.mutex.Lock()
		s.clients[conn] = state
		s.mutex.Unlock()
		s.signalActiveClient()

		s.logInfo("新的 IPC 客户端已连接")

		go s.clientWriter(state)
		go s.handleClient(conn, state)
	}
}

func (s *Server) isRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// clientWriter 是该连接唯一的写入者。响应优先于事件：请求-响应时延直接决定
// 对端是否判定超时，而事件迟到只影响界面刷新。
func (s *Server) clientWriter(state *clientState) {
	for {
		// 先把已就绪的响应排空，再考虑事件。
		select {
		case data := <-state.respCh:
			if !s.writeToClient(state, data) {
				return
			}
			continue
		case <-state.closed:
			return
		default:
		}

		select {
		case data := <-state.respCh:
			if !s.writeToClient(state, data) {
				return
			}
		case data, ok := <-state.writeCh:
			if !ok {
				return
			}
			if !s.writeToClient(state, data) {
				return
			}
		case <-state.closed:
			return
		}
	}
}

// writeToClient 带超时地写出一帧数据。写超时按连接故障处理：对端假死时必须
// 主动断开并从 clients 中摘除，否则核心会永远认为 GUI 仍在线。
func (s *Server) writeToClient(state *clientState, data []byte) bool {
	if err := state.conn.SetWriteDeadline(time.Now().Add(clientWriteTimeout)); err != nil {
		// 少数传输不支持 deadline，此时退化为无超时写入而不是拒绝服务。
		s.logDebug("设置 IPC 写超时失败: %v", err)
	}
	if _, err := state.conn.Write(data); err != nil {
		s.logDebug("发送数据失败，断开该客户端: %v", err)
		s.closeClient(state)
		return false
	}
	return true
}

func (s *Server) closeClient(state *clientState) {
	state.closeOnce.Do(func() {
		close(state.closed)
		s.mutex.Lock()
		delete(s.clients, state.conn)
		s.mutex.Unlock()
		state.conn.Close()
	})
}

// handleClient 处理客户端连接。
//
// 每个请求交给独立的协程执行，读取循环立刻回到 ReadBytes 等下一条请求。
// 串行处理会造成队头阻塞：一次设备连接（就绪握手最长 6 秒）或一次桥接故障探测
// （最长 10 秒）会把同一条连接上后续所有请求一起拖过对端的超时阈值，
// 对端连续超时后便拆掉整条连接重连，表现为"核心服务不可用"的误报。
//
// 并发执行要求 handler 本身是协程安全的——这一点本来就必须成立：托盘回调、
// 全局快捷键、插件与温控循环都在各自的协程里调用同一批 CoreApp 方法，
// 仅把 IPC 这一条链路串行化从来不构成任何真实的互斥保证。
func (s *Server) handleClient(conn net.Conn, state *clientState) {
	defer func() {
		s.closeClient(state)
		s.logInfo("IPC 客户端已断开")
	}()

	reader := bufio.NewReader(conn)

	for s.isRunning() {
		line, err := readFrame(reader, maxRequestFrameSize)
		if err != nil {
			if errors.Is(err, errFrameTooLarge) {
				s.logError("IPC 请求帧超过 %d 字节上限，断开该客户端", maxRequestFrameSize)
			} else {
				s.logDebug("读取客户端请求失败: %v", err)
			}
			return
		}

		// 解析请求
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.logError("解析请求失败: %v", err)
			continue
		}
		if req.ProtocolVersion == "" {
			req.ProtocolVersion = appmeta.ProtocolVersion
		}
		if req.RequestID == "" {
			req.RequestID = newMessageID("req")
		}
		if req.Timestamp == 0 {
			req.Timestamp = time.Now().UnixMilli()
		}

		// 信号量给并发处理数设上限，避免异常对端灌请求导致协程无上限增长。
		select {
		case state.sem <- struct{}{}:
		case <-state.closed:
			return
		}

		go func(req Request) {
			defer func() { <-state.sem }()
			s.serveRequest(state, req)
		}(req)
	}
}

// readFrame 读取一条以换行结尾的帧，累计超过 limit 字节即返回 errFrameTooLarge。
// ReadSlice 每次最多交出一个缓冲区的数据，因此内存占用受 limit 约束，而不是像
// ReadBytes 那样一直增长到读到换行为止。
func readFrame(reader *bufio.Reader, limit int) ([]byte, error) {
	var frame []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(frame)+len(chunk) > limit {
			return nil, errFrameTooLarge
		}
		// ReadSlice 返回的切片在下次读取时失效，必须拷贝出来。
		frame = append(frame, chunk...)
		if err == nil {
			return frame, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

// serveRequest 执行单个请求并把响应交给写协程。
func (s *Server) serveRequest(state *clientState, req Request) {
	s.logDebug("IPC 请求[%s]: %s", req.RequestID, req.Type)

	resp, handled := s.handleSubscription(state, req)
	if !handled {
		resp = s.invokeHandler(req)
	}
	if resp.ProtocolVersion == "" {
		resp.ProtocolVersion = appmeta.ProtocolVersion
	}
	if resp.RequestID == "" {
		resp.RequestID = req.RequestID
	}
	if resp.Timestamp == 0 {
		resp.Timestamp = time.Now().UnixMilli()
	}
	resp.IsResponse = true

	respBytes, err := json.Marshal(resp)
	if err != nil {
		s.logError("序列化响应失败: %v", err)
		return
	}

	// 响应不参与"队列满即丢"：丢弃只会让对端白等一个超时。这里阻塞等待队列空位，
	// 连接一旦被判定为故障（写超时/对端断开）就会关闭 closed 从而解除阻塞。
	select {
	case state.respCh <- append(respBytes, '\n'):
	case <-state.closed:
		s.logDebug("客户端已断开，丢弃响应: requestID=%s", resp.RequestID)
	}
}

// handleSubscription 处理订阅类请求。订阅状态属于连接本身，由服务端直接维护，
// 不经过业务 handler。
func (s *Server) handleSubscription(state *clientState, req Request) (Response, bool) {
	switch req.Type {
	case ReqSubscribeEvents:
		var params SubscribeEventsParams
		if len(req.Data) > 0 {
			if err := json.Unmarshal(req.Data, &params); err != nil {
				return Response{Success: false, Error: fmt.Sprintf("解析订阅参数失败: %v", err)}, true
			}
		}
		var features uint32
		for _, name := range params.Features {
			features |= clientFeatureBits[name]
		}
		state.features.Store(features)
		if !state.subscribed.Swap(true) {
			s.logDebug("IPC 客户端恢复订阅高频事件")
			s.signalActiveClient()
		}
		return Response{Success: true}, true
	case ReqUnsubscribeEvents:
		if state.subscribed.Swap(false) {
			s.logDebug("IPC 客户端暂停订阅高频事件")
		}
		return Response{Success: true}, true
	}
	return Response{}, false
}

func (s *Server) signalActiveClient() {
	select {
	case s.activeCh <- struct{}{}:
	default:
	}
}

// invokeHandler 兜底 handler 的 panic。请求现在在独立协程里执行，
// 未捕获的 panic 会直接终止整个核心进程，因此必须在此拦下并回一条错误响应。
func (s *Server) invokeHandler(req Request) (resp Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logError("处理 IPC 请求[%s] type=%s 时发生 panic: %v", req.RequestID, req.Type, recovered)
			resp = Response{
				Success:   false,
				ErrorCode: "internal_panic",
				Error:     fmt.Sprintf("核心服务内部错误: %v", recovered),
			}
		}
	}()

	// 慢请求看门狗。客户端等 12 秒就会放弃，但服务端仍卡在原处继续持有锁——
	// 这种情况下客户端只会显示"等待 IPC 响应超时"，核心日志里一个字都没有，
	// 事后完全无法定位。这里每 5 秒记一条，直到 handler 返回为止。
	started := time.Now()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(slowRequestWarnInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.logError("IPC 请求[%s] type=%s 已运行 %v 仍未返回（疑似卡死）",
					req.RequestID, req.Type, time.Since(started).Round(time.Second))
			}
		}
	}()

	return s.handler(req)
}

var highFrequencyEventTypes = map[string]time.Duration{
	EventFanDataUpdate:            250 * time.Millisecond,
	EventTemperatureUpdate:        250 * time.Millisecond,
	EventTemperatureHistoryUpdate: 1000 * time.Millisecond,
}

func isHighFrequencyEvent(eventType string) bool {
	_, ok := highFrequencyEventTypes[eventType]
	return ok
}

func (s *Server) shouldDropEvent(eventType string) bool {
	threshold, ok := highFrequencyEventTypes[eventType]
	if !ok {
		return false
	}
	now := time.Now()
	s.throttleMutex.Lock()
	defer s.throttleMutex.Unlock()
	last, exists := s.lastEventEmit[eventType]
	if exists && now.Sub(last) < threshold {
		return true
	}
	s.lastEventEmit[eventType] = now
	return false
}

// BroadcastEvent 广播事件给客户端。
//
// 高频遥测（风扇、温度、温度历史）只发给仍在订阅的客户端：窗口隐藏的 GUI 不需要，
// 恢复显示时会重新订阅并主动拉取一次完整状态。其余事件（配置、设备连接状态、错误、
// 心跳等）照常发给全部客户端，保证 GUI 恢复显示时状态是对的，心跳也继续承担探活。
func (s *Server) BroadcastEvent(eventType string, data any) {
	highFrequency := isHighFrequencyEvent(eventType)
	if highFrequency {
		if !s.HasActiveClients() {
			return
		}
	} else if !s.HasClients() {
		return
	}

	if s.shouldDropEvent(eventType) {
		return
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		s.logError("序列化事件数据失败: %v", err)
		return
	}

	payload := appendEventFrame(make([]byte, 0, len(dataBytes)+160), newMessageID("evt"), time.Now().UnixMilli(), eventType, dataBytes)

	s.mutex.RLock()
	clients := make([]*clientState, 0, len(s.clients))
	for _, state := range s.clients {
		if highFrequency && !state.subscribed.Load() {
			continue
		}
		clients = append(clients, state)
	}
	s.mutex.RUnlock()

	for _, state := range clients {
		if highFrequency {
			select {
			case state.writeCh <- payload:
			case <-state.closed:
			default:
				s.logDebug("客户端写队列已满，丢弃高频事件: %s", eventType)
			}
			continue
		}

		timer := time.NewTimer(serverCriticalEventEnqueueTimeout)
		select {
		case state.writeCh <- payload:
			timer.Stop()
		case <-state.closed:
			timer.Stop()
		case <-timer.C:
			s.logDebug("客户端写队列持续拥塞，丢弃关键事件: %s", eventType)
		}
	}
}

// appendEventFrame 拼出一帧事件（含结尾换行），字节上与 json.Marshal(Event{...}) 完全一致。
//
// data 必须是 json.Marshal 的输出：它已经是紧凑且做过 HTML 转义的 JSON，
// 而 json.Marshal 处理 RawMessage 字段时正是原样紧凑拷贝，所以直接拼接结果相同，
// 省掉了对整段载荷的第二次扫描与拷贝。
func appendEventFrame(dst []byte, eventID string, timestamp int64, eventType string, data []byte) []byte {
	dst = append(dst, '{')
	if appmeta.ProtocolVersion != "" {
		dst = append(dst, `"schemaVersion":`...)
		dst = appendJSONString(dst, appmeta.ProtocolVersion)
		dst = append(dst, ',')
	}
	if eventID != "" {
		dst = append(dst, `"eventId":`...)
		dst = appendJSONString(dst, eventID)
		dst = append(dst, ',')
	}
	if timestamp != 0 {
		dst = append(dst, `"timestamp":`...)
		dst = strconv.AppendInt(dst, timestamp, 10)
		dst = append(dst, ',')
	}
	dst = append(dst, `"source":"core","isEvent":true,"type":`...)
	dst = appendJSONString(dst, eventType)
	if len(data) > 0 {
		dst = append(dst, `,"data":`...)
		dst = append(dst, data...)
	}
	return append(dst, '}', '\n')
}

// appendJSONString 追加一个 JSON 字符串。事件类型、ID 都是纯 ASCII 标识符，走快路径；
// 含需要转义的字符时交给 encoding/json，保证与 json.Marshal 的转义规则一致。
func appendJSONString(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c >= 0x80 || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			encoded, _ := json.Marshal(s)
			return append(dst, encoded...)
		}
	}
	dst = append(dst, '"')
	dst = append(dst, s...)
	return append(dst, '"')
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.mutex.Lock()
	s.running = false
	listener := s.listener
	s.listener = nil
	clients := make([]*clientState, 0, len(s.clients))
	for _, state := range s.clients {
		clients = append(clients, state)
	}
	s.clients = make(map[net.Conn]*clientState)
	s.mutex.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, state := range clients {
		s.closeClient(state)
	}

	s.logInfo("IPC 服务器已停止")
}

// HasClients 检查是否有客户端连接
func (s *Server) HasClients() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.clients) > 0
}

// HasActiveClients 检查是否有仍在订阅高频遥测的客户端（窗口可见或正在跑测试的 GUI）。
// 只连着但已退订的 GUI（窗口隐藏）不算，核心据此回到后台低频模式。
func (s *Server) HasActiveClients() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, state := range s.clients {
		if state.subscribed.Load() {
			return true
		}
	}
	return false
}

// ActiveClientsSupport 判断是否所有订阅中的客户端都声明了 feature（且至少有一个）。
// 高频事件对所有订阅者共用同一份编码，只要有一个旧客户端就得按旧格式发送。
func (s *Server) ActiveClientsSupport(feature string) bool {
	bit, known := clientFeatureBits[feature]
	if !known {
		return false
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	active := false
	for _, state := range s.clients {
		if !state.subscribed.Load() {
			continue
		}
		if state.features.Load()&bit == 0 {
			return false
		}
		active = true
	}
	return active
}

// ActiveClientsChanged 在出现新的订阅客户端（新连接或重新订阅）时收到信号。
// 信号可能合并，收到后应以 HasActiveClients 的当前值为准。
func (s *Server) ActiveClientsChanged() <-chan struct{} {
	return s.activeCh
}

// 日志辅助方法
func (s *Server) logInfo(format string, v ...any) {
	if s.logger != nil {
		logger.Wrapped(s.logger).Info(format, v...)
	}
}

func (s *Server) logError(format string, v ...any) {
	if s.logger != nil {
		logger.Wrapped(s.logger).Error(format, v...)
	}
}

func (s *Server) logDebug(format string, v ...any) {
	if s.logger != nil {
		logger.Wrapped(s.logger).Debug(format, v...)
	}
}

// Client IPC 客户端
//
// 响应路由：每条 SendRequest 注册一个 (requestID -> chan *Response)，readLoop 收到响应时
// 按 requestID 派发到对应 channel。这样并发请求互不串扰，且超时未取消的旧响应被自动丢弃。
type Client struct {
	conn         net.Conn
	mutex        sync.Mutex
	reader       *bufio.Reader
	logger       types.Logger
	eventHandler func(Event)
	eventMutex   sync.RWMutex

	pendingMutex sync.Mutex
	pending      map[string]pendingCall

	connected bool
	// gen 是当前连接的代号，每次成功 Connect 加一。调用方据此只作废真正出错的那条连接。
	gen       uint64
	connMutex sync.RWMutex
}

// pendingCall 是一个等待响应的请求。记下它所在的连接，连接断开时才能只让
// 这条连接上的等待者立即失败，而不是各自白等完整的超时。
//
// ch 的所有权归"把它从 pending 中删除的一方"：删除者负责送达响应或关闭它，
// 因此同一个 ch 不会被重复写入或关闭。
type pendingCall struct {
	conn net.Conn
	ch   chan *Response
}

const (
	clientEventQueueSize              = 64
	clientCriticalEventEnqueueTimeout = 500 * time.Millisecond
)

// NewClient 创建 IPC 客户端
func NewClient(logger types.Logger) *Client {
	return &Client{
		logger:  logger,
		pending: make(map[string]pendingCall),
	}
}

// Connect 连接到服务器
func (c *Client) Connect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		return nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
	}

	timeout := 5 * time.Second
	var conn net.Conn
	var err error
	for _, pipeName := range ipcPipeCandidates() {
		endpoint := ipcEndpointFromName(pipeName)
		conn, err = dialIPC(endpoint, timeout)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("连接 IPC 服务器失败: %v", err)
	}

	reader := bufio.NewReader(conn)
	c.conn = conn
	c.reader = reader
	c.connected = true
	c.gen++
	c.logInfo("已连接到 IPC 服务器")

	// 将读取循环绑定到本次连接，旧连接延迟返回时不能覆盖新连接状态。
	go c.readLoop(conn, reader)

	return nil
}

// readLoop 统一的消息读取循环
func (c *Client) readLoop(conn net.Conn, reader *bufio.Reader) {
	events := make(chan Event, clientEventQueueSize)
	eventLoopDone := make(chan struct{})
	go c.dispatchEvents(events, eventLoopDone)
	defer close(eventLoopDone)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			c.logDebug("读取消息失败: %v", err)
			c.connMutex.Lock()
			if c.conn == conn {
				c.connected = false
				c.conn = nil
				c.reader = nil
			}
			c.connMutex.Unlock()
			_ = conn.Close()
			// 必须在关闭连接之后再清理：此后新登记到这条连接上的请求会因写入失败
			// 自行返回，不会漏在 pending 里等超时。
			c.failPending(conn)
			return
		}

		// 响应与事件的字段并集，一行只解码一次（此前先探测类型再整行重解一遍）。
		var msg inboundMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logDebug("解析消息失败: %v", err)
			continue
		}

		if msg.IsResponse {
			resp := msg.response()
			// 按 RequestID 路由到对应等待者；找不到则说明请求已超时取消，直接丢弃
			c.pendingMutex.Lock()
			call, ok := c.pending[resp.RequestID]
			if ok {
				delete(c.pending, resp.RequestID)
			}
			c.pendingMutex.Unlock()
			if ok {
				// channel 容量 1 + delete 后立即送达，不会阻塞
				call.ch <- &resp
			} else {
				c.logDebug("收到无主响应，丢弃: requestID=%s", resp.RequestID)
			}
		} else if msg.IsEvent {
			if event := msg.event(); event.Type != "" {
				if isHighFrequencyEvent(event.Type) {
					select {
					case events <- event:
					default:
						c.logDebug("客户端事件队列已满，丢弃高频事件: %s", event.Type)
					}
					continue
				}

				timer := time.NewTimer(clientCriticalEventEnqueueTimeout)
				select {
				case events <- event:
					timer.Stop()
				case <-timer.C:
					c.logDebug("客户端事件队列持续拥塞，丢弃关键事件: %s", event.Type)
				}
			}
		}
	}
}

// inboundMessage 是 Response 与 Event 的字段并集。两者共用 timestamp 与 data，
// 其余字段互不重叠，因此解码一次即可还原出任意一种。
type inboundMessage struct {
	ProtocolVersion string          `json:"protocolVersion"`
	RequestID       string          `json:"requestId"`
	IsResponse      bool            `json:"isResponse"`
	Success         bool            `json:"success"`
	ErrorCode       string          `json:"errorCode"`
	Error           string          `json:"error"`
	SchemaVersion   string          `json:"schemaVersion"`
	EventID         string          `json:"eventId"`
	Source          string          `json:"source"`
	IsEvent         bool            `json:"isEvent"`
	Type            string          `json:"type"`
	Timestamp       int64           `json:"timestamp"`
	Data            json.RawMessage `json:"data"`
}

func (m *inboundMessage) response() Response {
	return Response{
		ProtocolVersion: m.ProtocolVersion,
		RequestID:       m.RequestID,
		Timestamp:       m.Timestamp,
		IsResponse:      m.IsResponse,
		Success:         m.Success,
		ErrorCode:       m.ErrorCode,
		Error:           m.Error,
		Data:            m.Data,
	}
}

func (m *inboundMessage) event() Event {
	return Event{
		SchemaVersion: m.SchemaVersion,
		EventID:       m.EventID,
		Timestamp:     m.Timestamp,
		Source:        m.Source,
		IsEvent:       m.IsEvent,
		Type:          m.Type,
		Data:          m.Data,
	}
}

// failPending 让登记在 conn 上、仍在等待响应的请求立即以 ErrConnectionClosed 返回。
// 连接断开后这些响应不可能再到达，不清理的话每个等待者都要白等完整的超时。
func (c *Client) failPending(conn net.Conn) {
	c.pendingMutex.Lock()
	defer c.pendingMutex.Unlock()
	for id, call := range c.pending {
		if call.conn == conn {
			delete(c.pending, id)
			close(call.ch)
		}
	}
}

func (c *Client) dispatchEvents(events <-chan Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case event := <-events:
			c.eventMutex.RLock()
			handler := c.eventHandler
			c.eventMutex.RUnlock()
			if handler == nil {
				continue
			}
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						c.logDebug("处理 IPC 事件时发生 panic: %v", recovered)
					}
				}()
				handler(event)
			}()
		}
	}
}

// SetEventHandler 设置事件处理函数
func (c *Client) SetEventHandler(handler func(Event)) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()
	c.eventHandler = handler
}

// SendRequest 发送请求并等待响应
func (c *Client) SendRequest(reqType RequestType, data any) (*Response, error) {
	return c.SendRequestWithTimeout(reqType, data, defaultRequestTimeout)
}

func (c *Client) SendRequestWithTimeout(reqType RequestType, data any, timeout time.Duration) (*Response, error) {
	resp, _, err := c.SendRequestTracked(reqType, data, timeout)
	return resp, err
}

// SendRequestTracked 与 SendRequestWithTimeout 相同，额外返回本次请求实际使用的连接代号
// （未连接时为 0）。请求失败后需要拆连接的调用方应以该代号调用 CloseGeneration：
// 直接 Close() 关掉的是"当前"连接，而它可能是失败期间别处已经重连成功的新连接。
func (c *Client) SendRequestTracked(reqType RequestType, data any, timeout time.Duration) (*Response, uint64, error) {
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	c.connMutex.RLock()
	if !c.connected || c.conn == nil {
		c.connMutex.RUnlock()
		return nil, 0, fmt.Errorf("未连接到服务器")
	}
	conn := c.conn
	gen := c.gen
	c.connMutex.RUnlock()

	var dataBytes json.RawMessage
	if data != nil {
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, gen, fmt.Errorf("序列化请求数据失败: %v", err)
		}
	}

	requestID := newMessageID("req")
	req := Request{
		ProtocolVersion: appmeta.ProtocolVersion,
		RequestID:       requestID,
		Timestamp:       time.Now().UnixMilli(),
		Type:            reqType,
		Data:            dataBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, gen, fmt.Errorf("序列化请求失败: %v", err)
	}

	respCh := make(chan *Response, 1)
	c.pendingMutex.Lock()
	c.pending[requestID] = pendingCall{conn: conn, ch: respCh}
	c.pendingMutex.Unlock()

	// 写入必须带超时。没有 deadline 时，核心侧假死（不再读取管道）会让这次写入
	// 永久阻塞在持有 c.mutex 的状态下，后续所有请求跟着永久阻塞——而下面的
	// timeout 只覆盖"等响应"，对卡在写入上的调用完全无效。
	c.mutex.Lock()
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		c.logDebug("设置 IPC 写超时失败: %v", err)
	}
	_, err = conn.Write(append(reqBytes, '\n'))
	c.mutex.Unlock()
	if err != nil {
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		// 写超时说明对端已不可用，直接标记连接失效，让上层走重连而不是反复超时。
		// 注意不能用 errors.Is(err, os.ErrDeadlineExceeded)：Windows 命名管道
		// （go-winio）返回的是它自己的 ErrTimeout，只有 net.Error.Timeout() 能统一识别。
		if isTimeoutError(err) {
			c.invalidateConn(conn)
			return nil, gen, fmt.Errorf("%w: request=%s 发送超时", ErrRequestTimeout, reqType)
		}
		return nil, gen, fmt.Errorf("发送请求失败: %v", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp, ok := <-respCh:
		if !ok {
			// readLoop 在连接断开时关闭了 channel，响应不会再来了。
			return nil, gen, fmt.Errorf("%w: request=%s", ErrConnectionClosed, reqType)
		}
		return resp, gen, nil
	case <-timer.C:
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		return nil, gen, fmt.Errorf("%w: request=%s, timeout=%s", ErrRequestTimeout, reqType, timeout)
	}
}

// isTimeoutError 判断错误是否为 I/O 超时。各传输实现返回的超时错误类型不同
// （go-winio 有自己的 ErrTimeout，标准库用 os.ErrDeadlineExceeded），
// 但都实现了 net.Error 且 Timeout() 为 true。
func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// invalidateConn 把指定连接标记为失效。只在它仍是当前连接时生效，
// 避免旧连接的延迟失败覆盖掉新建立的连接状态。
func (c *Client) invalidateConn(conn net.Conn) {
	c.connMutex.Lock()
	stale := c.conn == conn
	if stale {
		c.connected = false
		c.conn = nil
		c.reader = nil
	}
	c.connMutex.Unlock()
	if stale {
		_ = conn.Close()
	}
}

// CloseGeneration 仅当当前连接仍是 gen 代时才关闭它，返回是否确实关闭。
// 用于"某次请求失败后拆掉它所在的连接"：那条连接若已被替换，就什么都不做。
func (c *Client) CloseGeneration(gen uint64) bool {
	c.connMutex.Lock()
	conn := c.conn
	if gen == 0 || c.gen != gen || conn == nil {
		c.connMutex.Unlock()
		return false
	}
	c.connected = false
	c.conn = nil
	c.reader = nil
	c.connMutex.Unlock()

	_ = conn.Close()
	return true
}

// Close 关闭连接
func (c *Client) Close() {
	c.connMutex.Lock()
	conn := c.conn
	c.connected = false
	c.conn = nil
	c.reader = nil
	c.connMutex.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// 日志辅助方法
func (c *Client) logInfo(format string, v ...any) {
	if c.logger != nil {
		logger.Wrapped(c.logger).Info(format, v...)
	}
}

func (c *Client) logDebug(format string, v ...any) {
	if c.logger != nil {
		logger.Wrapped(c.logger).Debug(format, v...)
	}
}

// CheckCoreServiceRunning 检查核心服务是否正在运行
func CheckCoreServiceRunning() bool {
	timeout := 1 * time.Second
	for _, pipeName := range ipcPipeCandidates() {
		endpoint := ipcEndpointFromName(pipeName)
		conn, err := dialIPC(endpoint, timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// GetCoreLockFilePath 获取核心服务锁文件路径
func GetCoreLockFilePath() string {
	tempDir := os.TempDir()
	return fmt.Sprintf("%s/thrm core.lock", tempDir)
}

// StartCoreRequestParams 启动核心服务的请求参数
type StartCoreRequestParams struct {
	ShowGUI bool `json:"showGUI"`
}

// SetAutoControlParams 设置智能变频参数
type SetAutoControlParams struct {
	Enabled bool `json:"enabled"`
}

// SetManualGearParams 设置手动挡位参数
type SetManualGearParams struct {
	Gear  string `json:"gear"`
	Level string `json:"level"`
}

// SetCustomSpeedParams 设置自定义转速参数
type SetCustomSpeedParams struct {
	Enabled bool `json:"enabled"`
	RPM     int  `json:"rpm"`
}

// SetBoolParams 布尔参数
type SetBoolParams struct {
	Enabled bool `json:"enabled"`
}

// FlashFirmwareParams 是 ReqFlashFirmware 的参数。
type FlashFirmwareParams struct {
	Source    string `json:"source"`    // official | hosted | local
	URL       string `json:"url"`       // 官方/托管的下载地址
	LocalPath string `json:"localPath"` // 本地文件路径
	Version   string `json:"version"`   // 仅用于展示
}

// SetStringParams 字符串参数
type SetStringParams struct {
	Value string `json:"value"`
}

// SetIntParams 整数参数
type SetIntParams struct {
	Value int `json:"value"`
}

type PreviewRTSSPositionParams struct {
	Mode string `json:"mode"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

// DeviceDebugCommandParams contains a raw protocol command for the debug panel.
type DeviceDebugCommandParams struct {
	Hex    string `json:"hex"`
	WaitMs int    `json:"waitMs"`
}

// SetAutoStartWithMethodParams 设置自启动方式参数
type SetAutoStartWithMethodParams struct {
	Enable bool   `json:"enable"`
	Method string `json:"method"`
}

// SetLightStripParams 设置灯带参数
type SetLightStripParams struct {
	Config types.LightStripConfig `json:"config"`
}

// SetActiveFanCurveProfileParams 设置激活曲线方案参数
// SaveCoolingBenefitReportParams 提交一次散热收益扫描测试的原始结果。
// 分析在核心侧做：那段逻辑有单元测试守着，也保证 GUI 与后续读取看到的是同一份结论。
type SaveCoolingBenefitReportParams struct {
	DeviceModel string                     `json:"deviceModel"`
	CPUModel    string                     `json:"cpuModel"`
	GPUModel    string                     `json:"gpuModel"`
	LoadLabel   string                     `json:"loadLabel"`
	Steps       []types.CoolingBenefitStep `json:"steps"`
}

type SetActiveFanCurveProfileParams struct {
	ID string `json:"id"`
}

// SaveFanCurveProfileParams 保存曲线方案参数
type SaveFanCurveProfileParams struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Curve     []types.FanCurvePoint `json:"curve"`
	SetActive bool                  `json:"setActive"`
}

// DeleteFanCurveProfileParams 删除曲线方案参数
type DeleteFanCurveProfileParams struct {
	ID string `json:"id"`
}

// ImportFanCurveProfilesParams 导入曲线方案参数
type ImportFanCurveProfilesParams struct {
	Code string `json:"code"`
}

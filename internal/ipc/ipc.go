// Package ipc 提供核心服务与 GUI 之间的进程间通信
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
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

var ErrRequestTimeout = errors.New("等待 IPC 响应超时")

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

		s.mutex.Lock()
		s.clients[conn] = state
		s.mutex.Unlock()

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
		line, err := reader.ReadBytes('\n')
		if err != nil {
			s.logDebug("读取客户端请求失败: %v", err)
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

// serveRequest 执行单个请求并把响应交给写协程。
func (s *Server) serveRequest(state *clientState, req Request) {
	s.logDebug("IPC 请求[%s]: %s", req.RequestID, req.Type)

	resp := s.invokeHandler(req)
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

// BroadcastEvent 广播事件给所有客户端
func (s *Server) BroadcastEvent(eventType string, data any) {
	if !s.HasClients() {
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

	event := Event{
		SchemaVersion: appmeta.ProtocolVersion,
		EventID:       newMessageID("evt"),
		Timestamp:     time.Now().UnixMilli(),
		Source:        "core",
		IsEvent:       true,
		Type:          eventType,
		Data:          dataBytes,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		s.logError("序列化事件失败: %v", err)
		return
	}
	payload := append(eventBytes, '\n')

	s.mutex.RLock()
	clients := make([]*clientState, 0, len(s.clients))
	for _, state := range s.clients {
		clients = append(clients, state)
	}
	s.mutex.RUnlock()

	for _, state := range clients {
		if isHighFrequencyEvent(eventType) {
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

// 日志辅助方法
func (s *Server) logInfo(format string, v ...any) {
	if s.logger != nil {
		s.logger.Info(format, v...)
	}
}

func (s *Server) logError(format string, v ...any) {
	if s.logger != nil {
		s.logger.Error(format, v...)
	}
}

func (s *Server) logDebug(format string, v ...any) {
	if s.logger != nil {
		s.logger.Debug(format, v...)
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
	pending      map[string]chan *Response

	connected bool
	connMutex sync.RWMutex
}

const (
	clientEventQueueSize              = 64
	clientCriticalEventEnqueueTimeout = 500 * time.Millisecond
)

// NewClient 创建 IPC 客户端
func NewClient(logger types.Logger) *Client {
	return &Client{
		logger:  logger,
		pending: make(map[string]chan *Response),
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
			return
		}

		// 使用通用结构来检测消息类型
		var msg struct {
			IsResponse bool `json:"isResponse"`
			IsEvent    bool `json:"isEvent"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logDebug("解析消息类型失败: %v", err)
			continue
		}

		if msg.IsResponse {
			var resp Response
			if err := json.Unmarshal(line, &resp); err == nil {
				// 按 RequestID 路由到对应等待者；找不到则说明请求已超时取消，直接丢弃
				c.pendingMutex.Lock()
				ch, ok := c.pending[resp.RequestID]
				if ok {
					delete(c.pending, resp.RequestID)
				}
				c.pendingMutex.Unlock()
				if ok {
					// channel 容量 1 + delete 后立即送达，不会阻塞
					ch <- &resp
				} else {
					c.logDebug("收到无主响应，丢弃: requestID=%s", resp.RequestID)
				}
			}
		} else if msg.IsEvent {
			var event Event
			if err := json.Unmarshal(line, &event); err == nil && event.Type != "" {
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
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	c.connMutex.RLock()
	if !c.connected || c.conn == nil {
		c.connMutex.RUnlock()
		return nil, fmt.Errorf("未连接到服务器")
	}
	conn := c.conn
	c.connMutex.RUnlock()

	var dataBytes json.RawMessage
	if data != nil {
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("序列化请求数据失败: %v", err)
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
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	respCh := make(chan *Response, 1)
	c.pendingMutex.Lock()
	c.pending[requestID] = respCh
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
			return nil, fmt.Errorf("%w: request=%s 发送超时", ErrRequestTimeout, reqType)
		}
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp := <-respCh:
		return resp, nil
	case <-timer.C:
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		return nil, fmt.Errorf("%w: request=%s, timeout=%s", ErrRequestTimeout, reqType, timeout)
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
		c.logger.Info(format, v...)
	}
}

func (c *Client) logDebug(format string, v ...any) {
	if c.logger != nil {
		c.logger.Debug(format, v...)
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

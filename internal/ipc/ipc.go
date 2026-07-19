// Package ipc 提供核心服务与 GUI 之间的进程间通信
package ipc

import (
	"bufio"
	"encoding/json"
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

	// 控制相关
	ReqSetAutoControl    RequestType = "SetAutoControl"
	ReqSetManualGear     RequestType = "SetManualGear"
	ReqGetAvailableGears RequestType = "GetAvailableGears"
	ReqSetCustomSpeed    RequestType = "SetCustomSpeed"
	ReqSetGearLight      RequestType = "SetGearLight"
	ReqSetPowerOnStart   RequestType = "SetPowerOnStart"
	ReqSetSmartStartStop RequestType = "SetSmartStartStop"
	ReqSetBrightness     RequestType = "SetBrightness"
	ReqSetLightStrip     RequestType = "SetLightStrip"

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
	EventHotkeyTriggered          = "hotkey-triggered"
	EventLegionPowerModeUpdate    = "legion-power-mode-update"
	EventLegionFnQSupportUpdate   = "legion-fnq-support-update"
	EventHealthPing               = "health-ping"
	EventHeartbeat                = "heartbeat"
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
	conn      net.Conn
	writeCh   chan []byte
	closeOnce sync.Once
	closed    chan struct{}
}

const clientWriteQueueSize = 64
const defaultRequestTimeout = 15 * time.Second

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

	s.listener = listener
	s.running = true
	s.logInfo("IPC 服务器已启动: %s", addr)

	// 接受连接
	go s.acceptConnections()

	return nil
}

// acceptConnections 接受客户端连接
func (s *Server) acceptConnections() {
	consecutiveFailures := 0
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running {
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

func (s *Server) clientWriter(state *clientState) {
	for {
		select {
		case data, ok := <-state.writeCh:
			if !ok {
				return
			}
			if _, err := state.conn.Write(data); err != nil {
				s.logDebug("发送数据失败: %v", err)
				s.closeClient(state)
				return
			}
		case <-state.closed:
			return
		}
	}
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

// handleClient 处理客户端连接
func (s *Server) handleClient(conn net.Conn, state *clientState) {
	defer func() {
		s.closeClient(state)
		s.logInfo("IPC 客户端已断开")
	}()

	reader := bufio.NewReader(conn)

	for s.running {
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
		s.logDebug("IPC 请求[%s]: %s", req.RequestID, req.Type)
		resp := s.handler(req)
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
			continue
		}

		if _, err := conn.Write(append(respBytes, '\n')); err != nil {
			s.logError("发送响应失败: %v", err)
			return
		}
	}
}

var highFrequencyEventTypes = map[string]time.Duration{
	EventFanDataUpdate:            250 * time.Millisecond,
	EventTemperatureUpdate:        250 * time.Millisecond,
	EventTemperatureHistoryUpdate: 1000 * time.Millisecond,
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
	for _, state := range s.clients {
		select {
		case state.writeCh <- payload:
		default:
			s.logDebug("客户端写队列已满，丢弃事件: %s", eventType)
		}
	}
	s.mutex.RUnlock()
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}

	s.mutex.Lock()
	for conn, state := range s.clients {
		state.closeOnce.Do(func() {
			close(state.closed)
			conn.Close()
		})
	}
	s.clients = make(map[net.Conn]*clientState)
	s.mutex.Unlock()

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

	pendingMutex sync.Mutex
	pending      map[string]chan *Response

	connected bool
	connMutex sync.RWMutex
}

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

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.connected = true
	c.logInfo("已连接到 IPC 服务器")

	// 启动消息接收循环
	go c.readLoop()

	return nil
}

// readLoop 统一的消息读取循环
func (c *Client) readLoop() {
	for {
		c.connMutex.RLock()
		if !c.connected || c.reader == nil {
			c.connMutex.RUnlock()
			return
		}
		reader := c.reader
		c.connMutex.RUnlock()

		line, err := reader.ReadBytes('\n')
		if err != nil {
			c.logDebug("读取消息失败: %v", err)
			c.connMutex.Lock()
			c.connected = false
			c.connMutex.Unlock()
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
				if c.eventHandler != nil {
					go c.eventHandler(event)
				}
			}
		}
	}
}

// SetEventHandler 设置事件处理函数
func (c *Client) SetEventHandler(handler func(Event)) {
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

	c.mutex.Lock()
	_, err = conn.Write(append(reqBytes, '\n'))
	c.mutex.Unlock()
	if err != nil {
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		c.pendingMutex.Lock()
		delete(c.pending, requestID)
		c.pendingMutex.Unlock()
		return nil, fmt.Errorf("等待响应超时")
	}
}

// Close 关闭连接
func (c *Client) Close() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
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

// Package ipc 提供核心服务与 GUI 之间的进程间通信
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

const (
	// PipeName 命名管道名称
	PipeName = "BS2PRO-Controller-IPC"
	// PipePath 命名管道完整路径
	PipePath = `\\.\pipe\` + PipeName
)

// RequestType 请求类型
type RequestType string

const (
	// 设备相关
	ReqConnect           RequestType = "Connect"
	ReqDisconnect        RequestType = "Disconnect"
	ReqGetDeviceStatus   RequestType = "GetDeviceStatus"
	ReqGetCurrentFanData RequestType = "GetCurrentFanData"

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
	ReqGetTemperature         RequestType = "GetTemperature"
	ReqTestTemperatureReading RequestType = "TestTemperatureReading"
	ReqTestBridgeProgram      RequestType = "TestBridgeProgram"
	ReqGetBridgeProgramStatus RequestType = "GetBridgeProgramStatus"
	ReqRestartPawnIO          RequestType = "RestartPawnIO"

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
	ReqGetDebugInfo          RequestType = "GetDebugInfo"
	ReqSetDebugMode          RequestType = "SetDebugMode"
	ReqUpdateGuiResponseTime RequestType = "UpdateGuiResponseTime"

	// 系统相关
	ReqPing              RequestType = "Ping"
	ReqIsAutoStartLaunch RequestType = "IsAutoStartLaunch"
	ReqSubscribeEvents   RequestType = "SubscribeEvents"
	ReqUnsubscribeEvents RequestType = "UnsubscribeEvents"
)

// Request IPC 请求
type Request struct {
	Type RequestType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Response IPC 响应
type Response struct {
	IsResponse bool            `json:"isResponse"` // 标识这是响应而非事件
	Success    bool            `json:"success"`
	Error      string          `json:"error,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// Event IPC 事件（服务器推送给客户端）
type Event struct {
	IsEvent bool            `json:"isEvent"` // 标识这是事件
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// EventType 事件类型
const (
	EventFanDataUpdate         = "fan-data-update"
	EventTemperatureUpdate     = "temperature-update"
	EventDeviceConnected       = "device-connected"
	EventDeviceDisconnected    = "device-disconnected"
	EventDeviceError           = "device-error"
	EventConfigUpdate          = "config-update"
	EventHotkeyTriggered       = "hotkey-triggered"
	EventLegionPowerModeUpdate = "legion-power-mode-update"
	EventHealthPing            = "health-ping"
	EventHeartbeat             = "heartbeat"
)

// Server IPC 服务器
type Server struct {
	listener net.Listener
	clients  map[net.Conn]bool
	mutex    sync.RWMutex
	handler  RequestHandler
	logger   types.Logger
	running  bool
}

// RequestHandler 请求处理函数类型
type RequestHandler func(req Request) Response

// NewServer 创建 IPC 服务器
func NewServer(handler RequestHandler, logger types.Logger) *Server {
	return &Server{
		clients: make(map[net.Conn]bool),
		handler: handler,
		logger:  logger,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 创建命名管道监听器
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;WD)", // 允许所有用户访问
	}

	listener, err := winio.ListenPipe(PipePath, cfg)
	if err != nil {
		return fmt.Errorf("创建命名管道失败: %v", err)
	}

	s.listener = listener
	s.running = true
	s.logInfo("IPC 服务器已启动: %s", PipePath)

	// 接受连接
	go s.acceptConnections()

	return nil
}

// acceptConnections 接受客户端连接
func (s *Server) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				s.logError("接受连接失败: %v", err)
			}
			continue
		}

		s.mutex.Lock()
		s.clients[conn] = true
		s.mutex.Unlock()

		s.logInfo("新的 IPC 客户端已连接")
		go s.handleClient(conn)
	}
}

// handleClient 处理客户端连接
func (s *Server) handleClient(conn net.Conn) {
	defer func() {
		s.mutex.Lock()
		delete(s.clients, conn)
		s.mutex.Unlock()
		conn.Close()
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
		resp := s.handler(req)
		resp.IsResponse = true

		// 发送响应
		respBytes, err := json.Marshal(resp)
		if err != nil {
			s.logError("序列化响应失败: %v", err)
			continue
		}

		_, err = conn.Write(append(respBytes, '\n'))
		if err != nil {
			s.logError("发送响应失败: %v", err)
			return
		}
	}
}

// BroadcastEvent 广播事件给所有客户端
func (s *Server) BroadcastEvent(eventType string, data any) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		s.logError("序列化事件数据失败: %v", err)
		return
	}

	event := Event{
		IsEvent: true, // 标记为事件
		Type:    eventType,
		Data:    dataBytes,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		s.logError("序列化事件失败: %v", err)
		return
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for conn := range s.clients {
		go func(c net.Conn) {
			_, err := c.Write(append(eventBytes, '\n'))
			if err != nil {
				s.logDebug("发送事件失败: %v", err)
			}
		}(conn)
	}
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}

	s.mutex.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clients = make(map[net.Conn]bool)
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
type Client struct {
	conn         net.Conn
	mutex        sync.Mutex
	reader       *bufio.Reader
	logger       types.Logger
	eventHandler func(Event)
	responseChan chan *Response
	connected    bool
	connMutex    sync.RWMutex
}

// NewClient 创建 IPC 客户端
func NewClient(logger types.Logger) *Client {
	return &Client{
		logger:       logger,
		responseChan: make(chan *Response, 1),
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
	conn, err := winio.DialPipe(PipePath, &timeout)
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
				select {
				case c.responseChan <- &resp:
				default:
					c.logDebug("响应通道已满，丢弃响应")
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
	c.mutex.Lock()
	defer c.mutex.Unlock()

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

	req := Request{
		Type: reqType,
		Data: dataBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 清空响应通道
	select {
	case <-c.responseChan:
	default:
	}

	_, err = conn.Write(append(reqBytes, '\n'))
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	select {
	case resp := <-c.responseChan:
		return resp, nil
	case <-time.After(10 * time.Second):
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
	conn, err := winio.DialPipe(PipePath, &timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetCoreLockFilePath 获取核心服务锁文件路径
func GetCoreLockFilePath() string {
	tempDir := os.TempDir()
	return fmt.Sprintf("%s/bs2pro-core.lock", tempDir)
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

package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
	"golang.org/x/sys/windows"
)

type Manager struct {
	cmd       *exec.Cmd
	conn      net.Conn
	pipeName  string
	ownsCmd   bool
	state     string
	lastError string
	mutex     sync.Mutex
	logger    types.Logger
}

const (
	bridgeDefaultCommandTimeout = 3 * time.Second
	bridgeGetTemperatureTimeout = 10 * time.Second
	bridgeRestartPawnIOTimeout  = 20 * time.Second
	bridgeExitTimeout           = 2 * time.Second
	bridgeProcessExitWait       = 8 * time.Second
	bridgeReconnectTimeout      = 2 * time.Second
	windowsStillActive          = 259

	BridgeStateNotStarted = "not_started"
	BridgeStateStarting   = "starting"
	BridgeStateRunning    = "running_owned"
	BridgeStateAttached   = "attached"
	BridgeStateDegraded   = "degraded"
	BridgeStateStopping   = "stopping"
	BridgeStateStopped    = "stopped"
	BridgeStateFailed     = "failed"
)

func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
		state:  BridgeStateNotStarted,
	}
}

func (m *Manager) setState(state string, err error) {
	m.state = state
	if err != nil {
		m.lastError = err.Error()
		return
	}

	switch state {
	case BridgeStateRunning, BridgeStateAttached, BridgeStateStopped, BridgeStateNotStarted:
		m.lastError = ""
	}
}

func (m *Manager) EnsureRunning() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// If we already have a pipe connection, trust it and avoid an eager Ping.
	// Slow hardware reads can legitimately occupy TempBridge for several seconds,
	// and probing here would turn "busy" into a restart loop.
	if m.conn != nil {
		if m.ownsCmd {
			m.setState(BridgeStateRunning, nil)
		} else {
			m.setState(BridgeStateAttached, nil)
		}
		return nil
	}

	// If the bridge process is still around, prefer reconnecting to its pipe
	// instead of killing and relaunching it immediately.
	if m.pipeName != "" {
		conn, err := m.connectToPipe(m.pipeName, bridgeReconnectTimeout)
		if err == nil {
			m.conn = conn
			if m.ownsCmd {
				m.setState(BridgeStateRunning, nil)
			} else {
				m.setState(BridgeStateAttached, nil)
			}
			return nil
		}

		m.setState(BridgeStateDegraded, err)
		if m.ownsCmd && isProcessRunning(m.cmd) {
			return fmt.Errorf("bridge reconnect failed: %w", err)
		}

		m.releaseOwnedProcessUnsafe()
		m.pipeName = ""
	}

	return m.start()
}

func (m *Manager) start() error {
	m.setState(BridgeStateStarting, nil)

	if conn, pipeName, err := m.connectToAnyPipe(appmeta.BridgePipeCandidates(), 500*time.Millisecond); err == nil {
		m.conn = conn
		m.pipeName = pipeName
		m.ownsCmd = false
		m.setState(BridgeStateAttached, nil)
		m.logger.Info("复用已存在的桥接程序，管道名称: %s", pipeName)
		return nil
	}

	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("获取程序目录失败: %v", err)
	}

	possiblePaths := appmeta.BridgeExecutableCandidates(exeDir)
	bridgePath := appmeta.FirstExistingPath(possiblePaths)
	if bridgePath == "" {
		err := fmt.Errorf("%s 不存在，已尝试以下路径: %v", appmeta.BridgeExecutableName, possiblePaths)
		m.setState(BridgeStateFailed, err)
		return err
	}

	m.logger.Info("找到桥接程序: %s", bridgePath)

	cmd := exec.Command(bridgePath, "--pipe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("启动桥接程序失败: %v", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				m.logger.Error("桥接程序 stderr: %s", line)
			}
		}
		if err := scanner.Err(); err != nil {
			m.logger.Debug("读取桥接程序 stderr 失败: %v", err)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	var pipeName string
	var attachMode bool
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	done := make(chan struct{})
	go func() {
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if after, ok := strings.CutPrefix(line, "PIPE:"); ok {
				parts := strings.SplitN(after, "|", 2)
				pipeName = strings.TrimSpace(parts[0])
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "ATTACH") {
					attachMode = true
				}
			} else if after, ok := strings.CutPrefix(line, "ERROR:"); ok {
				m.logger.Error("桥接程序启动错误: %s", strings.TrimSpace(after))
			}
		}
		close(done)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				m.logger.Debug("桥接程序 stdout: %s", line)
			}
		}
		if err := scanner.Err(); err != nil {
			m.logger.Debug("读取桥接程序 stdout 失败: %v", err)
		}
	}()

	select {
	case <-done:
		if pipeName == "" {
			_ = cmd.Process.Kill()
			err := fmt.Errorf("未能获取管道名称")
			m.setState(BridgeStateFailed, err)
			return err
		}
	case <-timeout.C:
		_ = cmd.Process.Kill()
		err := fmt.Errorf("等待桥接程序启动超时")
		m.setState(BridgeStateFailed, err)
		return err
	}

	conn, err := m.connectToPipe(pipeName, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("连接管道失败: %v", err)
	}

	m.conn = conn
	m.pipeName = pipeName
	m.ownsCmd = !attachMode
	if attachMode {
		go func() {
			_ = cmd.Wait()
		}()
		m.setState(BridgeStateAttached, nil)
		m.logger.Info("桥接程序已存在，附着到共享实例，管道名称: %s", pipeName)
		return nil
	}

	m.cmd = cmd
	m.setState(BridgeStateRunning, nil)
	m.logger.Info("桥接程序启动成功，管道名称: %s", pipeName)
	return nil
}

func (m *Manager) connectToAnyPipe(pipeNames []string, timeout time.Duration) (net.Conn, string, error) {
	var lastErr error
	for _, pipeName := range pipeNames {
		conn, err := m.connectToPipe(pipeName, timeout)
		if err == nil {
			return conn, pipeName, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("未找到可用桥接管道")
	}
	return nil, "", lastErr
}

func bridgeCommandTimeoutFor(cmdType string) time.Duration {
	switch cmdType {
	case "GetTemperature":
		return bridgeGetTemperatureTimeout
	case "RestartPawnIO":
		return bridgeRestartPawnIOTimeout
	case "Exit":
		return bridgeExitTimeout
	default:
		return bridgeDefaultCommandTimeout
	}
}

func (m *Manager) connectToPipe(pipeName string, timeout time.Duration) (net.Conn, error) {
	pipePath := `\\.\pipe\` + pipeName
	deadline := time.Now().Add(timeout)
	retryCount := 0
	backoff := 100 * time.Millisecond
	const maxBackoff = 1000 * time.Millisecond

	m.logger.Debug("尝试连接到管道: %s", pipePath)

	for time.Now().Before(deadline) {
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			m.logger.Info("成功连接到管道，重试次数: %d", retryCount)
			return conn, nil
		}

		retryCount++
		if retryCount%5 == 0 {
			m.logger.Debug("连接管道重试中... 第%d次尝试，错误: %v", retryCount, err)
		}

		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	return nil, fmt.Errorf("连接管道超时，总计重试%d次，最后错误可能是权限或管道未就绪", retryCount)
}

func (m *Manager) SendCommand(cmdType, data string) (*types.BridgeResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendCommandUnsafe(cmdType, data)
}

func (m *Manager) sendCommandUnsafe(cmdType, data string) (*types.BridgeResponse, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("桥接程序未连接")
	}

	conn := m.conn
	timeout := bridgeCommandTimeoutFor(cmdType)

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		m.logger.Debug("设置桥接命令超时失败: %v", err)
	}
	defer func() {
		_ = conn.SetDeadline(time.Time{})
	}()

	cmdBytes, err := json.Marshal(types.BridgeCommand{
		Type: cmdType,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化命令失败: %v", err)
	}

	if _, err := conn.Write(append(cmdBytes, '\n')); err != nil {
		m.setState(BridgeStateDegraded, err)
		m.closeConnUnsafe()
		return nil, fmt.Errorf("发送 %s 命令失败 (timeout=%s): %v", cmdType, timeout, err)
	}

	responseBytes, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		m.setState(BridgeStateDegraded, err)
		m.closeConnUnsafe()
		return nil, fmt.Errorf("读取 %s 响应失败 (timeout=%s): %v", cmdType, timeout, err)
	}

	var response types.BridgeResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		m.setState(BridgeStateDegraded, err)
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if m.ownsCmd {
		m.setState(BridgeStateRunning, nil)
	} else {
		m.setState(BridgeStateAttached, nil)
	}

	return &response, nil
}

func (m *Manager) closeConnUnsafe() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
}

func (m *Manager) releaseOwnedProcessUnsafe() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Release()
	}
	m.cmd = nil
	m.ownsCmd = false
}

func isProcessRunning(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}

	return exitCode == windowsStillActive
}

func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stopUnsafe()
}

func (m *Manager) stopUnsafe() {
	m.setState(BridgeStateStopping, nil)

	ownedCmd := m.cmd
	ownsCmd := m.ownsCmd

	m.cmd = nil
	m.ownsCmd = false
	m.pipeName = ""

	if m.conn != nil {
		if ownsCmd {
			_, _ = m.sendCommandUnsafe("Exit", "")
		}
		m.closeConnUnsafe()
	}

	if ownsCmd && ownedCmd != nil && ownedCmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- ownedCmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(bridgeProcessExitWait):
			_ = ownedCmd.Process.Kill()
		}
	}

	m.setState(BridgeStateStopped, nil)
}

func (m *Manager) GetTemperature(selection types.TemperatureSelection) types.BridgeTemperatureData {
	selection = types.NormalizeTemperatureSelection(selection)
	selectionPayload, err := json.Marshal(selection)
	if err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("序列化温度选择配置失败: %v", err),
		}
	}

	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("启动桥接程序失败: %v", err),
		}
	}

	response, err := m.SendCommand("GetTemperature", string(selectionPayload))
	if err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("桥接程序通信失败: %v", err),
		}
	}

	if !response.Success {
		if response.Data != nil {
			result := *response.Data
			result.Success = false
			if strings.TrimSpace(response.Error) != "" {
				result.Error = response.Error
			}
			return result
		}
		return types.BridgeTemperatureData{
			Success: false,
			Error:   response.Error,
		}
	}

	if response.Data == nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   "桥接程序返回空数据",
		}
	}

	return *response.Data
}

func (m *Manager) GetStatus() map[string]any {
	m.mutex.Lock()
	state := m.state
	ownsCmd := m.ownsCmd
	pipeName := m.pipeName
	lastError := m.lastError
	m.mutex.Unlock()

	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return map[string]any{
			"exists": false,
			"error":  fmt.Sprintf("获取程序目录失败: %v", err),
			"state":  state,
		}
	}

	possiblePaths := appmeta.BridgeExecutableCandidates(exeDir)
	bridgePath := appmeta.FirstExistingPath(possiblePaths)
	if bridgePath == "" {
		return map[string]any{
			"exists":      false,
			"state":       state,
			"ownsProcess": ownsCmd,
			"pipeName":    pipeName,
			"lastError":   lastError,
			"triedPaths":  possiblePaths,
			"error":       fmt.Sprintf("%s 不存在", appmeta.BridgeExecutableName),
		}
	}

	testResult := m.GetTemperature(types.GetDefaultTemperatureSelection())

	m.mutex.Lock()
	state = m.state
	ownsCmd = m.ownsCmd
	pipeName = m.pipeName
	lastError = m.lastError
	m.mutex.Unlock()

	return map[string]any{
		"exists":      true,
		"path":        bridgePath,
		"working":     testResult.Success,
		"state":       state,
		"ownsProcess": ownsCmd,
		"pipeName":    pipeName,
		"lastError":   lastError,
		"testData":    testResult,
	}
}

func (m *Manager) RestartPawnIO() (types.BridgeTemperatureData, error) {
	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("启动桥接程序失败: %v", err),
		}, err
	}

	m.logger.Info("正在通过桥接程序重启 PawnIO 驱动...")
	response, err := m.SendCommand("RestartPawnIO", "")
	if err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("发送 RestartPawnIO 命令失败: %v", err),
		}, err
	}

	if !response.Success {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   response.Error,
		}, fmt.Errorf("RestartPawnIO 失败: %s", response.Error)
	}

	result := types.BridgeTemperatureData{Success: true}
	if response.Data != nil {
		result = *response.Data
	}

	m.logger.Info("PawnIO 驱动重启成功")
	return result, nil
}

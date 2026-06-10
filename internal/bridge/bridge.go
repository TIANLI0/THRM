package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// Manager 管理温度桥接子进程的生命周期与 stdio 通信。
//
// 重要设计说明：stdin/stdout 匿名管道不支持 SetDeadline，直接在调用方协程上做阻塞读，
// 一旦桥接进程卡死（例如 LibreHardwareMonitor 在睡眠/唤醒前后卡在传感器读取上），
// 调用方会在持有 mutex 的情况下永久阻塞，进而拖死温度监控循环、挂起/唤醒回调等所有
// 依赖桥接的路径。因此所有 stdout 读取都由一个持久的行读取协程完成，命令的收发通过
// channel + 超时实现真正可取消的等待。
type Manager struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	lineCh    chan string
	pipeName  string
	transport string
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
	bridgeProcessExitWait       = 3 * time.Second
	bridgeStartupTimeout        = 30 * time.Second

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

	if m.stdin != nil && m.lineCh != nil {
		if isProcessRunning(m.cmd) {
			m.setState(BridgeStateRunning, nil)
			return nil
		}

		err := fmt.Errorf("bridge process exited unexpectedly")
		m.logger.Error("检测到桥接进程已意外退出，准备重新启动")
		m.setState(BridgeStateDegraded, err)
		m.teardownProcessUnsafe(false)
	}

	return m.startStdio()
}

func (m *Manager) startStdio() error {
	m.setState(BridgeStateStarting, nil)

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

	cmd := exec.Command(bridgePath)
	configureCmdSysProcAttr(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("创建 stdin 管道失败: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("创建 stdout 管道失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("创建 stderr 管道失败: %v", err)
	}

	startAt := time.Now()
	if err := cmd.Start(); err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("启动桥接程序失败: %v", err)
	}

	go m.forwardBridgeStderr(stderr)

	lineCh := startLineReader(stdout)
	if err := m.waitForReady(lineCh, bridgeStartupTimeout); err != nil {
		m.logger.Error("桥接程序启动握手失败（已等待 %s）: %v", time.Since(startAt).Round(time.Millisecond), err)
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		// 异步回收进程，避免句柄/僵尸进程泄漏；Wait 会同时关闭残留管道。
		go func() { _ = cmd.Wait() }()
		go drainLines(lineCh)
		m.setState(BridgeStateFailed, err)
		return err
	}

	m.cmd = cmd
	m.stdin = stdin
	m.stdout = stdout
	m.lineCh = lineCh
	m.pipeName = ""
	m.transport = "stdio"
	m.ownsCmd = true
	m.setState(BridgeStateRunning, nil)
	m.logger.Info("桥接程序启动成功（耗时 %s），通信方式: stdio", time.Since(startAt).Round(time.Millisecond))
	return nil
}

// forwardBridgeStderr 把桥接进程的 stderr 输出转发到日志。
func (m *Manager) forwardBridgeStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 4096), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[init]") {
			m.logger.Info("桥接程序: %s", line)
		} else {
			m.logger.Error("桥接程序 stderr: %s", line)
		}
	}
	if err := scanner.Err(); err != nil {
		m.logger.Debug("读取桥接程序 stderr 失败: %v", err)
	}
}

// startLineReader 启动持久的 stdout 行读取协程，所有 stdout 数据均经由返回的 channel 交付。
func startLineReader(stdout io.Reader) chan string {
	ch := make(chan string, 8)
	go func() {
		defer close(ch)
		reader := bufio.NewReaderSize(stdout, 64*1024)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				ch <- strings.TrimRight(line, "\r\n")
			}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

// drainLines 排空行通道，让阻塞在发送上的读取协程得以退出，避免协程泄漏。
func drainLines(ch <-chan string) {
	for range ch {
	}
}

func (m *Manager) waitForReady(lineCh <-chan string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("等待桥接程序启动握手超时 (timeout=%s)", timeout)
		}

		timer := time.NewTimer(remaining)
		select {
		case line, ok := <-lineCh:
			timer.Stop()
			if !ok {
				return fmt.Errorf("桥接程序在输出启动握手前已退出")
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.EqualFold(line, "READY:STDIO"):
				return nil
			case strings.HasPrefix(line, "ERROR:"):
				return fmt.Errorf("桥接程序启动失败: %s", strings.TrimSpace(strings.TrimPrefix(line, "ERROR:")))
			case line == "":
				continue
			default:
				// 容忍握手前的额外输出（如调试信息），继续等待真正的握手，避免误判启动失败。
				m.logger.Debug("等待桥接握手时收到额外输出: %s", line)
				continue
			}
		case <-timer.C:
			return fmt.Errorf("等待桥接程序启动握手超时 (timeout=%s)", timeout)
		}
	}
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

func (m *Manager) SendCommand(cmdType, data string) (*types.BridgeResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendCommandUnsafe(cmdType, data)
}

func (m *Manager) sendCommandUnsafe(cmdType, data string) (*types.BridgeResponse, error) {
	if m.stdin == nil || m.lineCh == nil {
		return nil, fmt.Errorf("桥接程序未连接")
	}

	timeout := bridgeCommandTimeoutFor(cmdType)

	// 先丢弃残留的过期输出，防止上一条超时命令的迟到响应被错配到本次请求。
drainLoop:
	for {
		select {
		case line, ok := <-m.lineCh:
			if !ok {
				err := fmt.Errorf("桥接程序输出流已关闭")
				m.failBridgeUnsafe(err)
				return nil, err
			}
			m.logger.Debug("丢弃桥接程序过期输出: %s", line)
		default:
			break drainLoop
		}
	}

	cmdBytes, err := json.Marshal(types.BridgeCommand{
		Type: cmdType,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化命令失败: %v", err)
	}

	// 写入也可能在桥接进程卡死、管道缓冲填满时阻塞，同样需要超时保护。
	stdin := m.stdin
	writeDone := make(chan error, 1)
	go func() {
		_, werr := stdin.Write(append(cmdBytes, '\n'))
		writeDone <- werr
	}()

	writeTimer := time.NewTimer(timeout)
	select {
	case werr := <-writeDone:
		writeTimer.Stop()
		if werr != nil {
			m.failBridgeUnsafe(werr)
			return nil, fmt.Errorf("发送 %s 命令失败 (timeout=%s): %v", cmdType, timeout, werr)
		}
	case <-writeTimer.C:
		timeoutErr := fmt.Errorf("发送 %s 命令超时 (timeout=%s)", cmdType, timeout)
		m.failBridgeUnsafe(timeoutErr)
		return nil, timeoutErr
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			timeoutErr := fmt.Errorf("读取 %s 响应超时 (timeout=%s)", cmdType, timeout)
			m.failBridgeUnsafe(timeoutErr)
			return nil, timeoutErr
		}

		readTimer := time.NewTimer(remaining)
		select {
		case line, ok := <-m.lineCh:
			readTimer.Stop()
			if !ok {
				readErr := fmt.Errorf("读取 %s 响应失败: 桥接程序输出流已关闭", cmdType)
				m.failBridgeUnsafe(readErr)
				return nil, readErr
			}

			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			var response types.BridgeResponse
			if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
				// 跳过混入的非 JSON 输出，在剩余时间内继续等待真正的响应。
				m.logger.Debug("忽略桥接程序非 JSON 输出: %s", trimmed)
				continue
			}

			if m.ownsCmd {
				m.setState(BridgeStateRunning, nil)
			} else {
				m.setState(BridgeStateAttached, nil)
			}
			return &response, nil
		case <-readTimer.C:
			timeoutErr := fmt.Errorf("读取 %s 响应超时 (timeout=%s)", cmdType, timeout)
			m.failBridgeUnsafe(timeoutErr)
			return nil, timeoutErr
		}
	}
}

// failBridgeUnsafe 在命令失败/超时后将桥接标记为降级，并终止已经不可信任的自有进程，
func (m *Manager) failBridgeUnsafe(err error) {
	m.setState(BridgeStateDegraded, err)
	m.teardownProcessUnsafe(true)
}

// teardownProcessUnsafe 关闭通信管道并按需终止/回收桥接进程。调用方必须持有 m.mutex。
func (m *Manager) teardownProcessUnsafe(kill bool) {
	cmd := m.cmd
	ownsCmd := m.ownsCmd
	m.cmd = nil
	m.ownsCmd = false
	m.pipeName = ""
	m.closeConnUnsafe()

	if cmd == nil || cmd.Process == nil {
		return
	}
	if ownsCmd {
		if kill {
			_ = cmd.Process.Kill()
		}
		// 异步回收，避免阻塞调用方；Wait 同时会关闭残留的管道句柄。
		go func() { _ = cmd.Wait() }()
	} else {
		_ = cmd.Process.Release()
	}
}

func (m *Manager) closeConnUnsafe() {
	if m.stdin != nil {
		_ = m.stdin.Close()
		m.stdin = nil
	}
	if m.stdout != nil {
		_ = m.stdout.Close()
		m.stdout = nil
	}
	if m.lineCh != nil {
		// 排空通道，让读取协程随管道关闭尽快退出，避免其阻塞在发送上泄漏。
		go drainLines(m.lineCh)
		m.lineCh = nil
	}
	m.transport = ""
}

func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stopUnsafe()
}

func (m *Manager) stopUnsafe() {
	if m.cmd == nil && m.stdin == nil {
		m.setState(BridgeStateStopped, nil)
		return
	}

	m.setState(BridgeStateStopping, nil)

	if m.stdin != nil && m.ownsCmd {
		// Exit 命令带 2 秒超时；若失败，sendCommandUnsafe 内部已终止并回收进程。
		if _, err := m.sendCommandUnsafe("Exit", ""); err != nil {
			m.logger.Debug("发送 Exit 命令失败（进程将被强制终止）: %v", err)
		}
	}

	ownedCmd := m.cmd
	ownsCmd := m.ownsCmd
	m.cmd = nil
	m.ownsCmd = false
	m.pipeName = ""
	m.closeConnUnsafe()

	if ownsCmd && ownedCmd != nil && ownedCmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- ownedCmd.Wait()
		}()

		exitTimer := time.NewTimer(bridgeProcessExitWait)
		select {
		case <-done:
			exitTimer.Stop()
		case <-exitTimer.C:
			m.logger.Error("等待桥接进程退出超时（%s），强制终止", bridgeProcessExitWait)
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
	transport := m.transport
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
			"transport":   transport,
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
	transport = m.transport
	lastError = m.lastError
	m.mutex.Unlock()

	return map[string]any{
		"exists":      true,
		"path":        bridgePath,
		"working":     testResult.Success,
		"state":       state,
		"ownsProcess": ownsCmd,
		"pipeName":    pipeName,
		"transport":   transport,
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

// Package tray 提供系统托盘管理功能
package tray

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"fyne.io/systray"
	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/coremsg"
	"github.com/TIANLI0/THRM/internal/logger"
	"github.com/TIANLI0/THRM/internal/mainthread"
	"github.com/TIANLI0/THRM/internal/types"
)

const (
	// trayAutoStartSettleDelay 自启动首次注册托盘前，要求任务栏通知区域持续稳定的时长。
	trayAutoStartSettleDelay = 3 * time.Second
	// trayFirstRegisterSettleDelay 非自启动的首次注册所要求的稳定时长。看门狗重启、
	// GUI 首次拉起核心也可能落在登录初期，只是通常已晚于登录高峰，等待取得更短。
	trayFirstRegisterSettleDelay = 1 * time.Second
	// trayAutoStartSettleTimeout 等待通知区域稳定的最长时间，超时后仍会尝试注册以免永不显示。
	trayAutoStartSettleTimeout = 25 * time.Second
	// trayLoginPhaseWindow 开机后视为登录阶段的时长；超过它才启动的实例不再等待稳定。
	trayLoginPhaseWindow = 3 * time.Minute
	// trayNotifyAreaWatchWindow 注册图标后继续监视通知区域重建的时长，过后交给定期健康检查。
	trayNotifyAreaWatchWindow = 30 * time.Second
	// trayIconCheckInterval 是健康监控检查通知区域是否被重建的间隔，只是一次窗口查找，代价很低。
	trayIconCheckInterval = 30 * time.Second
	// trayIconFallbackRefresh 是无条件重设托盘图标的兜底间隔。每次重设都要对图标做 md5、
	// 写临时文件检查并跨进程 Shell_NotifyIcon，Explorer 还会随之重绘；重建已由句柄检查与
	// TaskbarCreated 覆盖，兜底只需应对两者都漏掉的情况。
	trayIconFallbackRefresh = 10 * time.Minute
	// trayReadyRecoveryDelay is the grace period before rebuilding a systray
	// instance that is initialized but never becomes ready.
	trayReadyRecoveryDelay = 75 * time.Second
	// trayRestartThrottle prevents repeated restart requests while the
	// supervisor is already trying to recover the tray.
	trayRestartThrottle = 45 * time.Second
	// trayStatusRefreshInterval balances fresh visible status with the cost of
	// cross-thread Windows tray updates. Fan control is independent of this UI refresh.
	trayStatusRefreshInterval = 5 * time.Second
	// maxSystrayInstances 限制单个进程生命周期内创建 systray 实例的总次数，
	// 理由见 systrayBudgetExhausted。取 200：正常使用下 Explorer 重启、休眠唤醒、
	// 可见性开关加起来也远达不到，而进程级回调槽位上限约为它的十倍。
	maxSystrayInstances = 200
)

// Manager 系统托盘管理器
type Manager struct {
	logger          types.Logger
	initialized     int32 // atomic: 0=未初始化, 1=已初始化
	readyState      int32 // atomic: 0=未就绪, 1=就绪
	mutex           sync.Mutex
	done            chan struct{} // 关闭此通道以通知所有 goroutine 退出（进程级，仅退出时关闭）
	uiQueue         chan func()
	iconData        []byte
	menuItems       *MenuItems
	onShowWindow    func()
	onQuit          func()
	onToggleAuto    func() bool
	onSetCurve      func(profileID string) string
	getCurveOptions func() ([]CurveOption, string)
	getStatus       func() Status
	curveMenuItems  map[string]*systray.MenuItem

	superviseOnce sync.Once
	instanceMu    sync.Mutex
	instanceDone  chan struct{}

	// 监控托盘健康状态
	lastIconRefresh  atomic.Int64
	consecutiveFails atomic.Int32 // 连续失败计数
	readyFalseSince  atomic.Int64
	lastRestartTry   atomic.Int64

	// 防止托盘动作重入导致偶发无响应
	showWindowInFlight int32
	toggleAutoInFlight int32
	quitInFlight       int32

	// 开机自启动相关：自启动时延时注册，等待任务栏通知区域稳定后再注册托盘
	autoStartLaunch int32 // atomic: 1=本次进程由开机自启动触发
	instanceCount   int32 // atomic: systray 实例运行计数，用于识别首次注册

	// 托盘可见性：用户可在设置里关闭系统托盘图标。关闭期间监督协程停在等待状态，
	// 不再创建 systray 实例；重新打开时经 enableCh 唤醒它重新注册图标。
	disabled int32         // atomic: 1=用户已关闭托盘
	enableCh chan struct{} // 容量 1 的唤醒信号

	// lastTooltip 是最近一次按真实状态渲染的悬浮提示（string）。图标刷新必须原样恢复它：
	// 状态轮询只在状态变化时才重写提示，刷新若换成通用文案，真实状态会一直丢到下次变化。
	lastTooltip atomic.Value
}

// MenuItems 托盘菜单项结构
type MenuItems struct {
	Show           *systray.MenuItem
	DeviceStatus   *systray.MenuItem
	CPUTemperature *systray.MenuItem
	GPUTemperature *systray.MenuItem
	CPUPower       *systray.MenuItem
	GPUPower       *systray.MenuItem
	FanSpeed       *systray.MenuItem
	CurveSelect    *systray.MenuItem
	AutoControl    *systray.MenuItem
	Quit           *systray.MenuItem
}

// CurveOption 托盘曲线选项
type CurveOption struct {
	ID   string
	Name string
}

// Status 状态信息
type Status struct {
	Connected bool
	CPUTemp   int
	GPUTemp   int
	// 功耗读数并非所有机型都有（需要 PawnIO 且 CPU/GPU 支持），0 表示读不到。
	// 读不到时托盘不显示对应条目，而不是显示一个会误导人的 0 W。
	CPUPower float64
	GPUPower float64
	// GPUMonitoringDisabled 表示用户主动关闭了 GPU 监测（混合显卡防止独显被唤醒）。
	// 这与"读不到"是两码事：读不到显示"无数据"是有用的诊断信息，而用户自己关掉的
	// 东西再摆在菜单里只会让人以为出了故障，直接隐藏。
	GPUMonitoringDisabled bool
	CurrentRPM            uint16
	AutoControlState      bool
	ActiveCurveProfileID  string
	CurveProfiles         []CurveOption
}

// trayTooltipMaxUnits 是 Windows 通知图标提示文本的硬上限（UTF-16 单元）。
//
// fyne.io/systray 的结构体里 Tip 虽然是 [128]uint16，但它从不调用 NIM_SETVERSION，
// 于是 shell 按旧版协议处理，实际只显示前 64 个单元——而且是直接截断，不给任何提示。
// 早先那版三行提示正好 65 个单元，用户看到的是"风扇: 180"，最后一位被吃掉了。
const trayTooltipMaxUnits = 64

// utf16Len 按 UTF-16 单元数计长度。中文和数字都在 BMP 内各占 1 个单元，
// 补充平面字符（表情之类）占 2 个，用 rune 数会低估。
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// buildTrayTooltip 在 64 单元预算内拼装提示文本。
//
// 放不下就整行丢弃，绝不交给 Windows 从中间截断——被切掉半个数字的"风扇: 180"
// 比没有这一行更糟，用户会把它当成真实读数。
func buildTrayTooltip(header string, lines []string) string {
	out := header
	for _, line := range lines {
		if line == "" {
			continue
		}
		candidate := out + "\n" + line
		if utf16Len(candidate) > trayTooltipMaxUnits {
			break
		}
		out = candidate
	}
	return out
}

// formatTooltipReadings 拼出悬浮提示里的读数行。
//
// 温度与功耗按设备合并成一行（"CPU 54°C 28W"）而不是各占一行：预算只有 64 个单元，
// 分行写光是标签就要吃掉十几个。功耗取整——悬浮提示是用来扫一眼的，小数位不值那两格。
//
// 可见性与右键菜单同一套规则：用户关掉 GPU 监测就不提 GPU，功耗读不到就不提功耗。
func formatTooltipReadings(status Status) []string {
	cpu := fmt.Sprintf("CPU %d°C", status.CPUTemp)
	if status.CPUPower > 0 {
		cpu += fmt.Sprintf(" %.0fW", status.CPUPower)
	}
	readings := cpu

	if !status.GPUMonitoringDisabled {
		gpu := fmt.Sprintf("GPU %d°C", status.GPUTemp)
		if status.GPUPower > 0 {
			gpu += fmt.Sprintf(" %.0fW", status.GPUPower)
		}
		readings += "  " + gpu
	}

	lines := []string{readings}
	if status.CurrentRPM > 0 {
		lines = append(lines, coremsg.Tf(coremsg.TrayTooltipFan, status.CurrentRPM))
	}
	return lines
}

// statusTooltip 按设备状态生成悬浮提示。
func statusTooltip(status Status) string {
	if !status.Connected {
		return coremsg.Tf(coremsg.TrayTooltipDisconnected, appmeta.AppName)
	}
	if status.AutoControlState {
		return buildTrayTooltip(coremsg.Tf(coremsg.TrayTooltipSmartControl, appmeta.AppName), formatTooltipReadings(status))
	}
	return buildTrayTooltip(coremsg.Tf(coremsg.TrayTooltipManualMode, appmeta.AppName), formatTooltipReadings(status))
}

// runningTooltip 是尚未渲染真实状态时的通用悬浮提示。
func runningTooltip() string {
	return coremsg.Tf(coremsg.TrayTooltipRunning, appmeta.AppName)
}

// setTooltip 设置悬浮提示并记住它，供图标刷新时原样恢复。只能在 UI 队列中调用。
func (m *Manager) setTooltip(text string) {
	m.lastTooltip.Store(text)
	systray.SetTooltip(text)
}

// currentTooltip 返回最近一次渲染的悬浮提示；尚未渲染过真实状态时退回通用文案。
func (m *Manager) currentTooltip() string {
	if text, ok := m.lastTooltip.Load().(string); ok && text != "" {
		return text
	}
	return runningTooltip()
}

func statusEqual(left, right Status) bool {
	if left.Connected != right.Connected ||
		left.CPUTemp != right.CPUTemp ||
		left.GPUTemp != right.GPUTemp ||
		left.CPUPower != right.CPUPower ||
		left.GPUPower != right.GPUPower ||
		left.GPUMonitoringDisabled != right.GPUMonitoringDisabled ||
		left.CurrentRPM != right.CurrentRPM ||
		left.AutoControlState != right.AutoControlState ||
		left.ActiveCurveProfileID != right.ActiveCurveProfileID ||
		len(left.CurveProfiles) != len(right.CurveProfiles) {
		return false
	}
	for i := range left.CurveProfiles {
		if left.CurveProfiles[i] != right.CurveProfiles[i] {
			return false
		}
	}
	return true
}

// NewManager 创建新的托盘管理器
func NewManager(logger types.Logger, iconData []byte) *Manager {
	return &Manager{
		logger:         logger,
		done:           make(chan struct{}),
		uiQueue:        make(chan func(), 64),
		iconData:       iconData,
		curveMenuItems: make(map[string]*systray.MenuItem),
		enableCh:       make(chan struct{}, 1),
	}
}

// SetCallbacks 设置回调函数
func (m *Manager) SetCallbacks(
	onShowWindow func(),
	onQuit func(),
	onToggleAuto func() bool,
	onSetCurve func(profileID string) string,
	getCurveOptions func() ([]CurveOption, string),
	getStatus func() Status,
) {
	m.onShowWindow = onShowWindow
	m.onQuit = onQuit
	m.onToggleAuto = onToggleAuto
	m.onSetCurve = onSetCurve
	m.getCurveOptions = getCurveOptions
	m.getStatus = getStatus
}

// SetAutoStartLaunch 标记本次进程是否由开机自启动触发。
//
// 自启动场景下，托盘会在首次注册前等待任务栏通知区域稳定，避免开机快速启动时
// 因通知区域尚未就绪导致图标被静默丢弃。应在 Init 之前调用。
func (m *Manager) SetAutoStartLaunch(v bool) {
	if v {
		atomic.StoreInt32(&m.autoStartLaunch, 1)
	} else {
		atomic.StoreInt32(&m.autoStartLaunch, 0)
	}
}

func (m *Manager) isAutoStartLaunch() bool {
	return atomic.LoadInt32(&m.autoStartLaunch) == 1
}

// SetEnabled 设置系统托盘图标是否可见。
//
// 关闭时立即结束当前 systray 消息循环以移除通知区域图标，并让监督协程停止重建；
// 重新打开时唤醒监督协程重新注册。可在 Init 之前调用，用于决定启动时是否显示托盘。
func (m *Manager) SetEnabled(enabled bool) {
	if enabled {
		if !atomic.CompareAndSwapInt32(&m.disabled, 1, 0) {
			return
		}
		m.logInfo("系统托盘已启用，准备重新注册托盘图标")
		select {
		case m.enableCh <- struct{}{}:
		default:
		}
		return
	}

	if !atomic.CompareAndSwapInt32(&m.disabled, 0, 1) {
		return
	}
	m.logInfo("系统托盘已在设置中关闭，正在移除托盘图标")
	atomic.StoreInt32(&m.readyState, 0)
	// 尚未 Init 时消息循环还没起来，无需（也不应）调用 systray.Quit；
	// 监督协程会在启动前看到 disabled 并直接停在等待状态。
	if atomic.LoadInt32(&m.initialized) == 0 {
		return
	}
	go m.quitSystrayInstance()
}

// systrayBudgetExhausted 报告本进程是否已用尽允许创建的 systray 实例数。
//
// 每建一次实例，systray 的 initInstance 都会调用 windows.NewCallback 注册窗口过程，
// 而回调槽位是进程级的、数量有限且永不回收。托盘若陷入"建起来就失效"的病态循环，
// 无限重试最终会耗尽槽位，此后连 initInstance 自己都会 panic——重建从此静默失效，
// 现场也没有任何线索。到达上限时明确放弃，把状态变成一条可操作的日志。
func (m *Manager) systrayBudgetExhausted() bool {
	return atomic.LoadInt32(&m.instanceCount) >= maxSystrayInstances
}

// quitSystrayInstance 结束当前 systray 消息循环。
//
// 不能直接用 systray.Quit()：它被包级 `quitOnce sync.Once` 保护，而 Register/Run 都不
// 重置该 Once，因此整个进程生命周期内只有第一次调用真正生效。托盘的每条自愈路径
// （ready 超时重建、图标/菜单创建失败重建、可见性开关）都靠"结束消息循环 → 监督协程
// 重建实例"来恢复，一旦这个 Once 被用掉（比如用户开关过一次托盘可见性），后续调用全
// 变成静默空操作：核心进程还在跑、风扇还在控，托盘图标却再也回不来，用户看到的就是
// "后台没了"。因此优先直接向本进程的托盘窗口投递 WM_CLOSE——那正是 systray.Quit()
// 内部所做的事——只在拿不到窗口句柄时才退回 systray.Quit()。
func (m *Manager) quitSystrayInstance() {
	defer func() {
		if r := recover(); r != nil {
			m.logDebug("结束系统托盘消息循环时发生错误（可忽略）: %v", r)
		}
	}()

	if postSystrayClose() {
		return
	}
	systray.Quit()
}

// IsEnabled 返回系统托盘图标当前是否处于启用状态。
func (m *Manager) IsEnabled() bool {
	return atomic.LoadInt32(&m.disabled) == 0
}

func (m *Manager) isDisabled() bool {
	return atomic.LoadInt32(&m.disabled) == 1
}

// waitForEnable 阻塞直到托盘被重新启用，返回 false 表示进程正在退出。
func (m *Manager) waitForEnable() bool {
	m.logDebug("系统托盘处于关闭状态，等待被重新启用")
	for {
		select {
		case <-m.done:
			return false
		case <-m.enableCh:
			if !m.isDisabled() {
				return true
			}
		}
	}
}

// Init 初始化系统托盘
func (m *Manager) Init() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已经初始化
	if !atomic.CompareAndSwapInt32(&m.initialized, 0, 1) {
		m.logDebug("托盘已经初始化，跳过重复初始化")
		return
	}

	m.logInfo("正在初始化系统托盘")

	// 启动监督协程：负责等待外壳就绪、运行 systray，并在消息循环异常退出后自动重建。
	m.superviseOnce.Do(func() {
		go m.supervise()
	})
}

// supervise 监督系统托盘实例，确保其在意外退出后能够自动恢复。
//
// 正常情况下 systray 的消息循环会一直阻塞直到进程退出；只有当消息循环因错误/外部原因
// 退出时，本协程才会重建实例。这能应对“开机自启动时外壳未就绪”以及“休眠唤醒/Explorer
// 重启后消息循环失效”导致的托盘永久失效问题。
func (m *Manager) supervise() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘监督协程发生panic: %v", r)
		}
	}()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-m.done:
			return
		default:
		}

		// 用户在设置里关掉了托盘：不创建实例，停在这里等重新启用。
		if m.isDisabled() {
			if !m.waitForEnable() {
				return
			}
			backoff = time.Second
			continue
		}

		if m.systrayBudgetExhausted() {
			m.logError("系统托盘已重建 %d 次仍未稳定，停止继续重建以免耗尽系统资源；请重启 THRM 核心服务",
				atomic.LoadInt32(&m.instanceCount))
			return
		}

		ran := m.runSystrayInstance()

		select {
		case <-m.done:
			return
		default:
		}

		// 消息循环是被 SetEnabled 主动结束的，不算异常退出，也不该走重建退避。
		if m.isDisabled() {
			continue
		}
		select {
		case <-m.enableCh:
			// 刚关掉又立刻打开：退出同样是我们主动触发的，直接重建。
			continue
		default:
		}

		// 实例已退出但进程未请求退出，说明托盘消息循环异常终止，尝试重建。
		if ran > 60*time.Second {
			backoff = time.Second // 长时间正常运行后重置退避
		}
		m.logError("系统托盘消息循环已退出（运行时长 %v），%v 后尝试重建托盘", ran.Round(time.Second), backoff)

		select {
		case <-m.done:
			return
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// runSystrayInstance 运行一次完整的 systray 生命周期，阻塞直到消息循环退出，返回本次运行时长。
func (m *Manager) runSystrayInstance() (ran time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘实例运行过程中发生panic: %v", r)
		}
	}()

	// 为本次实例创建独立的停止信号，旧实例的附属 goroutine 据此退出。
	instanceDone := make(chan struct{})
	m.instanceMu.Lock()
	m.instanceDone = instanceDone
	m.menuItems = nil
	m.curveMenuItems = make(map[string]*systray.MenuItem)
	m.instanceMu.Unlock()
	atomic.StoreInt32(&m.readyState, 0)
	m.readyFalseSince.Store(time.Now().Unix())

	// 等待 Windows 外壳就绪，避免 Shell_NotifyIcon(NIM_ADD) 在外壳未启动时失败。
	if !waitForShellReady(m.done, 60*time.Second) {
		close(instanceDone)
		return 0
	}

	// 外壳窗口可能创建得很早，但通知区域尚未稳定，过早注册会让图标被静默丢弃。
	// 首次注册一律追加稳定等待（自启动等更久）；后续由 Explorer 重启等触发的重建
	// 已经晚于登录高峰，systray 自己会响应 TaskbarCreated，无需再延时。
	if atomic.AddInt32(&m.instanceCount, 1) == 1 {
		settle := trayFirstRegisterSettleDelay
		if m.isAutoStartLaunch() {
			settle = trayAutoStartSettleDelay
		}
		// 开机很久之后才启动时任务栏早已稳定，再等只是延后图标出现；
		// 万一之后仍被重建，watchNotifyAreaRebuild 会补上。
		if systemUptime() > trayLoginPhaseWindow {
			settle = 0
		}
		if settle > 0 {
			m.logInfo("等待任务栏通知区域稳定 %v 后再注册系统托盘", settle)
			waitForTraySettle(m.done, settle, trayAutoStartSettleTimeout)
			select {
			case <-m.done:
				close(instanceDone)
				return 0
			default:
			}
			m.logInfo("任务栏通知区域已就绪，开始注册系统托盘")
		}
	}

	// 等待期间用户可能已经关掉了托盘：此时不要注册图标，让监督协程停在等待状态。
	if m.isDisabled() {
		close(instanceDone)
		return 0
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	start := time.Now()
	// macOS 的 NSApp 只能在进程主线程上运行，mainthread.Call 把消息循环交给主线程；
	// 其它平台就地执行，仍在当前已锁定的线程上。
	mainthread.Call(func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘消息循环发生panic: %v", r)
			}
		}()
		prepareNativeTrayApp()
		// onReady 由 systray 在内部 goroutine 触发；消息循环在此阻塞。
		systray.Run(m.onTrayReady, m.onTrayExit)
	})
	ran = time.Since(start)

	atomic.StoreInt32(&m.readyState, 0)
	m.readyFalseSince.Store(time.Now().Unix())
	// 通知本实例的附属 goroutine 退出。
	close(instanceDone)
	return ran
}

// currentInstanceDone 返回当前实例的停止信号通道。
func (m *Manager) currentInstanceDone() <-chan struct{} {
	m.instanceMu.Lock()
	defer m.instanceMu.Unlock()
	return m.instanceDone
}

// onTrayReady 托盘准备就绪时的回调
func (m *Manager) onTrayReady() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘回调函数中发生panic: %v", r)
			atomic.StoreInt32(&m.readyState, 0)
		}
	}()

	m.logInfo("托盘回调函数已启动")

	if err := m.setupIcon(); err != nil {
		m.logError("设置托盘图标失败: %v", err)
		atomic.StoreInt32(&m.readyState, 0)
		m.quitSystrayInstance()
		return
	}

	// 左键单击托盘图标：显示主窗口；右键保持默认行为（打开托盘菜单）
	systray.SetOnTapped(func() {
		m.logDebug("托盘图标左键点击: 显示主窗口")
		if m.onShowWindow != nil {
			m.runTrayActionAsync("icon-show-window", &m.showWindowInFlight, m.onShowWindow)
		}
	})

	// 创建托盘菜单
	menuItems, err := m.createMenu()
	if err != nil {
		m.logError("创建托盘菜单失败: %v", err)
		atomic.StoreInt32(&m.readyState, 0)
		m.quitSystrayInstance()
		return
	}
	m.menuItems = menuItems
	instanceDone := m.currentInstanceDone()
	m.startUIWorker(instanceDone)

	atomic.StoreInt32(&m.readyState, 1)
	m.readyFalseSince.Store(0)
	m.lastIconRefresh.Store(time.Now().Unix())
	m.consecutiveFails.Store(0)
	m.logInfo("系统托盘初始化完成")

	// 处理托盘菜单事件
	go m.handleMenuEvents(instanceDone)

	// 定期更新托盘菜单状态
	go m.updateMenuStatus(instanceDone)

	// 启动托盘健康监控（定期刷新图标以应对 Explorer 重启等）
	go m.startIconHealthMonitor(instanceDone)

	// 盯住注册后的一小段时间：通知区域若在此期间被重建，立刻补一次图标。
	go m.watchNotifyAreaRebuild(instanceDone, notifyAreaWindow())
}

// watchNotifyAreaRebuild 在注册图标后监视通知区域是否被重建，重建后立即补加图标。
//
// systray 靠 Explorer 广播的 TaskbarCreated 自行重添，但图标加到一个随即被销毁的
// 通知区域上、广播又早于我们建窗时，既收不到广播图标也已经丢了。此时只有句柄变化
// 能反映出来，比等定期健康检查快得多。
func (m *Manager) watchNotifyAreaRebuild(instanceDone <-chan struct{}, registeredAt uintptr) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("通知区域监视协程发生panic: %v", r)
		}
	}()

	if registeredAt == 0 {
		return // 非 Windows，或注册时通知区域已消失（健康检查会兜住）
	}

	deadline := time.Now().Add(trayNotifyAreaWatchWindow)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-instanceDone:
			return
		case <-m.done:
			return
		case <-ticker.C:
			current := notifyAreaWindow()
			if current != 0 && current != registeredAt {
				m.logInfo("检测到任务栏通知区域已重建，重新添加托盘图标")
				registeredAt = current
				m.refreshTrayIcon()
			}
			if time.Now().After(deadline) {
				return
			}
		}
	}
}

// setupIcon 设置托盘图标
func (m *Manager) setupIcon() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("设置托盘图标时发生panic: %v", r)
		}
	}()

	if len(m.iconData) == 0 {
		return fmt.Errorf("托盘图标数据为空")
	}

	systray.SetIcon(m.iconData)
	systray.SetTitle(appmeta.AppName)
	systray.SetTooltip(runningTooltip())
	return nil
}

// createMenu 创建托盘菜单
func (m *Manager) createMenu() (items *MenuItems, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("创建托盘菜单时发生panic: %v", r)
		}
	}()

	items = &MenuItems{}

	items.Show = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuShow), coremsg.T(coremsg.TrayMenuShowTip))
	systray.AddSeparator()

	items.DeviceStatus = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuDeviceStatus), coremsg.T(coremsg.TrayMenuDeviceStatusTip))
	items.DeviceStatus.Disable()

	items.CPUTemperature = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuCPUTemp), coremsg.T(coremsg.TrayMenuCPUTempTip))
	items.CPUTemperature.Disable()

	items.GPUTemperature = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuGPUTemp), coremsg.T(coremsg.TrayMenuGPUTempTip))
	items.GPUTemperature.Disable()

	items.CPUPower = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuCPUPower), coremsg.T(coremsg.TrayMenuCPUPowerTip))
	items.CPUPower.Disable()
	items.CPUPower.Hide()

	items.GPUPower = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuGPUPower), coremsg.T(coremsg.TrayMenuGPUPowerTip))
	items.GPUPower.Disable()
	items.GPUPower.Hide()

	items.FanSpeed = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuFanSpeed), coremsg.T(coremsg.TrayMenuFanSpeedTip))
	items.FanSpeed.Disable()
	items.CurveSelect = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuCurveSelect), coremsg.T(coremsg.TrayMenuCurveSelectTip))

	if m.getCurveOptions != nil {
		profiles, activeID := m.getCurveOptions()
		m.ensureCurveMenuItems(items.CurveSelect, profiles)
		m.updateCurveMenuSelection(activeID)
	}

	// 智能变频状态 - 获取当前配置状态
	autoControlEnabled := false
	if m.getStatus != nil {
		autoControlEnabled = m.getStatus().AutoControlState
	}
	items.AutoControl = systray.AddMenuItemCheckbox(coremsg.T(coremsg.TrayMenuAutoControl), coremsg.T(coremsg.TrayMenuAutoControlTip), autoControlEnabled)

	systray.AddSeparator()
	items.Quit = systray.AddMenuItem(coremsg.T(coremsg.TrayMenuQuit), coremsg.T(coremsg.TrayMenuQuitTip))

	return items, nil
}

// RefreshLanguage 在界面语言切换后立即按新语言重写托盘文字（菜单项、提示、悬浮提示）。
//
// 只改已有菜单项的标题与提示，不重建菜单：systray 的菜单项创建后无法删除，
// 重建会在每次切换语言时多出一整套隐藏项。托盘尚未就绪时无需处理，下次建菜单
// 自然会用新语言；但要丢掉旧语言的悬浮提示缓存，免得图标刷新把它恢复回来。
// 调用方不得持有核心的配置锁：这里会同步取一次状态。
func (m *Manager) RefreshLanguage() {
	if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 || m.isDisabled() {
		m.lastTooltip.Store("")
		return
	}

	var status Status
	hasStatus := m.getStatus != nil
	if hasStatus {
		status = m.getStatus()
	}

	m.enqueueUI("refresh-language", func() {
		if m.menuItems == nil {
			return
		}
		m.applyMenuTexts(m.menuItems)
		if hasStatus {
			m.renderStatus(status)
		} else {
			m.setTooltip(runningTooltip())
		}
	})
}

// applyMenuTexts 按当前语言重写不随状态变化的菜单文字。随状态变化的标题由 renderStatus 覆盖。
// 只能在 UI 队列中调用。
func (m *Manager) applyMenuTexts(items *MenuItems) {
	setText := func(item *systray.MenuItem, title, tooltip coremsg.Key) {
		if item == nil {
			return
		}
		item.SetTitle(coremsg.T(title))
		item.SetTooltip(coremsg.T(tooltip))
	}
	setText(items.Show, coremsg.TrayMenuShow, coremsg.TrayMenuShowTip)
	setText(items.DeviceStatus, coremsg.TrayMenuDeviceStatus, coremsg.TrayMenuDeviceStatusTip)
	setText(items.CPUTemperature, coremsg.TrayMenuCPUTemp, coremsg.TrayMenuCPUTempTip)
	setText(items.GPUTemperature, coremsg.TrayMenuGPUTemp, coremsg.TrayMenuGPUTempTip)
	setText(items.CPUPower, coremsg.TrayMenuCPUPower, coremsg.TrayMenuCPUPowerTip)
	setText(items.GPUPower, coremsg.TrayMenuGPUPower, coremsg.TrayMenuGPUPowerTip)
	setText(items.FanSpeed, coremsg.TrayMenuFanSpeed, coremsg.TrayMenuFanSpeedTip)
	setText(items.CurveSelect, coremsg.TrayMenuCurveSelect, coremsg.TrayMenuCurveSelectTip)
	setText(items.AutoControl, coremsg.TrayMenuAutoControl, coremsg.TrayMenuAutoControlTip)
	setText(items.Quit, coremsg.TrayMenuQuit, coremsg.TrayMenuQuitTip)

	for id, item := range m.curveMenuItems {
		if item == nil {
			continue
		}
		if id == "__empty__" {
			item.SetTitle(coremsg.T(coremsg.TrayMenuCurveEmpty))
			continue
		}
		// 曲线项标题是用户起的方案名，不翻译，只更新提示。
		item.SetTooltip(coremsg.T(coremsg.TrayMenuCurveItemTip))
	}
}

// handleMenuEvents 处理托盘菜单事件
func (m *Manager) handleMenuEvents(instanceDone <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("处理托盘菜单事件时发生panic: %v", r)
		}
	}()

	if m.menuItems == nil || m.menuItems.Show == nil || m.menuItems.AutoControl == nil || m.menuItems.Quit == nil {
		m.logError("托盘菜单未正确初始化，无法处理菜单事件")
		return
	}

	for {
		select {
		case <-m.menuItems.Show.ClickedCh:
			m.logDebug("托盘菜单: 显示主窗口")
			if m.onShowWindow != nil {
				m.runTrayActionAsync("menu-show-window", &m.showWindowInFlight, m.onShowWindow)
			}
		case <-m.menuItems.AutoControl.ClickedCh:
			m.logDebug("托盘菜单: 切换智能变频状态")
			if m.onToggleAuto != nil {
				m.runTrayActionAsync("menu-toggle-auto", &m.toggleAutoInFlight, func() {
					newState := m.onToggleAuto()
					m.enqueueUI("menu-toggle-auto-ui", func() {
						if m.menuItems == nil || m.menuItems.AutoControl == nil {
							return
						}
						if newState {
							m.menuItems.AutoControl.Check()
						} else {
							m.menuItems.AutoControl.Uncheck()
						}
					})
				})
			}
		case <-m.menuItems.Quit.ClickedCh:
			m.logInfo("托盘菜单: 用户请求退出应用")
			if m.onQuit != nil {
				m.runTrayActionAsync("menu-quit", &m.quitInFlight, m.onQuit)
			}
			return
		case <-instanceDone:
			return
		case <-m.done:
			return
		}
	}
}

// runTrayActionAsync 异步执行托盘动作，避免阻塞托盘消息处理
func (m *Manager) runTrayActionAsync(action string, inFlight *int32, fn func()) {
	if fn == nil {
		return
	}

	if inFlight != nil && !atomic.CompareAndSwapInt32(inFlight, 0, 1) {
		m.logDebug("托盘动作[%s]仍在执行，忽略重复触发", action)
		return
	}

	go func() {
		startedAt := time.Now()
		defer func() {
			if inFlight != nil {
				atomic.StoreInt32(inFlight, 0)
			}
			if r := recover(); r != nil {
				m.logError("托盘动作[%s]发生panic: %v", action, r)
			}

			d := time.Since(startedAt)
			if d > 800*time.Millisecond {
				m.logError("托盘动作[%s]执行耗时较长: %v", action, d)
			} else {
				m.logDebug("托盘动作[%s]执行完成: %v", action, d)
			}
		}()

		fn()
	}()
}

// updateMenuStatus 定期更新托盘菜单状态
func (m *Manager) updateMenuStatus(instanceDone <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("更新托盘菜单状态时发生panic: %v", r)
		}
	}()

	ticker := time.NewTicker(trayStatusRefreshInterval)
	defer ticker.Stop()
	var previousStatus Status
	hasPreviousStatus := false

	for {
		select {
		case <-ticker.C:
			// 如果托盘不可用，跳过本次更新但不退出，等待恢复
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue
			}

			if m.getStatus == nil {
				continue
			}

			status := m.getStatus()
			if hasPreviousStatus && statusEqual(status, previousStatus) {
				continue
			}
			if m.enqueueUI("update-menu-status", func() {
				m.renderStatus(status)
			}) {
				previousStatus = status
				hasPreviousStatus = true
			}
		case <-instanceDone:
			return
		case <-m.done:
			return
		}
	}
}

// renderStatus 按状态与当前语言重写菜单读数和悬浮提示。只能在 UI 队列中调用。
func (m *Manager) renderStatus(status Status) {
	if m.menuItems == nil {
		return
	}

	if status.Connected {
		m.menuItems.DeviceStatus.SetTitle(coremsg.T(coremsg.TrayMenuDeviceConnected))
	} else {
		m.menuItems.DeviceStatus.SetTitle(coremsg.T(coremsg.TrayMenuDeviceDisconnected))
	}

	if status.CPUTemp > 0 {
		m.menuItems.CPUTemperature.SetTitle(coremsg.Tf(coremsg.TrayMenuCPUTempValue, status.CPUTemp))
	} else {
		m.menuItems.CPUTemperature.SetTitle(coremsg.T(coremsg.TrayMenuCPUTempNoData))
	}

	if status.GPUMonitoringDisabled {
		m.menuItems.GPUTemperature.Hide()
	} else {
		m.menuItems.GPUTemperature.Show()
		if status.GPUTemp > 0 {
			m.menuItems.GPUTemperature.SetTitle(coremsg.Tf(coremsg.TrayMenuGPUTempValue, status.GPUTemp))
		} else {
			m.menuItems.GPUTemperature.SetTitle(coremsg.T(coremsg.TrayMenuGPUTempNoData))
		}
	}

	// 功耗要 PawnIO 加上 CPU/GPU 本身支持才读得到，不少机型没有。
	// 读不到就整行隐藏，而不是摆一个会误导人的 0 W。
	if status.CPUPower > 0 {
		m.menuItems.CPUPower.SetTitle(coremsg.Tf(coremsg.TrayMenuCPUPowerValue, status.CPUPower))
		m.menuItems.CPUPower.Show()
	} else {
		m.menuItems.CPUPower.Hide()
	}
	if status.GPUPower > 0 && !status.GPUMonitoringDisabled {
		m.menuItems.GPUPower.SetTitle(coremsg.Tf(coremsg.TrayMenuGPUPowerValue, status.GPUPower))
		m.menuItems.GPUPower.Show()
	} else {
		m.menuItems.GPUPower.Hide()
	}

	if status.CurrentRPM > 0 {
		m.menuItems.FanSpeed.SetTitle(coremsg.Tf(coremsg.TrayMenuFanSpeedValue, status.CurrentRPM))
	} else {
		m.menuItems.FanSpeed.SetTitle(coremsg.T(coremsg.TrayMenuFanSpeedNoData))
	}

	if m.menuItems.CurveSelect != nil {
		m.ensureCurveMenuItems(m.menuItems.CurveSelect, status.CurveProfiles)
		m.updateCurveMenuSelection(status.ActiveCurveProfileID)
	}

	if status.AutoControlState {
		m.menuItems.AutoControl.Check()
	} else {
		m.menuItems.AutoControl.Uncheck()
	}

	m.setTooltip(statusTooltip(status))
}

// onTrayExit 托盘退出时的回调
//
// 注意：此处只清除就绪状态，不重置 initialized。initialized 表示托盘子系统（监督协程）
// 是否处于活动状态，仅在 Init/Quit 时变更；这样消息循环临时退出并被监督协程重建期间，
// 健康检查与状态上报仍能正确反映子系统在运行。
func (m *Manager) onTrayExit() {
	m.logDebug("托盘退出回调被触发")
	atomic.StoreInt32(&m.readyState, 0)
	m.readyFalseSince.Store(time.Now().Unix())
}

// startIconHealthMonitor 启动托盘图标健康监控
func (m *Manager) startIconHealthMonitor(instanceDone <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘图标健康监控发生panic: %v", r)
		}
	}()

	// 通知区域窗口句柄变化说明 Explorer 重建过，立刻补加图标；否则只按兜底间隔重设。
	ticker := time.NewTicker(trayIconCheckInterval)
	defer ticker.Stop()
	notifyArea := notifyAreaWindow()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue // 不退出，等待恢复
			}
			current := notifyAreaWindow()
			rebuilt := current != 0 && current != notifyArea
			if current != 0 {
				notifyArea = current
			}
			if rebuilt {
				m.logInfo("检测到任务栏通知区域已重建，重新添加托盘图标")
				m.refreshTrayIcon()
			} else if time.Since(time.Unix(m.lastIconRefresh.Load(), 0)) >= trayIconFallbackRefresh {
				m.refreshTrayIcon()
			}
		case <-instanceDone:
			return
		case <-m.done:
			return
		}
	}
}

// refreshTrayIcon 刷新托盘图标
func (m *Manager) refreshTrayIcon() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("刷新托盘图标时发生panic: %v", r)
			m.consecutiveFails.Add(1)
		}
	}()

	queued := m.enqueueUI("refresh-tray-icon", func() {
		if len(m.iconData) == 0 {
			m.consecutiveFails.Add(1)
			m.logError("刷新托盘图标失败: 图标数据为空")
			return
		}

		systray.SetIcon(m.iconData)
		systray.SetTooltip(m.currentTooltip())

		m.consecutiveFails.Store(0)
		m.lastIconRefresh.Store(time.Now().Unix())

		m.logDebug("托盘图标已刷新")
	})

	if !queued {
		m.consecutiveFails.Add(1)
	}
}

func (m *Manager) startUIWorker(instanceDone <-chan struct{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘UI队列处理发生panic: %v", r)
			}
		}()

		for {
			select {
			case fn := <-m.uiQueue:
				if fn != nil {
					fn()
				}
			case <-instanceDone:
				return
			case <-m.done:
				return
			}
		}
	}()
}

func (m *Manager) enqueueUI(action string, fn func()) bool {
	if fn == nil {
		return false
	}

	select {
	case <-m.done:
		return false
	default:
	}

	wrapped := func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘UI动作[%s]发生panic: %v", action, r)
			}
		}()
		fn()
	}

	select {
	case m.uiQueue <- wrapped:
		return true
	default:
		m.logError("托盘UI队列繁忙，丢弃动作: %s", action)
		return false
	}
}

// IsReady 检查托盘是否就绪
func (m *Manager) IsReady() bool {
	return atomic.LoadInt32(&m.readyState) == 1
}

// IsInitialized 检查托盘是否已初始化
func (m *Manager) IsInitialized() bool {
	return atomic.LoadInt32(&m.initialized) == 1
}

// Quit 退出托盘
func (m *Manager) Quit() {
	atomic.StoreInt32(&m.readyState, 0)

	m.mutex.Lock()
	select {
	case <-m.done:
		// 已经关闭
	default:
		close(m.done)
	}
	m.mutex.Unlock()

	m.quitSystrayInstance()
}

// RefreshIcon 主动刷新托盘图标。
//
// 主要用于系统从休眠/睡眠唤醒、或 Explorer 重启后，及时恢复通知区域图标，
// 避免出现图标丢失或显示异常。仅在托盘就绪时生效。
func (m *Manager) RefreshIcon() {
	if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 || m.isDisabled() {
		return
	}
	m.refreshTrayIcon()
}

// CheckHealth 检查托盘健康状态
func (m *Manager) CheckHealth() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("检查托盘健康状态时发生panic: %v", r)
		}
	}()

	// 如果托盘未初始化或已被用户关闭，无需检查
	if atomic.LoadInt32(&m.initialized) == 0 || m.isDisabled() {
		return
	}

	if atomic.LoadInt32(&m.readyState) == 0 {
		m.recoverNotReadyTray()
		return
	}
	m.readyFalseSince.Store(0)

	// 检查图标是否长时间未刷新：健康监控本该按 trayIconFallbackRefresh 重设，
	// 再超出一个检查周期仍没刷新，说明监控协程卡住或刷新一直没成功。
	lastRefresh := m.lastIconRefresh.Load()
	if lastRefresh > 0 && time.Since(time.Unix(lastRefresh, 0)) > trayIconFallbackRefresh+2*trayIconCheckInterval {
		m.logInfo("检测到托盘图标长时间未刷新，尝试刷新")
		m.refreshTrayIcon()
	}

	// 如果连续失败，也强制刷新图标
	if m.consecutiveFails.Load() >= 3 {
		m.logError("检测到托盘连续失败，尝试刷新图标")
		m.refreshTrayIcon()
	}
}

func (m *Manager) recoverNotReadyTray() {
	now := time.Now()
	nowUnix := now.Unix()
	firstUnix := m.readyFalseSince.Load()
	if firstUnix == 0 {
		if m.readyFalseSince.CompareAndSwap(0, nowUnix) {
			firstUnix = nowUnix
		} else {
			firstUnix = m.readyFalseSince.Load()
		}
	}

	notReadyFor := now.Sub(time.Unix(firstUnix, 0))
	if notReadyFor < trayReadyRecoveryDelay {
		m.logDebug("系统托盘尚未就绪，已等待 %v，继续等待", notReadyFor.Round(time.Second))
		return
	}

	if !isShellReady() {
		m.logInfo("系统托盘尚未就绪，但任务栏通知区域未稳定，暂缓重建")
		return
	}

	m.requestTrayRestart(fmt.Sprintf("ready=false 持续 %v", notReadyFor.Round(time.Second)))
}

func (m *Manager) requestTrayRestart(reason string) {
	nowUnix := time.Now().Unix()
	throttleSeconds := int64(trayRestartThrottle / time.Second)
	for {
		lastTry := m.lastRestartTry.Load()
		if lastTry > 0 && nowUnix-lastTry < throttleSeconds {
			m.logDebug("系统托盘重建请求被节流: %s", reason)
			return
		}
		if m.lastRestartTry.CompareAndSwap(lastTry, nowUnix) {
			break
		}
	}

	m.logError("系统托盘状态异常，准备重建: %s", reason)
	atomic.StoreInt32(&m.readyState, 0)

	go m.quitSystrayInstance()
}

// 日志辅助方法
func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		logger.Wrapped(m.logger).Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		logger.Wrapped(m.logger).Error(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		logger.Wrapped(m.logger).Debug(format, v...)
	}
}

func (m *Manager) ensureCurveMenuItems(parent *systray.MenuItem, options []CurveOption) {
	if parent == nil {
		return
	}

	if len(options) == 0 {
		if len(m.curveMenuItems) == 0 {
			emptyItem := parent.AddSubMenuItem(coremsg.T(coremsg.TrayMenuCurveEmpty), "")
			emptyItem.Disable()
			m.curveMenuItems["__empty__"] = emptyItem
		}
		return
	}

	if empty, ok := m.curveMenuItems["__empty__"]; ok && empty != nil {
		empty.Hide()
		delete(m.curveMenuItems, "__empty__")
	}

	activeIDs := map[string]bool{}
	for _, option := range options {
		if option.ID != "" {
			activeIDs[option.ID] = true
		}
	}
	for id, item := range m.curveMenuItems {
		if id == "__empty__" || item == nil {
			continue
		}
		if !activeIDs[id] {
			item.Hide()
			delete(m.curveMenuItems, id)
		}
	}

	for _, option := range options {
		if option.ID == "" {
			continue
		}
		if existing, ok := m.curveMenuItems[option.ID]; ok && existing != nil {
			existing.Show()
			existing.SetTitle(option.Name)
			continue
		}

		item := parent.AddSubMenuItemCheckbox(option.Name, coremsg.T(coremsg.TrayMenuCurveItemTip), false)
		m.curveMenuItems[option.ID] = item

		profileID := option.ID
		instanceDone := m.currentInstanceDone()
		go func(menuItem *systray.MenuItem, pid string, instanceDone <-chan struct{}) {
			for {
				select {
				case <-menuItem.ClickedCh:
					if m.onSetCurve == nil {
						continue
					}
					m.runTrayActionAsync("menu-set-curve", nil, func() {
						_ = m.onSetCurve(pid)
						m.enqueueUI("menu-set-curve-ui", func() {
							m.updateCurveMenuSelection(pid)
						})
					})
				case <-instanceDone:
					return
				case <-m.done:
					return
				}
			}
		}(item, profileID, instanceDone)
	}
}

func (m *Manager) updateCurveMenuSelection(activeID string) {
	for id, item := range m.curveMenuItems {
		if item == nil || id == "__empty__" {
			continue
		}
		if id == activeID {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}

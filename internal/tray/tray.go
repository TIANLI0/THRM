// Package tray 提供系统托盘管理功能
package tray

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
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
	onRestart       func()
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

	// 防止托盘动作重入导致偶发无响应
	showWindowInFlight int32
	toggleAutoInFlight int32
	restartInFlight    int32
	quitInFlight       int32
}

// MenuItems 托盘菜单项结构
type MenuItems struct {
	Show           *systray.MenuItem
	DeviceStatus   *systray.MenuItem
	CPUTemperature *systray.MenuItem
	GPUTemperature *systray.MenuItem
	FanSpeed       *systray.MenuItem
	CurveSelect    *systray.MenuItem
	AutoControl    *systray.MenuItem
	Restart        *systray.MenuItem
	Quit           *systray.MenuItem
}

// CurveOption 托盘曲线选项
type CurveOption struct {
	ID   string
	Name string
}

// Status 状态信息
type Status struct {
	Connected            bool
	CPUTemp              int
	GPUTemp              int
	CurrentRPM           uint16
	AutoControlState     bool
	ActiveCurveProfileID string
	CurveProfiles        []CurveOption
}

// NewManager 创建新的托盘管理器
func NewManager(logger types.Logger, iconData []byte) *Manager {
	return &Manager{
		logger:         logger,
		done:           make(chan struct{}),
		uiQueue:        make(chan func(), 64),
		iconData:       iconData,
		curveMenuItems: make(map[string]*systray.MenuItem),
	}
}

// SetCallbacks 设置回调函数
func (m *Manager) SetCallbacks(
	onShowWindow func(),
	onQuit func(),
	onRestart func(),
	onToggleAuto func() bool,
	onSetCurve func(profileID string) string,
	getCurveOptions func() ([]CurveOption, string),
	getStatus func() Status,
) {
	m.onShowWindow = onShowWindow
	m.onQuit = onQuit
	m.onRestart = onRestart
	m.onToggleAuto = onToggleAuto
	m.onSetCurve = onSetCurve
	m.getCurveOptions = getCurveOptions
	m.getStatus = getStatus
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

		ran := m.runSystrayInstance()

		select {
		case <-m.done:
			return
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

	// 等待 Windows 外壳就绪，避免 Shell_NotifyIcon(NIM_ADD) 在外壳未启动时失败。
	if !waitForShellReady(m.done, 60*time.Second) {
		close(instanceDone)
		return 0
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘消息循环发生panic: %v", r)
			}
		}()
		// onReady 由 systray 在内部 goroutine 触发；消息循环在此阻塞。
		systray.Run(m.onTrayReady, m.onTrayExit)
	}()
	ran = time.Since(start)

	atomic.StoreInt32(&m.readyState, 0)
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
		systray.Quit()
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
		systray.Quit()
		return
	}
	m.menuItems = menuItems
	instanceDone := m.currentInstanceDone()
	m.startUIWorker(instanceDone)

	atomic.StoreInt32(&m.readyState, 1)
	m.lastIconRefresh.Store(time.Now().Unix())
	m.consecutiveFails.Store(0)
	m.logInfo("系统托盘初始化完成")

	// 处理托盘菜单事件
	go m.handleMenuEvents(instanceDone)

	// 定期更新托盘菜单状态
	go m.updateMenuStatus(instanceDone)

	// 启动托盘健康监控（定期刷新图标以应对 Explorer 重启等）
	go m.startIconHealthMonitor(instanceDone)
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
	systray.SetTooltip(appmeta.AppName + " - 运行中")
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

	items.Show = systray.AddMenuItem("显示主窗口", "显示控制器主窗口")
	systray.AddSeparator()

	items.DeviceStatus = systray.AddMenuItem("设备状态", "查看设备连接状态")
	items.DeviceStatus.Disable()

	items.CPUTemperature = systray.AddMenuItem("CPU温度", "显示当前CPU温度")
	items.CPUTemperature.Disable()

	items.GPUTemperature = systray.AddMenuItem("GPU温度", "显示当前GPU温度")
	items.GPUTemperature.Disable()

	items.FanSpeed = systray.AddMenuItem("风扇转速", "显示当前风扇转速")
	items.FanSpeed.Disable()
	items.CurveSelect = systray.AddMenuItem("选择温控曲线", "直接切换到指定温控曲线")

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
	items.AutoControl = systray.AddMenuItemCheckbox("智能变频", "启用/禁用智能变频", autoControlEnabled)

	systray.AddSeparator()
	items.Restart = systray.AddMenuItem("重启软件", "重启软件")
	items.Quit = systray.AddMenuItem("退出", "完全退出应用")

	return items, nil
}

// handleMenuEvents 处理托盘菜单事件
func (m *Manager) handleMenuEvents(instanceDone <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("处理托盘菜单事件时发生panic: %v", r)
		}
	}()

	if m.menuItems == nil || m.menuItems.Show == nil || m.menuItems.AutoControl == nil || m.menuItems.Restart == nil || m.menuItems.Quit == nil {
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
		case <-m.menuItems.Restart.ClickedCh:
			m.logInfo("托盘菜单: 用户请求重启应用")
			if m.onRestart != nil {
				m.runTrayActionAsync("menu-restart", &m.restartInFlight, m.onRestart)
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

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

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
			m.enqueueUI("update-menu-status", func() {
				if m.menuItems == nil {
					return
				}

				if status.Connected {
					m.menuItems.DeviceStatus.SetTitle("设备状态: 已连接")
				} else {
					m.menuItems.DeviceStatus.SetTitle("设备状态: 未连接")
				}

				if status.CPUTemp > 0 {
					m.menuItems.CPUTemperature.SetTitle(fmt.Sprintf("CPU温度: %d°C", status.CPUTemp))
				} else {
					m.menuItems.CPUTemperature.SetTitle("CPU温度: 无数据")
				}

				if status.GPUTemp > 0 {
					m.menuItems.GPUTemperature.SetTitle(fmt.Sprintf("GPU温度: %d°C", status.GPUTemp))
				} else {
					m.menuItems.GPUTemperature.SetTitle("GPU温度: 无数据")
				}

				if status.CurrentRPM > 0 {
					m.menuItems.FanSpeed.SetTitle(fmt.Sprintf("风扇转速: %d RPM", status.CurrentRPM))
				} else {
					m.menuItems.FanSpeed.SetTitle("风扇转速: 无数据")
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

				if status.Connected {
					if status.AutoControlState {
						tooltipText := fmt.Sprintf("%s - 智能变频中\nCPU: %d°C GPU: %d°C", appmeta.AppName, status.CPUTemp, status.GPUTemp)
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\n风扇: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					} else {
						tooltipText := appmeta.AppName + " - 手动模式"
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\n风扇: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					}
				} else {
					systray.SetTooltip(appmeta.AppName + " - 设备未连接")
				}
			})
		case <-instanceDone:
			return
		case <-m.done:
			return
		}
	}
}

// onTrayExit 托盘退出时的回调
//
// 注意：此处只清除就绪状态，不重置 initialized。initialized 表示托盘子系统（监督协程）
// 是否处于活动状态，仅在 Init/Quit 时变更；这样消息循环临时退出并被监督协程重建期间，
// 健康检查与状态上报仍能正确反映子系统在运行。
func (m *Manager) onTrayExit() {
	m.logDebug("托盘退出回调被触发")
	atomic.StoreInt32(&m.readyState, 0)
}

// startIconHealthMonitor 启动托盘图标健康监控
func (m *Manager) startIconHealthMonitor(instanceDone <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘图标健康监控发生panic: %v", r)
		}
	}()

	// 每30秒刷新一次托盘图标，更及时地恢复 Explorer 重启后的图标
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue // 不退出，等待恢复
			}
			m.refreshTrayIcon()
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
		systray.SetTooltip(appmeta.AppName + " - 运行中")

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

	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logDebug("退出托盘时发生错误（可忽略）: %v", r)
			}
		}()
		systray.Quit()
	}()
}

// RefreshIcon 主动刷新托盘图标。
//
// 主要用于系统从休眠/睡眠唤醒、或 Explorer 重启后，及时恢复通知区域图标，
// 避免出现图标丢失或显示异常。仅在托盘就绪时生效。
func (m *Manager) RefreshIcon() {
	if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
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

	// 如果托盘未初始化，无需检查
	if atomic.LoadInt32(&m.initialized) == 0 {
		return
	}

	// 检查图标是否长时间未刷新
	lastRefresh := m.lastIconRefresh.Load()
	if lastRefresh > 0 && time.Now().Unix()-lastRefresh > 90 {
		m.logInfo("检测到托盘图标长时间未刷新，尝试刷新")
		m.refreshTrayIcon()
	}

	// 如果连续失败，也强制刷新图标
	if m.consecutiveFails.Load() >= 3 {
		m.logError("检测到托盘连续失败，尝试刷新图标")
		m.refreshTrayIcon()
	}
}

// 日志辅助方法
func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}

func (m *Manager) ensureCurveMenuItems(parent *systray.MenuItem, options []CurveOption) {
	if parent == nil {
		return
	}

	if len(options) == 0 {
		if len(m.curveMenuItems) == 0 {
			emptyItem := parent.AddSubMenuItem("暂无可用曲线", "")
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

		item := parent.AddSubMenuItemCheckbox(option.Name, "切换温控曲线", false)
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

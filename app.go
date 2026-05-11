package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/TIANLI0/BS2PRO-Controller/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// App struct - GUI 应用程序结构
type App struct {
	ctx       context.Context
	ipcClient *ipc.Client
	mutex     sync.RWMutex

	// 缓存的状态
	isConnected bool
	currentTemp types.TemperatureData
}

// 为了与前端 API 兼容，重新导出类型
type (
	FanCurvePoint           = types.FanCurvePoint
	FanCurveProfile         = types.FanCurveProfile
	FanCurveProfilesPayload = types.FanCurveProfilesPayload
	FanData                 = types.FanData
	GearCommand             = types.GearCommand
	TemperatureData         = types.TemperatureData
	BridgeTemperatureData   = types.BridgeTemperatureData
	AppConfig               = types.AppConfig
)

var guiLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	guiLogger = logger.Sugar()
}

// NewApp 创建 GUI 应用实例
func NewApp() *App {
	return &App{
		ipcClient:   ipc.NewClient(nil),
		currentTemp: types.TemperatureData{BridgeOk: true},
	}
}

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	guiLogger.Info("=== BS2PRO GUI 启动 ===")

	// 连接到核心服务
	if err := a.ipcClient.Connect(); err != nil {
		guiLogger.Errorf("连接核心服务失败: %v", err)
		runtime.EventsEmit(ctx, "core-service-error", "无法连接到核心服务")
	} else {
		guiLogger.Info("已连接到核心服务")

		// 设置事件处理器
		a.ipcClient.SetEventHandler(a.handleCoreEvent)
	}

	guiLogger.Info("=== BS2PRO GUI 启动完成 ===")
}

// GetAppVersion 返回应用版本号（来自版本模块）
func (a *App) GetAppVersion() string {
	return version.Get()
}

// handleCoreEvent 处理核心服务推送的事件
func (a *App) handleCoreEvent(event ipc.Event) {
	if a.ctx == nil {
		return
	}

	switch event.Type {
	case ipc.EventFanDataUpdate:
		var fanData types.FanData
		if err := json.Unmarshal(event.Data, &fanData); err == nil {
			runtime.EventsEmit(a.ctx, "fan-data-update", fanData)
		}

	case ipc.EventTemperatureUpdate:
		var temp types.TemperatureData
		if err := json.Unmarshal(event.Data, &temp); err == nil {
			a.mutex.Lock()
			a.currentTemp = temp
			a.mutex.Unlock()
			runtime.EventsEmit(a.ctx, "temperature-update", temp)
		}

	case ipc.EventDeviceConnected:
		var deviceInfo map[string]string
		json.Unmarshal(event.Data, &deviceInfo)
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-connected", deviceInfo)

	case ipc.EventDeviceDisconnected:
		a.mutex.Lock()
		a.isConnected = false
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-disconnected", nil)

	case ipc.EventDeviceError:
		var errMsg string
		json.Unmarshal(event.Data, &errMsg)
		runtime.EventsEmit(a.ctx, "device-error", errMsg)

	case ipc.EventConfigUpdate:
		var cfg types.AppConfig
		if err := json.Unmarshal(event.Data, &cfg); err == nil {
			runtime.EventsEmit(a.ctx, "config-update", cfg)
		}

	case ipc.EventHotkeyTriggered:
		var payload map[string]any
		if err := json.Unmarshal(event.Data, &payload); err == nil {
			runtime.EventsEmit(a.ctx, "hotkey-triggered", payload)
		}

	case ipc.EventLegionPowerModeUpdate:
		var payload map[string]any
		if err := json.Unmarshal(event.Data, &payload); err == nil {
			runtime.EventsEmit(a.ctx, "legion-power-mode-update", payload)
		}

	case ipc.EventHealthPing:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "health-ping", timestamp)

	case ipc.EventHeartbeat:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "heartbeat", timestamp)

	case "show-window":
		a.ShowWindow()

	case "quit":
		a.QuitApp()
	}
}

// sendRequest 发送请求到核心服务
func (a *App) sendRequest(reqType ipc.RequestType, data any) (*ipc.Response, error) {
	if !a.ipcClient.IsConnected() {
		// 尝试重新连接
		if err := a.ipcClient.Connect(); err != nil {
			return nil, fmt.Errorf("未连接到核心服务: %v", err)
		}
	}
	return a.ipcClient.SendRequest(reqType, data)
}

// === 前端 API 方法 ===
// 以下所有公开方法保持与原始 app.go 完全兼容

// ConnectDevice 连接HID设备
func (a *App) ConnectDevice() bool {
	resp, err := a.sendRequest(ipc.ReqConnect, nil)
	if err != nil {
		guiLogger.Errorf("连接设备请求失败: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("连接设备失败: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// DisconnectDevice 断开设备连接
func (a *App) DisconnectDevice() {
	a.sendRequest(ipc.ReqDisconnect, nil)
}

// GetDeviceStatus 获取设备连接状态
func (a *App) GetDeviceStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDeviceStatus, nil)
	if err != nil {
		return map[string]any{"connected": false, "error": err.Error()}
	}
	if !resp.Success {
		return map[string]any{"connected": false, "error": resp.Error}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// GetConfig 获取当前配置
func (a *App) GetConfig() AppConfig {
	resp, err := a.sendRequest(ipc.ReqGetConfig, nil)
	if err != nil {
		guiLogger.Errorf("获取配置失败: %v", err)
		return types.GetDefaultConfig(false)
	}
	if !resp.Success {
		guiLogger.Errorf("获取配置失败: %s", resp.Error)
		return types.GetDefaultConfig(false)
	}
	var cfg AppConfig
	json.Unmarshal(resp.Data, &cfg)
	return cfg
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(cfg AppConfig) error {
	resp, err := a.sendRequest(ipc.ReqUpdateConfig, cfg)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetFanCurve 设置风扇曲线
func (a *App) SetFanCurve(curve []FanCurvePoint) error {
	resp, err := a.sendRequest(ipc.ReqSetFanCurve, curve)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetFanCurve 获取风扇曲线
func (a *App) GetFanCurve() []FanCurvePoint {
	resp, err := a.sendRequest(ipc.ReqGetFanCurve, nil)
	if err != nil {
		return types.GetDefaultFanCurve()
	}
	if !resp.Success {
		return types.GetDefaultFanCurve()
	}
	var curve []FanCurvePoint
	json.Unmarshal(resp.Data, &curve)
	return curve
}

// GetFanCurveProfiles 获取曲线方案列表
func (a *App) GetFanCurveProfiles() FanCurveProfilesPayload {
	resp, err := a.sendRequest(ipc.ReqGetFanCurveProfiles, nil)
	if err != nil || !resp.Success {
		cfg := a.GetConfig()
		return types.FanCurveProfilesPayload{
			Profiles: cfg.FanCurveProfiles,
			ActiveID: cfg.ActiveFanCurveProfileID,
		}
	}
	var payload FanCurveProfilesPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SetActiveFanCurveProfile 设置当前激活曲线方案
func (a *App) SetActiveFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqSetActiveFanCurveProfile, ipc.SetActiveFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SaveFanCurveProfile 保存曲线方案
func (a *App) SaveFanCurveProfile(profileID, name string, curve []FanCurvePoint, setActive bool) (FanCurveProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSaveFanCurveProfile, ipc.SaveFanCurveProfileParams{
		ID:        profileID,
		Name:      name,
		Curve:     curve,
		SetActive: setActive,
	})
	if err != nil {
		return FanCurveProfile{}, err
	}
	if !resp.Success {
		return FanCurveProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var profile FanCurveProfile
	json.Unmarshal(resp.Data, &profile)
	return profile, nil
}

// DeleteFanCurveProfile 删除曲线方案
func (a *App) DeleteFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqDeleteFanCurveProfile, ipc.DeleteFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ExportFanCurveProfiles 导出曲线方案
func (a *App) ExportFanCurveProfiles() (string, error) {
	resp, err := a.sendRequest(ipc.ReqExportFanCurveProfiles, nil)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var code string
	json.Unmarshal(resp.Data, &code)
	return code, nil
}

// ImportFanCurveProfiles 导入曲线方案
func (a *App) ImportFanCurveProfiles(code string) error {
	resp, err := a.sendRequest(ipc.ReqImportFanCurveProfiles, ipc.ImportFanCurveProfilesParams{Code: code})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetAutoControl 设置智能变频
func (a *App) SetAutoControl(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetAutoControl, ipc.SetAutoControlParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetManualGear 设置手动挡位
func (a *App) SetManualGear(gear, level string) bool {
	resp, err := a.sendRequest(ipc.ReqSetManualGear, ipc.SetManualGearParams{Gear: gear, Level: level})
	if err != nil {
		return false
	}
	if !resp.Success {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// GetAvailableGears 获取可用挡位
func (a *App) GetAvailableGears() map[string][]GearCommand {
	resp, err := a.sendRequest(ipc.ReqGetAvailableGears, nil)
	if err != nil {
		return types.GearCommands
	}
	if !resp.Success {
		return types.GearCommands
	}
	var gears map[string][]GearCommand
	json.Unmarshal(resp.Data, &gears)
	return gears
}

// ManualSetFanSpeed 废弃方法
func (a *App) ManualSetFanSpeed(rpm int) bool {
	guiLogger.Warn("ManualSetFanSpeed 已废弃，请使用 SetManualGear")
	return false
}

// SetCustomSpeed 设置自定义转速
func (a *App) SetCustomSpeed(enabled bool, rpm int) error {
	resp, err := a.sendRequest(ipc.ReqSetCustomSpeed, ipc.SetCustomSpeedParams{Enabled: enabled, RPM: rpm})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetGearLight 设置挡位灯
func (a *App) SetGearLight(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetGearLight, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetPowerOnStart 设置通电自启动
func (a *App) SetPowerOnStart(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetPowerOnStart, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetSmartStartStop 设置智能启停
func (a *App) SetSmartStartStop(mode string) bool {
	resp, err := a.sendRequest(ipc.ReqSetSmartStartStop, ipc.SetStringParams{Value: mode})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetBrightness 设置亮度
func (a *App) SetBrightness(percentage int) bool {
	resp, err := a.sendRequest(ipc.ReqSetBrightness, ipc.SetIntParams{Value: percentage})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetLightStrip 设置灯带
func (a *App) SetLightStrip(cfg types.LightStripConfig) error {
	resp, err := a.sendRequest(ipc.ReqSetLightStrip, ipc.SetLightStripParams{Config: cfg})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetTemperature 获取当前温度
func (a *App) GetTemperature() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqGetTemperature, nil)
	if err != nil {
		a.mutex.RLock()
		defer a.mutex.RUnlock()
		return a.currentTemp
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// GetCurrentFanData 获取当前风扇数据
func (a *App) GetCurrentFanData() *FanData {
	resp, err := a.sendRequest(ipc.ReqGetCurrentFanData, nil)
	if err != nil {
		return nil
	}
	var fanData FanData
	if err := json.Unmarshal(resp.Data, &fanData); err != nil {
		return nil
	}
	return &fanData
}

// TestTemperatureReading 测试温度读取
func (a *App) TestTemperatureReading() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestTemperatureReading, nil)
	if err != nil {
		return TemperatureData{}
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// TestBridgeProgram 测试桥接程序
func (a *App) TestBridgeProgram() BridgeTemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestBridgeProgram, nil)
	if err != nil {
		return BridgeTemperatureData{Success: false, Error: err.Error()}
	}
	var data BridgeTemperatureData
	json.Unmarshal(resp.Data, &data)
	return data
}

// GetBridgeProgramStatus 获取桥接程序状态
func (a *App) GetBridgeProgramStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetBridgeProgramStatus, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// RestartPawnIO 重启 PawnIO 驱动并重新初始化温度读取
func (a *App) RestartPawnIO() BridgeTemperatureData {
	resp, err := a.sendRequest(ipc.ReqRestartPawnIO, nil)
	if err != nil {
		return BridgeTemperatureData{Success: false, Error: err.Error()}
	}
	var data BridgeTemperatureData
	json.Unmarshal(resp.Data, &data)
	return data
}

// SetWindowsAutoStart 设置Windows开机自启动
func (a *App) SetWindowsAutoStart(enable bool) error {
	resp, err := a.sendRequest(ipc.ReqSetWindowsAutoStart, ipc.SetBoolParams{Enabled: enable})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// IsRunningAsAdmin 检查是否以管理员权限运行
func (a *App) IsRunningAsAdmin() bool {
	resp, err := a.sendRequest(ipc.ReqIsRunningAsAdmin, nil)
	if err != nil {
		return false
	}
	var isAdmin bool
	json.Unmarshal(resp.Data, &isAdmin)
	return isAdmin
}

// GetAutoStartMethod 获取当前的自启动方式
func (a *App) GetAutoStartMethod() string {
	resp, err := a.sendRequest(ipc.ReqGetAutoStartMethod, nil)
	if err != nil {
		return "none"
	}
	var method string
	json.Unmarshal(resp.Data, &method)
	return method
}

// SetAutoStartWithMethod 使用指定方式设置自启动
func (a *App) SetAutoStartWithMethod(enable bool, method string) error {
	resp, err := a.sendRequest(ipc.ReqSetAutoStartWithMethod, ipc.SetAutoStartWithMethodParams{Enable: enable, Method: method})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// CheckWindowsAutoStart 检查Windows开机自启动状态
func (a *App) CheckWindowsAutoStart() bool {
	resp, err := a.sendRequest(ipc.ReqCheckWindowsAutoStart, nil)
	if err != nil {
		return false
	}
	var enabled bool
	json.Unmarshal(resp.Data, &enabled)
	return enabled
}

// IsAutoStartLaunch 返回当前是否为自启动启动
func (a *App) IsAutoStartLaunch() bool {
	resp, err := a.sendRequest(ipc.ReqIsAutoStartLaunch, nil)
	if err != nil {
		return false
	}
	var isAutoStart bool
	json.Unmarshal(resp.Data, &isAutoStart)
	return isAutoStart
}

// ShowWindow 显示主窗口
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}

// HideWindow 隐藏主窗口到托盘
func (a *App) HideWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// QuitApp 完全退出应用
func (a *App) QuitApp() {
	guiLogger.Info("GUI 请求退出")

	// 关闭 IPC 连接
	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	// 退出 GUI
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// QuitAll 完全退出应用（包括核心服务）
func (a *App) QuitAll() {
	guiLogger.Info("GUI 请求完全退出（包括核心服务）")

	// 通知核心服务退出
	a.sendRequest(ipc.ReqQuitApp, nil)

	// 关闭 IPC 连接
	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	// 退出 GUI
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// OnWindowClosing 窗口关闭事件处理
func (a *App) OnWindowClosing(ctx context.Context) bool {
	// 返回 false 允许窗口正常关闭并退出 GUI
	// 核心服务会继续在后台运行
	return false
}

// InitSystemTray 初始化系统托盘（保持API兼容，实际由核心服务处理）
func (a *App) InitSystemTray() {
	// 托盘由核心服务管理，GUI 不需要处理
}

// UpdateGuiResponseTime 更新GUI响应时间（供前端调用）
func (a *App) UpdateGuiResponseTime() {
	a.sendRequest(ipc.ReqUpdateGuiResponseTime, nil)
}

// GetDebugInfo 获取调试信息
func (a *App) GetDebugInfo() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDebugInfo, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var info map[string]any
	json.Unmarshal(resp.Data, &info)
	return info
}

// SetDebugMode 设置调试模式
func (a *App) SetDebugMode(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetDebugMode, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

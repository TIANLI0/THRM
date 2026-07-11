// Wails API 服务封装
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import { 
  ConnectDevice, 
  DisconnectDevice, 
  GetDeviceStatus,
  GetConfig,
  UpdateConfig,
  SetFanCurve,
  ResetLearnedOffsets,
  GetFanCurve,
  SetAutoControl,
  GetAppVersion,
  SetManualGear,
  GetAvailableGears,
  SetGearLight,
  SetPowerOnStart,
  SetSmartStartStop,
  SetBrightness,
  SetLightStrip,
  GetTemperature,
  GetTemperatureHistory,
  SetTemperatureHistoryEnabled,
  GetCurrentFanData,
  TestTemperatureReading,
  GetDebugInfo,
  SetDebugMode,
  UpdateGuiResponseTime,
  SetCustomSpeed
  // CheckWindowsAutoStart,
  // SetWindowsAutoStart
} from '../../../wailsjs/go/main/App';

import { types } from '../../../wailsjs/go/models';

import type {
  DeviceInfo,
  DeviceDebugCommandResult,
  DeviceDebugFrame,
  DeviceSettings,
  DebugInfo,
  LegionFnQSupportPayload,
  LegionPowerModePayload,
  ThemeMeta,
} from '../types/app';

class ApiService {
  // 设备连接
  async connectDevice(): Promise<boolean> {
    return await ConnectDevice();
  }

  async disconnectDevice(): Promise<void> {
    return await DisconnectDevice();
  }

  async getDeviceStatus(): Promise<any> {
    return await GetDeviceStatus();
  }

  async refreshDeviceSettings(): Promise<DeviceSettings | null> {
    return await (window as any).go?.main?.App?.RefreshDeviceSettings?.();
  }

  // 配置管理
  async getConfig(): Promise<types.AppConfig> {
    return await GetConfig();
  }

  async getAppVersion(): Promise<string> {
    return await GetAppVersion();
  }

  // 下载并全自动静默安装应用更新（Windows），期间弹出 CMD 状态窗口展示更新动态。
  // 下载进度通过 update-download-progress 事件推送；三个文案参数为状态窗口的本地化文字。
  async downloadAndInstallUpdate(
    downloadURL: string,
    windowTitle: string,
    windowBody: string,
    windowRestarting: string,
  ): Promise<void> {
    return await (window as any).go?.main?.App?.DownloadAndInstallUpdate(
      downloadURL,
      windowTitle,
      windowBody,
      windowRestarting,
    );
  }

  onUpdateDownloadProgress(
    callback: (payload: { percent: number; received: number; total: number; stage: string; message: string }) => void,
  ): () => void {
    return EventsOn('update-download-progress', callback);
  }

  async updateConfig(config: types.AppConfig): Promise<void> {
    return await UpdateConfig(config);
  }

  // 风扇曲线
  async setFanCurve(curve: types.FanCurvePoint[]): Promise<void> {
    return await SetFanCurve(curve);
  }

  // 清空学习到的曲线偏移；后端清零所有 LearnedOffsets。
  async resetLearnedOffsets(): Promise<void> {
    return await ResetLearnedOffsets();
  }

  async getFanCurve(): Promise<types.FanCurvePoint[]> {
    return await GetFanCurve();
  }

  async getFanCurveProfiles(): Promise<{ profiles: Array<{ id: string; name: string; curve: types.FanCurvePoint[] }>; activeId: string }> {
    return await (window as any).go?.main?.App?.GetFanCurveProfiles();
  }

  async setActiveFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.SetActiveFanCurveProfile(profileID);
  }

  async saveFanCurveProfile(profileID: string, name: string, curve: types.FanCurvePoint[], setActive: boolean): Promise<{ id: string; name: string; curve: types.FanCurvePoint[] }> {
    return await (window as any).go?.main?.App?.SaveFanCurveProfile(profileID, name, curve, setActive);
  }

  async deleteFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.DeleteFanCurveProfile(profileID);
  }

  async exportFanCurveProfiles(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfiles();
  }

  async importFanCurveProfiles(code: string): Promise<void> {
    return await (window as any).go?.main?.App?.ImportFanCurveProfiles(code);
  }

  // 智能变频
  async setAutoControl(enabled: boolean): Promise<void> {
    return await SetAutoControl(enabled);
  }

  // 自定义转速
  async setCustomSpeed(enabled: boolean, rpm: number): Promise<void> {
    return await SetCustomSpeed(enabled, rpm);
  }

  // 手动挡位控制
  async setManualGear(gear: string, level: string): Promise<boolean> {
    return await SetManualGear(gear, level);
  }

  // 获取可用挡位
  async getAvailableGears(): Promise<any> {
    return await GetAvailableGears();
  }

  // 设备设置
  async setGearLight(enabled: boolean): Promise<boolean> {
    return await SetGearLight(enabled);
  }

  async setPowerOnStart(enabled: boolean): Promise<boolean> {
    return await SetPowerOnStart(enabled);
  }

  async setSmartStartStop(mode: string): Promise<boolean> {
    return await SetSmartStartStop(mode);
  }

  async setBrightness(percentage: number): Promise<boolean> {
    return await SetBrightness(percentage);
  }

  async setLightStrip(config: types.LightStripConfig): Promise<void> {
    return await SetLightStrip(config);
  }

  // Windows自启动相关
  async checkWindowsAutoStart(): Promise<boolean> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.CheckWindowsAutoStart();
  }

  async setWindowsAutoStart(enabled: boolean): Promise<void> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.SetWindowsAutoStart(enabled);
  }

  async getAutoStartMethod(): Promise<string> {
    // 获取当前自启动方式
    return await (window as any).go?.main?.App?.GetAutoStartMethod();
  }

  async setAutoStartWithMethod(enabled: boolean, method: string): Promise<void> {
    // 使用指定方式设置自启动
    return await (window as any).go?.main?.App?.SetAutoStartWithMethod(enabled, method);
  }

  async isRunningAsAdmin(): Promise<boolean> {
    // 检查是否以管理员权限运行
    return await (window as any).go?.main?.App?.IsRunningAsAdmin();
  }

  // 数据获取
  async getTemperature(): Promise<types.TemperatureData> {
    return await GetTemperature();
  }

  async getTemperatureHistory(): Promise<types.TemperatureHistoryPayload> {
    return await GetTemperatureHistory();
  }

  async setTemperatureHistoryEnabled(enabled: boolean): Promise<void> {
    return await SetTemperatureHistoryEnabled(enabled);
  }

  async getCurrentFanData(): Promise<types.FanData | null> {
    return await GetCurrentFanData();
  }

  async testTemperatureReading(): Promise<types.TemperatureData> {
    return await TestTemperatureReading();
  }

  // 桥接程序相关
  async getBridgeProgramStatus(): Promise<any> {
    return await (window as any).go?.main?.App?.GetBridgeProgramStatus();
  }

  async testBridgeProgram(): Promise<any> {
    return await (window as any).go?.main?.App?.TestBridgeProgram();
  }

  async restartPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.RestartPawnIO();
  }

  async reinstallPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.ReinstallPawnIO();
  }

  // 事件监听
  onDeviceConnected(callback: (data: DeviceInfo) => void): () => void {
    return EventsOn('device-connected', callback);
  }

  onDeviceDisconnected(callback: () => void): () => void {
    return EventsOn('device-disconnected', callback);
  }

  onDeviceError(callback: (error: string) => void): () => void {
    return EventsOn('device-error', callback);
  }

  onDeviceSettingsUpdate(callback: (data: DeviceSettings) => void): () => void {
    return EventsOn('device-settings-update', callback);
  }

  onFanDataUpdate(callback: (data: types.FanData) => void): () => void {
    return EventsOn('fan-data-update', callback);
  }

  onTemperatureUpdate(callback: (data: types.TemperatureData) => void): () => void {
    return EventsOn('temperature-update', callback);
  }

  onTemperatureHistoryUpdate(callback: (data: { timestamp: number; cpuTemp: number; gpuTemp: number; cpuPower?: number; gpuPower?: number; fanRpm?: number }) => void): () => void {
    return EventsOn('temperature-history-update', callback);
  }

  onConfigUpdate(callback: (config: types.AppConfig) => void): () => void {
    return EventsOn('config-update', callback);
  }

  onHotkeyTriggered(callback: (payload: { action: string; shortcut: string; success: boolean; message: string }) => void): () => void {
    return EventsOn('hotkey-triggered', callback);
  }

  // 调试相关方法
  onLegionPowerModeUpdate(callback: (payload: LegionPowerModePayload) => void): () => void {
    return EventsOn('legion-power-mode-update', callback);
  }

  onLegionFnQSupportUpdate(callback: (payload: LegionFnQSupportPayload) => void): () => void {
    return EventsOn('legion-fnq-support-update', callback);
  }

  async getDebugInfo(): Promise<DebugInfo> {
    return await GetDebugInfo() as DebugInfo;
  }

  async setDebugMode(enabled: boolean): Promise<void> {
    return await SetDebugMode(enabled);
  }

  async sendDeviceDebugCommand(hexCommand: string, waitMs = 800): Promise<DeviceDebugCommandResult> {
    return await (window as any).go?.main?.App?.SendDeviceDebugCommand(hexCommand, waitMs);
  }

  async getDeviceDebugFrames(): Promise<DeviceDebugFrame[]> {
    const frames = await (window as any).go?.main?.App?.GetDeviceDebugFrames();
    return Array.isArray(frames) ? frames as DeviceDebugFrame[] : [];
  }

  // ── 自定义主题 ──
  // 注：以下方法走 Wails 运行时自动暴露的 window.go.main.App 代理，
  // 因此无需重新生成强类型绑定即可调用（与上面若干方法同理）。

  // 列出安装目录/用户目录下发现的全部自定义主题。
  async listThemes(): Promise<ThemeMeta[]> {
    const list = await (window as any).go?.main?.App?.ListThemes?.();
    return Array.isArray(list) ? (list as ThemeMeta[]) : [];
  }

  // 读取指定主题的 CSS 文本（用于注入页面）。
  async getThemeCSS(id: string): Promise<string> {
    const css = await (window as any).go?.main?.App?.GetThemeCSS?.(id);
    return typeof css === 'string' ? css : '';
  }

  // 在系统文件管理器中打开主题文件夹，便于用户编辑/新增主题。
  async openThemesFolder(): Promise<void> {
    return await (window as any).go?.main?.App?.OpenThemesFolder?.();
  }

  async updateGuiResponseTime(): Promise<void> {
    return await UpdateGuiResponseTime();
  }

  // 调试事件监听
  onHealthPing(callback: (timestamp: number) => void): () => void {
    return EventsOn('health-ping', callback);
  }

  onHeartbeat(callback: (timestamp: number) => void): () => void {
    return EventsOn('heartbeat', callback);
  }

  onCoreServiceError(callback: (message: string) => void): () => void {
    return EventsOn('core-service-error', callback);
  }

  onCoreServiceOK(callback: () => void): () => void {
    return EventsOn('core-service-ok', callback);
  }
}

export const apiService = new ApiService();

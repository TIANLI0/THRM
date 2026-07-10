// 应用类型定义

// 风扇曲线点
export interface FanCurvePoint {
  temperature: number; // 温度 °C
  rpm: number;         // 转速 RPM
}

export interface FanCurveProfile {
  id: string;
  name: string;
  curve: FanCurvePoint[];
}

// 风扇数据结构
export interface FanData {
  reportId: number;
  magicSync: number;
  command: number;
  status: number;
  gearSettings: number;
  currentMode: number;
  reserved1: number;
  currentRpm: number;
  targetRpm: number;
  maxGear: string;
  setGear: string;
  workMode: string;
}

// 温度数据
export interface TemperatureData {
  cpuTemp: number;     // CPU温度
  gpuTemp: number;     // GPU温度
  cpuPower?: number;   // CPU package power (W)
  gpuPower?: number;   // selected GPU power (W)
  maxTemp: number;     // 最高温度
  controlTemp?: number; // 当前控温基准温度
  controlSource?: 'max' | 'cpu' | 'gpu'; // 当前控温基准来源
  cpuModel?: string;   // 当前识别的 CPU 型号
  gpuModel?: string;   // 当前识别的 GPU 型号
  cpuSensors?: TemperatureSensor[]; // 当前识别的 CPU 温度传感器
  gpuSensors?: TemperatureSensor[]; // 当前识别的 GPU 温度传感器
  cpuPowerSensors?: PowerSensor[];
  gpuPowerSensors?: PowerSensor[];
  updateTime: number;  // 更新时间戳
  bridgeOk?: boolean;  // 桥接程序是否正常
  bridgeMessage?: string; // 桥接程序提示
}

export interface TemperatureSensor {
  key: string;
  name: string;
  value: number;
}

export interface PowerSensor {
  key: string;
  name: string;
  value: number;
}

// 应用配置
export interface AppConfig {
  legionFnQ?: LegionFnQConfig;
  legionFnQSupport?: LegionFnQSupportCache;
  autoControl: boolean;         // 智能变频开关
  curveProfileToggleHotkey?: string; // 切换曲线方案快捷键
  fanCurve: FanCurvePoint[];   // 风扇曲线
  fanCurveProfiles?: FanCurveProfile[];
  activeFanCurveProfileId?: string;
  gearLight: boolean;          // 挡位灯
  powerOnStart: boolean;       // 通电自启动
  windowsAutoStart: boolean;   // Windows开机自启动
  // 主题模式：system/light/dark 为内置基础主题；其它字符串为自定义主题 id（如 'thrm'）
  themeMode?: string;
  smartStartStop: string;      // 智能启停
  brightness: number;          // 亮度
  tempUpdateRate: number;      // 温度更新频率(秒)
  tempSampleCount?: number;
  tempSource?: 'max' | 'cpu' | 'gpu';
  cpuSensor?: string;
  cpuSensors?: string[];       // CPU 多传感器选择(多核平均)；为空则按 cpuSensor 单选/自动
  gpuSensor?: string;
  windowBlur?: 'auto' | 'on' | 'off'; // 窗口模糊效果(Win11 默认开, Win10 默认关)
  suspendFanOff?: boolean;     // 系统休眠时归零转速并关闭挡位灯/RGB
  configPath: string;          // 配置文件路径
  manualGear: string;          // 手动挡位设置
  manualLevel: string;         // 手动挡位级别(低中高)
  debugMode: boolean;          // 调试模式
  guiMonitoring: boolean;      // GUI监控开关
  customSpeedEnabled: boolean; // 自定义转速开关
  customSpeedRPM: number;      // 自定义转速值(无上下限)
  speedAvoidance?: SpeedAvoidanceConfig;
  timeCurveSchedule?: TimeCurveScheduleConfig;
  smartControl: SmartControlConfig; // 学习型智能控温
}

export interface SpeedAvoidanceConfig {
  enabled: boolean;
  minRpm: number;
  maxRpm: number;
  marginRpm: number;
  emergencyBypassTemp: number;
}

export interface TimeCurveScheduleConfig {
  enabled: boolean;
  rules: TimeCurveScheduleRule[];
}

export interface TimeCurveScheduleRule {
  id: string;
  name: string;
  enabled: boolean;
  weekdays: number[];
  startTime: string;
  endTime: string;
  curveProfileId: string;
}

// 噪音测试采样点：以测试中最安静点为 0 dB 的相对噪音
export interface NoiseProfilePoint {
  rpm: number;
  db: number;
}

export interface SmartControlConfig {
  enabled: boolean;
  learning: boolean;
  learningBias: string;
  filterTransientSpike: boolean;
  targetTemp: number;
  aggressiveness: number;
  hysteresis: number;
  minRpmChange: number;
  rampUpLimit: number;
  rampDownLimit: number;
  learnRate: number;
  learnWindow: number;
  learnDelay: number;
  overheatWeight: number;
  rpmDeltaWeight: number;
  noiseWeight: number;
  trendGain: number;
  maxLearnOffset: number;
  learnedOffsets: number[];
  learnedOffsetsHeat: number[];
  learnedOffsetsCool: number[];
  learnedRateHeat: number[];
  learnedRateCool: number[];
  noiseProfile?: NoiseProfilePoint[];      // 实测转速-噪音档案
  noiseProfileUpdatedAt?: number;          // 噪音测试完成时间(Unix 秒)
}

// 调试信息
export interface FanGearTarget {
  gear: string;
  level: string;
}

export interface LegionFnQConfig {
  enabled: boolean;
  takeOverFan: boolean;
  modeMapping: Record<string, FanGearTarget>;
}

export interface LegionFnQSupportCache {
  checked: boolean;
  supported: boolean;
}

export interface LegionPowerModePayload {
  raw: number;
  mapped: number;
  mode: string;
  source: string;
  timestamp: number;
}

export interface LegionFnQSupportPayload {
  supported: boolean;
}

export interface DebugInfo {
  debugMode: boolean;
  trayReady: boolean;
  trayInitialized: boolean;
  isConnected: boolean;
  autoReconnectSuppressed?: boolean;
  legionFnQSupported?: boolean;
  guiLastResponse: string;
  monitoringTemp: boolean;
  autoStartLaunch: boolean;
  pawnIOInstallerPath?: string;
  plugins?: Array<{ id: string; name: string; running: boolean; lastError?: string }>;
}

export interface DeviceDebugFrame {
  id: number;
  direction: string;
  transport: string;
  timestamp: string;
  rawHex: string;
  frameHex: string;
  command: string;
  length: number;
  payloadHex: string;
  checksumOk: boolean;
  description: string;
  decoded?: string;
  parsed?: unknown;
}

export interface DeviceDebugCommandResult {
  transport: string;
  inputHex: string;
  frameHex: string;
  rawHex: string;
  waitMs: number;
  frames: DeviceDebugFrame[];
}

export interface DeviceGearRPM {
  gear: number;
  label: string;
  rpm: number;
}

export interface DeviceStatusRead {
  gearSetting?: string;
  maxGear?: string;
  selected?: string;
  mode?: string;
  modeName?: string;
  smartStartStop?: string;
  smartStartStopName?: string;
  currentRpm?: number;
  targetRpm?: number;
}

export interface DeviceSettings {
  available: boolean;
  source: string;
  readAt: string;
  model?: string;
  gearRpmTable?: DeviceGearRPM[];
  workMode?: string;
  workModeName?: string;
  rgbState?: string;
  rgbStateName?: string;
  status?: DeviceStatusRead;
  rawFrames?: DeviceDebugFrame[];
}

// 自启动方式
export type AutoStartMethod = 'none' | 'task_scheduler' | 'registry';

// 自启动信息
export interface AutoStartInfo {
  enabled: boolean;
  method: AutoStartMethod;
  isAdmin: boolean;
}

// 挡位命令
export interface GearCommand {
  name: string;    // 挡位名称
  command: number[]; // 命令字节
  rpm: number;     // 对应转速
}

// 设备状态
export interface DeviceStatus {
  connected: boolean;
  monitoring: boolean;
  currentData: FanData | null;
  temperature: TemperatureData;
  productId?: string;
  model?: string;
}

// 自定义主题元数据（由后端 ListThemes 返回）
export interface ThemeMeta {
  id: string;
  name: string;
  base: string;        // light | dark
  author?: string;
  version?: string;
  description?: string;
  source: string;      // user | install | builtin
}

// 设备信息
export interface DeviceInfo {
  manufacturer: string;
  product: string;
  serial: string;
  model?: string;
  productId?: string;
}

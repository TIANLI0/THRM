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
  maxTemp: number;     // 最高温度
  controlTemp?: number; // 当前控温基准温度
  controlSource?: 'max' | 'cpu' | 'gpu'; // 当前控温基准来源
  cpuModel?: string;   // 当前识别的 CPU 型号
  gpuModel?: string;   // 当前识别的 GPU 型号
  cpuSensors?: TemperatureSensor[]; // 当前识别的 CPU 温度传感器
  gpuSensors?: TemperatureSensor[]; // 当前识别的 GPU 温度传感器
  updateTime: number;  // 更新时间戳
  bridgeOk?: boolean;  // 桥接程序是否正常
  bridgeMessage?: string; // 桥接程序提示
}

export interface TemperatureSensor {
  key: string;
  name: string;
  value: number;
}

// 应用配置
export interface AppConfig {
  legionFnQ?: LegionFnQConfig;
  autoControl: boolean;         // 智能变频开关
  curveProfileToggleHotkey?: string; // 切换曲线方案快捷键
  fanCurve: FanCurvePoint[];   // 风扇曲线
  fanCurveProfiles?: FanCurveProfile[];
  activeFanCurveProfileId?: string;
  gearLight: boolean;          // 挡位灯
  powerOnStart: boolean;       // 通电自启动
  windowsAutoStart: boolean;   // Windows开机自启动
  themeMode?: 'system' | 'light' | 'dark'; // 主题模式
  smartStartStop: string;      // 智能启停
  brightness: number;          // 亮度
  tempUpdateRate: number;      // 温度更新频率(秒)
  tempSampleCount?: number;
  tempSource?: 'max' | 'cpu' | 'gpu';
  cpuSensor?: string;
  gpuSensor?: string;
  configPath: string;          // 配置文件路径
  manualGear: string;          // 手动挡位设置
  manualLevel: string;         // 手动挡位级别(低中高)
  debugMode: boolean;          // 调试模式
  guiMonitoring: boolean;      // GUI监控开关
  customSpeedEnabled: boolean; // 自定义转速开关
  customSpeedRPM: number;      // 自定义转速值(无上下限)
  smartControl: SmartControlConfig; // 学习型智能控温
}

export interface SmartControlConfig {
  enabled: boolean;
  learning: boolean;
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

export interface LegionPowerModePayload {
  raw: number;
  mapped: number;
  mode: string;
  source: string;
  timestamp: number;
}

export interface DebugInfo {
  debugMode: boolean;
  trayReady: boolean;
  trayInitialized: boolean;
  isConnected: boolean;
  guiLastResponse: string;
  monitoringTemp: boolean;
  autoStartLaunch: boolean;
  plugins?: Array<{ id: string; name: string; running: boolean; lastError?: string }>;
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

// 设备信息
export interface DeviceInfo {
  manufacturer: string;
  product: string;
  serial: string;
  model?: string;
  productId?: string;
}

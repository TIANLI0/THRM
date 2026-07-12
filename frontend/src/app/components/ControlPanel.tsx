'use client';

import React, { useState, useCallback, useEffect, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Play,
  Pause,
  Settings,
  Languages,
  Lightbulb,
  Power,
  Zap,
  Monitor,
  Cpu,
  Gpu,
  Bug,
  TriangleAlert,
  CheckCircle2,
  ChevronDown,
  Flame,
  Clock3,
  BarChart3,
  Spline,
  Sparkles,
  X,
  RotateCw,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { toast } from 'sonner';
import { DebugInfo, type DeviceDebugCommandResult, type DeviceSettings, type ThemeMeta } from '../types/app';
import { type AppLocale, useLocale } from '../lib/i18n';
import { getManualGearLabel, getManualLevelLabel } from '../lib/manualGearPresets';
import FanCurveProfileSelect from './FanCurveProfileSelect';
import { ToggleSwitch, Button, Select, MultiSelect, Slider } from './ui/index';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  legionFnQSupported: boolean;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
}

type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };
type ParsedGearTable = { type?: string; table?: Array<{ gear?: number; label?: string; rpm?: number }> };

/* ── Helpers ── */

function getDefaultLightStripConfig(): types.LightStripConfig {
  return types.LightStripConfig.createFrom({
    mode: 'smart_temp',
    speed: 'medium',
    brightness: 100,
    colors: [
      { r: 255, g: 0, b: 0 },
      { r: 0, g: 255, b: 0 },
      { r: 0, g: 128, b: 255 },
    ],
  });
}

function normalizeLightStripConfig(config: types.AppConfig): types.LightStripConfig {
  const defaults = getDefaultLightStripConfig();
  const raw = (config as any).lightStrip;
  if (!raw) return defaults;

  const normalized = types.LightStripConfig.createFrom({
    mode: raw.mode || defaults.mode,
    speed: raw.speed || defaults.speed,
    brightness: typeof raw.brightness === 'number' ? Math.max(0, Math.min(100, raw.brightness)) : defaults.brightness,
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : defaults.colors,
  });

  if ((normalized.colors || []).length < 3) {
    const merged = [...(normalized.colors || [])];
    while (merged.length < 3) merged.push(defaults.colors[merged.length]);
    normalized.colors = merged;
  }
  return normalized;
}

function renderDebugFrameSummary(frame: { decoded?: string; parsed?: unknown; command?: string; payloadHex?: string }) {
  const parsed = frame.parsed as ParsedGearTable | null | undefined;
  if (parsed?.type === 'gearRpmTable' && Array.isArray(parsed.table)) {
    return parsed.table
      .map((item) => `${item.label || item.gear}: ${item.rpm} RPM`)
      .join(' | ');
  }
  return frame.decoded || `${frame.command || '--'} ${frame.payloadHex || ''}`.trim();
}

function rgbToHex(color: types.RGBColor): string {
  const h = (v: number) => v.toString(16).padStart(2, '0');
  return `#${h(color.r || 0)}${h(color.g || 0)}${h(color.b || 0)}`;
}

function hexToRgb(hex: string): types.RGBColor {
  const n = Number.parseInt(hex.replace('#', ''), 16);
  return types.RGBColor.createFrom({ r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 });
}

function getRequiredColorCount(mode: string): number {
  switch (mode) {
    case 'static_single': return 1;
    case 'off': case 'smart_temp': case 'flowing': return 0;
    case 'static_multi': return 3;
    default: return 3;
  }
}

const LEGION_POWER_MODE_VALUES = ['Quiet', 'Balance', 'Performance', 'Extreme', 'GodMode'] as const;
const FAN_GEAR_VALUES = ['静音', '标准', '强劲', '超频'] as const;
const FAN_LEVEL_VALUES = ['低', '中', '高'] as const;

// 高危调试命令：直接读写固件底层/调试寄存器，误用可能导致设备异常甚至变砖，需在发送前红色提醒。
const DANGEROUS_DEBUG_COMMANDS = new Set<number>([0xed, 0xee, 0xf0, 0xf1, 0xf2]);

// 从用户输入中解析出命令字节：兼容 "27" 与 "5A A5 27 02 29"（带帧头时命令为第三字节）。
function parseDebugCommandByte(input: string): number | null {
  const bytes = input
    .trim()
    .split(/[^0-9a-fA-F]+/)
    .filter(Boolean)
    .map((h) => Number.parseInt(h, 16));
  if (bytes.length === 0 || bytes.some((b) => Number.isNaN(b))) return null;
  if (bytes.length >= 3 && bytes[0] === 0x5a && bytes[1] === 0xa5) return bytes[2];
  return bytes[0];
}

// 模块级常量：在组件外定义，所有 render 共享同一个引用，避免每次重渲染都新建数组导致下游 Select 等组件 props 引用变化。
const SMART_START_STOP_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.smartStartStop.off.label', descriptionKey: 'controlPanel.options.smartStartStop.off.description' },
  { value: 'immediate', labelKey: 'controlPanel.options.smartStartStop.immediate.label', descriptionKey: 'controlPanel.options.smartStartStop.immediate.description' },
  { value: 'delayed', labelKey: 'controlPanel.options.smartStartStop.delayed.label', descriptionKey: 'controlPanel.options.smartStartStop.delayed.description' },
];

// 温度平滑度（EMA 系数）选项。
// 数字越大平滑越强、对突发温度反应越慢；越小越灵敏但可能跟着抖。
// 后端 α = 2/(N+1)：1→即时，5→约 5 周期窗口，10→约 10 周期窗口。
const SAMPLE_COUNT_OPTIONS = [
  { value: 1, labelKey: 'controlPanel.options.sampleCount.1' },
  { value: 2, labelKey: 'controlPanel.options.sampleCount.2' },
  { value: 3, labelKey: 'controlPanel.options.sampleCount.3' },
  { value: 5, labelKey: 'controlPanel.options.sampleCount.5' },
  { value: 10, labelKey: 'controlPanel.options.sampleCount.10' },
];

const TEMP_SOURCE_OPTIONS = [
  { value: 'max', labelKey: 'controlPanel.options.tempSource.max' },
  { value: 'cpu', labelKey: 'controlPanel.options.tempSource.cpu' },
  { value: 'gpu', labelKey: 'controlPanel.options.tempSource.gpu' },
];

// 内置基础主题选项。自定义主题（如 THRM）改为运行时从安装目录动态发现并追加。
const THEME_MODE_OPTIONS = [
  { value: 'light', labelKey: 'controlPanel.options.themeMode.light' },
  { value: 'dark', labelKey: 'controlPanel.options.themeMode.dark' },
  { value: 'system', labelKey: 'controlPanel.options.themeMode.system' },
];

const LIGHT_MODE_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.lightMode.off.label', descriptionKey: 'controlPanel.options.lightMode.off.description' },
  { value: 'smart_temp', labelKey: 'controlPanel.options.lightMode.smart_temp.label', descriptionKey: 'controlPanel.options.lightMode.smart_temp.description' },
  { value: 'static_single', labelKey: 'controlPanel.options.lightMode.static_single.label', descriptionKey: 'controlPanel.options.lightMode.static_single.description' },
  { value: 'static_multi', labelKey: 'controlPanel.options.lightMode.static_multi.label', descriptionKey: 'controlPanel.options.lightMode.static_multi.description' },
  { value: 'rotation', labelKey: 'controlPanel.options.lightMode.rotation.label', descriptionKey: 'controlPanel.options.lightMode.rotation.description' },
  { value: 'flowing', labelKey: 'controlPanel.options.lightMode.flowing.label', descriptionKey: 'controlPanel.options.lightMode.flowing.description' },
  { value: 'breathing', labelKey: 'controlPanel.options.lightMode.breathing.label', descriptionKey: 'controlPanel.options.lightMode.breathing.description' },
];

const LIGHT_SPEED_OPTIONS = [
  { value: 'fast', labelKey: 'controlPanel.options.lightSpeed.fast' },
  { value: 'medium', labelKey: 'controlPanel.options.lightSpeed.medium' },
  { value: 'slow', labelKey: 'controlPanel.options.lightSpeed.slow' },
];

const LIGHT_COLOR_PRESETS = [
  { nameKey: 'controlPanel.options.lightPresets.neon', colors: [{ r: 255, g: 0, b: 128 }, { r: 0, g: 255, b: 255 }, { r: 128, g: 0, b: 255 }] },
  { nameKey: 'controlPanel.options.lightPresets.forest', colors: [{ r: 86, g: 169, b: 84 }, { r: 161, g: 210, b: 106 }, { r: 44, g: 120, b: 115 }] },
  { nameKey: 'controlPanel.options.lightPresets.glacier', colors: [{ r: 80, g: 170, b: 255 }, { r: 116, g: 214, b: 255 }, { r: 200, g: 240, b: 255 }] },
];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function translateWorkMode(
  workMode: string | null | undefined,
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  switch (workMode) {
    case '挡位工作模式':
      return t('controlPanel.overview.workModes.manual');
    case '自动模式(实时转速)':
      return t('controlPanel.overview.workModes.auto');
    default:
      return workMode || '--';
  }
}

function translateControlSource(
  source: string | null | undefined,
  t: (key: string) => string,
) {
  switch (source) {
    case 'cpu':
      return t('controlPanel.options.tempSource.cpu');
    case 'gpu':
      return t('controlPanel.options.tempSource.gpu');
    default:
      return t('controlPanel.options.tempSource.max');
  }
}

function getDefaultLegionFnQConfig() {
  return {
    enabled: false,
    takeOverFan: false,
    modeMapping: {
      Quiet: { gear: '静音', level: '中' },
      Balance: { gear: '标准', level: '中' },
      Performance: { gear: '强劲', level: '中' },
      Extreme: { gear: '超频', level: '中' },
      GodMode: { gear: '超频', level: '高' },
    },
  };
}

function normalizeLegionFnQConfig(raw: any) {
  const defaults = getDefaultLegionFnQConfig();
  const mapping = { ...defaults.modeMapping, ...(raw?.modeMapping || {}) };
  return {
    enabled: !!raw?.enabled,
    takeOverFan: !!raw?.takeOverFan,
    modeMapping: mapping,
  };
}

function normalizeHotkeyForDisplay(value: string): string {
  return (value || '').trim();
}

function buildShortcutFromKeyboardEvent(e: {
  key: string;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
}): string {
  if (e.key === 'Backspace' || e.key === 'Delete') {
    return '';
  }

  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Win');

  const key = e.key;
  if (['Control', 'Alt', 'Shift', 'Meta'].includes(key)) {
    return '';
  }

  let mainKey = '';
  if (/^[a-z]$/i.test(key)) {
    mainKey = key.toUpperCase();
  } else if (/^[0-9]$/.test(key)) {
    mainKey = key;
  } else if (/^F\d{1,2}$/i.test(key)) {
    mainKey = key.toUpperCase();
  }

  if (!mainKey || parts.length === 0) {
    return '';
  }

  return [...parts, mainKey].join('+');
}

/* ── Section wrapper ── */

function Section({
  title,
  icon: Icon,
  children,
  className,
}: {
  title: string;
  icon: React.ComponentType<{ className?: string }>;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={clsx('rounded-2xl border border-border bg-card shadow-sm', className)}>
      <div className="flex items-center gap-2.5 border-b border-border/60 px-5 py-4">
        <Icon className="h-4.5 w-4.5 text-muted-foreground" />
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
      </div>
      <div className="divide-y divide-border/60">{children}</div>
    </section>
  );
}

/* ── Setting row ── */

function SettingRow({
  icon,
  title,
  description,
  tip,
  children,
  disabled,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  tip?: string;
  children: React.ReactNode;
  disabled?: boolean;
}) {
  return (
    <div className={clsx('flex items-center justify-between gap-4 px-5 py-4', disabled && 'opacity-50')}>
      <div className="flex items-center gap-3 min-w-0">
        {icon && (
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <div className="text-base font-medium text-foreground">{title}</div>
          {description && <div className="text-sm text-muted-foreground line-clamp-2">{description}</div>}
          {tip && <div className="mt-0.5 text-xs text-primary/80">{tip}</div>}
        </div>
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function HotkeyField({
  title,
  description,
  value,
  placeholder,
  clearAriaLabel,
  recording,
  onFocus,
  onBlur,
  onKeyDown,
  onClear,
}: {
  title: string;
  description: string;
  value: string;
  placeholder: string;
  clearAriaLabel: string;
  recording: boolean;
  onFocus: () => void;
  onBlur: () => void;
  onKeyDown: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  onClear: () => void;
}) {
  return (
    <div className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 md:flex-row md:items-center md:gap-4">
      <div className="min-w-0 flex-1 pr-2">
        <div className="text-sm text-foreground">{title}</div>
        <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{description}</div>
      </div>

      <div className="w-full md:ml-auto md:w-[240px] md:flex-none">
        <div className="relative">
          <input
            value={value}
            onFocus={onFocus}
            onBlur={onBlur}
            onKeyDown={onKeyDown}
            readOnly
            placeholder={placeholder}
            className={clsx(
              'w-full rounded-lg border bg-background px-3 py-2.5 pr-9 text-center font-mono text-sm text-foreground outline-none transition',
              recording
                ? 'border-primary shadow-[0_0_0_1px_var(--color-primary)] ring-2 ring-primary/20'
                : 'border-border/70 hover:border-border',
            )}
          />
          {value && (
            <button
              type="button"
              aria-label={clearAriaLabel}
              onMouseDown={(e) => e.preventDefault()}
              onClick={onClear}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function SelectionField({
  label,
  children,
  hint,
}: {
  label: string;
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <div className="space-y-1.5">
      <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</div>
      {children}
      {hint && <div className="text-[11px] leading-relaxed text-muted-foreground">{hint}</div>}
    </div>
  );
}

/* ── Main ControlPanel ── */

export default function ControlPanel({ config, onConfigChange, isConnected, fanData, temperature, legionFnQSupported, deviceModel }: ControlPanelProps) {
  const { t } = useTranslation();
  const { locale, setLocale } = useLocale();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null);
  const [debugInfoLoading, setDebugInfoLoading] = useState(false);
  const [debugPanelOpen, setDebugPanelOpen] = useState(false);
  const [debugCommandInput, setDebugCommandInput] = useState('27');
  const [debugCommandResult, setDebugCommandResult] = useState<DeviceDebugCommandResult | null>(null);
  const [debugCommandLoading, setDebugCommandLoading] = useState(false);
  const debugCommandByte = useMemo(() => parseDebugCommandByte(debugCommandInput), [debugCommandInput]);
  const isDangerousDebugCommand = debugCommandByte !== null && DANGEROUS_DEBUG_COMMANDS.has(debugCommandByte);
  const [showCustomSpeedWarning, setShowCustomSpeedWarning] = useState(false);
  // 安装目录/用户目录下发现的自定义主题（用于「界面主题」下拉动态渲染）
  const [customThemes, setCustomThemes] = useState<ThemeMeta[]>([]);
  const [customSpeedInput, setCustomSpeedInput] = useState<number>((config as any).customSpeedRPM || 2000);
  const [lightStripConfig, setLightStripConfig] = useState<types.LightStripConfig>(() => normalizeLightStripConfig(config));
  const [manualHotkeyInput, setManualHotkeyInput] = useState(
    normalizeHotkeyForDisplay((config as any).manualGearToggleHotkey)
  );
  const [autoHotkeyInput, setAutoHotkeyInput] = useState(
    normalizeHotkeyForDisplay((config as any).autoControlToggleHotkey)
  );
  const [curveProfileHotkeyInput, setCurveProfileHotkeyInput] = useState(
    normalizeHotkeyForDisplay((config as any).curveProfileToggleHotkey)
  );
  const [recordingTarget, setRecordingTarget] = useState<'manual' | 'auto' | 'curve' | null>(null);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [curveProfileLoading, setCurveProfileLoading] = useState(false);
  const [temperatureHistoryEnabled, setTemperatureHistoryEnabled] = useState(false);

  const activeCurveProfileId = ((config as any).activeFanCurveProfileId || '') as string;
  const isBs1 = deviceModel === 'BS1';
  const currentTempSource = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
  const cpuSensors = useMemo(() => (Array.isArray(temperature?.cpuSensors) ? temperature.cpuSensors : []), [temperature?.cpuSensors]);
  const gpuSensors = useMemo(() => (Array.isArray(temperature?.gpuSensors) ? temperature.gpuSensors : []), [temperature?.gpuSensors]);
  // 收窄依赖：旧实现依赖整个 temperature 对象，温度每秒推送都会重算并产生新数组引用，
  // 导致下游 Select 等组件无谓重渲染。改为只依赖具体字段。
  const rawGpuDevices = (temperature as any)?.gpuDevices;
  const gpuDevices = useMemo(
    () => (Array.isArray(rawGpuDevices) ? (rawGpuDevices as types.TemperatureGPUDevice[]) : []),
    [rawGpuDevices],
  );
  const selectedGpuDevice = useMemo(() => {
    const configured = (((config as any).gpuDevice as string) || 'auto');
    return configured === 'auto' || gpuDevices.some((device) => device.key === configured) ? configured : 'auto';
  }, [config, gpuDevices]);
  const detectedGpuDevice = (temperature as any)?.selectedGpuDevice;
  const activeGpuDeviceKey = useMemo(() => {
    if (selectedGpuDevice !== 'auto') {
      return selectedGpuDevice;
    }
    const detected = (detectedGpuDevice as string) || 'auto';
    return gpuDevices.some((device) => device.key === detected) ? detected : 'auto';
  }, [selectedGpuDevice, detectedGpuDevice, gpuDevices]);
  const activeGpuDevice = useMemo(() => {
    return gpuDevices.find((device) => device.key === activeGpuDeviceKey) || null;
  }, [activeGpuDeviceKey, gpuDevices]);
  const effectiveGpuSensors = useMemo(() => {
    if (activeGpuDevice && Array.isArray(activeGpuDevice.sensors) && activeGpuDevice.sensors.length > 0) {
      return activeGpuDevice.sensors;
    }
    return gpuSensors;
  }, [activeGpuDevice, gpuSensors]);
  // 多选 CPU 传感器（多核平均）：仅保留当前仍存在的传感器 key；为空表示自动。
  const selectedCpuSensors = useMemo(() => {
    const arr = Array.isArray((config as any).cpuSensors) ? ((config as any).cpuSensors as string[]) : [];
    return arr.filter((key) => cpuSensors.some((sensor) => sensor.key === key));
  }, [config, cpuSensors]);
  const selectedGpuSensor = useMemo(() => {
    const configured = (((config as any).gpuSensor as string) || 'auto');
    return effectiveGpuSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, effectiveGpuSensors]);
  const legionFnQConfig = useMemo(() => normalizeLegionFnQConfig((config as any).legionFnQ), [config]);
  const legionPowerModes = useMemo(
    () => LEGION_POWER_MODE_VALUES.map((value) => ({ value, label: t(`controlPanel.options.legionPowerModes.${value}`) })),
    [locale, t],
  );
  const fanGearOptions = useMemo(
    () => FAN_GEAR_VALUES.map((value) => ({ value, label: getManualGearLabel(value) })),
    [locale],
  );
  const fanLevelOptions = useMemo(
    () => FAN_LEVEL_VALUES.map((value) => ({ value, label: getManualLevelLabel(value) })),
    [locale],
  );
  const smartStartStopOptions = useMemo(
    () => SMART_START_STOP_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey), description: t(item.descriptionKey) })),
    [locale, t],
  );
  const sampleCountOptions = useMemo(
    () => SAMPLE_COUNT_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const tempSourceOptions = useMemo(
    () => TEMP_SOURCE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  // 内置基础主题选项 + 运行时发现的自定义主题（如 THRM）。
  const themeModeOptions = useMemo(
    () => [
      ...THEME_MODE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
      ...customThemes.map((theme) => ({ value: theme.id, label: theme.name })),
    ],
    [locale, t, customThemes],
  );
  const windowBlurOptions = useMemo(
    () => [
      { value: 'auto', label: t('controlPanel.options.windowBlur.auto') },
      { value: 'acrylic', label: t('controlPanel.options.windowBlur.acrylic') },
      { value: 'mica', label: t('controlPanel.options.windowBlur.mica') },
      { value: 'tabbed', label: t('controlPanel.options.windowBlur.tabbed') },
      { value: 'off', label: t('controlPanel.options.windowBlur.off') },
    ],
    [locale, t],
  );
  const lightModeOptions = useMemo(
    () => LIGHT_MODE_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey), description: t(item.descriptionKey) })),
    [locale, t],
  );
  const lightSpeedOptions = useMemo(
    () => LIGHT_SPEED_OPTIONS.map((item) => ({ value: item.value, label: t(item.labelKey) })),
    [locale, t],
  );
  const lightColorPresets = useMemo(
    () => LIGHT_COLOR_PRESETS.map((item) => ({ name: t(item.nameKey), colors: item.colors })),
    [locale, t],
  );
  const languageOptions = useMemo(
    () => ([
      { value: 'zh-CN', label: t('common.languages.zh-CN') },
      { value: 'en-US', label: t('common.languages.en-US') },
      { value: 'ja-JP', label: t('common.languages.ja-JP') },
    ]),
    [locale, t],
  );

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

  /* ── Handlers (same logic as before) ── */

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    setLoading('autoControl', true);
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch { /* noop */ } finally { setLoading('autoControl', false); }
  }, [config, onConfigChange]);

  const handleCustomSpeedApply = useCallback(async (enabled: boolean, rpm: number) => {
    setLoading('customSpeed', true);
    try {
      await apiService.setCustomSpeed(enabled, rpm);
      onConfigChange(types.AppConfig.createFrom({
        ...config,
        customSpeedEnabled: enabled,
        customSpeedRPM: rpm,
        autoControl: enabled ? false : config.autoControl,
      }));
    } catch { /* noop */ } finally { setLoading('customSpeed', false); }
  }, [config, onConfigChange]);

  const handleCustomSpeedToggle = useCallback((enabled: boolean) => {
    if (enabled) setShowCustomSpeedWarning(true);
    else handleCustomSpeedApply(false, customSpeedInput);
  }, [customSpeedInput, handleCustomSpeedApply]);

  const handleGearLightChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('gearLight', true);
    try {
      const ok = await apiService.setGearLight(enabled);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, gearLight: enabled }));
    } catch { /* noop */ } finally { setLoading('gearLight', false); }
  }, [config, onConfigChange, isConnected]);

  const handlePowerOnStartChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('powerOnStart', true);
    try {
      const ok = await apiService.setPowerOnStart(enabled);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, powerOnStart: enabled }));
    } catch { /* noop */ } finally { setLoading('powerOnStart', false); }
  }, [config, onConfigChange, isConnected]);

  const handleWindowsAutoStartChange = useCallback(async (enabled: boolean) => {
    setLoading('windowsAutoStart', true);
    try {
      const isAdmin = await apiService.isRunningAsAdmin();
      if (enabled) await apiService.setAutoStartWithMethod(true, isAdmin ? 'task_scheduler' : 'registry');
      else await apiService.setAutoStartWithMethod(false, '');
      onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
    } catch (e) { alert(t('controlPanel.alerts.autoStartFailed', { error: getErrorMessage(e) })); } finally { setLoading('windowsAutoStart', false); }
  }, [config, onConfigChange, t]);

  const handleIgnoreDeviceOnReconnectChange = useCallback(async (enabled: boolean) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, ignoreDeviceOnReconnect: enabled });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const handleSmartStartStopChange = useCallback(async (mode: string) => {
    if (!isConnected) return;
    try {
      const ok = await apiService.setSmartStartStop(mode);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, smartStartStop: mode }));
    } catch { /* noop */ }
  }, [config, onConfigChange, isConnected]);

  const toggleDebugMode = useCallback(async () => {
    try {
      await apiService.setDebugMode(!config.debugMode);
      onConfigChange(types.AppConfig.createFrom({ ...config, debugMode: !config.debugMode }));
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const fetchDebugInfo = useCallback(async () => {
    setDebugInfoLoading(true);
    try { setDebugInfo(await apiService.getDebugInfo()); } catch { /* noop */ } finally { setDebugInfoLoading(false); }
  }, []);

  const sendDeviceDebugCommand = useCallback(async (command?: string) => {
    const hexCommand = (command ?? debugCommandInput).trim();
    if (!hexCommand || !isConnected || !config.debugMode) return;
    setDebugCommandLoading(true);
    try {
      const result = await apiService.sendDeviceDebugCommand(hexCommand, 900);
      setDebugCommandResult(result);
      toast.success(`已发送 ${result.frameHex}`);
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setDebugCommandLoading(false);
    }
  }, [config.debugMode, debugCommandInput, isConnected]);

  const handleReinstallPawnIO = useCallback(async () => {
    setLoading('pawnIOReinstall', true);
    try {
      const result = await apiService.reinstallPawnIO();
      toast.success(t('controlPanel.debug.toasts.reinstallExecuted'));
      if (result?.warning) {
        toast.warning(result.warning);
      }
      if (result?.uninstallWarning) {
        toast.warning(t('controlPanel.debug.toasts.uninstallWarning', { warning: result.uninstallWarning }));
      }
      if (result?.bridgeWarning) {
        toast.warning(t('controlPanel.debug.toasts.bridgeWarning', { warning: result.bridgeWarning }));
      }
      await fetchDebugInfo();
    } catch (error) {
      toast.error(t('controlPanel.debug.toasts.reinstallFailed', { error: getErrorMessage(error) }));
    } finally {
      setLoading('pawnIOReinstall', false);
    }
  }, [fetchDebugInfo, t]);

  const handleSampleCountChange = useCallback(async (count: number) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, tempSampleCount: count });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const handleTempSourceChange = useCallback(async (source: string) => {
    setLoading('tempSource', true);
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, tempSource: source });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('tempSource', false);
    }
  }, [config, onConfigChange]);

  const handleGpuDeviceChange = useCallback(async (deviceKey: string) => {
    setLoading('gpuDevice', true);
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        gpuDevice: deviceKey,
        gpuSensor: 'auto',
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('gpuDevice', false);
    }
  }, [config, onConfigChange]);

  const handleTempSensorChange = useCallback(async (kind: 'cpu' | 'gpu', sensorKey: string) => {
    const loadingKey = kind === 'cpu' ? 'cpuSensor' : 'gpuSensor';
    setLoading(loadingKey, true);
    try {
      const patch = kind === 'cpu' ? { cpuSensor: sensorKey } : { gpuSensor: sensorKey };
      const newCfg = types.AppConfig.createFrom({ ...config, ...patch });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading(loadingKey, false);
    }
  }, [config, onConfigChange]);

  const handleCpuSensorsChange = useCallback(async (keys: string[]) => {
    setLoading('cpuSensors', true);
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, cpuSensors: keys });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('cpuSensors', false);
    }
  }, [config, onConfigChange]);

  const handleWindowBlurChange = useCallback(async (mode: string) => {
    setLoading('windowBlur', true);
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, windowBlur: mode });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('windowBlur', false);
    }
  }, [config, onConfigChange]);

  const handleSuspendFanOffChange = useCallback(async (enabled: boolean) => {
    setLoading('suspendFanOff', true);
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, suspendFanOff: enabled });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('suspendFanOff', false);
    }
  }, [config, onConfigChange]);

  const handleTransientSpikeFilterChange = useCallback(async (enabled: boolean) => {
    setLoading('transientSpikeFilter', true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({
        ...(config.smartControl || {}),
        filterTransientSpike: enabled,
      });
      const newCfg = types.AppConfig.createFrom({
        ...config,
        smartControl: nextSmartControl,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('transientSpikeFilter', false);
    }
  }, [config, onConfigChange]);

  const handleLearningToggle = useCallback(async (enabled: boolean) => {
    setLoading('learning', true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({
        ...(config.smartControl || {}),
        learning: enabled,
      });
      const newCfg = types.AppConfig.createFrom({
        ...config,
        smartControl: nextSmartControl,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ } finally {
      setLoading('learning', false);
    }
  }, [config, onConfigChange]);

  const handleTemperatureHistoryChange = useCallback(async (enabled: boolean) => {
    setLoading('temperatureHistory', true);
    try {
      await apiService.setTemperatureHistoryEnabled(enabled);
      setTemperatureHistoryEnabled(enabled);
    } catch { /* noop */ } finally {
      setLoading('temperatureHistory', false);
    }
  }, []);

  const updateLegionFnQConfig = useCallback(async (patch: any) => {
    if (!legionFnQSupported) return;
    setLoading('legionFnQ', true);
    try {
      const nextLegionFnQ = normalizeLegionFnQConfig({
        ...legionFnQConfig,
        ...patch,
        modeMapping: patch.modeMapping || legionFnQConfig.modeMapping,
      });
      const newCfg = types.AppConfig.createFrom({
        ...config,
        legionFnQ: nextLegionFnQ,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch (e) {
      toast.error(t('controlPanel.legionFnQ.toasts.saveFailed', { error: getErrorMessage(e) }));
    } finally {
      setLoading('legionFnQ', false);
    }
  }, [config, legionFnQConfig, legionFnQSupported, onConfigChange, t]);

  const handleLegionFnQMappingChange = useCallback(async (mode: string, patch: { gear?: string; level?: string }) => {
    const current = legionFnQConfig.modeMapping[mode] || (getDefaultLegionFnQConfig().modeMapping as any)[mode];
    await updateLegionFnQConfig({
      modeMapping: {
        ...legionFnQConfig.modeMapping,
        [mode]: {
          ...current,
          ...patch,
        },
      },
    });
  }, [legionFnQConfig, updateLegionFnQConfig]);

  const loadCurveProfiles = useCallback(async () => {
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      setCurveProfiles(profiles);
    } catch {
      setCurveProfiles([]);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const loadTelemetryState = async () => {
      try {
        const payload = await apiService.getTemperatureHistory();
        if (!cancelled) {
          setTemperatureHistoryEnabled(payload?.enabled !== false);
        }
      } catch {
        if (!cancelled) {
          setTemperatureHistoryEnabled(false);
        }
      }
    };

    loadTelemetryState();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleCurveProfileChange = useCallback(async (profileId: string) => {
    if (!profileId || profileId === activeCurveProfileId) return;
    try {
      setCurveProfileLoading(true);
      await apiService.setActiveFanCurveProfile(profileId);
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
      await loadCurveProfiles();
      toast.success(t('controlPanel.fan.toasts.profileSwitched'));
    } catch (e) {
      toast.error(t('controlPanel.fan.toasts.profileSwitchFailed', { error: getErrorMessage(e) }));
    } finally {
      setCurveProfileLoading(false);
    }
  }, [activeCurveProfileId, loadCurveProfiles, onConfigChange, t]);

  useEffect(() => {
    const i = window.setInterval(() => {
      if (document.hidden) {
        return;
      }
      apiService.updateGuiResponseTime().catch(() => {});
    }, 60000);
    return () => window.clearInterval(i);
  }, []);
  useEffect(() => { loadCurveProfiles(); }, [loadCurveProfiles]);
  useEffect(() => { setLightStripConfig(normalizeLightStripConfig(config)); }, [config]);
  useEffect(() => {
    setManualHotkeyInput(normalizeHotkeyForDisplay((config as any).manualGearToggleHotkey));
    setAutoHotkeyInput(normalizeHotkeyForDisplay((config as any).autoControlToggleHotkey));
    setCurveProfileHotkeyInput(normalizeHotkeyForDisplay((config as any).curveProfileToggleHotkey));
  }, [(config as any).manualGearToggleHotkey, (config as any).autoControlToggleHotkey, (config as any).curveProfileToggleHotkey]);

  /* ── Options data ── */

  // 仅依赖 props 的 options 用 useMemo 缓存；纯静态选项已外提到模块级常量。
  const gpuDeviceOptions = useMemo(() => [
    { value: 'auto', label: gpuDevices.length > 0 ? t('controlPanel.options.gpuDevice.autoPreferred') : t('controlPanel.options.gpuDevice.auto') },
    ...gpuDevices.map((device) => ({
      value: device.key,
      label: `${device.vendor ? `${device.vendor.toUpperCase()} · ` : ''}${device.name}`,
    })),
  ], [gpuDevices, locale, t]);

  const gpuSensorOptions = useMemo(() => [
    { value: 'auto', label: effectiveGpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...effectiveGpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ], [effectiveGpuSensors, locale, t]);

  const requiredColorCount = getRequiredColorCount(lightStripConfig.mode);

  const handleLightColorChange = useCallback((index: number, hex: string) => {
    setLightStripConfig((prev) => {
      const colors = [...(prev.colors || [])];
      while (colors.length < 3) colors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
      colors[index] = hexToRgb(hex);
      return types.LightStripConfig.createFrom({ ...prev, colors });
    });
  }, []);

  const handleApplyLightStrip = useCallback(async () => {
    setLoading('lightStrip', true);
    try {
      const normalizedColors = [...(lightStripConfig.colors || [])];
      if (requiredColorCount > 0) while (normalizedColors.length < requiredColorCount) normalizedColors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
      const submitConfig = types.LightStripConfig.createFrom({
        ...lightStripConfig,
        colors: requiredColorCount > 0 ? normalizedColors.slice(0, Math.max(requiredColorCount, 3)) : normalizedColors,
      });
      await apiService.setLightStrip(submitConfig);
      onConfigChange(types.AppConfig.createFrom({ ...config, lightStrip: submitConfig }));
    } catch (e) { alert(t('controlPanel.alerts.lightStripFailed', { error: getErrorMessage(e) })); } finally { setLoading('lightStrip', false); }
  }, [lightStripConfig, config, onConfigChange, requiredColorCount, t]);

  // 加载安装目录/用户目录下发现的自定义主题。
  useEffect(() => {
    let cancelled = false;
    apiService
      .listThemes()
      .then((themes) => {
        if (!cancelled) setCustomThemes(themes);
      })
      .catch(() => {
        /* 列表获取失败时仅保留内置基础主题 */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleThemeModeChange = useCallback(async (mode: string) => {
    // system/light/dark 为内置；其余按自定义主题 id 校验（须在已发现列表中）。
    const isBuiltin = mode === 'light' || mode === 'dark' || mode === 'system';
    const isKnownCustom = customThemes.some((theme) => theme.id === mode);
    const nextMode = isBuiltin || isKnownCustom ? mode : 'system';
    try {
      const newCfg = types.AppConfig.createFrom({
        ...config,
        themeMode: nextMode,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch {
      /* noop */
    }
  }, [config, onConfigChange, customThemes]);

  const handleOpenThemesFolder = useCallback(async () => {
    try {
      await apiService.openThemesFolder();
    } catch {
      /* noop */
    }
  }, []);

  const saveHotkeys = useCallback(async (silent = false) => {
    setLoading('hotkeys', true);
    try {
      const manualValue = normalizeHotkeyForDisplay(manualHotkeyInput);
      const autoValue = normalizeHotkeyForDisplay(autoHotkeyInput);
      const curveValue = normalizeHotkeyForDisplay(curveProfileHotkeyInput);

      const nonEmptyValues = [manualValue, autoValue, curveValue].filter((value) => value !== '');
      const uniq = new Set(nonEmptyValues);
      if (uniq.size !== nonEmptyValues.length) {
        if (!silent) toast.error(t('controlPanel.system.hotkeys.toasts.duplicate'));
        return false;
      }

      const newCfg = types.AppConfig.createFrom({
        ...config,
        manualGearToggleHotkey: manualValue,
        autoControlToggleHotkey: autoValue,
        curveProfileToggleHotkey: curveValue,
      });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
      if (!silent) toast.success(t('controlPanel.system.hotkeys.toasts.saved'));
      return true;
    } catch (e) {
      if (!silent) toast.error(t('controlPanel.system.hotkeys.toasts.saveFailed', { error: getErrorMessage(e) }));
      return false;
    } finally {
      setLoading('hotkeys', false);
    }
  }, [autoHotkeyInput, config, curveProfileHotkeyInput, manualHotkeyInput, onConfigChange, t]);

  const handleHotkeyInputKeyDown = (target: 'manual' | 'auto' | 'curve') => (e: React.KeyboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    e.stopPropagation();

    if (e.key === 'Escape') {
      setRecordingTarget(null);
      e.currentTarget.blur();
      return;
    }

    if (e.key === 'Backspace' || e.key === 'Delete') {
      if (target === 'manual') setManualHotkeyInput('');
      else if (target === 'auto') setAutoHotkeyInput('');
      else setCurveProfileHotkeyInput('');
      return;
    }

    const shortcut = buildShortcutFromKeyboardEvent(e);
    if (!shortcut) return;

    if (target === 'manual') setManualHotkeyInput(shortcut);
    else if (target === 'auto') setAutoHotkeyInput(shortcut);
    else setCurveProfileHotkeyInput(shortcut);
  };

  const handleHotkeyInputBlur = useCallback(async () => {
    setRecordingTarget(null);
    await saveHotkeys(true);
  }, [saveHotkeys]);

  const clearHotkeyInput = useCallback(async (target: 'manual' | 'auto' | 'curve') => {
    if (target === 'manual') setManualHotkeyInput('');
    else if (target === 'auto') setAutoHotkeyInput('');
    else setCurveProfileHotkeyInput('');
    await saveHotkeys(true);
  }, [saveHotkeys]);

  return (
    <>
      <div className="space-y-4">
        <Section title={t('controlPanel.overview.title')} icon={Settings}>
          <div className="grid grid-cols-1 gap-4 p-5 md:grid-cols-3">
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">{t('controlPanel.overview.currentTemperature')}</div>
              <div className={clsx(
                'mt-1 text-2xl font-semibold tabular-nums',
                (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' : (temperature?.maxTemp ?? 0) > 70 ? 'text-amber-500' : 'text-primary'
              )}>
                {temperature?.maxTemp ?? '--'}°C
              </div>
              <div className="mt-1 text-xs text-muted-foreground">{t('controlPanel.overview.cpuGpuTemperature', { cpu: temperature?.cpuTemp ?? '--', gpu: temperature?.gpuTemp ?? '--' })}</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">{t('controlPanel.overview.currentRpm')}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{fanData?.currentRpm ?? '--'} RPM</div>
              <div className="mt-1 text-xs text-muted-foreground">{translateWorkMode(fanData?.workMode, t)}</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">{isBs1 ? t('controlPanel.overview.currentGear') : t('controlPanel.overview.targetRpm')}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{isBs1 ? (getManualGearLabel(fanData?.setGear) || '--') : `${fanData?.targetRpm ?? '--'} RPM`}</div>
              <div className="mt-1 text-xs text-muted-foreground">{isBs1 ? translateWorkMode(fanData?.workMode, t) : t('controlPanel.overview.gearValue', { gear: getManualGearLabel(fanData?.setGear) || '--' })}</div>
            </div>
          </div>
        </Section>

        {/* ═══════════ 1. 灯光效果 ═══════════ */}
        {!isBs1 && (
        <Section title={t('controlPanel.light.sectionTitle')} icon={Sparkles}>
          <div className="space-y-4 p-5">
            <div className="grid grid-cols-2 gap-3">
              <Select
                value={lightStripConfig.mode}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, mode: v as string }))}
                options={lightModeOptions}
                size="sm"
                label={t('controlPanel.light.effectMode')}
              />
              <Select
                value={lightStripConfig.speed}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, speed: v as string }))}
                options={lightSpeedOptions}
                size="sm"
                label={t('controlPanel.light.animationSpeed')}
                disabled={['off', 'smart_temp', 'static_single', 'static_multi'].includes(lightStripConfig.mode)}
              />
            </div>

            <Slider
              min={0} max={100} step={1}
              value={lightStripConfig.brightness}
              onChange={(v) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, brightness: v }))}
              label={t('controlPanel.light.brightness')}
              valueFormatter={(v) => `${v}%`}
              disabled={lightStripConfig.mode === 'off' || lightStripConfig.mode === 'smart_temp'}
            />

            {lightStripConfig.mode === 'smart_temp' && (
              <div className="rounded-lg border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
                {t('controlPanel.light.smartTempWarning')}
              </div>
            )}

            <AnimatePresence>
              {requiredColorCount > 0 && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="space-y-3 overflow-hidden"
                >
                  <div className="flex flex-wrap gap-2">
                    {lightColorPresets.map((preset) => (
                      <button
                        key={preset.name}
                        type="button"
                        onClick={() => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, colors: preset.colors }))}
                        className="cursor-pointer rounded-lg border border-border px-3 py-1.5 text-xs text-foreground transition-colors hover:bg-muted"
                      >
                        {preset.name}
                      </button>
                    ))}
                  </div>

                  <div className={clsx('grid gap-3', requiredColorCount === 1 ? 'grid-cols-1' : 'grid-cols-3')}>
                    {Array.from({ length: requiredColorCount }).map((_, i) => (
                      <div key={i}>
                        <label className="mb-1 block text-xs text-muted-foreground">{t('controlPanel.light.colorLabel', { index: i + 1 })}</label>
                        <input
                          type="color"
                          value={rgbToHex((lightStripConfig.colors || [])[i] || types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }))}
                          onChange={(e) => handleLightColorChange(i, e.target.value)}
                          className="h-9 w-full cursor-pointer rounded-lg border border-border bg-card"
                        />
                      </div>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>

            <div className="flex items-center justify-between pt-1">
              <span className="text-xs text-muted-foreground">
                {isConnected ? t('controlPanel.light.applyHintConnected') : t('controlPanel.light.applyHintDisconnected')}
              </span>
              <Button variant="primary" size="sm" onClick={handleApplyLightStrip} loading={loadingStates.lightStrip}>
                {t('common.actions.apply')}
              </Button>
            </div>
          </div>
        </Section>
        )}

        {/* ═══════════ 2. 风扇控制 ═══════════ */}
        <Section title={t('controlPanel.fan.sectionTitle')} icon={Settings}>
          {/* Auto control */}
          <SettingRow
            icon={config.autoControl ? <Play className="h-4 w-4 text-emerald-500" /> : <Pause className="h-4 w-4" />}
            title={t('controlPanel.fan.autoControlTitle')}
            description={t('controlPanel.fan.autoControlDescription')}
            disabled={(config as any).customSpeedEnabled}
          >
            <ToggleSwitch
              enabled={config.autoControl}
              onChange={handleAutoControlChange}
              disabled={(config as any).customSpeedEnabled}
              loading={loadingStates.autoControl}
              size="sm"
              color="green"
            />
          </SettingRow>

          {/* Sample count (conditional) */}
          <AnimatePresence>
            {config.autoControl && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="overflow-hidden"
              >
                <SettingRow
                  icon={<BarChart3 className="h-4 w-4" />}
                  title={t('controlPanel.fan.sampleSmoothingTitle')}
                  description={t('controlPanel.fan.sampleSmoothingDescription')}
                >
                  <div className="w-32">
                    <Select
                      value={(config as any).tempSampleCount || 1}
                      onChange={(v: string | number) => handleSampleCountChange(v as number)}
                      options={sampleCountOptions}
                      size="sm"
                    />
                  </div>
                </SettingRow>
              </motion.div>
            )}
          </AnimatePresence>

          <div className="px-5 py-4">
            <div className="rounded-2xl border border-border/70 bg-muted/25 p-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div className="flex min-w-0 items-center gap-3">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                    <BarChart3 className="h-4 w-4" />
                  </div>
                  <div className="min-w-0">
                    <div className="text-base font-medium text-foreground">{t('controlPanel.fan.temperatureBaselineTitle')}</div>
                    <div className="text-sm text-muted-foreground">{t('controlPanel.fan.temperatureBaselineDescription')}</div>
                  </div>
                </div>
                <div className="w-full md:w-40">
                  <Select
                    value={currentTempSource}
                    onChange={(value: string | number) => handleTempSourceChange(String(value))}
                    options={tempSourceOptions}
                    size="sm"
                    className="w-full min-w-0"
                  />
                </div>
              </div>

              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Cpu className="h-4 w-4 text-primary" />
                    <span>{t('controlPanel.fan.cpuBaseline')}</span>
                  </div>
                  <div className="mt-3 space-y-3">
                    <SelectionField
                      label={t('controlPanel.fan.processorDevice')}
                      hint={temperature?.cpuModel?.trim() ? t('controlPanel.fan.processorDeviceHintDetected') : t('controlPanel.fan.processorDeviceHintWaiting')}
                    >
                      <div className="flex h-10 items-center rounded-lg border border-border/70 bg-background px-3 text-sm text-foreground">
                        <span className="truncate">{temperature?.cpuModel?.trim() || t('controlPanel.fan.waitingRecognition')}</span>
                      </div>
                    </SelectionField>

                    <SelectionField
                      label={t('controlPanel.fan.temperatureSensorMulti')}
                      hint={t('controlPanel.fan.temperatureSensorMultiHint')}
                    >
                      <MultiSelect
                        values={selectedCpuSensors}
                        onChange={(values) => void handleCpuSensorsChange(values)}
                        options={cpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` }))}
                        autoOptionLabel={t('controlPanel.options.sensor.auto')}
                        emptyLabel={cpuSensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto')}
                        disabled={!cpuSensors.length || loadingStates.cpuSensors}
                        size="sm"
                        className="w-full min-w-0"
                      />
                    </SelectionField>
                  </div>
                  <div className="mt-2 text-xs text-muted-foreground">
                    {temperature?.cpuTemp && temperature.cpuTemp > 0 ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.cpuTemp }) : t('controlPanel.fan.noCpuTemperatureData')}
                  </div>
                </div>

                <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Gpu className="h-4 w-4 text-primary" />
                    <span>{t('controlPanel.fan.gpuBaseline')}</span>
                  </div>
                  <div className="mt-3 space-y-3">
                    <SelectionField
                      label={t('controlPanel.fan.gpuDevice')}
                      hint={selectedGpuDevice === 'auto'
                        ? (temperature?.gpuModel?.trim() ? t('controlPanel.fan.gpuDeviceHintDetected', { model: temperature.gpuModel }) : t('controlPanel.fan.gpuDeviceHintAuto'))
                        : t('controlPanel.fan.gpuDeviceHintLocked')}
                    >
                      <Select
                        value={selectedGpuDevice}
                        onChange={(value: string | number) => handleGpuDeviceChange(String(value))}
                        options={gpuDeviceOptions}
                        size="sm"
                        className="w-full min-w-0"
                        disabled={gpuDevices.length === 0}
                      />
                    </SelectionField>

                    <SelectionField label={t('controlPanel.fan.temperatureSensor')}>
                      <Select
                        value={selectedGpuSensor}
                        onChange={(value: string | number) => handleTempSensorChange('gpu', String(value))}
                        options={gpuSensorOptions}
                        size="sm"
                        className="w-full min-w-0"
                        disabled={!effectiveGpuSensors.length}
                      />
                    </SelectionField>
                  </div>
                  <div className="mt-2 text-xs text-muted-foreground">
                    {temperature?.gpuTemp && temperature.gpuTemp > 0 ? t('controlPanel.fan.currentBaselineTemperature', { temperature: temperature.gpuTemp }) : t('controlPanel.fan.noGpuTemperatureData')}
                  </div>
                </div>
              </div>

              <div className="mt-3 text-xs text-muted-foreground">
                {temperature?.controlTemp && temperature.controlTemp > 0
                  ? t('controlPanel.fan.currentControlSource', { source: translateControlSource(temperature.controlSource, t), temperature: temperature.controlTemp })
                  : t('controlPanel.fan.noControlTemperature')}
              </div>
            </div>
          </div>

          <SettingRow
            icon={<TriangleAlert className={clsx('h-4 w-4', (config.smartControl as any)?.filterTransientSpike !== false ? 'text-blue-500' : 'text-muted-foreground')} />}
            title={t('controlPanel.fan.transientSpikeFilterTitle')}
            description={t('controlPanel.fan.transientSpikeFilterDescription')}
          >
            <ToggleSwitch
              enabled={(config.smartControl as any)?.filterTransientSpike !== false}
              onChange={handleTransientSpikeFilterChange}
              loading={loadingStates.transientSpikeFilter}
              size="sm"
              color="blue"
              srLabel={t('controlPanel.fan.transientSpikeFilterAria')}
            />
          </SettingRow>

          <SettingRow
            icon={<Sparkles className={clsx('h-4 w-4', (config.smartControl as any)?.learning ? 'text-amber-500' : 'text-muted-foreground')} />}
            title={t('controlPanel.fan.learningTitle')}
            description={t('controlPanel.fan.learningDescription')}
          >
            <ToggleSwitch
              enabled={!!(config.smartControl as any)?.learning}
              onChange={handleLearningToggle}
              loading={loadingStates.learning}
              size="sm"
              color="purple"
              srLabel={t('controlPanel.fan.learningAria')}
            />
          </SettingRow>

          <SettingRow
            icon={<BarChart3 className="h-4 w-4" />}
            title={t('controlPanel.fan.temperatureHistoryTitle')}
            description={t('controlPanel.fan.temperatureHistoryDescription')}
          >
            <ToggleSwitch
              enabled={temperatureHistoryEnabled}
              onChange={handleTemperatureHistoryChange}
              loading={loadingStates.temperatureHistory}
              size="sm"
              color="blue"
              srLabel={t('controlPanel.fan.temperatureHistoryAria')}
            />
          </SettingRow>

          <SettingRow
            icon={<Spline className="h-4 w-4" />}
            title={t('controlPanel.fan.curveProfileTitle')}
            description={t('controlPanel.fan.curveProfileDescription')}
          >
            <FanCurveProfileSelect
              profiles={curveProfiles}
              activeProfileId={activeCurveProfileId}
              onChange={handleCurveProfileChange}
              loading={curveProfileLoading}
            />
          </SettingRow>

          {/* Custom speed */}
          <div className="px-5 py-4">
            <div className="flex items-center justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <div className={clsx(
                  'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors',
                  (config as any).customSpeedEnabled ? 'bg-amber-500/15 text-amber-600' : 'bg-muted text-muted-foreground',
                )}>
                  <Flame className="h-4 w-4" />
                </div>
                <div>
                  <div className="text-base font-medium text-foreground">{t('controlPanel.fan.customSpeedTitle')}</div>
                  <div className="text-sm text-muted-foreground">{t('controlPanel.fan.customSpeedDescription')}</div>
                </div>
              </div>
              <ToggleSwitch
                enabled={(config as any).customSpeedEnabled || false}
                onChange={handleCustomSpeedToggle}
                disabled={!isConnected}
                loading={loadingStates.customSpeed}
                size="sm"
                color="orange"
              />
            </div>

            <AnimatePresence>
              {(config as any).customSpeedEnabled && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mt-3 flex items-center gap-3 rounded-xl border border-amber-300/40 bg-amber-50/50 p-3.5 dark:bg-amber-900/10">
                    <input
                      type="number"
                      value={customSpeedInput}
                      onChange={(e) => setCustomSpeedInput(Number(e.target.value))}
                      className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-amber-500/50 focus:border-transparent"
                      min={1000} max={4000} step={50}
                    />
                    <Button variant="primary" size="sm" onClick={() => handleCustomSpeedApply(true, customSpeedInput)} className="bg-amber-600 hover:bg-amber-700 text-white">
                      {t('common.actions.apply')}
                    </Button>
                  </div>
                  <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-300">
                    {t('controlPanel.fan.customSpeedWarning')}
                  </p>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </Section>

        {/* ═══════════ 3. 设备设置 ═══════════ */}
        <Section title={t('controlPanel.device.sectionTitle')} icon={Zap}>
          {!isBs1 && (
          <SettingRow
            icon={<Lightbulb className={clsx('h-4 w-4', config.gearLight ? 'text-yellow-500' : '')} />}
            title={t('controlPanel.device.gearLightTitle')}
            description={t('controlPanel.device.gearLightDescription')}
            disabled={!isConnected}
          >
            <ToggleSwitch
              enabled={config.gearLight}
              onChange={handleGearLightChange}
              disabled={!isConnected}
              loading={loadingStates.gearLight}
              size="sm"
            />
          </SettingRow>
          )}

          <SettingRow
            icon={<Power className={clsx('h-4 w-4', config.powerOnStart ? 'text-primary' : '')} />}
            title={t('controlPanel.device.powerOnStartTitle')}
            description={t('controlPanel.device.powerOnStartDescription')}
            disabled={!isConnected}
          >
            <ToggleSwitch
              enabled={config.powerOnStart}
              onChange={handlePowerOnStartChange}
              disabled={!isConnected}
              loading={loadingStates.powerOnStart}
              size="sm"
            />
          </SettingRow>

          {!isBs1 && (
          <SettingRow
            icon={<Zap className="h-4 w-4" />}
            title={t('controlPanel.device.smartStartStopTitle')}
            description={t('controlPanel.device.smartStartStopDescription')}
            disabled={!isConnected}
          >
            <div className="w-40">
              <Select
                value={config.smartStartStop || 'off'}
                onChange={(v: string | number) => handleSmartStartStopChange(v as string)}
                options={smartStartStopOptions}
                disabled={!isConnected}
                size="sm"
              />
            </div>
          </SettingRow>
          )}
        </Section>

        {legionFnQSupported && <Section title={t('controlPanel.legionFnQ.sectionTitle')} icon={Zap}>
          <SettingRow
            icon={<Zap className={clsx('h-4 w-4', legionFnQConfig.enabled ? 'text-primary' : '')} />}
            title={t('controlPanel.legionFnQ.enableTitle')}
            description={t('controlPanel.legionFnQ.enableDescription')}
          >
            <ToggleSwitch
              enabled={legionFnQConfig.enabled}
              onChange={(enabled) => updateLegionFnQConfig({ enabled })}
              loading={loadingStates.legionFnQ}
              size="sm"
            />
          </SettingRow>

          <SettingRow
            icon={<Flame className={clsx('h-4 w-4', legionFnQConfig.takeOverFan ? 'text-orange-500' : '')} />}
            title={t('controlPanel.legionFnQ.takeOverTitle')}
            description={t('controlPanel.legionFnQ.takeOverDescription')}
            disabled={!legionFnQConfig.enabled}
          >
            <ToggleSwitch
              enabled={legionFnQConfig.takeOverFan}
              onChange={(takeOverFan) => updateLegionFnQConfig({ takeOverFan })}
              disabled={!legionFnQConfig.enabled}
              loading={loadingStates.legionFnQ}
              size="sm"
              color="orange"
            />
          </SettingRow>

          <div className={clsx('px-5 py-4', (!legionFnQConfig.enabled || !legionFnQConfig.takeOverFan) && 'opacity-60')}>
            <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <div className="text-sm font-medium text-foreground">{t('controlPanel.legionFnQ.mappingTitle')}</div>
                <div className="mt-1 text-xs text-muted-foreground">{t('controlPanel.legionFnQ.mappingDescription')}</div>
              </div>
            </div>
            <div className="mb-1 hidden grid-cols-[minmax(96px,1fr)_120px_96px] gap-3 px-3 text-xs text-muted-foreground sm:grid">
              <div>{t('controlPanel.legionFnQ.headers.mode')}</div>
              <div>{t('controlPanel.legionFnQ.headers.gear')}</div>
              <div>{t('controlPanel.legionFnQ.headers.level')}</div>
            </div>
            <div className="space-y-2">
              {legionPowerModes.map((mode) => {
                const target = legionFnQConfig.modeMapping[mode.value] || (getDefaultLegionFnQConfig().modeMapping as any)[mode.value];
                return (
                  <div key={mode.value} className="grid grid-cols-1 items-center gap-3 rounded-xl border border-border/70 bg-background/45 px-3 py-2.5 sm:grid-cols-[minmax(96px,1fr)_120px_96px]">
                    <div className="text-sm font-medium text-foreground">{mode.label}</div>
                    <div className="space-y-1">
                      <div className="text-xs text-muted-foreground sm:hidden">{t('controlPanel.legionFnQ.headers.gear')}</div>
                      <Select
                        value={target.gear}
                        onChange={(value) => handleLegionFnQMappingChange(mode.value, { gear: String(value) })}
                        options={fanGearOptions}
                        disabled={!legionFnQConfig.enabled || !legionFnQConfig.takeOverFan || loadingStates.legionFnQ}
                        size="sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <div className="text-xs text-muted-foreground sm:hidden">{t('controlPanel.legionFnQ.headers.level')}</div>
                      <Select
                        value={target.level}
                        onChange={(value) => handleLegionFnQMappingChange(mode.value, { level: String(value) })}
                        options={fanLevelOptions}
                        disabled={!legionFnQConfig.enabled || !legionFnQConfig.takeOverFan || loadingStates.legionFnQ}
                        size="sm"
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </Section>}

        {/* ═══════════ 4. 系统设置 ═══════════ */}
        <Section title={t('controlPanel.system.sectionTitle')} icon={Monitor}>
          <SettingRow
            icon={<Monitor className="h-4 w-4" />}
            title={t('controlPanel.system.themeTitle')}
            description={t('controlPanel.system.themeDescription')}
          >
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleOpenThemesFolder}
                title={t('controlPanel.system.themeOpenFolder')}
              >
                {t('controlPanel.system.themeOpenFolder')}
              </Button>
              <div className="w-36">
                <Select
                  value={((config as any).themeMode || 'system') as string}
                  onChange={(v: string | number) => handleThemeModeChange(String(v))}
                  options={themeModeOptions}
                  size="sm"
                />
              </div>
            </div>
          </SettingRow>

          <SettingRow
            icon={<Languages className="h-4 w-4" />}
            title={t('controlPanel.system.languageTitle')}
            description={t('controlPanel.system.languageDescription')}
          >
            <div className="w-36">
              <Select
                value={locale}
                onChange={(value: string | number) => setLocale(String(value) as AppLocale)}
                options={languageOptions}
                size="sm"
              />
            </div>
          </SettingRow>

          <div className="px-5 py-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <div className="text-base font-medium text-foreground">{t('controlPanel.system.hotkeys.title')}</div>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  {t('controlPanel.system.hotkeys.description')}
                </p>
              </div>
            </div>

            <div className="rounded-xl border border-border/70 bg-background/45 px-4 py-2">
              <HotkeyField
                title={t('controlPanel.system.hotkeys.manual.title')}
                description={t('controlPanel.system.hotkeys.manual.description')}
                value={manualHotkeyInput}
                placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
                clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
                recording={recordingTarget === 'manual'}
                onFocus={() => setRecordingTarget('manual')}
                onBlur={handleHotkeyInputBlur}
                onKeyDown={handleHotkeyInputKeyDown('manual')}
                onClear={() => clearHotkeyInput('manual')}
              />

              <div className="border-t border-border/60" />

              <HotkeyField
                title={t('controlPanel.system.hotkeys.auto.title')}
                description={t('controlPanel.system.hotkeys.auto.description')}
                value={autoHotkeyInput}
                placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
                clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
                recording={recordingTarget === 'auto'}
                onFocus={() => setRecordingTarget('auto')}
                onBlur={handleHotkeyInputBlur}
                onKeyDown={handleHotkeyInputKeyDown('auto')}
                onClear={() => clearHotkeyInput('auto')}
              />

              <div className="border-t border-border/60" />

              <HotkeyField
                title={t('controlPanel.system.hotkeys.curve.title')}
                description={t('controlPanel.system.hotkeys.curve.description')}
                value={curveProfileHotkeyInput}
                placeholder={t('controlPanel.system.hotkeys.emptyPlaceholder')}
                clearAriaLabel={t('controlPanel.system.hotkeys.clearAria')}
                recording={recordingTarget === 'curve'}
                onFocus={() => setRecordingTarget('curve')}
                onBlur={handleHotkeyInputBlur}
                onKeyDown={handleHotkeyInputKeyDown('curve')}
                onClear={() => clearHotkeyInput('curve')}
              />
            </div>
          </div>

          <SettingRow
            icon={<Monitor className={clsx('h-4 w-4', config.windowsAutoStart ? 'text-emerald-500' : '')} />}
            title={t('controlPanel.system.autoStartTitle')}
            description={t('controlPanel.system.autoStartDescription')}
            tip={t('controlPanel.system.autoStartTip')}
          >
            <ToggleSwitch
              enabled={config.windowsAutoStart}
              onChange={handleWindowsAutoStartChange}
              loading={loadingStates.windowsAutoStart}
              size="sm"
              color="green"
            />
          </SettingRow>

          <SettingRow
            icon={<Clock3 className={clsx('h-4 w-4', (config as any).ignoreDeviceOnReconnect ? 'text-emerald-500' : '')} />}
            title={t('controlPanel.system.reconnectTitle')}
            description={t('controlPanel.system.reconnectDescription')}
            tip={t('controlPanel.system.reconnectTip')}
          >
            <ToggleSwitch
              enabled={(config as any).ignoreDeviceOnReconnect ?? true}
              onChange={handleIgnoreDeviceOnReconnectChange}
              size="sm"
              color="green"
            />
          </SettingRow>

          <SettingRow
            icon={<Sparkles className={clsx('h-4 w-4', ((config as any).windowBlur || 'auto') !== 'off' ? 'text-primary' : '')} />}
            title={t('controlPanel.system.blurTitle')}
            description={t('controlPanel.system.blurDescription')}
          >
            <div className="w-36">
              <Select
                value={((config as any).windowBlur === 'on' ? 'mica' : ((config as any).windowBlur || 'auto')) as string}
                onChange={(v: string | number) => handleWindowBlurChange(String(v))}
                options={windowBlurOptions}
                size="sm"
              />
            </div>
          </SettingRow>

          <SettingRow
            icon={<Power className={clsx('h-4 w-4', ((config as any).suspendFanOff ?? false) ? 'text-emerald-500' : '')} />}
            title={t('controlPanel.system.suspendFanOffTitle')}
            description={t('controlPanel.system.suspendFanOffDescription')}
          >
            <ToggleSwitch
              enabled={(config as any).suspendFanOff ?? false}
              onChange={handleSuspendFanOffChange}
              loading={loadingStates.suspendFanOff}
              size="sm"
              color="green"
            />
          </SettingRow>
        </Section>

        {/* ═══════════ Offline tip ═══════════ */}
        {!isConnected && (
          <div className="flex items-center gap-2 rounded-2xl border border-border/70 bg-muted/30 px-5 py-4 text-sm text-muted-foreground">
            <TriangleAlert className="h-4 w-4 shrink-0" />
            {t('controlPanel.offline.message')}
          </div>
        )}

        {/* ═══════════ 5. 调试面板 ═══════════ */}
        <Collapsible open={debugPanelOpen} onOpenChange={setDebugPanelOpen}>
          <div className="rounded-2xl border border-border bg-card overflow-hidden">
            <CollapsibleTrigger asChild>
              <button type="button" className="flex w-full cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-muted/40">
                <div className="flex items-center gap-2">
                  <Bug className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-semibold text-foreground">{t('controlPanel.debug.panelTitle')}</span>
                </div>
                <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', debugPanelOpen && 'rotate-180')} />
              </button>
            </CollapsibleTrigger>

            <CollapsibleContent>
              <div className="space-y-3 border-t border-border/60 p-4">
                <div className="flex items-center justify-between rounded-xl border border-border/70 bg-muted/30 px-4 py-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <Bug className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-foreground">{t('controlPanel.debug.modeTitle')}</div>
                      <div className="text-xs text-muted-foreground">{t('controlPanel.debug.modeDescription')}</div>
                    </div>
                  </div>
                  <ToggleSwitch enabled={config.debugMode} onChange={toggleDebugMode} size="sm" color="purple" />
                </div>

                <Button variant="secondary" size="sm" onClick={fetchDebugInfo} loading={debugInfoLoading} className="w-full">
                  {t('controlPanel.debug.refresh')}
                </Button>

                <div className="rounded-xl border border-border/70 bg-background px-3 py-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-foreground">{t('controlPanel.debug.pawnTitle')}</div>
                      <div className="text-[11px] leading-relaxed text-muted-foreground">{t('controlPanel.debug.pawnDescription')}</div>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleReinstallPawnIO}
                      loading={loadingStates.pawnIOReinstall}
                      icon={<RotateCw className="h-3.5 w-3.5" />}
                    >
                      {t('controlPanel.debug.reinstall')}
                    </Button>
                  </div>
                </div>

                {debugInfo && (
                  <div className="min-h-56 max-h-[min(55vh,30rem)] w-full cursor-text overflow-auto rounded-xl border border-border bg-background overscroll-contain select-text">
                    <pre className="min-w-max whitespace-pre p-3 font-mono text-xs leading-5 text-foreground/90">{JSON.stringify(debugInfo, null, 2)}</pre>
                  </div>
                )}

                <div className="rounded-xl border border-border/70 bg-background px-3 py-3">
                  <div className="flex gap-2">
                    <input
                      value={debugCommandInput}
                      onChange={(event) => setDebugCommandInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') void sendDeviceDebugCommand();
                      }}
                      placeholder="27 或 5A A5 27 02 29"
                      className={clsx(
                        'h-9 min-w-0 flex-1 rounded-md border bg-background px-3 font-mono text-xs outline-none ring-offset-background transition-colors focus-visible:ring-2',
                        isDangerousDebugCommand
                          ? 'border-red-500 text-red-600 focus-visible:ring-red-500 dark:text-red-400'
                          : 'border-input focus-visible:ring-ring',
                      )}
                    />
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => sendDeviceDebugCommand()}
                      loading={debugCommandLoading}
                      disabled={!isConnected || !config.debugMode}
                      icon={<Play className="h-3.5 w-3.5" />}
                    >
                      发送
                    </Button>
                  </div>
                  <div className="mt-2 flex items-start gap-1.5 text-[11px] leading-relaxed text-red-600 dark:text-red-400">
                    <TriangleAlert className="mt-px h-3.5 w-3.5 shrink-0" />
                    {isDangerousDebugCommand ? (
                      <span className="font-semibold">
                        高危命令 0x{debugCommandByte?.toString(16).toUpperCase().padStart(2, '0')}：直接操作固件底层/调试寄存器，误用可能导致设备异常甚至变砖，请确认后再发送。
                      </span>
                    ) : (
                      <span>原始命令会直接下发到设备固件，错误命令可能导致设备异常，请谨慎操作。</span>
                    )}
                  </div>
                  {debugCommandResult && (
                    <div className="mt-3 max-h-48 cursor-text overflow-auto rounded-md bg-muted/45 p-2 font-mono text-[11px] leading-5 select-text">
                      <div>TX {debugCommandResult.rawHex}</div>
                      {(debugCommandResult.frames || []).map((frame) => (
                        <div key={frame.id} className={frame.direction === 'rx' ? 'text-emerald-600 dark:text-emerald-400' : 'text-sky-600 dark:text-sky-400'}>
                          <div>{frame.direction.toUpperCase()} {frame.command || '--'} {frame.frameHex || frame.rawHex} {frame.checksumOk ? 'OK' : 'BAD'}</div>
                          {frame.decoded && <div className="pl-4 text-foreground/80">{renderDebugFrameSummary(frame)}</div>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </CollapsibleContent>
          </div>
        </Collapsible>
      </div>

      {/* ═══════════ Custom speed warning dialog ═══════════ */}
      <AnimatePresence>
        {showCustomSpeedWarning && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="w-full max-w-sm rounded-2xl border border-border bg-card p-6 shadow-xl"
            >
              <div className="mb-4 flex justify-center">
                <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                  <TriangleAlert className="h-8 w-8 text-amber-600" />
                </div>
              </div>

              <h3 className="mb-3 text-center text-lg font-bold text-foreground">{t('controlPanel.customSpeedDialog.title')}</h3>

              <div className="mb-4 rounded-xl border border-amber-300/40 bg-amber-500/10 p-3 text-sm">
                <p className="mb-2 font-medium text-foreground">{t('controlPanel.customSpeedDialog.enabledTitle')}</p>
                <ul className="space-y-1 text-xs text-muted-foreground">
                  <li>{t('controlPanel.customSpeedDialog.bullets.disableAutoControl')}</li>
                  <li>{t('controlPanel.customSpeedDialog.bullets.fixedSpeed')}</li>
                  <li>{t('controlPanel.customSpeedDialog.bullets.insufficientCooling')}</li>
                </ul>
              </div>

              <div className="mb-5 rounded-xl bg-muted/60 p-3 text-center">
                <span className="text-xs text-muted-foreground">{t('controlPanel.customSpeedDialog.speedLabel')}</span>
                <div className="text-xl font-bold text-amber-600">{customSpeedInput} RPM</div>
              </div>

              <div className="flex gap-3">
                <Button variant="secondary" onClick={() => setShowCustomSpeedWarning(false)} className="flex-1">
                  {t('common.actions.cancel')}
                </Button>
                <Button
                  variant="primary"
                  onClick={() => { setShowCustomSpeedWarning(false); handleCustomSpeedApply(true, customSpeedInput); }}
                  className="flex-1 bg-amber-600 text-white hover:bg-amber-700"
                  icon={<CheckCircle2 className="h-4 w-4" />}
                >
                  {t('controlPanel.customSpeedDialog.confirm')}
                </Button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}

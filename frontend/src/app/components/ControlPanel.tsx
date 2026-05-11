'use client';

import React, { useState, useCallback, useEffect, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Play,
  Pause,
  Settings,
  Lightbulb,
  Power,
  Zap,
  Monitor,
  Cpu,
  Gpu,
  Bug,
  Eye,
  EyeOff,
  TriangleAlert,
  CheckCircle2,
  ChevronDown,
  Flame,
  Clock3,
  BarChart3,
  Spline,
  Sparkles,
  Rocket,
  X,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { toast } from 'sonner';
import { DebugInfo } from '../types/app';
import FanCurveProfileSelect from './FanCurveProfileSelect';
import { ToggleSwitch, Button, Select, ScrollArea, Slider } from './ui/index';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import clsx from 'clsx';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  deviceModel: string | null;
}

type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };

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

const LEGION_POWER_MODES = [
  { value: 'Quiet', label: '安静模式' },
  { value: 'Balance', label: '均衡模式' },
  { value: 'Performance', label: '野兽模式' },
  { value: 'Extreme', label: '超能模式' },
  { value: 'GodMode', label: '自定义模式' },
];

const FAN_GEAR_OPTIONS = [
  { value: '静音', label: '静音' },
  { value: '标准', label: '标准' },
  { value: '强劲', label: '强劲' },
  { value: '超频', label: '超频' },
];

const FAN_LEVEL_OPTIONS = [
  { value: '低', label: '低' },
  { value: '中', label: '中' },
  { value: '高', label: '高' },
];

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
              aria-label="清除快捷键"
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

export default function ControlPanel({ config, onConfigChange, isConnected, fanData, temperature, deviceModel }: ControlPanelProps) {
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null);
  const [debugInfoLoading, setDebugInfoLoading] = useState(false);
  const [debugPanelOpen, setDebugPanelOpen] = useState(false);
  const [showCustomSpeedWarning, setShowCustomSpeedWarning] = useState(false);
  const [customSpeedInput, setCustomSpeedInput] = useState<number>((config as any).customSpeedRPM || 2000);
  const [appVersion, setAppVersion] = useState('');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState('');
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
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

  const activeCurveProfileId = ((config as any).activeFanCurveProfileId || '') as string;
  const isBs1 = deviceModel === 'BS1';
  const currentTempSource = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
  const cpuSensors = useMemo(() => (Array.isArray(temperature?.cpuSensors) ? temperature.cpuSensors : []), [temperature?.cpuSensors]);
  const gpuSensors = useMemo(() => (Array.isArray(temperature?.gpuSensors) ? temperature.gpuSensors : []), [temperature?.gpuSensors]);
  const gpuDevices = useMemo(() => (Array.isArray((temperature as any)?.gpuDevices) ? (temperature as any).gpuDevices as types.TemperatureGPUDevice[] : []), [temperature]);
  const selectedGpuDevice = useMemo(() => {
    const configured = (((config as any).gpuDevice as string) || 'auto');
    return configured === 'auto' || gpuDevices.some((device) => device.key === configured) ? configured : 'auto';
  }, [config, gpuDevices]);
  const activeGpuDeviceKey = useMemo(() => {
    if (selectedGpuDevice !== 'auto') {
      return selectedGpuDevice;
    }
    const detected = ((temperature as any)?.selectedGpuDevice as string) || 'auto';
    return gpuDevices.some((device) => device.key === detected) ? detected : 'auto';
  }, [selectedGpuDevice, temperature, gpuDevices]);
  const activeGpuDevice = useMemo(() => {
    return gpuDevices.find((device) => device.key === activeGpuDeviceKey) || null;
  }, [activeGpuDeviceKey, gpuDevices]);
  const effectiveGpuSensors = useMemo(() => {
    if (activeGpuDevice && Array.isArray(activeGpuDevice.sensors) && activeGpuDevice.sensors.length > 0) {
      return activeGpuDevice.sensors;
    }
    return gpuSensors;
  }, [activeGpuDevice, gpuSensors]);
  const selectedCpuSensor = useMemo(() => {
    const configured = (((config as any).cpuSensor as string) || 'auto');
    return cpuSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, cpuSensors]);
  const selectedGpuSensor = useMemo(() => {
    const configured = (((config as any).gpuSensor as string) || 'auto');
    return effectiveGpuSensors.some((sensor) => sensor.key === configured) ? configured : 'auto';
  }, [config, effectiveGpuSensors]);
  const legionFnQConfig = useMemo(() => normalizeLegionFnQConfig((config as any).legionFnQ), [config]);

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

  const handleOpenUrl = useCallback((url: string) => {
    try { BrowserOpenURL(url); } catch { /* noop */ }
  }, []);

  const isLatestVersion = useCallback((currentVersion: string, latestVersion: string) => {
    const parse = (v: string) => (v.match(/\d+/g) || []).map((n) => Number(n));
    const current = parse(currentVersion);
    const latest = parse(latestVersion);
    const length = Math.max(current.length, latest.length);

    for (let i = 0; i < length; i += 1) {
      const currentPart = current[i] ?? 0;
      const latestPart = latest[i] ?? 0;
      if (latestPart > currentPart) return false;
      if (latestPart < currentPart) return true;
    }

    return true;
  }, []);

  const checkLatestRelease = useCallback(async () => {
    setReleaseLoading(true);
    setReleaseError('');
    try {
      const response = await fetch('https://api.github.com/repos/TIANLI0/BS2PRO-Controller/releases/latest', {
        headers: { Accept: 'application/vnd.github+json' },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = await response.json();
      setLatestReleaseTag(data?.tag_name || '');
      setLatestReleaseUrl(data?.html_url || 'https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
      setLatestReleaseBody(typeof data?.body === 'string' ? data.body.trim() : '');
    } catch {
      setReleaseError('检查更新失败，请稍后重试');
      setLatestReleaseTag('');
      setLatestReleaseUrl('https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
      setLatestReleaseBody('');
    } finally {
      setReleaseLoading(false);
    }
  }, []);

  const hasNewVersion = !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);

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
    } catch (e) { alert(`设置自启动失败: ${e}`); } finally { setLoading('windowsAutoStart', false); }
  }, [config, onConfigChange]);

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

  const toggleGuiMonitoring = useCallback(async () => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, guiMonitoring: !config.guiMonitoring });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const fetchDebugInfo = useCallback(async () => {
    setDebugInfoLoading(true);
    try { setDebugInfo(await apiService.getDebugInfo()); } catch { /* noop */ } finally { setDebugInfoLoading(false); }
  }, []);

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

  const updateLegionFnQConfig = useCallback(async (patch: any) => {
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
      toast.error(`保存 Fn+Q 插件设置失败: ${e}`);
    } finally {
      setLoading('legionFnQ', false);
    }
  }, [config, legionFnQConfig, onConfigChange]);

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

  const handleCurveProfileChange = useCallback(async (profileId: string) => {
    if (!profileId || profileId === activeCurveProfileId) return;
    try {
      setCurveProfileLoading(true);
      await apiService.setActiveFanCurveProfile(profileId);
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
      await loadCurveProfiles();
      toast.success('温控曲线已切换');
    } catch (e) {
      toast.error(`切换曲线失败: ${e}`);
    } finally {
      setCurveProfileLoading(false);
    }
  }, [activeCurveProfileId, loadCurveProfiles, onConfigChange]);

  useEffect(() => { const i = setInterval(() => { apiService.updateGuiResponseTime().catch(() => {}); }, 10000); return () => clearInterval(i); }, []);
  useEffect(() => { apiService.getAppVersion().then((v) => setAppVersion(v || '')).catch(() => setAppVersion('')); }, []);
  useEffect(() => { checkLatestRelease(); }, [checkLatestRelease]);
  useEffect(() => { loadCurveProfiles(); }, [loadCurveProfiles]);
  useEffect(() => { setLightStripConfig(normalizeLightStripConfig(config)); }, [config]);
  useEffect(() => {
    setManualHotkeyInput(normalizeHotkeyForDisplay((config as any).manualGearToggleHotkey));
    setAutoHotkeyInput(normalizeHotkeyForDisplay((config as any).autoControlToggleHotkey));
    setCurveProfileHotkeyInput(normalizeHotkeyForDisplay((config as any).curveProfileToggleHotkey));
  }, [(config as any).manualGearToggleHotkey, (config as any).autoControlToggleHotkey, (config as any).curveProfileToggleHotkey]);

  /* ── Options data ── */

  const smartStartStopOptions = [
    { value: 'off', label: '关闭', description: '禁用智能启停功能' },
    { value: 'immediate', label: '即时', description: '立即响应系统负载变化' },
    { value: 'delayed', label: '延时', description: '延时响应，避免频繁启停' },
  ];

  const sampleCountOptions = [
    { value: 1, label: '1次 (即时)' },
    { value: 2, label: '2次 (2s)' },
    { value: 3, label: '3次 (3s)' },
    { value: 5, label: '5次 (5s)' },
    { value: 10, label: '10次 (10s)' },
  ];

  const tempSourceOptions = [
    { value: 'max', label: '最高温度' },
    { value: 'cpu', label: '仅 CPU' },
    { value: 'gpu', label: '仅 GPU' },
  ];

  const gpuDeviceOptions = [
    { value: 'auto', label: gpuDevices.length > 0 ? '自动选择 (优先独显)' : '自动选择' },
    ...gpuDevices.map((device) => ({
      value: device.key,
      label: `${device.vendor ? `${device.vendor.toUpperCase()} · ` : ''}${device.name}`,
    })),
  ];

  const cpuSensorOptions = [
    { value: 'auto', label: cpuSensors.length > 0 ? '自动选择 (推荐)' : '自动选择' },
    ...cpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ];

  const gpuSensorOptions = [
    { value: 'auto', label: effectiveGpuSensors.length > 0 ? '自动选择 (推荐)' : '自动选择' },
    ...effectiveGpuSensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ];

  const themeModeOptions = [
    { value: 'system', label: '跟随系统' },
    { value: 'light', label: '浅色' },
    { value: 'dark', label: '深色' },
  ];

  const lightModeOptions = [
    { value: 'off', label: '关闭灯光', description: '关闭所有RGB灯光' },
    { value: 'smart_temp', label: '智能温控', description: '根据温度自动切换灯效' },
    { value: 'static_single', label: '单色常亮', description: '固定单色显示' },
    { value: 'static_multi', label: '多色常亮', description: '三色静态分区' },
    { value: 'rotation', label: '多色旋转', description: '颜色循环旋转' },
    { value: 'flowing', label: '流光', description: '预设流光效果' },
    { value: 'breathing', label: '呼吸', description: '多色呼吸变化' },
  ];

  const lightSpeedOptions = [
    { value: 'fast', label: '快速' },
    { value: 'medium', label: '中速' },
    { value: 'slow', label: '慢速' },
  ];

  const lightColorPresets = [
    { name: '霓虹', colors: [{ r: 255, g: 0, b: 128 }, { r: 0, g: 255, b: 255 }, { r: 128, g: 0, b: 255 }] },
    { name: '森林', colors: [{ r: 86, g: 169, b: 84 }, { r: 161, g: 210, b: 106 }, { r: 44, g: 120, b: 115 }] },
    { name: '冰川', colors: [{ r: 80, g: 170, b: 255 }, { r: 116, g: 214, b: 255 }, { r: 200, g: 240, b: 255 }] },
  ];

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
    } catch (e) { alert(`设置灯带失败: ${e}`); } finally { setLoading('lightStrip', false); }
  }, [lightStripConfig, config, onConfigChange, requiredColorCount]);

  const handleThemeModeChange = useCallback(async (mode: string) => {
    const nextMode = mode === 'light' || mode === 'dark' ? mode : 'system';
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
  }, [config, onConfigChange]);

  const saveHotkeys = useCallback(async (silent = false) => {
    setLoading('hotkeys', true);
    try {
      const manualValue = normalizeHotkeyForDisplay(manualHotkeyInput);
      const autoValue = normalizeHotkeyForDisplay(autoHotkeyInput);
      const curveValue = normalizeHotkeyForDisplay(curveProfileHotkeyInput);

      const nonEmptyValues = [manualValue, autoValue, curveValue].filter((value) => value !== '');
      const uniq = new Set(nonEmptyValues);
      if (uniq.size !== nonEmptyValues.length) {
        if (!silent) toast.error('三个快捷键不能设置为同一个组合');
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
      if (!silent) toast.success('快捷键保存成功');
      return true;
    } catch (e) {
      if (!silent) toast.error(`保存快捷键失败: ${e}`);
      return false;
    } finally {
      setLoading('hotkeys', false);
    }
  }, [autoHotkeyInput, config, curveProfileHotkeyInput, manualHotkeyInput, onConfigChange]);

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
        <section className="rounded-2xl border border-border bg-card p-5 shadow-sm">
          <div className="mb-4 flex items-center gap-2">
            <Settings className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-base font-semibold text-foreground">实时概览</h3>
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">当前温度</div>
              <div className={clsx(
                'mt-1 text-2xl font-semibold tabular-nums',
                (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' : (temperature?.maxTemp ?? 0) > 70 ? 'text-amber-500' : 'text-primary'
              )}>
                {temperature?.maxTemp ?? '--'}°C
              </div>
              <div className="mt-1 text-xs text-muted-foreground">CPU {temperature?.cpuTemp ?? '--'}°C · GPU {temperature?.gpuTemp ?? '--'}°C</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">实时转速</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{fanData?.currentRpm ?? '--'} RPM</div>
              <div className="mt-1 text-xs text-muted-foreground">{fanData?.workMode ?? '--'}</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">{isBs1 ? '当前挡位' : '目标转速'}</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{isBs1 ? (fanData?.setGear ?? '--') : `${fanData?.targetRpm ?? '--'} RPM`}</div>
              <div className="mt-1 text-xs text-muted-foreground">{isBs1 ? (fanData?.workMode ?? '--') : `挡位 ${fanData?.setGear ?? '--'}`}</div>
            </div>
          </div>
        </section>

        {/* ═══════════ 1. 灯光效果 ═══════════ */}
        {!isBs1 && (
        <Section title="灯光效果" icon={Sparkles}>
          <div className="space-y-4 p-5">
            <div className="grid grid-cols-2 gap-3">
              <Select
                value={lightStripConfig.mode}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, mode: v as string }))}
                options={lightModeOptions}
                size="sm"
                label="效果模式"
              />
              <Select
                value={lightStripConfig.speed}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, speed: v as string }))}
                options={lightSpeedOptions}
                size="sm"
                label="动画速度"
                disabled={['off', 'smart_temp', 'static_single', 'static_multi'].includes(lightStripConfig.mode)}
              />
            </div>

            <Slider
              min={0} max={100} step={1}
              value={lightStripConfig.brightness}
              onChange={(v) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, brightness: v }))}
              label="亮度"
              valueFormatter={(v) => `${v}%`}
              disabled={lightStripConfig.mode === 'off' || lightStripConfig.mode === 'smart_temp'}
            />

            {lightStripConfig.mode === 'smart_temp' && (
              <div className="rounded-lg border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
                智能温控模式由设备自动控制灯效，不支持手动调节颜色与亮度。
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
                        <label className="mb-1 block text-xs text-muted-foreground">颜色 {i + 1}</label>
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
                {isConnected ? '应用后立即生效' : '下次连接时自动生效'}
              </span>
              <Button variant="primary" size="sm" onClick={handleApplyLightStrip} loading={loadingStates.lightStrip}>
                应用
              </Button>
            </div>
          </div>
        </Section>
        )}

        {/* ═══════════ 2. 风扇控制 ═══════════ */}
        <Section title="风扇控制" icon={Settings}>
          {/* Auto control */}
          <SettingRow
            icon={config.autoControl ? <Play className="h-4 w-4 text-emerald-500" /> : <Pause className="h-4 w-4" />}
            title="自动温度控制"
            description="根据温度曲线自动调节风扇转速"
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
                  title="采样时间"
                  description="降低频繁调整带来的轴噪，不知道默认即可"
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
                    <div className="text-base font-medium text-foreground">温度基准</div>
                    <div className="text-sm text-muted-foreground">选择控温器件、显卡设备以及具体传感器。多显卡笔记本可在这里直接锁定独显。</div>
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
                    <span>CPU 基准</span>
                  </div>
                  <div className="mt-3 space-y-3">
                    <SelectionField
                      label="处理器设备"
                      hint={temperature?.cpuModel?.trim() ? '当前已识别的处理器，会随 TempBridge 检测自动更新。' : '尚未识别到 CPU 型号，TempBridge 可用后会自动显示。'}
                    >
                      <div className="flex h-10 items-center rounded-lg border border-border/70 bg-background px-3 text-sm text-foreground">
                        <span className="truncate">{temperature?.cpuModel?.trim() || '等待识别...'}</span>
                      </div>
                    </SelectionField>

                    <SelectionField label="温度传感器">
                      <Select
                        value={selectedCpuSensor}
                        onChange={(value: string | number) => handleTempSensorChange('cpu', String(value))}
                        options={cpuSensorOptions}
                        size="sm"
                        className="w-full min-w-0"
                        disabled={!cpuSensors.length}
                      />
                    </SelectionField>
                  </div>
                  <div className="mt-2 text-xs text-muted-foreground">
                    {temperature?.cpuTemp && temperature.cpuTemp > 0 ? `当前基准温度 ${temperature.cpuTemp}°C` : '当前无可用 CPU 温度数据'}
                  </div>
                </div>

                <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Gpu className="h-4 w-4 text-primary" />
                    <span>GPU 基准</span>
                  </div>
                  <div className="mt-3 space-y-3">
                    <SelectionField
                      label="显卡设备"
                      hint={selectedGpuDevice === 'auto'
                        ? (temperature?.gpuModel?.trim() ? `自动模式当前命中：${temperature.gpuModel}` : '自动模式会优先尝试独显。')
                        : '已锁定为你选择的显卡设备。'}
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

                    <SelectionField label="温度传感器">
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
                    {temperature?.gpuTemp && temperature.gpuTemp > 0 ? `当前基准温度 ${temperature.gpuTemp}°C` : '当前无可用 GPU 温度数据'}
                  </div>
                </div>
              </div>

              <div className="mt-3 text-xs text-muted-foreground">
                {temperature?.controlTemp && temperature.controlTemp > 0
                  ? `当前控温正在使用 ${temperature.controlSource === 'cpu' ? 'CPU' : temperature.controlSource === 'gpu' ? 'GPU' : '最高温度'} 基准，控制温度 ${temperature.controlTemp}°C。`
                  : '当前尚未拿到有效的控温基准温度。'}
              </div>
            </div>
          </div>

          <SettingRow
            icon={<TriangleAlert className={clsx('h-4 w-4', (config.smartControl as any)?.filterTransientSpike !== false ? 'text-blue-500' : 'text-muted-foreground')} />}
            title="温度尖峰过滤"
            description="忽略单次异常跳温，避免孤立跳点干扰智能温控采样和响应。"
          >
            <ToggleSwitch
              enabled={(config.smartControl as any)?.filterTransientSpike !== false}
              onChange={handleTransientSpikeFilterChange}
              loading={loadingStates.transientSpikeFilter}
              size="sm"
              color="blue"
              srLabel="切换温度尖峰过滤"
            />
          </SettingRow>

          <SettingRow
            icon={<Spline className="h-4 w-4" />}
            title="曲线方案"
            description="在设置页直接切换风扇温控曲线，避免来回切页。"
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
                  <div className="text-base font-medium text-foreground">自定义转速</div>
                  <div className="text-sm text-muted-foreground">固定转速，适合特殊场景</div>
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
                      应用
                    </Button>
                  </div>
                  <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-300">
                    ⚠ 自定义转速会禁用智能温控
                  </p>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </Section>

        {/* ═══════════ 3. 设备设置 ═══════════ */}
        <Section title="设备设置" icon={Zap}>
          {!isBs1 && (
          <SettingRow
            icon={<Lightbulb className={clsx('h-4 w-4', config.gearLight ? 'text-yellow-500' : '')} />}
            title="挡位灯"
            description="控制设备上的挡位指示灯"
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
            title="通电自启动"
            description="设备通电后自动运行"
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
            title="智能启停"
            description="系统关闭后何时停止散热"
            disabled={!isConnected}
          >
            <div className="w-40">
              <Select
                value={config.smartStartStop || 'off'}
                onChange={(v: string | number) => handleSmartStartStopChange(v as string)}
                options={smartStartStopOptions.map((item) => ({ value: item.value, label: item.label }))}
                disabled={!isConnected}
                size="sm"
              />
            </div>
          </SettingRow>
          )}
        </Section>

        <Section title="拯救者 Fn+Q 联动" icon={Zap}>
          <SettingRow
            icon={<Zap className={clsx('h-4 w-4', legionFnQConfig.enabled ? 'text-primary' : '')} />}
            title="启用插件"
            description="监听拯救者性能模式变化，包括 Fn+Q 和系统软件切换。"
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
            title="接管风扇转速"
            description="检测到性能模式变化后，将散热器切换到下方映射的手动档位。"
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
                <div className="text-sm font-medium text-foreground">模式映射</div>
                <div className="mt-1 text-xs text-muted-foreground">右侧“转速级别”是同一风扇档位内的低/中/高子档，影响实际 RPM。</div>
              </div>
            </div>
            <div className="mb-1 hidden grid-cols-[minmax(96px,1fr)_120px_96px] gap-3 px-3 text-xs text-muted-foreground sm:grid">
              <div>拯救者模式</div>
              <div>风扇档位</div>
              <div>转速级别</div>
            </div>
            <div className="space-y-2">
              {LEGION_POWER_MODES.map((mode) => {
                const target = legionFnQConfig.modeMapping[mode.value] || (getDefaultLegionFnQConfig().modeMapping as any)[mode.value];
                return (
                  <div key={mode.value} className="grid grid-cols-1 items-center gap-3 rounded-xl border border-border/70 bg-background/45 px-3 py-2.5 sm:grid-cols-[minmax(96px,1fr)_120px_96px]">
                    <div className="text-sm font-medium text-foreground">{mode.label}</div>
                    <div className="space-y-1">
                      <div className="text-xs text-muted-foreground sm:hidden">风扇档位</div>
                      <Select
                        value={target.gear}
                        onChange={(value) => handleLegionFnQMappingChange(mode.value, { gear: String(value) })}
                        options={FAN_GEAR_OPTIONS}
                        disabled={!legionFnQConfig.enabled || !legionFnQConfig.takeOverFan || loadingStates.legionFnQ}
                        size="sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <div className="text-xs text-muted-foreground sm:hidden">转速级别</div>
                      <Select
                        value={target.level}
                        onChange={(value) => handleLegionFnQMappingChange(mode.value, { level: String(value) })}
                        options={FAN_LEVEL_OPTIONS}
                        disabled={!legionFnQConfig.enabled || !legionFnQConfig.takeOverFan || loadingStates.legionFnQ}
                        size="sm"
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </Section>

        {/* ═══════════ 4. 系统设置 ═══════════ */}
        <Section title="系统设置" icon={Monitor}>
          <SettingRow
            icon={<Monitor className="h-4 w-4" />}
            title="界面主题"
            description="默认跟随系统，也可手动固定浅色或深色主题"
          >
            <div className="w-36">
              <Select
                value={((config as any).themeMode || 'system') as string}
                onChange={(v: string | number) => handleThemeModeChange(String(v))}
                options={themeModeOptions}
                size="sm"
              />
            </div>
          </SettingRow>

          <div className="px-5 py-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <div className="text-base font-medium text-foreground">全局快捷键</div>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  点击右侧输入框后直接按组合键，失焦自动保存；按 Backspace/Delete 或清除按钮可留空禁用该快捷键。
                </p>
              </div>
            </div>

            <div className="rounded-xl border border-border/70 bg-background/45 px-4 py-2">
              <HotkeyField
                title="切换手动挡位"
                description="在静音、标准、强劲、超频之间轮换，并保留各自的小挡位记忆。"
                value={manualHotkeyInput}
                placeholder="留空为不设置"
                recording={recordingTarget === 'manual'}
                onFocus={() => setRecordingTarget('manual')}
                onBlur={handleHotkeyInputBlur}
                onKeyDown={handleHotkeyInputKeyDown('manual')}
                onClear={() => clearHotkeyInput('manual')}
              />

              <div className="border-t border-border/60" />

              <HotkeyField
                title="开关智能变频"
                description="快速切换智能控温状态，适合游戏或安静场景之间来回切换。"
                value={autoHotkeyInput}
                placeholder="留空为不设置"
                recording={recordingTarget === 'auto'}
                onFocus={() => setRecordingTarget('auto')}
                onBlur={handleHotkeyInputBlur}
                onKeyDown={handleHotkeyInputKeyDown('auto')}
                onClear={() => clearHotkeyInput('auto')}
              />

              <div className="border-t border-border/60" />

              <HotkeyField
                title="切换温控曲线"
                description="快速轮换曲线方案，适合办公/游戏/夜间场景一键切换。"
                value={curveProfileHotkeyInput}
                placeholder="留空为不设置"
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
            title="开机自启动"
            description="Windows 启动时自动运行"
            tip="以管理员身份运行可避免每次 UAC 授权"
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
            title="断连保持配置"
            description="重连后继续使用 APP 配置"
            tip="推荐开启，防止断连后进入设备默认模式"
          >
            <ToggleSwitch
              enabled={(config as any).ignoreDeviceOnReconnect ?? true}
              onChange={handleIgnoreDeviceOnReconnectChange}
              size="sm"
              color="green"
            />
          </SettingRow>
        </Section>

        {/* ═══════════ Offline tip ═══════════ */}
        {!isConnected && (
          <div className="flex items-center gap-2 rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm text-muted-foreground">
            <TriangleAlert className="h-4 w-4 shrink-0" />
            设备未连接，部分功能不可用
          </div>
        )}

        {/* ═══════════ 5. 关于与更新 ═══════════ */}
        <section className="rounded-2xl border border-border bg-card">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
            <Rocket className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold text-foreground">关于与更新</h3>
            <span className="ml-auto text-[11px] text-muted-foreground">BS2PRO Controller</span>
          </div>

          <div className="space-y-3 border-b border-border/60 px-4 py-3.5">
            <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/70 bg-muted/35 px-3 py-3">
              <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs font-medium text-foreground">
                BS2PRO Controller
              </span>
              <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs text-muted-foreground">
                当前 {appVersion ? `v${appVersion}` : '--'}
              </span>
              <a
                href="https://github.com/TIANLI0/BS2PRO-Controller/releases/latest"
                onClick={(e) => {
                  e.preventDefault();
                  handleOpenUrl(latestReleaseUrl || 'https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
                }}
                className="inline-flex items-center gap-1.5 rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary transition-colors hover:bg-primary/15"
              >
                最新 {releaseLoading ? '检查中…' : latestReleaseTag || '--'}
                {hasNewVersion && !releaseLoading && <span className="h-2 w-2 rounded-full bg-destructive" />}
              </a>
            </div>

            {releaseError && <div className="text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}

            {hasNewVersion && (
              <div className="rounded-xl border border-border/70 bg-background/50 p-3">
                <div className="mb-2 text-xs font-medium text-muted-foreground">Release 日志</div>
                {latestReleaseBody ? (
                  <ScrollArea className="max-h-40">
                    <p className="whitespace-pre-wrap text-xs leading-relaxed text-foreground/90">{latestReleaseBody}</p>
                  </ScrollArea>
                ) : (
                  <p className="text-xs text-muted-foreground">暂无日志内容，或本次获取失败。</p>
                )}
              </div>
            )}
          </div>

          <div className="px-4 py-3">
            <div className="rounded-xl border border-border/70 bg-muted/35 p-3">
              <div className="mb-2 text-xs text-muted-foreground">开发者</div>
              <div className="flex items-center gap-3">
                <img
                  src="http://q1.qlogo.cn/g?b=qq&nk=507249007&s=640"
                  alt="Tianli 头像"
                  className="h-12 w-12 rounded-full border border-border object-cover"
                  referrerPolicy="no-referrer"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-foreground">Tianli</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">一个不知名开发者</div>
                </div>
              </div>

              <div className="mt-3 space-y-1.5 border-t border-border/60 pt-2.5 text-xs">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">邮箱</span>
                  <a
                    href="mailto:wutianli@tianli0.top"
                    onClick={(e) => {
                      e.preventDefault();
                      handleOpenUrl('mailto:wutianli@tianli0.top');
                    }}
                    className="text-foreground transition-colors hover:text-foreground/80"
                  >
                    wutianli@tianli0.top
                  </a>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">反馈群</span>
                  <a
                    href="https://qm.qq.com/q/2lEOycrLjq"
                    onClick={(e) => {
                      e.preventDefault();
                      handleOpenUrl('https://qm.qq.com/q/2lEOycrLjq');
                    }}
                    className="inline-flex items-center rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 font-medium text-primary transition-colors hover:bg-primary/15"
                  >
                    QQ 群入口
                  </a>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* ═══════════ 6. 调试面板 ═══════════ */}
        <Collapsible open={debugPanelOpen} onOpenChange={setDebugPanelOpen}>
          <div className="rounded-2xl border border-border bg-card overflow-hidden">
            <CollapsibleTrigger asChild>
              <button type="button" className="flex w-full cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-muted/40">
                <div className="flex items-center gap-2">
                  <Bug className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-semibold text-foreground">调试面板</span>
                </div>
                <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', debugPanelOpen && 'rotate-180')} />
              </button>
            </CollapsibleTrigger>

            <CollapsibleContent>
              <div className="space-y-3 border-t border-border/60 p-4">
                <div className="flex items-center justify-between rounded-xl bg-muted/50 px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    <Bug className="h-4 w-4 text-muted-foreground" />
                    <div>
                      <div className="text-sm font-medium">调试模式</div>
                      <div className="text-[11px] text-muted-foreground">启用详细日志</div>
                    </div>
                  </div>
                  <ToggleSwitch enabled={config.debugMode} onChange={toggleDebugMode} size="sm" color="purple" />
                </div>

                <div className="flex items-center justify-between rounded-xl bg-muted/50 px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    {config.guiMonitoring ? <Eye className="h-4 w-4 text-muted-foreground" /> : <EyeOff className="h-4 w-4 text-muted-foreground" />}
                    <div>
                      <div className="text-sm font-medium">GUI 监控</div>
                      <div className="text-[11px] text-muted-foreground">监控 GUI 响应</div>
                    </div>
                  </div>
                  <ToggleSwitch enabled={config.guiMonitoring} onChange={toggleGuiMonitoring} size="sm" color="purple" />
                </div>

                <Button variant="secondary" size="sm" onClick={fetchDebugInfo} loading={debugInfoLoading} className="w-full">
                  刷新调试信息
                </Button>

                {debugInfo && (
                  <ScrollArea className="max-h-48 rounded-xl border border-border bg-background">
                    <pre className="p-3 text-xs text-foreground/90">{JSON.stringify(debugInfo, null, 2)}</pre>
                  </ScrollArea>
                )}
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

              <h3 className="mb-3 text-center text-lg font-bold text-foreground">风险提示</h3>

              <div className="mb-4 rounded-xl border border-amber-300/40 bg-amber-500/10 p-3 text-sm">
                <p className="mb-2 font-medium text-foreground">启用后：</p>
                <ul className="space-y-1 text-xs text-muted-foreground">
                  <li>• 智能温控将被禁用</li>
                  <li>• 风扇以固定转速运行</li>
                  <li>• 可能导致散热不足</li>
                </ul>
              </div>

              <div className="mb-5 rounded-xl bg-muted/60 p-3 text-center">
                <span className="text-xs text-muted-foreground">设置转速</span>
                <div className="text-xl font-bold text-amber-600">{customSpeedInput} RPM</div>
              </div>

              <div className="flex gap-3">
                <Button variant="secondary" onClick={() => setShowCustomSpeedWarning(false)} className="flex-1">
                  取消
                </Button>
                <Button
                  variant="primary"
                  onClick={() => { setShowCustomSpeedWarning(false); handleCustomSpeedApply(true, customSpeedInput); }}
                  className="flex-1 bg-amber-600 text-white hover:bg-amber-700"
                  icon={<CheckCircle2 className="h-4 w-4" />}
                >
                  确认
                </Button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}

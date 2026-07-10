'use client';

import { memo, useEffect, useMemo, useState } from 'react';
import { motion } from 'framer-motion';
import {
  AlertTriangle,
  ArrowUpRight,
  Bluetooth,
  CircleHelp,
  Cpu,
  Zap,
  RotateCw,
  Fan,
  Gpu,
  Settings,
  Gauge,
  Power,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { type TemperatureHistoryPoint } from '../lib/temperature-history';
import { getManualGearLabel, getReportedMaxRpm } from '../lib/manualGearPresets';
import type { DeviceSettings } from '../types/app';
import { useTranslation } from 'react-i18next';
import { ToggleSwitch, Button } from './ui/index';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import clsx from 'clsx';

interface DeviceStatusProps {
  isConnected: boolean;
  deviceProductId: string | null;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  config: types.AppConfig;
  coreServiceError?: string | null;
  onConnect: () => void;
  onDisconnect: () => void;
  onConfigChange: (config: types.AppConfig) => void;
  onOpenCurveEditor: () => void;
  onOpenHistoryDetails: () => void;
}

interface BridgeRuntimeStatus {
  state?: string;
  working?: boolean;
  ownsProcess?: boolean;
  pipeName?: string;
  transport?: string;
  lastError?: string;
}

const getTempStatus = (temp: number) => {
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', labelKey: 'deviceStatus.tempStatus.overheat' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', labelKey: 'deviceStatus.tempStatus.high' };
  if (temp > 60) return { color: 'text-primary', bg: 'bg-primary', labelKey: 'deviceStatus.tempStatus.normal' };
  return { color: 'text-primary', bg: 'bg-primary', labelKey: 'deviceStatus.tempStatus.good' };
};

const getFanSpinDuration = (rpm?: number) => {
  if (!rpm || rpm <= 0) return 0;
  if (rpm >= 4200) return 0.45;
  if (rpm >= 3200) return 0.7;
  if (rpm >= 2200) return 1;
  return 1.35;
};

const getTranslatedWorkMode = (
  workMode: string | null | undefined,
  t: (key: string) => string,
) => {
  switch (workMode) {
    case '挡位工作模式':
      return t('controlPanel.overview.workModes.manual');
    case '自动模式(实时转速)':
      return t('controlPanel.overview.workModes.auto');
    default:
      return workMode || '--';
  }
};

const AnimatedTemperatureValue = memo(function AnimatedTemperatureValue({ temp, colorClass }: { temp: number | undefined; colorClass: string }) {
  return <span className={clsx('text-[28px] font-bold leading-none tabular-nums tracking-tight', colorClass)}>{temp ?? '--'}</span>;
});

const AnimatedRpmValue = memo(function AnimatedRpmValue({ rpm }: { rpm: number | undefined }) {
  return <span className="text-[28px] font-bold leading-none tabular-nums tracking-tight text-primary">{rpm ?? '--'}</span>;
});

interface SemiGaugeProps {
  /** 当前归一化进度 0~1 */
  value: number;
  /** 进度弧颜色，例如 "var(--primary)"、"#f97316" */
  color: string;
  /** 居中区域 — 数值 + 单位 */
  children?: React.ReactNode;
}

const SemiGauge = memo(function SemiGauge({ value, color, children }: SemiGaugeProps) {
  const r = 84;
  const cx = 100;
  const cy = 100;
  const arc = Math.PI * r;
  const safe = Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
  const dashOffset = arc * (1 - safe);

  return (
    <div className="relative w-full max-w-[15rem]">
      <svg
        viewBox="0 0 200 116"
        className="block w-full"
        preserveAspectRatio="xMidYMid meet"
        aria-hidden="true"
      >
        {/* 背景轨道 */}
        <path
          d={`M ${cx - r} ${cy} A ${r} ${r} 0 0 1 ${cx + r} ${cy}`}
          fill="none"
          stroke="var(--muted)"
          strokeWidth="10"
          strokeLinecap="round"
        />
        {/* 进度弧 — 纯色，无滤镜 */}
        <path
          d={`M ${cx - r} ${cy} A ${r} ${r} 0 0 1 ${cx + r} ${cy}`}
          fill="none"
          stroke={color}
          strokeWidth="10"
          strokeLinecap="round"
          strokeDasharray={arc}
          strokeDashoffset={dashOffset}
          style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(0.22, 1, 0.36, 1)' }}
        />
      </svg>
      {/* 居中区域 — 数值 + 单位 + 状态标签 全部塞进半圆几何中心略偏下 */}
      <div className="pointer-events-none absolute inset-x-0 top-[68%] -translate-y-1/2 flex flex-col items-center justify-center">
        {children}
      </div>
    </div>
  );
});

const SpinningFanIcon = memo(function SpinningFanIcon({ duration, className }: { duration: number; className: string }) {
  return (
    <span className={clsx('inline-flex', duration > 0 && 'animate-spin')} style={duration > 0 ? { animationDuration: `${duration}s` } : undefined}>
      <Fan className={className} />
    </span>
  );
});

const MetricHeader = memo(function MetricHeader({
  icon,
  label,
}: {
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <div className="mb-2 flex items-center justify-center">
      <div className="flex min-w-0 max-w-full items-center justify-center gap-2 text-[13px] font-medium text-muted-foreground">
        <span className="shrink-0">{icon}</span>
        <span className="shrink-0">{label}</span>
      </div>
    </div>
  );
});

const HardwareIdentitySummary = memo(function HardwareIdentitySummary({
  cpuModel,
  gpuModel,
}: {
  cpuModel: string | undefined;
  gpuModel: string | undefined;
}) {
  const items = useMemo(() => [
    { key: 'cpu', model: cpuModel?.trim(), icon: Cpu },
    { key: 'gpu', model: gpuModel?.trim(), icon: Gpu },
  ].filter((item) => item.model), [cpuModel, gpuModel]);

  if (items.length === 0) {
    return null;
  }

  return (
    <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
      {items.map((item) => {
        const Icon = item.icon;
        return (
          <Tooltip key={item.key}>
            <TooltipTrigger asChild>
              <div className="flex min-w-0 max-w-[18rem] items-center gap-1.5 rounded-full border border-border/70 bg-background/75 px-2.5 py-1 text-[11px] shadow-sm shadow-black/5 backdrop-blur-xl">
                <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 truncate text-foreground/85">{item.model}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent>{item.model}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
});

/* ── Memo sub-components to avoid parent re-renders ── */

// 温度状态 → 仪表盘弧色（CSS 变量 / 字面色值，避免依赖 Tailwind class）
const getTempArcColor = (temp: number) => {
  if (temp > 85) return '#ef4444';
  if (temp > 75) return '#f97316';
  return 'var(--primary)';
};

const TempGaugeDisplay = memo(function TempGaugeDisplay({
  temp,
  ready,
}: {
  temp: number | undefined;
  /** 后端首次推送有效温度后置为 true；之前显示占位避免误读 0 °C */
  ready: boolean;
}) {
  const { t } = useTranslation();

  // 未就绪 → 占位：灰色弧、"--"、"读取中…"，不进入正常状态色
  if (!ready) {
    return (
      <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-end">
        <SemiGauge value={0} color="var(--muted-foreground)">
          <div className="flex items-baseline gap-0.5">
            <span className="text-[28px] font-bold leading-none tabular-nums tracking-tight text-muted-foreground/70">--</span>
            <span className="text-xs font-medium text-muted-foreground/70">°C</span>
          </div>
          <span className="mt-1 inline-flex items-center gap-1 text-[11px] leading-none text-muted-foreground">
            <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-muted-foreground/60" />
            {t('deviceStatus.tempGauge.loading')}
          </span>
        </SemiGauge>
      </div>
    );
  }

  const status = getTempStatus(temp || 0);
  const ratio = Math.min(1, (temp || 0) / 100);
  const arcColor = getTempArcColor(temp || 0);
  return (
    <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-end">
      <SemiGauge value={ratio} color={arcColor}>
        <div className="flex items-baseline gap-0.5">
          <AnimatedTemperatureValue temp={temp} colorClass={status.color} />
          <span className="text-xs font-medium text-muted-foreground">°C</span>
        </div>
        <span className="mt-1 text-[11px] leading-none text-muted-foreground">{t(status.labelKey)}</span>
      </SemiGauge>
    </div>
  );
});

const FanRpmDisplay = memo(function FanRpmDisplay({
  currentRpm,
  targetRpm,
  setGear,
  isBs1,
  maxRpm,
}: {
  currentRpm: number | undefined;
  targetRpm: number | undefined;
  setGear: string | undefined;
  isBs1?: boolean;
  maxRpm: number;
}) {
  const { t } = useTranslation();
  const safeMax = maxRpm > 0 ? maxRpm : 4000;
  const ratio = Math.min(1, (currentRpm || 0) / safeMax);
  const subLabel = isBs1
    ? (getManualGearLabel(setGear) || '--')
    : t('deviceStatus.fan.targetSummary', { target: targetRpm ?? '--', gear: getManualGearLabel(setGear) || '--' });

  return (
    <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-end">
      <SemiGauge value={ratio} color="var(--primary)">
        <div className="flex items-baseline gap-0.5">
          <AnimatedRpmValue rpm={currentRpm} />
          <span className="text-[11px] font-medium text-muted-foreground">RPM</span>
        </div>
        <span className="mt-1 max-w-[11rem] truncate text-[11px] leading-none text-muted-foreground">
          {subLabel}
        </span>
      </SemiGauge>
    </div>
  );
});

const MiniFanCurveChart = memo(function MiniFanCurveChart({
  curve,
  currentTemp,
  onOpen,
}: {
  curve: types.FanCurvePoint[] | undefined;
  currentTemp: number | undefined;
  onOpen?: () => void;
}) {
  const { t } = useTranslation();

  const geometry = useMemo(() => {
    const points = Array.isArray(curve)
      ? curve.filter((point) => typeof point.temperature === 'number' && typeof point.rpm === 'number')
      : [];
    const source = points.length > 0 ? points : [
      { temperature: 30, rpm: 600 },
      { temperature: 45, rpm: 1200 },
      { temperature: 60, rpm: 2300 },
      { temperature: 75, rpm: 3300 },
      { temperature: 95, rpm: 4000 },
    ];
    // 单遍扫描计算 min/max，避免旧实现 4 次 Math.min/Math.max(...source.map(...)) 重建临时数组。
    let minTemp = 30;
    let maxTemp = 100;
    let maxRpm = 4000;
    for (const p of source) {
      if (p.temperature < minTemp) minTemp = p.temperature;
      if (p.temperature > maxTemp) maxTemp = p.temperature;
      if (p.rpm > maxRpm) maxRpm = p.rpm;
    }
    const width = 520;
    const height = 146;
    const pad = { left: 44, right: 20, top: 14, bottom: 18 };
    const plotWidth = width - pad.left - pad.right;
    const plotHeight = height - pad.top - pad.bottom;
    const tempRange = Math.max(1, maxTemp - minTemp);
    const xForTemp = (temp: number) => pad.left + ((temp - minTemp) / tempRange) * plotWidth;
    const yForRpm = (rpm: number) => pad.top + plotHeight - (rpm / maxRpm) * plotHeight;
    const linePoints = source
      .map((point) => `${xForTemp(point.temperature).toFixed(1)},${yForRpm(point.rpm).toFixed(1)}`)
      .join(' ');
    const areaPoints = `${pad.left},${pad.top + plotHeight} ${linePoints} ${pad.left + plotWidth},${pad.top + plotHeight}`;
    const yTicks: number[] = [0, 1000, 2000, 3000, 4000].filter((tick) => tick <= maxRpm);
    return { width, height, pad, plotWidth, plotHeight, minTemp, maxTemp, maxRpm, xForTemp, yForRpm, linePoints, areaPoints, yTicks };
  }, [curve]);

  const { width, height, pad, plotWidth, plotHeight, minTemp, maxTemp, xForTemp, yForRpm, linePoints, areaPoints, yTicks } = geometry;

  const currentX = typeof currentTemp === 'number' && currentTemp > 0
    ? Math.max(pad.left, Math.min(pad.left + plotWidth, xForTemp(currentTemp)))
    : null;

  return (
    <button
      type="button"
      onClick={onOpen}
      className={clsx(
        'glacier-chart-card group flex h-full w-full flex-col rounded-xl border border-border bg-card p-3 text-left shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md',
      )}
    >
      <div className="mb-2 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-semibold text-foreground">{t('deviceStatus.chart.fanCurve')}</div>
          <div className="text-[11px] text-muted-foreground">RPM</div>
        </div>
        {onOpen && (
          <span className="inline-flex items-center gap-1 text-[11px] font-medium text-primary opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
            {t('deviceStatus.chart.openCurve')}
            <ArrowUpRight className="h-3 w-3" />
          </span>
        )}
      </div>
      <div className="glacier-chart-canvas aspect-[520/146] w-full overflow-hidden">
        <svg viewBox={`0 0 ${width} ${height}`} className="h-full w-full" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
          {yTicks.map((tick) => {
            const y = yForRpm(tick);
            return (
              <g key={tick}>
                <line x1={pad.left} y1={y} x2={pad.left + plotWidth} y2={y} stroke="var(--chart-grid)" strokeWidth="1" />
                <text x={pad.left - 8} y={y + 4} textAnchor="end" fontSize="10" fill="var(--chart-tick)">{tick}</text>
              </g>
            );
          })}
          <polygon points={areaPoints} fill="var(--chart-primary)" opacity="0.14" />
          <polyline points={linePoints} fill="none" stroke="var(--chart-primary)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
          {currentX !== null && (
            <line x1={currentX} y1={pad.top} x2={currentX} y2={pad.top + plotHeight} stroke="var(--chart-temperature-indicator)" strokeWidth="1.5" strokeDasharray="4 4" opacity="0.9" />
          )}
          <text x={pad.left} y={height - 7} fontSize="10" fill="var(--chart-tick)">{minTemp}</text>
          <text x={pad.left + plotWidth} y={height - 7} textAnchor="end" fontSize="10" fill="var(--chart-tick)">{maxTemp} °C</text>
        </svg>
      </div>
    </button>
  );
});

const TemperatureHistoryPanel = memo(function TemperatureHistoryPanel({
  points,
  enabled,
  source,
  onOpen,
}: {
  points: TemperatureHistoryPoint[];
  enabled: boolean;
  source: 'core' | 'session';
  onOpen?: () => void;
}) {
  const { t } = useTranslation();
  const sourceLabel = source === 'core' ? t('deviceStatus.history.source.core') : t('deviceStatus.history.source.session');
  const chart = useMemo(() => {
    const width = 520;
    const height = 168;
    const pad = { left: 8, right: 8, top: 10, bottom: 10 };
    const plotWidth = width - pad.left - pad.right;
    const plotHeight = height - pad.top - pad.bottom;
    let minTemp = 35;
    let maxTemp = 80;
    let maxFanRpm = 4000;

    for (const point of points) {
      if (point.cpuTemp > 0) {
        minTemp = Math.min(minTemp, point.cpuTemp);
        maxTemp = Math.max(maxTemp, point.cpuTemp);
      }
      if (point.gpuTemp > 0) {
        minTemp = Math.min(minTemp, point.gpuTemp);
        maxTemp = Math.max(maxTemp, point.gpuTemp);
      }
      if (point.fanRpm > 0) {
        maxFanRpm = Math.max(maxFanRpm, point.fanRpm);
      }
    }

    const minY = Math.max(0, Math.floor((minTemp - 6) / 5) * 5);
    const maxY = Math.min(110, Math.ceil((maxTemp + 6) / 5) * 5);
    const rangeY = Math.max(10, maxY - minY);
    const minTs = points[0]?.timestamp ?? 0;
    const maxTs = points[points.length - 1]?.timestamp ?? minTs;
    const rangeTs = Math.max(1, maxTs - minTs);
    const xFor = (timestamp: number, index: number) => {
      if (points.length <= 1) return pad.left + plotWidth / 2;
      if (rangeTs <= 1 && points.length > 1) return pad.left + (index / Math.max(1, points.length - 1)) * plotWidth;
      return pad.left + ((timestamp - minTs) / rangeTs) * plotWidth;
    };
    const yForTemp = (temp: number) => pad.top + plotHeight - ((temp - minY) / rangeY) * plotHeight;
    const yForFan = (rpm: number) => pad.top + plotHeight - (rpm / Math.max(1, maxFanRpm)) * plotHeight;
    const buildPath = (selector: (point: TemperatureHistoryPoint) => number, projectY: (value: number) => number) => {
      let path = '';
      let started = false;
      points.forEach((point, index) => {
        const value = selector(point);
        if (value <= 0) {
          started = false;
          return;
        }
        path += `${started ? 'L' : 'M'} ${xFor(point.timestamp, index).toFixed(1)} ${projectY(value).toFixed(1)} `;
        started = true;
      });
      return path.trim();
    };

    return {
      width,
      height,
      pad,
      plotWidth,
      plotHeight,
      cpuPath: buildPath((point) => point.cpuTemp, yForTemp),
      gpuPath: buildPath((point) => point.gpuTemp, yForTemp),
      fanPath: buildPath((point) => point.fanRpm, yForFan),
      gridLines: [0.2, 0.5, 0.8],
    };
  }, [points]);
  const { width, height, pad, plotWidth, plotHeight, cpuPath, gpuPath, fanPath, gridLines } = chart;
  const handlePanelKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!onOpen) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onOpen();
    }
  };

  return (
    <div
      role={onOpen ? 'button' : undefined}
      tabIndex={onOpen ? 0 : undefined}
      onClick={onOpen}
      onKeyDown={handlePanelKeyDown}
      className={clsx(
        'glacier-chart-card group flex h-full min-h-[239px] flex-col rounded-xl border border-border bg-card p-3 shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/30',
      )}
    >
      <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <div className="text-xs font-semibold text-foreground">{t('deviceStatus.history.title')}</div>
          <span className="rounded-full border border-border/70 bg-background/70 px-2 py-0.5 text-[10px] text-muted-foreground">{sourceLabel}</span>
          {onOpen && (
            <span className="inline-flex items-center gap-1 text-[11px] font-medium text-primary opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
              {t('deviceStatus.history.details')}
              <ArrowUpRight className="h-3 w-3" />
            </span>
          )}
        </div>
      </div>

      <div className="glacier-chart-canvas flex min-h-[163px] flex-1 overflow-hidden rounded-lg bg-muted/25 p-2.5">
        {points.length === 0 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {enabled ? t('deviceStatus.history.waiting') : t('deviceStatus.history.disabled')}
          </div>
        ) : points.length < 2 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {source === 'core' ? t('deviceStatus.history.singleSampleCore') : t('deviceStatus.history.singleSampleSession')}
          </div>
        ) : (
          <div className="h-full w-full overflow-hidden">
            <svg viewBox={`0 0 ${width} ${height}`} className="h-full w-full" preserveAspectRatio="none" aria-hidden="true">
            {gridLines.map((ratio) => {
              const y = pad.top + plotHeight * ratio;
              return (
                <g key={ratio}>
                  <line x1={pad.left} y1={y} x2={pad.left + plotWidth} y2={y} stroke="var(--chart-grid)" strokeWidth="1" opacity="0.7" />
                </g>
              );
            })}
            {cpuPath && <path d={cpuPath} fill="none" stroke="#2f6df6" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />}
            {gpuPath && <path d={gpuPath} fill="none" stroke="#f97316" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />}
            {fanPath && <path d={fanPath} fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />}
            </svg>
          </div>
        )}
      </div>
    </div>
  );
});

/* ── Main component ── */

export default function DeviceStatus({
  isConnected,
  deviceProductId,
  deviceModel,
  deviceSettings,
  fanData,
  temperature,
  config,
  coreServiceError,
  onConnect,
  onDisconnect,
  onConfigChange,
      onOpenCurveEditor,
      onOpenHistoryDetails,
}: DeviceStatusProps) {
  const { t } = useTranslation();
  const [bridgeWarningReady, setBridgeWarningReady] = useState(false);
  const [activeCurveProfileName, setActiveCurveProfileName] = useState('');
  const [bridgeStatus, setBridgeStatus] = useState<BridgeRuntimeStatus | null>(null);
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    source: temperatureHistorySource,
  } = useTemperatureHistory();
  const hasBridgeWarning = isConnected && temperature?.bridgeOk === false;

  useEffect(() => {
    if (!hasBridgeWarning) {
      setBridgeWarningReady(false);
      return;
    }
    const timer = window.setTimeout(() => setBridgeWarningReady(true), 2000);
    return () => window.clearTimeout(timer);
  }, [hasBridgeWarning]);

  useEffect(() => {
    if (!hasBridgeWarning || !bridgeWarningReady) {
      setBridgeStatus(null);
      return;
    }

    let cancelled = false;
    const loadBridgeStatus = async () => {
      try {
        const status = await apiService.getBridgeProgramStatus();
        if (!cancelled) {
          setBridgeStatus((status || null) as BridgeRuntimeStatus | null);
        }
      } catch {
        if (!cancelled) {
          setBridgeStatus(null);
        }
      }
    };

    loadBridgeStatus();
    return () => {
      cancelled = true;
    };
  }, [bridgeWarningReady, hasBridgeWarning]);

  useEffect(() => {
    let cancelled = false;

    const loadActiveCurveProfile = async () => {
      try {
        const payload = await apiService.getFanCurveProfiles();
        const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
        const preferredActiveId = ((config as any).activeFanCurveProfileId || payload?.activeId || profiles[0]?.id || '') as string;
        const activeProfile = profiles.find((p) => p.id === preferredActiveId) ?? profiles[0];
        if (!cancelled) {
          setActiveCurveProfileName(activeProfile?.name || '');
        }
      } catch {
        if (!cancelled) {
          setActiveCurveProfileName('');
        }
      }
    };

    loadActiveCurveProfile();
    return () => {
      cancelled = true;
    };
  }, [isConnected, (config as any).activeFanCurveProfileId]);

  const handleAutoControlChange = async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch (err) {
      console.error('设置智能变频失败:', err);
    }
  };

  const normalizedProductId = deviceProductId?.trim().toUpperCase() ?? '';
  const isBs3Model = deviceModel === 'BS3' || normalizedProductId === '0X1003';
  const isBs3ProModel = deviceModel === 'BS3PRO' || normalizedProductId === '0X1004';
  const isBs2ProModel = deviceModel === 'BS2PRO' || normalizedProductId === '0X1002';
  const isProModel = isBs2ProModel || isBs3ProModel;
  const isBs2Model = deviceModel === 'BS2' || normalizedProductId === '0X1001';
  const isBs1Model = deviceModel === 'BS1';
  const deviceModelName = isBs1Model ? 'BS1' : isBs3ProModel ? 'BS3 PRO' : isBs3Model ? 'BS3' : isBs2ProModel ? 'BS2 PRO' : isBs2Model ? 'BS2' : t('deviceStatus.device.unknown');
  const deviceImageSrc = isBs1Model ? '/bs2.png' : isBs2Model ? '/bs2.png' : '/bs2pro.png';
  const modeTitle = config.autoControl ? t('deviceStatus.mode.smartControl') : config.customSpeedEnabled ? t('deviceStatus.mode.fixedSpeed') : t('deviceStatus.mode.manualStrategy');
  const modeDesc = config.autoControl
    ? t('deviceStatus.mode.smartDescription')
    : config.customSpeedEnabled
      ? t('deviceStatus.mode.fixedDescription', { rpm: config.customSpeedRPM || fanData?.currentRpm || '--' })
      : t('deviceStatus.mode.manualDescription');
  const modeDisplayTitle = activeCurveProfileName ? t('deviceStatus.mode.withProfile', { mode: modeTitle, profile: activeCurveProfileName }) : modeTitle;
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);
  const maxRpmInfo = useMemo(() => getReportedMaxRpm(fanData?.gearSettings, fanData?.maxGear), [fanData?.gearSettings, fanData?.maxGear]);
  const deviceExtremeRPM = deviceSettings?.gearRpmTable?.find((item) => item.label === 'extreme')?.rpm;
  const reportedMaxRpm = maxRpmInfo.rpm;
  const maxGearHighLevelRpm = (isBs1Model || isBs2Model || isBs3Model)
    ? 3300
    : reportedMaxRpm || deviceExtremeRPM;
  // 温度就绪判定：后端首次推送（updateTime > 0）且该路传感器读到非零值。
  // 单独按通路判 — 只有 GPU 没装独显时仍会保持 0，但 CPU 已就绪则只显示 GPU 占位。
  const tempPushed = (temperature?.updateTime ?? 0) > 0;
  const cpuReady = tempPushed && (temperature?.cpuTemp ?? 0) > 0;
  const gpuReady = tempPushed && (temperature?.gpuTemp ?? 0) > 0;
  // 参考温度：跟随设置页“控温温度来源”(max/cpu/gpu)，无该路读数时回退到综合最高温。
  const referenceTemp = (() => {
    const source = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
    const cpu = temperature?.cpuTemp ?? 0;
    const gpu = temperature?.gpuTemp ?? 0;
    const max = temperature?.maxTemp ?? 0;
    if (source === 'cpu') return cpu > 0 ? cpu : max;
    if (source === 'gpu') return gpu > 0 ? gpu : max;
    return max;
  })();
  const bridgeStateLabel = bridgeStatus?.state === 'running_owned'
    ? t('deviceStatus.bridgeState.runningOwned')
    : bridgeStatus?.state === 'attached'
      ? t('deviceStatus.bridgeState.attached')
      : bridgeStatus?.state === 'starting'
        ? t('deviceStatus.bridgeState.starting')
        : bridgeStatus?.state === 'degraded'
          ? t('deviceStatus.bridgeState.degraded')
          : bridgeStatus?.state === 'failed'
            ? t('deviceStatus.bridgeState.failed')
            : bridgeStatus?.state === 'stopping'
              ? t('deviceStatus.bridgeState.stopping')
              : bridgeStatus?.state === 'stopped'
                ? t('deviceStatus.bridgeState.stopped')
                : bridgeStatus?.state === 'not_started'
                  ? t('deviceStatus.bridgeState.notStarted')
                  : '';
  const maxRpmHint = isBs1Model
    ? t('deviceStatus.maxRpmHint.bs1')
    : maxGearHighLevelRpm === 4000
      ? t('deviceStatus.maxRpmHint.max4000')
      : maxGearHighLevelRpm === 3300
        ? t('deviceStatus.maxRpmHint.max3300')
        : maxGearHighLevelRpm === 2760
          ? t('deviceStatus.maxRpmHint.max2760')
          : maxRpmInfo.codeHex
            ? t('deviceStatus.maxRpmHint.unknownCode', { code: maxRpmInfo.codeHex })
            : t('deviceStatus.maxRpmHint.waiting');
  const maxTempStatus = getTempStatus(temperature?.maxTemp || 0);

  return (
    <div className="space-y-3">
      {/* ── Device header card ── */}
      <div className="glacier-hero-card relative overflow-hidden rounded-xl border border-border bg-card p-4 shadow-sm shadow-black/5">
        <div className="theme-thrm-only glacier-hero-art pointer-events-none absolute inset-y-0 right-0 hidden overflow-hidden md:block" aria-hidden="true">
          <img
            src="/theme/ice-operator-banner.png"
            alt=""
            draggable={false}
            className="glacier-operator-art h-full w-full object-cover object-right opacity-[0.58] mix-blend-multiply"
          />
          <div className="absolute inset-0 bg-gradient-to-r from-card/80 via-card/25 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-b from-white/20 via-transparent to-card/30" />
        </div>
        <div className="theme-thrm-only glacier-hero-art-label pointer-events-none absolute top-3 hidden text-[10px] font-semibold uppercase tracking-[0.32em] text-primary/45 md:block" aria-hidden="true">
          AURORA AUX / GLACIER CORE
        </div>
        <div className="glacier-hero-content relative z-10 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div
              className="flex h-14 w-20 items-center justify-center overflow-hidden rounded-xl bg-muted/45 p-1.5"
            >
              <img
                src={deviceImageSrc}
                alt={t('deviceStatus.device.imageAlt', { model: deviceModelName })}
                className="h-full w-full object-contain"
                draggable={false}
              />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-base font-semibold text-foreground">{deviceModelName}</span>
                <span
                  className={clsx(
                    'rounded-md px-2 py-0.5 text-[11px] font-semibold',
                    isConnected
                      ? 'bg-primary/10 text-primary'
                      : 'bg-red-500/10 text-red-500',
                  )}
                >
                  {isConnected ? t('deviceStatus.connectStatus.connected') : t('deviceStatus.connectStatus.offline')}
                </span>
              </div>
              {isConnected && (
                <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                  {config.autoControl ? (
                    <Zap className="h-3 w-3 text-primary" />
                  ) : (
                    <Settings className="h-3 w-3" />
                  )}
                  <span>{t('deviceStatus.hero.modeLine', { mode: modeTitle, description: modeDesc })}</span>
                </div>
              )}
              {!isConnected && (
                <p className={clsx('mt-1 text-xs', coreServiceError ? 'text-destructive' : 'text-muted-foreground')}>
                  {coreServiceError ? t('deviceStatus.hero.coreUnavailable') : t('deviceStatus.hero.waitingBluetooth')}
                </p>
              )}
            </div>
          </div>

          <div className="glacier-hero-actions flex items-center gap-3">
            {isConnected && (
              <ToggleSwitch
                enabled={config.autoControl}
                onChange={handleAutoControlChange}
                label={t('deviceStatus.actions.smartControl')}
                size="md"
                color="blue"
              />
            )}
            <Button
              variant={isConnected ? 'secondary' : 'primary'}
              size="sm"
              onClick={isConnected ? onDisconnect : onConnect}
            >
              {isConnected ? t('deviceStatus.actions.disconnect') : t('deviceStatus.actions.connect')}
            </Button>
          </div>
        </div>
      </div>

      {/* ── Metric cards ── */}
      {isConnected ? (
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          className="grid grid-cols-1 items-stretch gap-3 md:grid-cols-3"
        >
          {/* CPU */}
          <div className="glacier-metric-card flex h-full min-h-[155px] flex-col items-center rounded-xl border border-border bg-card px-5 py-3 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[171px]">
            <MetricHeader
              icon={<Cpu className="h-4 w-4" />}
              label={t('deviceStatus.metrics.cpuTemperature')}
            />
            <TempGaugeDisplay temp={temperature?.cpuTemp} ready={cpuReady} />
          </div>

          {/* GPU */}
          <div className="glacier-metric-card flex h-full min-h-[155px] flex-col items-center rounded-xl border border-border bg-card px-5 py-3 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[171px]">
            <MetricHeader
              icon={<Gpu className="h-4 w-4" />}
              label={t('deviceStatus.metrics.gpuTemperature')}
            />
            <TempGaugeDisplay temp={temperature?.gpuTemp} ready={gpuReady} />
          </div>

          {/* Fan */}
          <div className="glacier-metric-card flex h-full min-h-[155px] flex-col items-center rounded-xl border border-border bg-card px-5 py-3 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[171px]">
            <MetricHeader
              icon={(
                <SpinningFanIcon duration={fanSpinDuration} className="h-4 w-4" />
              )}
              label={t('deviceStatus.metrics.fanRpm')}
            />
            <FanRpmDisplay
              currentRpm={fanData?.currentRpm}
              targetRpm={fanData?.targetRpm}
              setGear={fanData?.setGear}
              isBs1={isBs1Model}
              maxRpm={maxGearHighLevelRpm || 4000}
            />
          </div>
        </motion.div>
      ) : (
        <motion.div
          initial={{ opacity: 0, scale: 0.98 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.3 }}
          className="rounded-xl border border-dashed border-border bg-card p-14 text-center"
        >
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-muted">
            <Bluetooth className="h-7 w-7 text-muted-foreground" />
          </div>
          <h3 className="mb-1.5 text-lg font-semibold">{t('deviceStatus.disconnected.title')}</h3>
          <p className="mb-5 text-base text-muted-foreground">{t('deviceStatus.disconnected.description')}</p>
          <Button onClick={onConnect} size="md" icon={<RotateCw className="h-4 w-4" />}>
            {t('deviceStatus.actions.connectDevice')}
          </Button>
        </motion.div>
      )}

      {/* ── Bridge warning ── */}
      {bridgeWarningReady && (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          className="overflow-hidden"
        >
          <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-sm dark:border-amber-800/60 dark:bg-amber-900/20">
            <div className="flex items-start gap-2 text-amber-800 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
              <div className="flex-1">
                <p>{temperature?.bridgeMessage || t('deviceStatus.bridgeWarning.default')}</p>
                {bridgeStatus && (
                  <div className="mt-2 space-y-1 text-xs text-amber-700/90 dark:text-amber-200/80">
                    {bridgeStateLabel && (
                      <p>
                        {t('deviceStatus.bridgeWarning.stateLine', { state: bridgeStateLabel })}
                        {typeof bridgeStatus.ownsProcess === 'boolean' ? ` · ${bridgeStatus.ownsProcess ? t('deviceStatus.bridgeWarning.ownsProcess') : t('deviceStatus.bridgeWarning.sharedProcess')}` : ''}
                      </p>
                    )}
                    {bridgeStatus.transport && <p>{t('deviceStatus.bridgeWarning.transportLine', { transport: bridgeStatus.transport })}</p>}
                    {bridgeStatus.pipeName && <p>{t('deviceStatus.bridgeWarning.pipeLine', { pipe: bridgeStatus.pipeName })}</p>}
                    {bridgeStatus.lastError && bridgeStatus.lastError !== temperature?.bridgeMessage && <p>{t('deviceStatus.bridgeWarning.diagnosticsLine', { message: bridgeStatus.lastError })}</p>}
                  </div>
                )}
                <button
                  onClick={async () => {
                    try {
                      await apiService.restartPawnIO();
                    } catch { /* ignore */ }
                  }}
                  className="mt-2 inline-flex items-center gap-1.5 rounded-lg border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs font-medium text-amber-900 transition-colors hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-200 dark:hover:bg-amber-800/60"
                >
                  <RotateCw className="h-3 w-3" />
                  {t('deviceStatus.bridgeWarning.reinitialize')}
                </button>
              </div>
            </div>
          </div>
        </motion.div>
      )}

      {/* ── Running details ── */}
      {isConnected && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.15, duration: 0.3 }}
          className="glacier-control-card rounded-xl border border-border bg-card p-3 shadow-sm shadow-black/5"
        >
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2 px-1">
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-muted-foreground" />
              <h3 className="text-xs font-semibold text-muted-foreground">
                {t('deviceStatus.controlProtection')}
              </h3>
            </div>
            <HardwareIdentitySummary cpuModel={temperature?.cpuModel} gpuModel={temperature?.gpuModel} />
          </div>

          <div className="grid grid-cols-2 gap-2.5 md:grid-cols-4">
            <div className="glacier-stat-tile rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                {t('deviceStatus.stats.controlMode')}
              </div>
              <div className={clsx('text-sm font-semibold', config.autoControl ? 'text-primary' : 'text-amber-600 dark:text-amber-400')}>
                {modeDisplayTitle}
              </div>
            </div>

            <div className="glacier-stat-tile group rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Power className="h-3.5 w-3.5" />
                  {t('deviceStatus.stats.maxRpm')}
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="inline-flex h-4 w-4 items-center justify-center rounded text-muted-foreground/80 opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
                      aria-label={t('deviceStatus.stats.maxRpmHintAria')}
                    >
                      <CircleHelp className="h-3.5 w-3.5" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>{maxRpmHint}</TooltipContent>
                </Tooltip>
              </div>
              <div className="text-sm font-semibold">
                {maxGearHighLevelRpm
                  ? `${maxGearHighLevelRpm} RPM`
                  : maxRpmInfo.codeHex || '--'}
              </div>
            </div>

            <div className="glacier-stat-tile rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Fan className="h-3.5 w-3.5" />
                {t('deviceStatus.stats.workMode')}
              </div>
              <div className="text-sm font-semibold">{getTranslatedWorkMode(fanData?.workMode, t)}</div>
            </div>

            <div className="glacier-stat-tile rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                {t('deviceStatus.stats.tempStatus')}
              </div>
              <div className={clsx('text-sm font-semibold tabular-nums', maxTempStatus.color)}>
                {t(maxTempStatus.labelKey)}
              </div>
            </div>
          </div>

        </motion.div>
      )}

      {isConnected && (
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.3 }}
          className="grid grid-cols-1 items-stretch gap-2.5 lg:grid-cols-[minmax(0,1.55fr)_minmax(280px,0.95fr)]"
        >
          <MiniFanCurveChart curve={config.fanCurve} currentTemp={referenceTemp} onOpen={onOpenCurveEditor} />
          <TemperatureHistoryPanel
            points={temperatureHistory}
            enabled={temperatureHistoryEnabled}
            source={temperatureHistorySource}
            onOpen={onOpenHistoryDetails}
          />
        </motion.div>
      )}
    </div>
  );
}

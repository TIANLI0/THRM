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
import { getReportedMaxRpm } from '../lib/manualGearPresets';
import { ToggleSwitch, Button } from './ui/index';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import clsx from 'clsx';

interface DeviceStatusProps {
  isConnected: boolean;
  deviceProductId: string | null;
  deviceModel: string | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  config: types.AppConfig;
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
  lastError?: string;
}

const getTempStatus = (temp: number) => {
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', label: '过热' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', label: '偏高' };
  if (temp > 60) return { color: 'text-primary', bg: 'bg-primary', label: '正常' };
  return { color: 'text-primary', bg: 'bg-primary', label: '良好' };
};

const getFanSpinDuration = (rpm?: number) => {
  if (!rpm || rpm <= 0) return 0;
  if (rpm >= 4200) return 0.45;
  if (rpm >= 3200) return 0.7;
  if (rpm >= 2200) return 1;
  return 1.35;
};

const AnimatedTemperatureValue = memo(function AnimatedTemperatureValue({ temp, colorClass }: { temp: number | undefined; colorClass: string }) {
  return <span className={clsx('text-[28px] font-bold leading-none tabular-nums', colorClass)}>{temp ?? '--'}</span>;
});

const AnimatedRpmValue = memo(function AnimatedRpmValue({ rpm }: { rpm: number | undefined }) {
  return <span className="text-[28px] font-bold leading-none tabular-nums text-primary">{rpm ?? '--'}</span>;
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
    <div className="mb-4 flex items-center justify-center">
      <div className="flex min-w-0 max-w-full items-center justify-center gap-1.5 text-xs font-semibold text-muted-foreground">
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
  const items = [
    { key: 'cpu', model: cpuModel?.trim(), icon: Cpu },
    { key: 'gpu', model: gpuModel?.trim(), icon: Gpu },
  ].filter((item) => item.model);

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

const CpuTempDisplay = memo(function CpuTempDisplay({ temp }: { temp: number | undefined }) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-center text-center">
      <div className="flex flex-col items-center justify-center">
        <div className="flex items-baseline gap-0.5">
          <AnimatedTemperatureValue temp={temp} colorClass={status.color} />
          <span className="text-xs text-muted-foreground">°C</span>
        </div>
        <span className="mt-1.5 text-[11px] text-muted-foreground">{status.label}</span>
      </div>
      <div className="mt-6 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

const GpuTempDisplay = memo(function GpuTempDisplay({ temp }: { temp: number | undefined }) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-center text-center">
      <div className="flex flex-col items-center justify-center">
        <div className="flex items-baseline gap-0.5">
          <AnimatedTemperatureValue temp={temp} colorClass={status.color} />
          <span className="text-xs text-muted-foreground">°C</span>
        </div>
        <span className="mt-1.5 text-[11px] text-muted-foreground">{status.label}</span>
      </div>
      <div className="mt-6 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

const FanRpmDisplay = memo(function FanRpmDisplay({
  currentRpm,
  targetRpm,
  setGear,
  isBs1,
}: {
  currentRpm: number | undefined;
  targetRpm: number | undefined;
  setGear: string | undefined;
  isBs1?: boolean;
}) {
  const pct = Math.min(100, ((currentRpm || 0) / 4000) * 100);

  return (
    <div className="flex h-full w-full max-w-[20rem] flex-1 flex-col items-center justify-center text-center">
      <div className="flex flex-col items-center justify-center">
        <div className="flex items-baseline gap-0.5">
          <AnimatedRpmValue rpm={currentRpm} />
          <span className="text-xs text-muted-foreground">RPM</span>
        </div>
        <span className="mt-1.5 text-[11px] text-muted-foreground">
          {isBs1 ? (setGear || '--') : `目标 ${targetRpm ?? '--'} · ${setGear || '--'}`}
        </span>
      </div>
      <div className="mt-6 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary transition-all duration-300" style={{ width: `${pct}%` }} />
      </div>
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
        'group flex h-full w-full flex-col rounded-xl border border-border bg-card p-3 text-left shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md',
      )}
    >
      <div className="mb-2 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-xs font-semibold text-foreground">风扇转速曲线</div>
          <div className="text-[11px] text-muted-foreground">RPM</div>
        </div>
        {onOpen && (
          <span className="inline-flex items-center gap-1 text-[11px] font-medium text-primary opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
            曲线页
            <ArrowUpRight className="h-3 w-3" />
          </span>
        )}
      </div>
      <div className="aspect-[520/146] w-full overflow-hidden">
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
  points: Array<{ timestamp: number; cpuTemp: number; gpuTemp: number; fanRpm: number }>;
  enabled: boolean;
  source: 'core' | 'session';
  onOpen?: () => void;
}) {
  const width = 520;
  const height = 168;
  const pad = { left: 8, right: 8, top: 10, bottom: 10 };
  const plotWidth = width - pad.left - pad.right;
  const plotHeight = height - pad.top - pad.bottom;
  const sourceLabel = source === 'core' ? '后台记录' : '本次打开';
  const validTemps = points.flatMap((point) => [point.cpuTemp, point.gpuTemp]).filter((value) => value > 0);
  const validFanRpm = points.map((point) => point.fanRpm).filter((value) => value > 0);
  const minY = Math.max(0, Math.floor((Math.min(...validTemps, 35) - 6) / 5) * 5);
  const maxY = Math.min(110, Math.ceil((Math.max(...validTemps, 80) + 6) / 5) * 5);
  const rangeY = Math.max(10, maxY - minY);
  const maxFanRpm = Math.max(4000, ...validFanRpm, 0);
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
  const cpuPath = buildPath((point) => point.cpuTemp, yForTemp);
  const gpuPath = buildPath((point) => point.gpuTemp, yForTemp);
  const fanPath = buildPath((point) => point.fanRpm, yForFan);
  const gridLines = [0.2, 0.5, 0.8];
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
        'group flex h-full min-h-[244px] flex-col rounded-xl border border-border bg-card p-3 shadow-sm shadow-black/5',
        onOpen && 'cursor-pointer transition-colors hover:border-primary/35 hover:bg-primary/5 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/30',
      )}
    >
      <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <div className="text-xs font-semibold text-foreground">温度与风扇历史</div>
          <span className="rounded-full border border-border/70 bg-background/70 px-2 py-0.5 text-[10px] text-muted-foreground">{sourceLabel}</span>
          {onOpen && (
            <span className="inline-flex items-center gap-1 text-[11px] font-medium text-primary opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100">
              详情
              <ArrowUpRight className="h-3 w-3" />
            </span>
          )}
        </div>
      </div>

      <div className="flex min-h-[168px] flex-1 overflow-hidden rounded-lg bg-muted/25 p-2.5">
        {points.length === 0 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {enabled ? '等待温度数据' : '后台记录已关闭，当前展示本次打开后的临时历史。'}
          </div>
        ) : points.length < 2 ? (
          <div className="flex h-full w-full items-center justify-center text-center text-[11px] leading-relaxed text-muted-foreground">
            {source === 'core' ? '已记录 1 条后台样本，等待更多数据。' : '已记录 1 条会话样本，等待更多数据。'}
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
            {fanPath && <path d={fanPath} fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" strokeDasharray="4 3" />}
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
  fanData,
  temperature,
  config,
  onConnect,
  onDisconnect,
  onConfigChange,
      onOpenCurveEditor,
      onOpenHistoryDetails,
}: DeviceStatusProps) {
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
  const deviceModelName = isBs1Model ? 'BS1' : isBs3ProModel ? 'BS3 PRO' : isBs3Model ? 'BS3' : isBs2ProModel ? 'BS2 PRO' : isBs2Model ? 'BS2' : '未知设备';
  const deviceImageSrc = isBs1Model ? '/bs2.png' : isBs2Model ? '/bs2.png' : '/bs2pro.png';
  const modeTitle = config.autoControl ? '智能控制' : config.customSpeedEnabled ? '固定转速' : '手动策略';
  const modeDesc = config.autoControl
    ? '根据实时温度自动调节转速'
    : config.customSpeedEnabled
      ? `当前固定为 ${config.customSpeedRPM || fanData?.currentRpm || '--'} RPM`
      : '可在设置页调整模式与参数';
  const modeDisplayTitle = activeCurveProfileName ? `${modeTitle}（${activeCurveProfileName}）` : modeTitle;
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);
  const maxRpmInfo = getReportedMaxRpm(fanData?.gearSettings, fanData?.maxGear);
  const maxGearHighLevelRpm = maxRpmInfo.rpm;
  const bridgeStateLabel = bridgeStatus?.state === 'running_owned'
    ? '独占运行'
    : bridgeStatus?.state === 'attached'
      ? '附着共享实例'
      : bridgeStatus?.state === 'starting'
        ? '启动中'
        : bridgeStatus?.state === 'degraded'
          ? '降级'
          : bridgeStatus?.state === 'failed'
            ? '失败'
            : bridgeStatus?.state === 'stopping'
              ? '停止中'
              : bridgeStatus?.state === 'stopped'
                ? '已停止'
                : bridgeStatus?.state === 'not_started'
                  ? '未启动'
                  : '';
  const maxRpmHint = isBs1Model
    ? 'BS1 最高可达超频挡 3000 RPM。'
    : maxGearHighLevelRpm === 4000
      ? '当前已解锁超频上限，最高可达 4000 RPM。'
      : maxGearHighLevelRpm === 3300
        ? '当前最高为强劲档，最高可达 3300 RPM，使用PD 27W充电头以解锁上限。'
        : maxGearHighLevelRpm === 2760
          ? '当前最高为标准档，最高可达 2760 RPM，使用PD 27W充电头以解锁上限。'
          : maxRpmInfo.codeHex
            ? `设备上报了未映射的最高挡位编码：${maxRpmInfo.codeHex}`
            : '等待设备上报最高转速能力。';
  const maxTempStatus = getTempStatus(temperature?.maxTemp || 0);

  return (
    <div className="space-y-3">
      {/* ── Device header card ── */}
      <div className="relative overflow-hidden rounded-xl border border-border bg-card p-4 shadow-sm shadow-black/5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div
              className="flex h-14 w-20 items-center justify-center overflow-hidden rounded-xl bg-muted/45 p-1.5"
            >
              <img
                src={deviceImageSrc}
                alt={`${deviceModelName} device`}
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
                  {isConnected ? '已连接' : '离线'}
                </span>
              </div>
              {isConnected && (
                <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                  {config.autoControl ? (
                    <Zap className="h-3 w-3 text-primary" />
                  ) : (
                    <Settings className="h-3 w-3" />
                  )}
                  <span>{modeTitle} · {modeDesc}</span>
                </div>
              )}
              {!isConnected && <p className="mt-1 text-xs text-muted-foreground">等待蓝牙连接…</p>}
            </div>
          </div>

          <div className="flex items-center gap-3">
            {isConnected && (
              <ToggleSwitch
                enabled={config.autoControl}
                onChange={handleAutoControlChange}
                label="智能变频"
                size="md"
                color="blue"
              />
            )}
            <Button
              variant={isConnected ? 'secondary' : 'primary'}
              size="sm"
              onClick={isConnected ? onDisconnect : onConnect}
            >
              {isConnected ? '断开' : '连接'}
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
          <div className="flex h-full min-h-[168px] flex-col items-center rounded-xl border border-border bg-card px-5 py-5 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[184px]">
            <MetricHeader
              icon={<Cpu className="h-4 w-4" />}
              label="CPU"
            />
            <CpuTempDisplay temp={temperature?.cpuTemp} />
          </div>

          {/* GPU */}
          <div className="flex h-full min-h-[168px] flex-col items-center rounded-xl border border-border bg-card px-5 py-5 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[184px]">
            <MetricHeader
              icon={<Gpu className="h-4 w-4" />}
              label="GPU"
            />
            <GpuTempDisplay temp={temperature?.gpuTemp} />
          </div>

          {/* Fan */}
          <div className="flex h-full min-h-[168px] flex-col items-center rounded-xl border border-border bg-card px-5 py-5 shadow-sm shadow-black/5 transition-shadow hover:shadow-md hover:shadow-primary/10 md:min-h-[184px]">
            <MetricHeader
              icon={(
                <SpinningFanIcon duration={fanSpinDuration} className="h-4 w-4" />
              )}
              label="风扇"
            />
            <FanRpmDisplay
              currentRpm={fanData?.currentRpm}
              targetRpm={fanData?.targetRpm}
              setGear={fanData?.setGear}
              isBs1={isBs1Model}
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
          <h3 className="mb-1.5 text-lg font-semibold">设备未连接</h3>
          <p className="mb-5 text-base text-muted-foreground">请将散热器通过蓝牙连接到电脑</p>
          <Button onClick={onConnect} size="md" icon={<RotateCw className="h-4 w-4" />}>
            连接设备
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
                <p>{temperature?.bridgeMessage || '温度桥接程序读取失败，可重新初始化温度监控后重试。'}</p>
                {bridgeStatus && (
                  <div className="mt-2 space-y-1 text-xs text-amber-700/90 dark:text-amber-200/80">
                    {bridgeStateLabel && (
                      <p>
                        桥接状态：{bridgeStateLabel}
                        {typeof bridgeStatus.ownsProcess === 'boolean' ? ` · ${bridgeStatus.ownsProcess ? '当前实例已接管进程' : '当前实例复用共享进程'}` : ''}
                      </p>
                    )}
                    {bridgeStatus.pipeName && <p>命名管道：{bridgeStatus.pipeName}</p>}
                    {bridgeStatus.lastError && bridgeStatus.lastError !== temperature?.bridgeMessage && <p>诊断信息：{bridgeStatus.lastError}</p>}
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
                  重新初始化温度监控
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
          className="rounded-xl border border-border bg-card p-3 shadow-sm shadow-black/5"
        >
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2 px-1">
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-muted-foreground" />
              <h3 className="text-xs font-semibold text-muted-foreground">
                控制与保护
              </h3>
            </div>
            <HardwareIdentitySummary cpuModel={temperature?.cpuModel} gpuModel={temperature?.gpuModel} />
          </div>

          <div className="grid grid-cols-2 gap-2.5 md:grid-cols-4">
            <div className="rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                控制模式
              </div>
              <div className={clsx('text-sm font-semibold', config.autoControl ? 'text-primary' : 'text-amber-600 dark:text-amber-400')}>
                {modeDisplayTitle}
              </div>
            </div>

            <div className="group rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Power className="h-3.5 w-3.5" />
                  最高转速
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="inline-flex h-4 w-4 items-center justify-center rounded text-muted-foreground/80 opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
                      aria-label="最高转速提示"
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

            <div className="rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Fan className="h-3.5 w-3.5" />
                工作模式
              </div>
              <div className="text-sm font-semibold">{fanData?.workMode || '--'}</div>
            </div>

            <div className="rounded-xl border border-border bg-background/55 p-3">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                温度状态
              </div>
              <div className={clsx('text-sm font-semibold tabular-nums', maxTempStatus.color)}>
                {maxTempStatus.label}
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
          <MiniFanCurveChart curve={config.fanCurve} currentTemp={temperature?.maxTemp} onOpen={onOpenCurveEditor} />
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

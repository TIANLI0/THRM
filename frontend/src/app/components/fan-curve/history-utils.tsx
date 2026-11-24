'use client';

import type { TFunction } from 'i18next';
import type { TimelineEvent } from '../../store/app-store';
import { type HistoryChartPoint, type HistorySeriesKey, type TemperatureHistoryPoint } from '../../lib/temperature-history';

export function formatHistoryTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatHistoryDateTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export interface HistorySeriesMetaItem {
  key: HistorySeriesKey;
  label: string;
  color: string;
}

const HISTORY_SERIES_FIELD: Record<HistorySeriesKey, keyof TemperatureHistoryPoint> = {
  cpu: 'cpuTemp',
  gpu: 'gpuTemp',
  fan: 'fanRpm',
  cpuFan: 'cpuFanRpm',
  gpuFan: 'gpuFanRpm',
  cpuPower: 'cpuPower',
  gpuPower: 'gpuPower',
};

function formatHistorySeriesValue(key: HistorySeriesKey, value: number) {
  if (key === 'fan' || key === 'cpuFan' || key === 'gpuFan') return `${Math.round(value)} RPM`;
  if (key === 'cpuPower' || key === 'gpuPower') return `${value.toFixed(1)} W`;
  return `${Math.round(value)} °C`;
}

export type HistoryNumericField = 'cpuTemp' | 'gpuTemp' | 'fanRpm' | 'cpuFanRpm' | 'gpuFanRpm' | 'cpuPower' | 'gpuPower';
// 趋势图渲染前抽稀到的上限点数（详见 downsampleHistoryPoints）。
export const HISTORY_RAW_CAP = 1500;

export type PositiveStats = { peak: number; sum: number; count: number };
type HistoryStats = Record<HistoryNumericField, PositiveStats> & { tempLowest: number };

const HISTORY_NUMERIC_FIELDS: HistoryNumericField[] = ['cpuTemp', 'gpuTemp', 'fanRpm', 'cpuFanRpm', 'gpuFanRpm', 'cpuPower', 'gpuPower'];

// 一遍扫完所有字段的有效（> 0）样本：峰值、累加和、个数，外加 CPU/GPU 温度的最低值。
// 24 小时窗口约 1.7 万点、每 5 秒一次，原先每个字段各扫一遍。
// 每个字段的累加顺序仍与逐点 reduce 相同，平均值逐位一致。
export function summarizeHistoryPoints(points: TemperatureHistoryPoint[]): HistoryStats {
  const stats = { tempLowest: Infinity } as HistoryStats;
  for (const field of HISTORY_NUMERIC_FIELDS) stats[field] = { peak: 0, sum: 0, count: 0 };
  for (const point of points) {
    for (const field of HISTORY_NUMERIC_FIELDS) {
      const value = point[field];
      if (value > 0) {
        const entry = stats[field];
        if (value > entry.peak) entry.peak = value;
        entry.sum += value;
        entry.count++;
      }
    }
    if (point.cpuTemp > 0 && point.cpuTemp < stats.tempLowest) stats.tempLowest = point.cpuTemp;
    if (point.gpuTemp > 0 && point.gpuTemp < stats.tempLowest) stats.tempLowest = point.gpuTemp;
  }
  return stats;
}

// 时间轴标记最多同屏显示的条数：错行布局只有 4 行，再多就互相压字看不清了。
export const TIMELINE_MARKER_CAP = 12;
// 超出上限时的取舍优先级，数字大的先保留。
export const TIMELINE_MARKER_PRIORITY: Record<TimelineEvent['type'], number> = {
  disconnect: 3,
  resume: 2,
  profile: 1,
  mode: 0,
};

// 每张趋势图使用各自的 tooltip：温度/风扇图只列温度与转速，功耗图只列功耗，避免互相堆叠遮挡。
// 仅展示当前已开启且有数据的曲线。
export function HistoryTrendTooltip(props: {
  active?: boolean;
  label?: string | number;
  // 记录中断处的数据点各系列为 null，下面按 `?? 0` 过滤掉，悬停空窗不弹提示框。
  payload?: Array<{ payload?: HistoryChartPoint }>;
  locale: string;
  meta: HistorySeriesMetaItem[];
  visibility: Record<HistorySeriesKey, boolean>;
  keys: HistorySeriesKey[];
}) {
  const { active, label, payload, locale, meta, visibility, keys } = props;
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;

  const allow = new Set(keys);
  const rows = meta
    .filter((series) => allow.has(series.key) && visibility[series.key])
    .map((series) => ({ series, value: Number(point[HISTORY_SERIES_FIELD[series.key]] ?? 0) }))
    .filter((row) => row.value > 0);
  if (rows.length === 0) return null;

  return (
    <div
      style={{
        backgroundColor: 'var(--chart-tooltip-bg)',
        border: '1px solid var(--chart-tooltip-border)',
        borderRadius: 10,
        boxShadow: 'var(--chart-tooltip-shadow)',
        padding: '8px 12px',
        color: 'var(--chart-tooltip-text)',
        backdropFilter: 'blur(16px) saturate(180%)',
        WebkitBackdropFilter: 'blur(16px) saturate(180%)',
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 4 }}>{formatHistoryDateTime(Number(label), locale)}</div>
      {rows.map(({ series, value }) => (
        <div key={series.key} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, lineHeight: '18px' }}>
          <span style={{ width: 8, height: 8, borderRadius: 9999, backgroundColor: series.color, display: 'inline-block' }} />
          <span>{series.label}</span>
          <span style={{ marginLeft: 'auto', fontWeight: 600 }}>{formatHistorySeriesValue(series.key, value)}</span>
        </div>
      ))}
    </div>
  );
}

export function formatHistoryDuration(
  startTimestamp: number,
  endTimestamp: number,
  t: TFunction,
) {
  const durationMs = Math.max(0, endTimestamp - startTimestamp);
  if (durationMs < 60_000) {
    return t('fanCurve.history.duration.ltOneMinute');
  }
  const totalMinutes = Math.round(durationMs / 60_000);
  if (totalMinutes < 60) {
    return t('fanCurve.history.duration.minutes', { count: totalMinutes });
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes > 0
    ? t('fanCurve.history.duration.hoursAndMinutes', { hours, minutes })
    : t('fanCurve.history.duration.hours', { hours });
}

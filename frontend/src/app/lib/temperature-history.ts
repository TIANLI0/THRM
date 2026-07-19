export interface TemperatureHistoryPoint {
  timestamp: number;
  cpuTemp: number;
  gpuTemp: number;
  cpuPower: number;
  gpuPower: number;
  fanRpm: number;
  /** 笔记本内置 CPU/GPU 风扇转速；0 表示本机不支持读取 */
  cpuFanRpm: number;
  gpuFanRpm: number;
}

export type HistorySeriesKey = 'cpu' | 'gpu' | 'fan' | 'cpuFan' | 'gpuFan' | 'cpuPower' | 'gpuPower';

export const CORE_HISTORY_LIMIT = 720;
export const SESSION_HISTORY_LIMIT = 60;
export const CORE_HISTORY_RETENTION_MS = 60 * 60 * 1000;
export const SESSION_HISTORY_RETENTION_MS = 5 * 60 * 1000;
export const HISTORY_SAMPLE_INTERVAL_MS = 5 * 1000;
export const HISTORY_LIMIT = CORE_HISTORY_LIMIT;
export const DEFAULT_HISTORY_RETENTION_HOURS = 1;
export const MAX_HISTORY_RETENTION_HOURS = 24;
export const HISTORY_RETENTION_HOUR_OPTIONS = [1, 2, 3, 6, 12, 24] as const;

export const clampHistoryRetentionHours = (hours: number | null | undefined): number => {
  const numeric = Math.round(Number(hours || 0));
  if (!Number.isFinite(numeric) || numeric < 1) return DEFAULT_HISTORY_RETENTION_HOURS;
  if (numeric > MAX_HISTORY_RETENTION_HOURS) return MAX_HISTORY_RETENTION_HOURS;
  return numeric;
};

export const normalizeHistoryTimestamp = (timestamp: number | null | undefined) => {
  const numeric = Number(timestamp || 0);
  if (numeric <= 0) return 0;
  if (numeric < 1_000_000_000_000) {
    return numeric * 1000;
  }
  return numeric;
};

export const normalizeHistoryPoint = (point: Partial<TemperatureHistoryPoint> | null | undefined): TemperatureHistoryPoint | null => {
  if (!point) return null;

  const timestamp = normalizeHistoryTimestamp(Number(point.timestamp || 0));
  const cpuTemp = Number(point.cpuTemp || 0);
  const gpuTemp = Number(point.gpuTemp || 0);
  const cpuPower = Number(point.cpuPower || 0);
  const gpuPower = Number(point.gpuPower || 0);
  const fanRpm = Number(point.fanRpm || 0);
  const cpuFanRpm = Number(point.cpuFanRpm || 0);
  const gpuFanRpm = Number(point.gpuFanRpm || 0);

  if (timestamp <= 0 || (cpuTemp <= 0 && gpuTemp <= 0 && fanRpm <= 0)) {
    return null;
  }

  return {
    timestamp,
    cpuTemp,
    gpuTemp,
    cpuPower: Number.isFinite(cpuPower) && cpuPower > 0 ? cpuPower : 0,
    gpuPower: Number.isFinite(gpuPower) && gpuPower > 0 ? gpuPower : 0,
    fanRpm,
    cpuFanRpm: Number.isFinite(cpuFanRpm) && cpuFanRpm > 0 ? cpuFanRpm : 0,
    gpuFanRpm: Number.isFinite(gpuFanRpm) && gpuFanRpm > 0 ? gpuFanRpm : 0,
  };
};

export const trimHistoryPoints = (
  points: TemperatureHistoryPoint[] | undefined,
  retentionMs = CORE_HISTORY_RETENTION_MS,
  limit = HISTORY_LIMIT,
) => {
  if (!Array.isArray(points)) return [];

  const normalized = points
    .map((point) => normalizeHistoryPoint(point))
    .filter((point): point is TemperatureHistoryPoint => !!point)
    .sort((a, b) => a.timestamp - b.timestamp);

  if (normalized.length === 0) {
    return [];
  }

  const newestTimestamp = normalized[normalized.length - 1]?.timestamp || 0;
  const cutoffTimestamp = newestTimestamp > 0 ? Math.max(0, newestTimestamp - retentionMs) : 0;

  return normalized
    .filter((point) => point.timestamp >= cutoffTimestamp)
    .slice(-limit);
};

export const normalizeHistoryPoints = (points: TemperatureHistoryPoint[] | undefined) => {
  return trimHistoryPoints(points, CORE_HISTORY_RETENTION_MS, CORE_HISTORY_LIMIT);
};

export const appendHistoryPoint = (
  points: TemperatureHistoryPoint[],
  point: TemperatureHistoryPoint | null,
  options?: { retentionMs?: number; limit?: number },
) => {
  const normalized = normalizeHistoryPoint(point);
  if (!normalized) return points;

  const retentionMs = options?.retentionMs ?? CORE_HISTORY_RETENTION_MS;
  const limit = options?.limit ?? HISTORY_LIMIT;
  const last = points[points.length - 1];

  // 乱序追加是异常路径，才需要全量归一化 + 排序。
  if (last && normalized.timestamp < last.timestamp) {
    return trimHistoryPoints([...points, normalized], retentionMs, limit);
  }

  // 快路径：数组本身已按时间有序且逐点归一化过，直接追加/替换末尾，再从头部裁剪过期点。
  const next = last && last.timestamp === normalized.timestamp
    ? [...points.slice(0, -1), normalized]
    : [...points, normalized];

  const cutoffTimestamp = Math.max(0, normalized.timestamp - retentionMs);
  let start = 0;
  while (start < next.length && next[start].timestamp < cutoffTimestamp) start++;
  if (next.length - start > limit) start = next.length - limit;
  return start > 0 ? next.slice(start) : next;
};

export const appendSampledHistoryPoint = (
  points: TemperatureHistoryPoint[],
  point: TemperatureHistoryPoint | null,
  options?: { retentionMs?: number; limit?: number; minIntervalMs?: number },
) => {
  const normalized = normalizeHistoryPoint(point);
  if (!normalized) return points;

  const last = points[points.length - 1];
  const minIntervalMs = options?.minIntervalMs ?? HISTORY_SAMPLE_INTERVAL_MS;
  if (last && normalized.timestamp-last.timestamp < minIntervalMs) {
    return points;
  }

  return appendHistoryPoint(points, normalized, options);
};

// 趋势图渲染前的等距抽稀：长时间窗口（24h ≈ 1.7 万点）远超图表像素宽度，
// 直接绘制既慢又卡。不做平均，保留原始采样值；始终保留最后一个点。
export const downsampleHistoryPoints = (
  points: TemperatureHistoryPoint[],
  maxPoints: number,
): TemperatureHistoryPoint[] => {
  if (maxPoints <= 0 || points.length <= maxPoints) return points;
  const stride = Math.ceil(points.length / maxPoints);
  const out: TemperatureHistoryPoint[] = [];
  for (let i = 0; i < points.length; i += stride) out.push(points[i]);
  const last = points[points.length - 1];
  if (out[out.length - 1] !== last) out.push(last);
  return out;
};

export const createLiveHistoryPoint = (
  payload: { updateTime?: number; cpuTemp?: number; gpuTemp?: number; cpuPower?: number; gpuPower?: number; cpuFanRpm?: number; gpuFanRpm?: number } | null | undefined,
  fanRpm = 0,
) => {
  if (!payload) return null;

  return normalizeHistoryPoint({
    timestamp: normalizeHistoryTimestamp(payload.updateTime ?? 0) || Date.now(),
    cpuTemp: Number(payload.cpuTemp || 0),
    gpuTemp: Number(payload.gpuTemp || 0),
    cpuPower: Number(payload.cpuPower || 0),
    gpuPower: Number(payload.gpuPower || 0),
    fanRpm: Number(fanRpm || 0),
    cpuFanRpm: Number(payload.cpuFanRpm || 0),
    gpuFanRpm: Number(payload.gpuFanRpm || 0),
  });
};

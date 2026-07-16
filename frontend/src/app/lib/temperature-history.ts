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

  const next = [...points];
  const last = next[next.length - 1];

  if (last && last.timestamp === normalized.timestamp) {
    next[next.length - 1] = normalized;
  } else if (last && normalized.timestamp < last.timestamp) {
    return normalizeHistoryPoints([...next, normalized]);
  } else {
    next.push(normalized);
  }

  return trimHistoryPoints(next, options?.retentionMs ?? CORE_HISTORY_RETENTION_MS, options?.limit ?? HISTORY_LIMIT);
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

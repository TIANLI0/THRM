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

export type TimelineEventType = 'mode' | 'disconnect' | 'resume' | 'profile';

/** 趋势图上标注的一次状态变化。由核心记录并跨进程重启持久化，`labelKey` 是 i18n 键。 */
export interface TimelineEvent {
  timestamp: number;
  type: TimelineEventType;
  labelKey: string;
}

export const TIMELINE_EVENT_LIMIT = 240;

const TIMELINE_EVENT_TYPES: readonly TimelineEventType[] = ['mode', 'disconnect', 'resume', 'profile'];

export const normalizeTimelineEvent = (event: Partial<TimelineEvent> | null | undefined): TimelineEvent | null => {
  if (!event) return null;
  const timestamp = normalizeHistoryTimestamp(Number(event.timestamp || 0));
  const labelKey = typeof event.labelKey === 'string' ? event.labelKey : '';
  if (timestamp <= 0 || !labelKey) return null;
  const type = TIMELINE_EVENT_TYPES.includes(event.type as TimelineEventType)
    ? (event.type as TimelineEventType)
    : 'mode';
  return { timestamp, type, labelKey };
};

const timelineEventId = (event: TimelineEvent) => `${event.timestamp}|${event.type}|${event.labelKey}`;

/**
 * 合并事件列表并按时间排序。
 *
 * 快照与实时推送会重叠——打开界面时先收到几条实时事件，随后拉回来的快照里也含有
 * 同样的事件，直接拼接会在图上叠出两条重合的参考线，所以按 时间+类型+文案 去重。
 */
export const mergeTimelineEvents = (...lists: Array<TimelineEvent[] | null | undefined>): TimelineEvent[] => {
  const byId = new Map<string, TimelineEvent>();
  for (const list of lists) {
    for (const raw of list || []) {
      const event = normalizeTimelineEvent(raw);
      if (event) byId.set(timelineEventId(event), event);
    }
  }
  return Array.from(byId.values())
    .sort((a, b) => a.timestamp - b.timestamp)
    .slice(-TIMELINE_EVENT_LIMIT);
};

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

// 采样间隔达到"常态节奏"的这个倍数就认为记录中断过。
const HISTORY_GAP_FACTOR = 3;
// 中断判定的绝对下限：短于此值的抖动（一次桥接慢读、一次配置保存）不算中断。
const HISTORY_GAP_FLOOR_MS = 30 * 1000;

/** 一段没有任何采样的时间窗口。 */
export interface HistoryGap {
  start: number;
  end: number;
}

/** 折线图的数据点。断点处各系列取 null，让 Recharts 断开而不是连成直线。 */
export type HistoryChartPoint = {
  timestamp: number;
} & {
  [K in Exclude<keyof TemperatureHistoryPoint, 'timestamp'>]: number | null;
};

const median = (values: number[]) => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0 ? (sorted[mid - 1] + sorted[mid]) / 2 : sorted[mid];
};

/**
 * 中断判定阈值取"相邻间隔中位数"的倍数，而不是固定的采样间隔。
 *
 * 核心在空闲（无界面且未开智能控温）时会把采样从 5 秒放慢到 10 秒，用固定阈值会把
 * 这种正常的降频误判成中断；用中位数则自动跟随当前实际节奏。
 */
export const historyGapThresholdMs = (points: TemperatureHistoryPoint[], sampleIntervalMs = HISTORY_SAMPLE_INTERVAL_MS) => {
  const deltas: number[] = [];
  for (let i = 1; i < points.length; i++) {
    const delta = points[i].timestamp - points[i - 1].timestamp;
    if (delta > 0) deltas.push(delta);
  }
  const cadence = Math.max(median(deltas), sampleIntervalMs);
  return Math.max(cadence * HISTORY_GAP_FACTOR, HISTORY_GAP_FLOOR_MS);
};

/** 找出记录中断的时间窗口。 */
export const findHistoryGaps = (
  points: TemperatureHistoryPoint[],
  thresholdMs: number,
): HistoryGap[] => {
  const gaps: HistoryGap[] = [];
  for (let i = 1; i < points.length; i++) {
    const start = points[i - 1].timestamp;
    const end = points[i].timestamp;
    if (end - start > thresholdMs) gaps.push({ start, end });
  }
  return gaps;
};

/**
 * 在每段中断中间插入一个全 null 的点，使折线在此断开。
 *
 * 不这样做的话 Recharts 会把中断两侧直接连成一条斜线，用户完全看不出这段时间
 * 其实压根没有数据——机器睡过一觉、核心重启过，图上却是一条平滑的曲线。
 */
export const insertHistoryGapBreaks = (
  points: TemperatureHistoryPoint[],
  gaps: HistoryGap[],
): HistoryChartPoint[] => {
  if (gaps.length === 0) return points as HistoryChartPoint[];

  const breakAt = new Map(gaps.map((gap) => [gap.start, gap]));
  const out: HistoryChartPoint[] = [];
  for (const point of points) {
    out.push(point as HistoryChartPoint);
    const gap = breakAt.get(point.timestamp);
    if (!gap) continue;
    out.push({
      timestamp: Math.round((gap.start + gap.end) / 2),
      cpuTemp: null,
      gpuTemp: null,
      cpuPower: null,
      gpuPower: null,
      fanRpm: null,
      cpuFanRpm: null,
      gpuFanRpm: null,
    });
  }
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

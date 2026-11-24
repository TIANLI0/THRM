'use client';

import React, { useState, useCallback, memo, useMemo } from 'react';
import { History } from 'lucide-react';
import type { TimelineEvent } from '../../store/app-store';
import { useTemperatureHistory } from '../../hooks/useTemperatureHistory';
import { useLocale } from '../../lib/i18n';
import {
  HISTORY_RETENTION_HOUR_OPTIONS,
  downsampleHistoryPoints,
  findHistoryGaps,
  historyGapThresholdMs,
  insertHistoryGapBreaks,
  type HistorySeriesKey,
} from '../../lib/temperature-history';
import { useTranslation } from 'react-i18next';
import { ToggleSwitch, Select } from '../ui/index';
import clsx from 'clsx';
import {
  formatHistoryDateTime,
  HISTORY_RAW_CAP,
  type PositiveStats,
  summarizeHistoryPoints,
  TIMELINE_MARKER_CAP,
  TIMELINE_MARKER_PRIORITY,
  formatHistoryDuration,
} from './history-utils';
import { type HistoryLineSpec, HistoryTempChart, HistoryPowerChart } from './HistoryCharts';

/* ── 历史趋势 ──

历史每 5 秒追加一个点，24 小时窗口下统计要扫约 1.7 万点。整段（订阅、缩放、统计、两张图）
放进独立组件：新点到达只重渲染这一节，曲线编辑器、方案、对话框都不跟着动。 */

export const HistoryDetailsSection = memo(function HistoryDetailsSection({
  sectionRef,
  timelineEvents,
}: {
  sectionRef: React.RefObject<HTMLElement | null>;
  timelineEvents: TimelineEvent[];
}) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [historySeriesVisibility, setHistorySeriesVisibility] = useState<Record<HistorySeriesKey, boolean>>({
    cpu: true,
    gpu: true,
    fan: true,
    cpuFan: true,
    gpuFan: true,
    cpuPower: true,
    gpuPower: true,
  });
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    saving: temperatureHistorySaving,
    setEnabled: setTemperatureHistoryEnabled,
    retentionHours: historyRetentionHours,
    setRetentionHours: setHistoryRetentionHours,
  } = useTemperatureHistory();

  // 保留时长可配置（最多 24h），全窗口用于域计算与汇总；实际渲染点数由下方降采样控制。
  const historyStats = useMemo(() => summarizeHistoryPoints(temperatureHistory), [temperatureHistory]);

  const historyTempDomain = useMemo<[number, number]>(() => {
    const { cpuTemp, gpuTemp, tempLowest } = historyStats;
    if (cpuTemp.count + gpuTemp.count === 0) {
      return [30, 90];
    }
    const highest = Math.max(cpuTemp.peak, gpuTemp.peak);
    const min = Math.max(0, Math.floor((tempLowest - 4) / 5) * 5);
    const max = Math.min(110, Math.ceil((highest + 4) / 5) * 5);
    return [min, Math.max(min + 10, max)];
  }, [historyStats]);

  // 上取整到 500 的整数倍，让右轴刻度落在整齐数值（如 4500）而非原始峰值 4114。
  const historyFanPeak = Math.max(historyStats.fanRpm.peak, historyStats.cpuFanRpm.peak, historyStats.gpuFanRpm.peak);
  const historyFanMax = Math.max(4000, Math.ceil((historyFanPeak + 200) / 500) * 500);

  // 笔记本内置风扇转速仅部分机型（Uniwill/同方准系统）可读；无任何有效样本时整组曲线与图例隐藏。
  const hasLaptopFanHistory = historyStats.cpuFanRpm.count > 0 || historyStats.gpuFanRpm.count > 0;

  const historyPowerPeak = Math.max(historyStats.cpuPower.peak, historyStats.gpuPower.peak);
  const historyPowerMax = historyPowerPeak <= 0 ? 0 : Math.max(20, Math.ceil((historyPowerPeak + 10) / 10) * 10);

  const historySummary = useMemo(() => {
    const latest = temperatureHistory[temperatureHistory.length - 1] ?? null;
    const first = temperatureHistory[0] ?? null;
    const { cpuTemp: cpu, gpuTemp: gpu, cpuPower, gpuPower, fanRpm: fan } = historyStats;
    const average = (stats: PositiveStats) => stats.count > 0 ? Math.round(stats.sum / stats.count) : 0;
    const averagePower = (stats: PositiveStats) => stats.count > 0 ? Math.round(stats.sum / stats.count * 10) / 10 : 0;

    return {
      sampleCount: temperatureHistory.length,
      latest,
      latestLabel: latest ? formatHistoryDateTime(latest.timestamp, locale) : '--',
      durationLabel: first && latest ? formatHistoryDuration(first.timestamp, latest.timestamp, t) : '--',
      cpuPeak: cpu.peak,
      cpuAverage: average(cpu),
      gpuPeak: gpu.peak,
      gpuAverage: average(gpu),
      cpuPowerPeak: cpuPower.peak,
      cpuPowerAverage: averagePower(cpuPower),
      gpuPowerPeak: gpuPower.peak,
      gpuPowerAverage: averagePower(gpuPower),
      fanPeak: fan.peak,
      fanAverage: average(fan),
    };
  }, [historyStats, locale, t, temperatureHistory]);
  const historyChartData = temperatureHistory;
  // 趋势图缩放区间（时间戳范围）；null 表示未缩放（跟随全部数据）。
  const [historyZoomDomain, setHistoryZoomDomain] = useState<[number, number] | null>(null);
  // 正在拖拽框选的临时区间，用于渲染半透明选区。
  const [historyZoomSelect, setHistoryZoomSelect] = useState<{ start: number; end: number } | null>(null);
  // 最近趋势 / 功耗趋势共用同一份裁切数据，天然保持缩放与指针联动。
  const zoomedHistoryChartData = useMemo(() => {
    if (!historyZoomDomain) return historyChartData;
    const [from, to] = historyZoomDomain;
    const sliced = historyChartData.filter((point) => point.timestamp >= from && point.timestamp <= to);
    return sliced.length >= 2 ? sliced : historyChartData;
  }, [historyChartData, historyZoomDomain]);
  // 渲染用数据：对可见窗口等距抽稀到上限点数。两图共用同一份，保持缩放与指针联动。
  //
  // 抽稀之后再找中断：抽稀本身会把相邻点的间隔按 stride 等比拉大，用原始数据的阈值
  // 会把每一段都判成中断；改用抽稀后自身的节奏做判据，缩放到哪一级就用哪一级的尺度。
  const { historyDisplayData, historyGaps } = useMemo(() => {
    const sampled = downsampleHistoryPoints(zoomedHistoryChartData, HISTORY_RAW_CAP);
    const gaps = findHistoryGaps(sampled, historyGapThresholdMs(sampled));
    return { historyDisplayData: insertHistoryGapBreaks(sampled, gaps), historyGaps: gaps };
  }, [zoomedHistoryChartData]);
  const handleHistoryZoomMouseDown = useCallback((state: { activeLabel?: string | number } | null) => {
    if (state?.activeLabel == null) return;
    const ts = Number(state.activeLabel);
    setHistoryZoomSelect({ start: ts, end: ts });
  }, []);
  const handleHistoryZoomMouseMove = useCallback((state: { activeLabel?: string | number } | null) => {
    if (state?.activeLabel == null) return;
    const ts = Number(state.activeLabel);
    setHistoryZoomSelect((prev) => (prev ? { ...prev, end: ts } : prev));
  }, []);
  const handleHistoryZoomMouseUp = useCallback(() => {
    setHistoryZoomSelect((prev) => {
      if (prev) {
        const from = Math.min(prev.start, prev.end);
        const to = Math.max(prev.start, prev.end);
        // 选区过窄（不足两个采样点）视为误触，忽略。
        if (historyChartData.filter((point) => point.timestamp >= from && point.timestamp <= to).length >= 2) {
          setHistoryZoomDomain([from, to]);
        }
      }
      return null;
    });
  }, [historyChartData]);
  const resetHistoryZoom = useCallback(() => {
    setHistoryZoomSelect(null);
    setHistoryZoomDomain(null);
  }, []);
  const visibleTimelineEvents = useMemo(() => {
    const first = zoomedHistoryChartData[0]?.timestamp ?? 0;
    const last = zoomedHistoryChartData[zoomedHistoryChartData.length - 1]?.timestamp ?? Number.MAX_SAFE_INTEGER;
    const inWindow = timelineEvents.filter((event) => event.timestamp >= first && event.timestamp <= last);
    if (inWindow.length <= TIMELINE_MARKER_CAP) return inWindow;
    // 超出可读上限时按重要性取舍，而不是简单地留最近的几条：断连和睡眠唤醒才是
    // 用户找的东西，几小时前的一次断连不该被后来一串"切换曲线方案"挤掉。
    return inWindow
      .map((event, index) => ({ event, index }))
      .sort((a, b) => (TIMELINE_MARKER_PRIORITY[b.event.type] - TIMELINE_MARKER_PRIORITY[a.event.type]) || (b.index - a.index))
      .slice(0, TIMELINE_MARKER_CAP)
      .sort((a, b) => a.index - b.index)
      .map(({ event }) => event);
  }, [zoomedHistoryChartData, timelineEvents]);
  // 为聚集在一起的时间线标记分配垂直行号，避免各条参考线的文字标签叠在同一处；
  // 同时按标记在时间轴的位置决定文字朝向，防止靠右的标记文字溢出图表。
  const timelineEventLayout = useMemo(() => {
    const first = zoomedHistoryChartData[0]?.timestamp ?? 0;
    const last = zoomedHistoryChartData[zoomedHistoryChartData.length - 1]?.timestamp ?? first;
    const range = Math.max(1, last - first);
    const minGap = range * 0.07; // 时间间隔小于可见跨度 7% 的标记视为“重叠”，需错行显示
    const maxRows = 4;
    const rowLastTs: number[] = [];
    return visibleTimelineEvents.map((event) => {
      let row = rowLastTs.findIndex((ts) => event.timestamp - ts >= minGap);
      if (row === -1) {
        row = rowLastTs.length < maxRows
          ? rowLastTs.length
          : rowLastTs.reduce((best, ts, i) => (ts < rowLastTs[best] ? i : best), 0);
      }
      rowLastTs[row] = event.timestamp;
      return { event, row, anchorEnd: (event.timestamp - first) / range > 0.55 };
    });
  }, [zoomedHistoryChartData, visibleTimelineEvents]);

  const historySeriesMeta = useMemo(() => ([
    { key: 'cpu' as const, label: t('fanCurve.history.series.cpu'), color: '#2f6df6' },
    { key: 'gpu' as const, label: t('fanCurve.history.series.gpu'), color: '#f97316' },
    { key: 'fan' as const, label: t('fanCurve.history.series.fan'), color: '#10b981' },
    ...(hasLaptopFanHistory ? [
      { key: 'cpuFan' as const, label: t('fanCurve.history.series.cpuFan'), color: '#0ea5e9' },
      { key: 'gpuFan' as const, label: t('fanCurve.history.series.gpuFan'), color: '#84cc16' },
    ] : []),
    { key: 'cpuPower' as const, label: t('fanCurve.history.series.cpuPower'), color: '#8b5cf6' },
    { key: 'gpuPower' as const, label: t('fanCurve.history.series.gpuPower'), color: '#ec4899' },
  ]), [t, locale, hasLaptopFanHistory]);

  const toggleHistorySeries = useCallback((series: HistorySeriesKey) => {
    setHistorySeriesVisibility((prev) => ({
      ...prev,
      [series]: !prev[series],
    }));
  }, []);

  const tempChartSeries = useMemo<HistoryLineSpec[]>(() => ([
    { key: 'cpu', dataKey: 'cpuTemp', axis: 'temp', color: '#2f6df6', width: 2.2 },
    { key: 'gpu', dataKey: 'gpuTemp', axis: 'temp', color: '#f97316', width: 2.2 },
    { key: 'fan', dataKey: 'fanRpm', axis: 'fan', color: '#10b981', width: 2 },
    ...(hasLaptopFanHistory ? [
      { key: 'cpuFan' as const, dataKey: 'cpuFanRpm' as const, axis: 'fan', color: '#0ea5e9', width: 1.4, dash: '5 4' },
      { key: 'gpuFan' as const, dataKey: 'gpuFanRpm' as const, axis: 'fan', color: '#84cc16', width: 1.4, dash: '5 4' },
    ] : []),
  ]), [hasLaptopFanHistory]);
  const powerChartSeries = useMemo<HistoryLineSpec[]>(() => ([
    { key: 'cpuPower', dataKey: 'cpuPower', axis: 'power', color: '#8b5cf6', width: 2.2 },
    { key: 'gpuPower', dataKey: 'gpuPower', axis: 'power', color: '#ec4899', width: 2.2 },
  ]), []);
  const tempChartKeys = useMemo<HistorySeriesKey[]>(() => tempChartSeries.map((s) => s.key), [tempChartSeries]);
  const powerChartKeys = useMemo<HistorySeriesKey[]>(() => powerChartSeries.map((s) => s.key), [powerChartSeries]);

  return (
    <section ref={sectionRef} className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <History className="h-4 w-4 text-primary" />
          <div>
            <div className="text-sm font-medium text-foreground">{t('fanCurve.history.detailsTitle')}</div>
            <div className="text-xs text-muted-foreground">{t('fanCurve.history.detailsDescription')}</div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ToggleSwitch
            enabled={temperatureHistoryEnabled}
            onChange={setTemperatureHistoryEnabled}
            loading={temperatureHistorySaving}
            label={temperatureHistorySaving ? t('fanCurve.history.saving') : t('fanCurve.history.backgroundRecording')}
            size="sm"
            color="blue"
          />
        </div>
      </div>

      {temperatureHistory.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/70 bg-background/35 px-4 py-8 text-center text-sm text-muted-foreground">
          {temperatureHistoryEnabled ? t('fanCurve.history.emptyEnabled') : t('fanCurve.history.emptyDisabled')}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-5">
            {[
              [t('fanCurve.history.summary.cpuPeak'), historySummary.cpuPeak ? `${historySummary.cpuPeak}°C` : '--', historySummary.cpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.cpuAverage }) : t('fanCurve.history.summary.noCpuTemperature')],
              [t('fanCurve.history.summary.gpuPeak'), historySummary.gpuPeak ? `${historySummary.gpuPeak}°C` : '--', historySummary.gpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.gpuAverage }) : t('fanCurve.history.summary.noGpuTemperature')],
              [t('fanCurve.history.summary.cpuPowerPeak'), historySummary.cpuPowerPeak ? `${historySummary.cpuPowerPeak.toFixed(1)} W` : '--', historySummary.cpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: historySummary.cpuPowerAverage.toFixed(1) }) : t('fanCurve.history.summary.noCpuPower')],
              [t('fanCurve.history.summary.gpuPowerPeak'), historySummary.gpuPowerPeak ? `${historySummary.gpuPowerPeak.toFixed(1)} W` : '--', historySummary.gpuPowerAverage ? t('fanCurve.history.summary.averagePower', { value: historySummary.gpuPowerAverage.toFixed(1) }) : t('fanCurve.history.summary.noGpuPower')],
              [t('fanCurve.history.summary.fanPeak'), historySummary.fanPeak ? `${historySummary.fanPeak} RPM` : '--', historySummary.fanAverage ? t('fanCurve.history.summary.averageFan', { value: historySummary.fanAverage }) : t('fanCurve.history.summary.noFanData')],
            ].map(([label, value, hint]) => (
              <div key={label} className="min-w-0 rounded-xl border border-border/70 bg-background/35 p-2.5">
                <div className="truncate text-[11px] text-muted-foreground" title={label}>{label}</div>
                <div className="mt-1 truncate text-sm font-semibold text-foreground" title={value}>{value}</div>
                <div className="mt-1 truncate text-[11px] text-muted-foreground" title={hint}>{hint}</div>
              </div>
            ))}
          </div>

          <div className="rounded-xl border border-border/70 bg-background/35 p-3 space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.history.recentTrend')}</div>
                {historyZoomDomain ? (
                  <button
                    type="button"
                    onClick={resetHistoryZoom}
                    className="cursor-pointer rounded-full border border-border/70 bg-card px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {t('fanCurve.history.resetZoom')}
                  </button>
                ) : (
                  <span className="text-[11px] text-muted-foreground/60">{t('fanCurve.history.zoomHint')}</span>
                )}
                <span className="mx-0.5 h-3 w-px bg-border/70" />
                <span className="text-[11px] text-muted-foreground">{t('fanCurve.history.retention.label')}</span>
                <Select
                  value={historyRetentionHours}
                  onChange={(value) => { void setHistoryRetentionHours(Number(value)); }}
                  disabled={temperatureHistorySaving}
                  size="sm"
                  className="min-w-23"
                  triggerClassName="h-7 rounded-full px-2.5 text-[11px]"
                  options={HISTORY_RETENTION_HOUR_OPTIONS.map((hours) => ({
                    value: hours,
                    label: t('fanCurve.history.retention.hours', { count: hours }),
                  }))}
                />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {historySeriesMeta.map((series) => (
                  <button
                    key={series.key}
                    type="button"
                    onClick={() => toggleHistorySeries(series.key)}
                    className={clsx(
                      'inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] transition-colors',
                      historySeriesVisibility[series.key]
                        ? 'border-border bg-card text-foreground'
                        : 'border-border/60 bg-transparent text-muted-foreground/65',
                    )}
                  >
                    <span className="h-2 w-2 rounded-full" style={{ backgroundColor: series.color }} />
                    {series.label}
                  </button>
                ))}
              </div>
            </div>

            {historyChartData.length < 2 ? (
              <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">{t('fanCurve.history.waitingMoreSamples')}</div>
            ) : (
              <div className="h-72 select-none">
                <HistoryTempChart
                  data={historyDisplayData}
                  gaps={historyGaps}
                  series={tempChartSeries}
                  seriesKeys={tempChartKeys}
                  seriesMeta={historySeriesMeta}
                  visibility={historySeriesVisibility}
                  zoomSelection={historyZoomSelect}
                  locale={locale}
                  onZoomMouseDown={handleHistoryZoomMouseDown}
                  onZoomMouseMove={handleHistoryZoomMouseMove}
                  onZoomMouseUp={handleHistoryZoomMouseUp}
                  onResetZoom={resetHistoryZoom}
                  tempDomain={historyTempDomain}
                  fanMax={historyFanMax}
                  timelineEventLayout={timelineEventLayout}
                />
              </div>
            )}

            {historyPowerMax > 0 && (historySeriesVisibility.cpuPower || historySeriesVisibility.gpuPower) && (
              <div className="border-t border-border/60 pt-3">
                <div className="mb-2 text-xs font-medium text-muted-foreground">{t('fanCurve.history.powerTrend')}</div>
                <div className="h-48 select-none">
                  <HistoryPowerChart
                    data={historyDisplayData}
                    gaps={historyGaps}
                    series={powerChartSeries}
                    seriesKeys={powerChartKeys}
                    seriesMeta={historySeriesMeta}
                    visibility={historySeriesVisibility}
                    zoomSelection={historyZoomSelect}
                    locale={locale}
                    onZoomMouseDown={handleHistoryZoomMouseDown}
                    onZoomMouseMove={handleHistoryZoomMouseMove}
                    onZoomMouseUp={handleHistoryZoomMouseUp}
                    onResetZoom={resetHistoryZoom}
                    powerMax={historyPowerMax}
                  />
                </div>
              </div>
            )}
          </div>
        </>
      )}
    </section>
  );
});

'use client';

import { useCallback, memo } from 'react';
import { ComposedChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer, ReferenceLine, ReferenceArea } from 'recharts';
import type { TimelineEvent } from '../../store/app-store';
import { type HistoryChartPoint, type HistoryGap, type HistorySeriesKey } from '../../lib/temperature-history';
import { useTranslation } from 'react-i18next';
import { formatHistoryTime, type HistorySeriesMetaItem, type HistoryNumericField, HistoryTrendTooltip } from './history-utils';

/* ── History trend charts ──

两张趋势图单独做成 memo 组件：曲线页会随当前温度、对话框开关、方案编辑等重渲染，
这些都与趋势图无关，却会让 Recharts 把最多 1500 点 × 5 条曲线整体重新布局一遍。
现在只有图表自己的数据、缩放选区、系列开关变化时才重绘。 */

// 各趋势图的曲线定义（数据键、所属 Y 轴、颜色、线宽、虚线）。本机内置风扇用更细的虚线弱化，突出主曲线。
export type HistoryLineSpec = { key: HistorySeriesKey; dataKey: HistoryNumericField; axis: string; color: string; width: number; dash?: string };
type HistoryZoomHandler = (state: { activeLabel?: string | number } | null) => void;
type HistoryZoomSelection = { start: number; end: number } | null;

// 点数超过此值改用折线：这么密的点上单调样条与折线在像素上看不出差别，样条却要多算一遍控制点。
const HISTORY_MONOTONE_MAX_POINTS = 500;

function renderHistoryGapAreas(gaps: HistoryGap[], yAxisId: 'temp' | 'power', label: string) {
  // 把每段没有采样的时间窗口画成一条灰色遮罩，并标出"记录中断"。
  // 只靠折线断开还不够：断口很窄时几乎看不见，遮罩才能让人一眼看出这段时间没数据。
  return gaps.map((gap) => (
    <ReferenceArea
      key={`gap-${gap.start}-${gap.end}`}
      yAxisId={yAxisId}
      x1={gap.start}
      x2={gap.end}
      fill="var(--chart-tick)"
      fillOpacity={0.12}
      strokeOpacity={0}
      label={{
        value: label,
        position: 'insideTop',
        fontSize: 10,
        fill: 'var(--chart-tick)',
      }}
    />
  ));
}

function renderHistoryLines(
  series: HistoryLineSpec[],
  visibility: Record<HistorySeriesKey, boolean>,
  pointCount: number,
) {
  const curveType = pointCount > HISTORY_MONOTONE_MAX_POINTS ? 'linear' : 'monotone';
  return series.flatMap((spec) => {
    if (!visibility[spec.key]) return [];
    return [
      <Line
        key={spec.key}
        yAxisId={spec.axis}
        type={curveType}
        dataKey={spec.dataKey}
        stroke={spec.color}
        strokeWidth={spec.width}
        strokeDasharray={spec.dash}
        dot={false}
        activeDot={false}
        isAnimationActive={false}
        // 记录中断处插了全 null 的点，必须断开而不是连成一条直线——
        // 连起来会让"睡了一小时"看上去像一段平缓的温度变化。
        connectNulls={false}
      />,
    ];
  });
}

function renderHistoryZoomSelection(selection: HistoryZoomSelection, yAxisId: 'temp' | 'power') {
  if (!selection || selection.start === selection.end) return null;
  return (
    <ReferenceArea
      yAxisId={yAxisId}
      x1={Math.min(selection.start, selection.end)}
      x2={Math.max(selection.start, selection.end)}
      fill="var(--chart-axis)"
      fillOpacity={0.15}
      strokeOpacity={0}
    />
  );
}

interface HistoryChartCommonProps {
  data: HistoryChartPoint[];
  gaps: HistoryGap[];
  series: HistoryLineSpec[];
  seriesKeys: HistorySeriesKey[];
  seriesMeta: HistorySeriesMetaItem[];
  visibility: Record<HistorySeriesKey, boolean>;
  zoomSelection: HistoryZoomSelection;
  locale: string;
  onZoomMouseDown: HistoryZoomHandler;
  onZoomMouseMove: HistoryZoomHandler;
  onZoomMouseUp: () => void;
  onResetZoom: () => void;
}

export const HistoryTempChart = memo(function HistoryTempChart({
  data,
  gaps,
  series,
  seriesKeys,
  seriesMeta,
  visibility,
  zoomSelection,
  locale,
  onZoomMouseDown,
  onZoomMouseMove,
  onZoomMouseUp,
  onResetZoom,
  tempDomain,
  fanMax,
  timelineEventLayout,
}: HistoryChartCommonProps & {
  tempDomain: [number, number];
  fanMax: number;
  timelineEventLayout: Array<{ event: TimelineEvent; row: number; anchorEnd: boolean }>;
}) {
  const { t } = useTranslation();
  const formatTick = useCallback((value: unknown) => formatHistoryTime(Number(value), locale), [locale]);

  return (
    <ResponsiveContainer width="100%" height="100%">
      <ComposedChart
        data={data}
        syncId="historyTrend"
        margin={{ top: 12, right: 16, left: 4, bottom: 8 }}
        onMouseDown={onZoomMouseDown}
        onMouseMove={onZoomMouseMove}
        onMouseUp={onZoomMouseUp}
        onMouseLeave={onZoomMouseUp}
        onDoubleClick={onResetZoom}
      >
        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
        <XAxis
          dataKey="timestamp"
          type="number"
          domain={['dataMin', 'dataMax']}
          tickFormatter={formatTick}
          tickLine={false}
          minTickGap={24}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
        />
        <YAxis
          yAxisId="temp"
          type="number"
          domain={tempDomain}
          tickLine={false}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
          width={48}
          unit="°C"
        />
        <YAxis
          yAxisId="fan"
          orientation="right"
          type="number"
          domain={[0, fanMax]}
          tickLine={false}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
          width={52}
        />
        <RechartsTooltip
          content={<HistoryTrendTooltip locale={locale} meta={seriesMeta} visibility={visibility} keys={seriesKeys} />}
        />
        {renderHistoryGapAreas(gaps, 'temp', t('fanCurve.history.recordingGap'))}
        {timelineEventLayout.map(({ event, row, anchorEnd }, index) => {
          const markerColor = event.type === 'disconnect' ? '#ef4444' : event.type === 'resume' ? '#8b5cf6' : '#0ea5e9';
          return (
            <ReferenceLine
              key={`${event.timestamp}-${event.type}-${index}`}
              x={event.timestamp}
              yAxisId="temp"
              stroke={markerColor}
              strokeDasharray="4 3"
              label={(labelProps: { viewBox?: { x?: number; y?: number } }) => {
                const x = labelProps.viewBox?.x ?? 0;
                const yTop = labelProps.viewBox?.y ?? 0;
                return (
                  <text
                    x={x}
                    y={yTop + 10 + row * 12}
                    dx={anchorEnd ? -5 : 5}
                    textAnchor={anchorEnd ? 'end' : 'start'}
                    fontSize={10}
                    fontWeight={500}
                    fill={markerColor}
                  >
                    {t(event.labelKey)}
                  </text>
                );
              }}
            />
          );
        })}
        {renderHistoryLines(series, visibility, data.length)}
        {renderHistoryZoomSelection(zoomSelection, 'temp')}
      </ComposedChart>
    </ResponsiveContainer>
  );
});

export const HistoryPowerChart = memo(function HistoryPowerChart({
  data,
  gaps,
  series,
  seriesKeys,
  seriesMeta,
  visibility,
  zoomSelection,
  locale,
  onZoomMouseDown,
  onZoomMouseMove,
  onZoomMouseUp,
  onResetZoom,
  powerMax,
}: HistoryChartCommonProps & { powerMax: number }) {
  const { t } = useTranslation();
  const formatTick = useCallback((value: unknown) => formatHistoryTime(Number(value), locale), [locale]);

  return (
    <ResponsiveContainer width="100%" height="100%">
      <ComposedChart
        data={data}
        syncId="historyTrend"
        margin={{ top: 8, right: 16, left: 4, bottom: 8 }}
        onMouseDown={onZoomMouseDown}
        onMouseMove={onZoomMouseMove}
        onMouseUp={onZoomMouseUp}
        onMouseLeave={onZoomMouseUp}
        onDoubleClick={onResetZoom}
      >
        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
        <XAxis
          dataKey="timestamp"
          type="number"
          domain={['dataMin', 'dataMax']}
          tickFormatter={formatTick}
          tickLine={false}
          minTickGap={24}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
        />
        <YAxis
          yAxisId="power"
          type="number"
          domain={[0, powerMax]}
          tickLine={false}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
          width={48}
          unit=" W"
        />
        <YAxis
          yAxisId="powerRight"
          orientation="right"
          type="number"
          domain={[0, powerMax]}
          tickLine={false}
          axisLine={{ stroke: 'var(--chart-axis)' }}
          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
          width={52}
          unit=" W"
        />
        <RechartsTooltip
          content={<HistoryTrendTooltip locale={locale} meta={seriesMeta} visibility={visibility} keys={seriesKeys} />}
        />
        {renderHistoryGapAreas(gaps, 'power', t('fanCurve.history.recordingGap'))}
        {renderHistoryLines(series, visibility, data.length)}
        {renderHistoryZoomSelection(zoomSelection, 'power')}
      </ComposedChart>
    </ResponsiveContainer>
  );
});

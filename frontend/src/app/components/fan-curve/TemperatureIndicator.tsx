'use client';

import React, { useState, useEffect, memo, useRef } from 'react';
import { useAppStore } from '../../store/app-store';
import * as types from '@go/types';
import { useTranslation } from 'react-i18next';

/* ── Temperature indicator overlay (memo, doesn't re-render chart) ── */

export type CurveTempSource = 'max' | 'cpu' | 'gpu';

// 参考温度跟随设置页"控温温度来源"(max/cpu/gpu)，该路无读数时回退到综合最高温。
const getCurveReferenceTemp = (temperature: types.TemperatureData | null, tempSource: CurveTempSource) => {
  if (!temperature) return null;
  const cpu = temperature.cpuTemp ?? 0;
  const gpu = temperature.gpuTemp ?? 0;
  const max = temperature.maxTemp ?? 0;
  if (tempSource === 'cpu') return cpu > 0 ? cpu : (max > 0 ? max : null);
  if (tempSource === 'gpu') return gpu > 0 ? gpu : (max > 0 ? max : null);
  return max > 0 ? max : null;
};

type IndicatorGeometry = { left: number; top: number; width: number; height: number };

export const TemperatureIndicator = memo(function TemperatureIndicator({
  tempSource,
  chartRef,
  temperatureRange,
}: {
  tempSource: CurveTempSource;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
}) {
  const { t } = useTranslation();
  // 参考温度在这里订阅：只有这条指示线随温度变化重渲染，曲线页主体不动。
  const temperature = useAppStore((state) => getCurveReferenceTemp(state.temperature, tempSource));
  // 网格区域相对图表容器的位置记在状态里，温度变化时只做一次换算。测量一律推迟到下一帧之后：
  // 那时页面已完成布局，读几何不会强制同步布局。
  const [geometry, setGeometry] = useState<IndicatorGeometry | null>(null);
  const measureRef = useRef<() => void>(() => {});

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;
    // recharts 的自适应容器在外层尺寸变化后才重绘网格，ResizeObserver 回调时网格往往还是旧的；
    // 只在回调里测会把旧尺寸缓存下来（改窗口大小后指示线错位、变短且不会自行恢复）。
    // 所以回调里只排期，隔两帧等重绘完成再测；温度变化时也会排期一次，兜住重绘更晚的情况。
    let wrapper: Element | null = null;
    let frame = 0;
    const measure = () => {
      const nextWrapper = chart.querySelector('.recharts-wrapper');
      if (nextWrapper !== wrapper) {
        if (wrapper) observer.unobserve(wrapper);
        wrapper = nextWrapper;
        if (wrapper) observer.observe(wrapper);
      }
      const grid = chart.querySelector('.recharts-cartesian-grid');
      const container = chart.querySelector('.recharts-responsive-container');
      if (!grid || !container) return;
      const rect = grid.getBoundingClientRect();
      const containerRect = container.getBoundingClientRect();
      const next = { left: rect.left - containerRect.left, top: rect.top - containerRect.top, width: rect.width, height: rect.height };
      setGeometry((prev) => (
        prev && prev.left === next.left && prev.top === next.top && prev.width === next.width && prev.height === next.height ? prev : next
      ));
    };
    const schedule = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        frame = requestAnimationFrame(measure);
      });
    };
    const observer = new ResizeObserver(schedule);
    observer.observe(chart);
    measureRef.current = schedule;
    schedule();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      measureRef.current = () => {};
    };
  }, [chartRef]);

  // 温度变化时补测一次（推迟到帧后，几何没变时 setGeometry 原样返回，不会多渲染）。
  useEffect(() => {
    if (temperature !== null) measureRef.current();
  }, [temperature]);

  if (!geometry || temperature === null) return null;

  const tempPercent = (temperature - temperatureRange.min) / (temperatureRange.max - temperatureRange.min);
  const position = { x: geometry.left + tempPercent * geometry.width, top: geometry.top, height: geometry.height };

  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={2} strokeDasharray="5 5" />
      <rect x={position.x - 45} y={position.top - 22} width={90} height={20} rx={4} fill="var(--chart-temperature-indicator)" />
      <text x={position.x} y={position.top - 8} textAnchor="middle" fill="white" fontSize={11} fontWeight={500}>{t('fanCurve.chart.currentTemperature', { temperature })}</text>
    </svg>
  );
});

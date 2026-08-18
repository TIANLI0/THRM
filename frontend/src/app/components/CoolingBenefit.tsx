'use client';

// 散热收益：直观展示"多吹 1000 转到底换来了什么"。
// 主动扫描测试给可信结论，日常统计给零成本参考，两者在界面上严格分开陈述。
//
// 图表刻意拆成温度、功耗、本机风扇三张，而不是叠在一张图的双 Y 轴上。双轴图会让
// 两条不同量纲的曲线互相扭曲对方的观感——几千 RPM 的柱子能把几十瓦的功耗曲线压成
// 直线，而读者无从判断哪根线该看哪边的刻度。拆开之后温度图还天然成了同量纲的多系列
// 图，正好承载"选不同传感器对比"。
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { Activity, Gauge, Play, Thermometer, TriangleAlert, X, Zap } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import clsx from 'clsx';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { captureFanState, restoreFanState } from '../lib/noise-test';
import {
  buildBenefitSteps,
  checkLoad,
  estimatedDurationMs,
  isAbortError,
  MIN_LOAD_WATTS,
  runBenefitSweep,
  type BenefitStepProgress,
} from '../lib/cooling-benefit';
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/index';

// 分类配色，固定顺序、不循环使用。选择上限就是配色槽位数——超出之后再加线条
// 就只能重复颜色，那样图例反而会骗人。
const SERIES_COLORS = ['var(--viz-1)', 'var(--viz-2)', 'var(--viz-3)', 'var(--viz-4)', 'var(--viz-5)', 'var(--viz-6)'];
const MAX_SELECTED_SENSORS = SERIES_COLORS.length;

type Phase = 'intro' | 'running' | 'done';

interface SeriesSpec {
  key: string;
  name: string;
  color: string;
}

interface CoolingBenefitProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config: types.AppConfig;
  deviceModel: string | null;
  temperature: types.TemperatureData | null;
  isConnected: boolean;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function formatMinutes(ms: number) {
  return Math.round(ms / 60000);
}

// paddedDomain 按数据实际范围给出留边距的坐标轴区间，并对齐到整数刻度。
// recharts 默认从 0 起画，而温度常年在 70~95、功耗在 60~130 —— 那样整条曲线会被
// 挤成一条平线，"多吹 1000 转降了几度"完全看不出来。
// 全部读数相同时也要撑开一点，否则会画出退化的轴。
function paddedDomain(values: number[], pad: number): [number, number] {
  const valid = values.filter((v) => Number.isFinite(v) && v > 0);
  if (valid.length === 0) return [0, 1];
  const lo = Math.min(...valid);
  const hi = Math.max(...valid);
  if (hi - lo < pad) {
    const mid = (hi + lo) / 2;
    return [Math.floor(mid - pad), Math.ceil(mid + pad)];
  }
  return [Math.floor(lo - pad), Math.ceil(hi + pad)];
}

export default function CoolingBenefit({ open, onOpenChange, config, deviceModel, temperature, isConnected }: CoolingBenefitProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<Phase>('intro');
  const [payload, setPayload] = useState<types.CoolingBenefitPayload | null>(null);
  const [progress, setProgress] = useState<BenefitStepProgress | null>(null);
  const [liveSteps, setLiveSteps] = useState<types.CoolingBenefitStep[]>([]);
  const [loadLabel, setLoadLabel] = useState('');
  const [restoring, setRestoring] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [selectedSensors, setSelectedSensors] = useState<string[] | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const rpmSteps = useMemo(() => buildBenefitSteps(), []);
  const report = payload?.report ?? null;
  const liveWatts = (temperature?.cpuPower ?? 0) + (temperature?.gpuPower ?? 0);
  const steps = phase === 'running' ? liveSteps : (report?.steps ?? []);

  const load = useCallback(async () => {
    try {
      setPayload(await apiService.getCoolingBenefit());
    } catch {
      /* noop：读不到就当没有历史报告 */
    }
  }, []);

  useEffect(() => {
    if (open) void load();
  }, [open, load]);

  useEffect(() => {
    if (!open) {
      abortRef.current?.abort();
      abortRef.current = null;
      setPhase('intro');
      setProgress(null);
      setLiveSteps([]);
    }
  }, [open]);

  // 换了一份报告就重置选择，让默认值（收益最大的两个）重新生效。
  useEffect(() => {
    setSelectedSensors(null);
  }, [report?.createdAt]);

  const sensorDeltas = useMemo(() => report?.analysis?.sensorDeltas ?? [], [report]);

  // 默认选中收益最大的两个：一进来就能看到最有信息量的对比，
  // 而不是一张空图等着用户自己去勾。
  const activeSensors = useMemo(() => {
    if (selectedSensors !== null) return selectedSensors;
    return sensorDeltas.slice(0, 2).map((sensor) => sensor.key);
  }, [selectedSensors, sensorDeltas]);

  // 颜色跟着传感器身份走，而不是跟着它在当前选中列表里的位次：取消勾选一个
  // 传感器时，其余几条线的颜色不该跟着换一遍。
  const sensorColors = useMemo(() => {
    const map = new Map<string, string>();
    activeSensors.forEach((key, index) => map.set(key, SERIES_COLORS[index % SERIES_COLORS.length]));
    return map;
  }, [activeSensors]);

  const toggleSensor = useCallback((key: string) => {
    setSelectedSensors((prev) => {
      const current = prev ?? sensorDeltas.slice(0, 2).map((sensor) => sensor.key);
      if (current.includes(key)) return current.filter((item) => item !== key);
      if (current.length >= MAX_SELECTED_SENSORS) {
        toast.error(t('fanCurve.benefit.sensorLimit', { count: MAX_SELECTED_SENSORS }));
        return current;
      }
      return [...current, key];
    });
  }, [sensorDeltas, t]);

  const start = useCallback(async () => {
    // 先开扩展传感器再做负载检查：打开会让桥接重建硬件监控实例，内存/硬盘/主板的
    // 读数要过一两个采样周期才补齐。抢在检查和第一档热稳定之前打开，第一档采样时
    // 传感器清单才是完整的——否则基准档缺的传感器会因为"没有可比的基准"被整条剔除。
    await apiService.setExtendedSensors(true).catch(() => false);

    const check = await checkLoad();
    if (!check.hasPowerReadings || !check.ok) {
      void apiService.setExtendedSensors(false).catch(() => false);
      toast.error(check.hasPowerReadings
        ? t('fanCurve.benefit.errors.idle', { watts: Math.round(check.watts), min: MIN_LOAD_WATTS })
        : t('fanCurve.benefit.errors.noPower'));
      return;
    }

    const snapshot = captureFanState(config);
    const controller = new AbortController();
    abortRef.current = controller;
    setPhase('running');
    setLiveSteps([]);
    setProgress(null);

    try {
      const collected = await runBenefitSweep((next, sofar) => {
        setProgress(next);
        setLiveSteps([...sofar]);
      }, controller.signal);

      const saved = await apiService.saveCoolingBenefitReport({
        deviceModel: deviceModel || '',
        cpuModel: temperature?.cpuModel || '',
        gpuModel: temperature?.gpuModel || '',
        loadLabel: loadLabel.trim(),
        steps: collected,
      });
      setPayload(saved);
      setSelectedSensors(null);
      setPhase('done');
    } catch (error) {
      if (!isAbortError(error)) {
        toast.error(t('fanCurve.benefit.errors.failed', { message: getErrorMessage(error) }));
      }
      setPhase('intro');
    } finally {
      abortRef.current = null;
      // 无论成功、失败还是中止，都必须收拾干净：风扇要交还给用户原本的控制方式
      // （测试期间它被锁在固定转速上，漏掉这一步风扇就永远停在最后一档），
      // 扩展传感器也要关掉，不能让它一直在后台多轮询一批硬件。
      setRestoring(true);
      try {
        await restoreFanState(snapshot);
      } catch {
        toast.error(t('fanCurve.benefit.errors.restore'));
      } finally {
        void apiService.setExtendedSensors(false).catch(() => false);
        setRestoring(false);
      }
    }
  }, [config, deviceModel, loadLabel, t, temperature?.cpuModel, temperature?.gpuModel]);

  const stop = useCallback(() => abortRef.current?.abort(), []);

  const clearReport = useCallback(async () => {
    setClearing(true);
    try {
      setPayload(await apiService.clearCoolingBenefit(true, false));
      setPhase('intro');
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setClearing(false);
    }
  }, []);

  /* ── 图表数据 ── */

  // 测试进行中还没有分析结果，先画 CPU/GPU；测完之后画用户选中的传感器。
  const tempSeries = useMemo<SeriesSpec[]>(() => {
    if (phase === 'running' || sensorDeltas.length === 0) {
      return [
        { key: 'cpuTemp', name: t('fanCurve.benefit.series.cpu'), color: SERIES_COLORS[0] },
        { key: 'gpuTemp', name: t('fanCurve.benefit.series.gpu'), color: SERIES_COLORS[1] },
      ];
    }
    return activeSensors
      .map((key) => sensorDeltas.find((sensor) => sensor.key === key))
      .filter((sensor): sensor is types.CoolingSensorDelta => !!sensor)
      .map((sensor) => ({
        key: `s:${sensor.key}`,
        name: sensor.name,
        color: sensorColors.get(sensor.key) ?? SERIES_COLORS[0],
      }));
  }, [activeSensors, phase, sensorColors, sensorDeltas, t]);

  const chartRows = useMemo(() => steps.map((step) => {
    const row: Record<string, number> = {
      rpm: step.targetRpm,
      cpuTemp: step.cpuTemp,
      gpuTemp: step.gpuTemp,
      power: step.cpuPower + step.gpuPower,
      laptopFan: Math.max(step.laptopCpuFanRpm, step.laptopGpuFanRpm),
    };
    for (const sensor of step.sensors ?? []) {
      row[`s:${sensor.key}`] = sensor.value;
    }
    return row;
  }), [steps]);

  const hasLaptopFan = chartRows.some((row) => row.laptopFan > 0);
  const hasCharts = chartRows.length >= 2;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88vh] max-w-3xl flex-col">
        <DialogHeader>
          <DialogTitle>{t('fanCurve.benefit.title')}</DialogTitle>
          <DialogDescription>{t('fanCurve.benefit.description')}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
          {phase === 'intro' && (
            <IntroPanel
              isConnected={isConnected}
              liveWatts={liveWatts}
              stepCount={rpmSteps.length}
              loadLabel={loadLabel}
              onLoadLabelChange={setLoadLabel}
            />
          )}

          {phase === 'running' && <RunningPanel progress={progress} stepCount={rpmSteps.length} />}

          {hasCharts && (
            <>
              <SeriesChart
                title={t('fanCurve.benefit.chartTemp')}
                rows={chartRows}
                series={tempSeries}
                unit="°C"
                pad={2}
                markRpm={report?.analysis?.sweetSpotRpm ?? 0}
                markLabel={t('fanCurve.benefit.sweetSpotMark')}
              />
              <SeriesChart
                title={t('fanCurve.benefit.chartPower')}
                rows={chartRows}
                series={[{ key: 'power', name: t('fanCurve.benefit.series.power'), color: SERIES_COLORS[0] }]}
                unit="W"
                pad={5}
              />
              {hasLaptopFan && (
                <SeriesChart
                  title={t('fanCurve.benefit.chartLaptopFan')}
                  rows={chartRows}
                  series={[{ key: 'laptopFan', name: t('fanCurve.benefit.series.laptopFan'), color: SERIES_COLORS[2] }]}
                  unit="RPM"
                  pad={100}
                />
              )}
            </>
          )}

          {phase !== 'running' && report && (
            <ReportPanel
              report={report}
              activeSensors={activeSensors}
              sensorColors={sensorColors}
              onToggleSensor={toggleSensor}
            />
          )}

          {phase !== 'running' && (
            <PassivePanel
              comparisons={payload?.passiveComparison ?? []}
              bounds={payload?.powerBucketBounds ?? []}
              minSamples={payload?.minCellSamples ?? 0}
            />
          )}
        </div>

        <DialogFooter>
          {phase === 'running' ? (
            <Button variant="danger" size="sm" onClick={stop} loading={restoring} icon={<X className="h-3.5 w-3.5" />}>
              {t('fanCurve.benefit.stop')}
            </Button>
          ) : (
            <>
              {report && (
                <Button variant="secondary" size="sm" onClick={clearReport} loading={clearing}>
                  {t('fanCurve.benefit.clear')}
                </Button>
              )}
              <Button variant="secondary" size="sm" onClick={() => onOpenChange(false)}>
                {t('common.actions.close')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={start}
                disabled={!isConnected || restoring}
                icon={<Play className="h-3.5 w-3.5" />}
              >
                {report ? t('fanCurve.benefit.retest') : t('fanCurve.benefit.start')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ── 图表 ── */

// SeriesChart 画一张单量纲的转速—读数折线图。单量纲是硬性约束：
// 温度、功耗、风扇转速各占一张，绝不共用一张图的两根 Y 轴。
function SeriesChart({
  title,
  rows,
  series,
  unit,
  pad,
  markRpm = 0,
  markLabel,
}: {
  title: string;
  rows: Record<string, number>[];
  series: SeriesSpec[];
  unit: string;
  pad: number;
  markRpm?: number;
  markLabel?: string;
}) {
  const { t } = useTranslation();

  const domain = useMemo(() => {
    const values: number[] = [];
    for (const row of rows) {
      for (const spec of series) {
        const value = row[spec.key];
        if (Number.isFinite(value)) values.push(value);
      }
    }
    return paddedDomain(values, pad);
  }, [pad, rows, series]);

  if (series.length === 0) {
    return (
      <div className="rounded-xl border border-border/70 bg-background/45 p-3">
        <div className="mb-1 text-xs font-medium text-muted-foreground">{title}</div>
        <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.benefit.noSeriesSelected')}</div>
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-border/70 bg-background/45 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="text-xs font-medium text-muted-foreground">{title}</div>
        {/* 两条以上必须有图例：身份不能只靠颜色传达。 */}
        {series.length > 1 && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            {series.map((spec) => (
              <span key={spec.key} className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: spec.color }} />
                <span className="max-w-40 truncate" title={spec.name}>{spec.name}</span>
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="h-52 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={rows} margin={{ top: 8, right: 12, left: 0, bottom: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
            <XAxis
              dataKey="rpm"
              type="number"
              domain={['dataMin', 'dataMax']}
              tickLine={false}
              axisLine={{ stroke: 'var(--chart-axis)' }}
              tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
              label={{ value: 'RPM', position: 'insideBottom', offset: -6, fill: 'var(--chart-tick)', fontSize: 11 }}
            />
            <YAxis
              domain={domain}
              allowDecimals={false}
              width={46}
              tickLine={false}
              axisLine={{ stroke: 'var(--chart-axis)' }}
              tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
              label={{ value: unit, angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 11 }}
            />
            <RechartsTooltip
              formatter={(value, name) => {
                const spec = series.find((item) => item.key === name);
                return [`${Number(value ?? 0).toFixed(1)} ${unit}`, spec?.name ?? String(name)];
              }}
              labelFormatter={(value) => `${value} RPM`}
              contentStyle={{
                backgroundColor: 'var(--chart-tooltip-bg)',
                border: '1px solid',
                borderColor: 'var(--chart-tooltip-border)',
                borderRadius: '8px',
                color: 'var(--chart-tooltip-text)',
              }}
            />
            {markRpm > 0 && (
              <ReferenceLine
                x={markRpm}
                stroke="var(--chart-adaptive)"
                strokeDasharray="4 4"
                label={{ value: markLabel, fill: 'var(--chart-tick)', fontSize: 10, position: 'top' }}
              />
            )}
            {series.map((spec) => (
              <Line
                key={spec.key}
                type="monotone"
                dataKey={spec.key}
                stroke={spec.color}
                strokeWidth={2}
                dot={{ r: 3, strokeWidth: 0, fill: spec.color }}
                activeDot={{ r: 5 }}
                connectNulls
                isAnimationActive={false}
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

/* ── 向导各阶段 ── */

function IntroPanel({
  isConnected,
  liveWatts,
  stepCount,
  loadLabel,
  onLoadLabelChange,
}: {
  isConnected: boolean;
  liveWatts: number;
  stepCount: number;
  loadLabel: string;
  onLoadLabelChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const loadReady = liveWatts >= MIN_LOAD_WATTS;

  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-border/70 bg-background/45 p-3 text-xs leading-relaxed text-muted-foreground">
        {t('fanCurve.benefit.intro', { steps: stepCount, minutes: formatMinutes(estimatedDurationMs()) })}
      </div>

      <div className={clsx(
        'flex items-center justify-between gap-3 rounded-xl border p-3',
        loadReady ? 'border-emerald-500/40 bg-emerald-500/10' : 'border-amber-400/40 bg-amber-500/10',
      )}>
        <div className="min-w-0">
          <div className="text-xs font-medium text-foreground">{t('fanCurve.benefit.loadCheckTitle')}</div>
          <div className="mt-1 text-xs leading-relaxed text-muted-foreground">
            {loadReady ? t('fanCurve.benefit.loadReady') : t('fanCurve.benefit.loadWaiting', { min: MIN_LOAD_WATTS })}
          </div>
        </div>
        <div className="shrink-0 text-right">
          <div className="text-lg font-semibold tabular-nums text-foreground">{Math.round(liveWatts)} W</div>
          <div className="text-[11px] text-muted-foreground">{t('fanCurve.benefit.currentLoad')}</div>
        </div>
      </div>

      <div className="rounded-xl border border-border/70 bg-background/45 p-3">
        <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.benefit.loadLabelTitle')}</div>
        <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.benefit.loadLabelDescription')}</div>
        <input
          value={loadLabel}
          onChange={(event) => onLoadLabelChange(event.target.value)}
          maxLength={40}
          placeholder={t('fanCurve.benefit.loadLabelPlaceholder')}
          className="mt-2 w-full rounded-lg border border-border/70 bg-card px-3 py-1.5 text-sm text-foreground outline-none focus:border-primary/50"
        />
      </div>

      {/* 说明扩展传感器是测试期间临时开的，免得用户以为程序在偷偷长期轮询硬盘。 */}
      <div className="rounded-xl border border-border/70 bg-background/45 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
        {t('fanCurve.benefit.extendedSensorsHint')}
      </div>

      {!isConnected && (
        <div className="rounded-xl border border-amber-400/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
          {t('fanCurve.benefit.errors.disconnected')}
        </div>
      )}
    </div>
  );
}

function RunningPanel({ progress, stepCount }: { progress: BenefitStepProgress | null; stepCount: number }) {
  const { t } = useTranslation();
  const index = (progress?.index ?? 0) + 1;
  const percent = Math.round(((progress?.index ?? 0) / Math.max(stepCount, 1)) * 100);

  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-border/70 bg-background/45 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-sm font-medium text-foreground">
            {t('fanCurve.benefit.runningStep', { index, total: stepCount, rpm: progress?.rpm ?? 0 })}
          </div>
          <Badge variant={progress?.phase === 'sampling' ? 'success' : 'info'}>
            {progress?.phase === 'sampling' ? t('fanCurve.benefit.phaseSampling') : t('fanCurve.benefit.phaseSettling')}
          </Badge>
        </div>
        <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
          <div className="h-full rounded-full bg-primary transition-[width] duration-500" style={{ width: `${percent}%` }} />
        </div>
        <div className="mt-2 text-xs leading-relaxed text-muted-foreground">
          {progress?.phase === 'settling'
            ? t('fanCurve.benefit.settlingHint', {
              seconds: Math.round((progress?.elapsedMs ?? 0) / 1000),
              range: (progress?.tempRangeC ?? 0).toFixed(1),
            })
            : t('fanCurve.benefit.samplingHint')}
        </div>
      </div>

      <div className="rounded-xl border border-dashed border-border/70 bg-background/45 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
        {t('fanCurve.benefit.keepLoadHint')}
      </div>
    </div>
  );
}

/* ── 报告 ── */

function ReportPanel({
  report,
  activeSensors,
  sensorColors,
  onToggleSensor,
}: {
  report: types.CoolingBenefitReport;
  activeSensors: string[];
  sensorColors: Map<string, string>;
  onToggleSensor: (key: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const analysis = report.analysis;
  const span = `${analysis.baselineRpm} → ${analysis.topRpm} RPM`;
  const sensorDeltas = useMemo(() => analysis.sensorDeltas ?? [], [analysis]);

  // 条形按本次最大变化归一，而不是写死一个满量程：不同机器的降幅量级差很多，
  // 固定量程会让降 3°C 的机器整排条都短得看不出差别。
  const sensorScale = useMemo(
    () => Math.max(...sensorDeltas.map((sensor) => Math.abs(sensor.delta)), 1),
    [sensorDeltas],
  );

  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-border/70 bg-background/45 p-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.benefit.verdictTitle')}</div>
          <Badge variant={analysis.regime === 'inconclusive' ? 'warning' : 'success'}>
            {t(`fanCurve.benefit.regime.${analysis.regime}`)}
          </Badge>
          <span className="text-[11px] text-muted-foreground">{span}</span>
        </div>
        <div className="mt-2 text-xs leading-relaxed text-muted-foreground">
          {t(`fanCurve.benefit.regimeHint.${analysis.regime}`)}
        </div>
        {report.loadLabel && (
          <div className="mt-2 text-[11px] text-muted-foreground">
            {t('fanCurve.benefit.underLoad', { label: report.loadLabel })}
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <StatTile
          icon={<Thermometer className="h-3.5 w-3.5" />}
          label={t('fanCurve.benefit.stats.temp')}
          value={`${analysis.tempDelta > 0 ? '+' : ''}${analysis.tempDelta} °C`}
          good={analysis.tempDelta < 0}
        />
        <StatTile
          icon={<Zap className="h-3.5 w-3.5" />}
          label={t('fanCurve.benefit.stats.power')}
          value={`${analysis.powerDelta > 0 ? '+' : ''}${analysis.powerDelta} W`}
          good={analysis.powerDelta > 0}
        />
        <StatTile
          icon={<Activity className="h-3.5 w-3.5" />}
          label={t('fanCurve.benefit.stats.laptopFan')}
          value={analysis.laptopFanDelta === 0 ? '—' : `${analysis.laptopFanDelta > 0 ? '+' : ''}${analysis.laptopFanDelta}`}
          good={analysis.laptopFanDelta < 0}
        />
        <StatTile
          icon={<Gauge className="h-3.5 w-3.5" />}
          label={analysis.sweetSpotHasNoise ? t('fanCurve.benefit.stats.sweetSpot') : t('fanCurve.benefit.stats.knee')}
          value={`${analysis.sweetSpotRpm} RPM`}
        />
      </div>

      <div className="rounded-xl border border-border/70 bg-background/45 p-3 text-xs leading-relaxed text-muted-foreground">
        {t('fanCurve.benefit.perKilo', {
          temp: analysis.tempPerKiloRpm.toFixed(1),
          power: analysis.powerPerKiloRpm.toFixed(1),
        })}
      </div>

      {analysis.warnings?.length > 0 && (
        <div className="space-y-1.5 rounded-xl border border-amber-400/40 bg-amber-500/10 p-3">
          <div className="flex items-center gap-1.5 text-xs font-medium text-amber-700 dark:text-amber-300">
            <TriangleAlert className="h-3.5 w-3.5" />
            {t('fanCurve.benefit.warningsTitle')}
          </div>
          {analysis.warnings.map((code) => (
            <div key={code} className="text-xs leading-relaxed text-amber-700 dark:text-amber-300">
              · {t(`fanCurve.benefit.warnings.${code}`)}
            </div>
          ))}
        </div>
      )}

      {sensorDeltas.length > 0 && (
        <div className="rounded-xl border border-border/70 bg-background/45 p-3">
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.benefit.sensorsTitle')}</div>
            <Badge variant="default">{t('fanCurve.benefit.sensorsCount', { count: sensorDeltas.length })}</Badge>
          </div>
          <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.benefit.sensorsDescription')}</div>
          {/* 这份列表同时充当图表的表格视图：每一行都写明基准档与最高档的实际读数，
              颜色不是唯一的识别手段。 */}
          <div className="mt-2 max-h-72 space-y-0.5 overflow-y-auto pr-1">
            {sensorDeltas.map((sensor) => (
              <SensorRow
                key={sensor.key}
                sensor={sensor}
                scale={sensorScale}
                selected={activeSensors.includes(sensor.key)}
                color={sensorColors.get(sensor.key)}
                onToggle={() => onToggleSensor(sensor.key)}
              />
            ))}
          </div>
        </div>
      )}

      <div className="text-[11px] text-muted-foreground">
        {t('fanCurve.benefit.measuredAt', { date: new Date(report.createdAt * 1000).toLocaleString(i18n.language) })}
      </div>
    </div>
  );
}

function SensorRow({
  sensor,
  scale,
  selected,
  color,
  onToggle,
}: {
  sensor: types.CoolingSensorDelta;
  scale: number;
  selected: boolean;
  color?: string;
  onToggle: () => void;
}) {
  // 用条形长度表达降幅，让"哪个部件最受益"一眼可见，而不必逐行读数字。
  const magnitude = Math.min(Math.abs(sensor.delta) / scale, 1) * 100;
  const cooled = sensor.delta < 0;

  return (
    <button
      type="button"
      onClick={onToggle}
      className={clsx(
        'flex w-full cursor-pointer items-center gap-2 rounded-lg px-1.5 py-1 text-left transition-colors',
        selected ? 'bg-primary/10' : 'hover:bg-muted/50',
      )}
    >
      <span
        className={clsx('h-2.5 w-2.5 shrink-0 rounded-full border', selected ? 'border-transparent' : 'border-border')}
        style={selected && color ? { backgroundColor: color } : undefined}
      />
      <span className="w-36 shrink-0 truncate text-xs text-foreground" title={`${sensor.name} (${sensor.key})`}>
        {sensor.name}
      </span>
      <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
        <span
          className={clsx('block h-full rounded-full', cooled ? 'bg-sky-500' : 'bg-orange-500')}
          style={{ width: `${magnitude}%` }}
        />
      </span>
      <span className={clsx('w-16 shrink-0 text-right text-xs font-semibold tabular-nums', cooled ? 'text-sky-500' : 'text-orange-500')}>
        {sensor.delta > 0 ? '+' : ''}{sensor.delta}°
      </span>
      <span className="w-24 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">
        {sensor.baseline} → {sensor.best}
      </span>
    </button>
  );
}

function StatTile({ icon, label, value, good }: { icon: React.ReactNode; label: string; value: string; good?: boolean }) {
  return (
    <div className="rounded-lg border border-border/70 bg-card/70 px-3 py-2">
      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        {icon}
        {label}
      </div>
      <div className={clsx(
        'mt-0.5 text-sm font-semibold tabular-nums',
        good === undefined ? 'text-foreground' : good ? 'text-emerald-500' : 'text-muted-foreground',
      )}>
        {value}
      </div>
    </div>
  );
}

function PassivePanel({
  comparisons,
  bounds,
  minSamples,
}: {
  comparisons: types.CoolingPassiveComparison[];
  bounds: number[];
  minSamples: number;
}) {
  const { t } = useTranslation();

  const bucketLabel = useCallback((index: number) => {
    if (index <= 0) return `< ${bounds[0] ?? 0} W`;
    if (index >= bounds.length) return `≥ ${bounds[bounds.length - 1] ?? 0} W`;
    return `${bounds[index - 1]}–${bounds[index]} W`;
  }, [bounds]);

  return (
    <div className="rounded-xl border border-dashed border-border/70 bg-background/45 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.benefit.passiveTitle')}</div>
        <Badge variant="info">{t('fanCurve.benefit.passiveBadge')}</Badge>
      </div>
      <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.benefit.passiveDescription')}</div>

      {comparisons.length === 0 ? (
        <div className="mt-2 text-xs leading-relaxed text-muted-foreground">
          {t('fanCurve.benefit.passiveEmpty', { samples: minSamples })}
        </div>
      ) : (
        <div className="mt-2 space-y-1">
          {comparisons.map((row) => (
            <div key={`${row.powerBucket}-${row.lowRpm}-${row.highRpm}`} className="flex flex-wrap items-center justify-between gap-2 text-xs">
              <span className="text-muted-foreground">{bucketLabel(row.powerBucket)}</span>
              <span className="tabular-nums text-muted-foreground">{row.lowRpm} → {row.highRpm} RPM</span>
              <span className={clsx('font-semibold tabular-nums', row.tempDelta < 0 ? 'text-sky-500' : 'text-orange-500')}>
                {row.tempDelta > 0 ? '+' : ''}{row.tempDelta} °C
              </span>
              <span className="text-[11px] text-muted-foreground">n={row.samples}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

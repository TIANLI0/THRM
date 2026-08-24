'use client';

// 风扇噪音测试向导：引导用户在安静环境下用麦克风实测 1000→4000 RPM
// 的噪音曲线，结果可写入学习模式噪音档案。
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer, ReferenceLine, ReferenceArea } from 'recharts';
import { AudioLines, Check, Mic, Play, RotateCw, TriangleAlert, Volume2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import clsx from 'clsx';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import {
  analyzeNoiseSamples,
  buildSweepSteps,
  captureFanState,
  isAbortError,
  listMicrophones,
  NoiseMeter,
  restoreFanState,
  runNoiseSweep,
  type MicrophoneOption,
  type NoiseAnalysis,
  type NoiseSweepSample,
  type SweepProgress,
} from '../lib/noise-test';
import { Badge, Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Select } from './ui/index';

const SWEEP_MIN_RPM = 1000;
const SWEEP_MAX_RPM = 4000;
const SWEEP_STEP_RPM = 250;
const LIVE_METER_INTERVAL_MS = 150;
const LIVE_METER_WINDOW = 14; // 约 2 秒滚动窗口
const ENV_STABLE_RANGE_DB = 4;

type Phase = 'intro' | 'setup' | 'running' | 'done';

interface NoiseTestProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

// 把 dBFS(A) 粗略映射成 0-100 的电平条
function levelPercent(db: number | null): number {
  if (db === null) return 0;
  return Math.max(0, Math.min(100, ((db + 90) / 60) * 100));
}

const NoiseTest = function NoiseTest({ open, onOpenChange, config, onConfigChange, isConnected }: NoiseTestProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<Phase>('intro');
  const [mics, setMics] = useState<MicrophoneOption[]>([]);
  const [selectedMic, setSelectedMic] = useState('');
  const [micLoading, setMicLoading] = useState(false);
  const [micError, setMicError] = useState<string | null>(null);
  const [meterReady, setMeterReady] = useState(false);
  const [liveDb, setLiveDb] = useState<number | null>(null);
  const [envRangeDb, setEnvRangeDb] = useState(0);
  const [progress, setProgress] = useState<SweepProgress | null>(null);
  const [samples, setSamples] = useState<NoiseSweepSample[]>([]);
  const [analysis, setAnalysis] = useState<NoiseAnalysis | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [applyProfileLoading, setApplyProfileLoading] = useState(false);
  const [profileApplied, setProfileApplied] = useState(false);

  const meterRef = useRef<NoiseMeter | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const recentLevelsRef = useRef<number[]>([]);
  const phaseRef = useRef<Phase>('intro');
  phaseRef.current = phase;

  const sweepSteps = useMemo(() => buildSweepSteps(SWEEP_MIN_RPM, SWEEP_MAX_RPM, SWEEP_STEP_RPM), []);
  const envStable = envRangeDb <= ENV_STABLE_RANGE_DB;

  const cleanupMeter = useCallback(() => {
    meterRef.current?.close();
    meterRef.current = null;
    recentLevelsRef.current = [];
    setMeterReady(false);
    setLiveDb(null);
    setEnvRangeDb(0);
  }, []);

  const resetAll = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    cleanupMeter();
    setPhase('intro');
    setProgress(null);
    setSamples([]);
    setAnalysis(null);
    setProfileApplied(false);
    setMicError(null);
  }, [cleanupMeter]);

  /* ── 麦克风打开与实时电平 ── */

  const openMeter = useCallback(async (deviceId: string) => {
    setMicError(null);
    setMeterReady(false);
    try {
      if (!meterRef.current) meterRef.current = new NoiseMeter();
      await meterRef.current.open(deviceId || undefined);
      recentLevelsRef.current = [];
      setMeterReady(true);
    } catch (err) {
      setMicError(getErrorMessage(err));
    }
  }, []);

  const loadMics = useCallback(async () => {
    setMicLoading(true);
    setMicError(null);
    try {
      const list = await listMicrophones();
      setMics(list);
      const preferred = list.find((m) => m.deviceId === selectedMic) ?? list[0];
      if (preferred) {
        setSelectedMic(preferred.deviceId);
        await openMeter(preferred.deviceId);
      } else {
        setMicError(t('fanCurve.noiseTest.setup.noMicFound'));
      }
    } catch (err) {
      setMicError(t('fanCurve.noiseTest.setup.permissionError', { error: getErrorMessage(err) }));
    } finally {
      setMicLoading(false);
    }
  }, [openMeter, selectedMic, t]);

  const handleMicChange = useCallback((deviceId: string) => {
    setSelectedMic(deviceId);
    void openMeter(deviceId);
  }, [openMeter]);

  // setup 阶段的实时电平表与环境稳定度（约 2 秒滚动窗口内的波动范围）
  useEffect(() => {
    if (!open || phase !== 'setup' || !meterReady) return;
    const timer = window.setInterval(() => {
      const meter = meterRef.current;
      if (!meter?.isOpen) return;
      const db = meter.readLevel();
      const recent = recentLevelsRef.current;
      recent.push(db);
      if (recent.length > LIVE_METER_WINDOW) recent.shift();
      setLiveDb(db);
      if (recent.length >= 6) {
        setEnvRangeDb(Math.max(...recent) - Math.min(...recent));
      }
    }, LIVE_METER_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [open, phase, meterReady]);

  // 关闭对话框时彻底清理
  useEffect(() => {
    if (!open) resetAll();
  }, [open, resetAll]);

  useEffect(() => () => {
    abortRef.current?.abort();
    meterRef.current?.close();
  }, []);

  /* ── 扫频执行 ── */

  const startSweep = useCallback(async () => {
    const meter = meterRef.current;
    if (!meter?.isOpen || !isConnected) return;

    const snapshot = captureFanState(config);
    const controller = new AbortController();
    abortRef.current = controller;
    setSamples([]);
    setAnalysis(null);
    setProfileApplied(false);
    setPhase('running');

    try {
      const result = await runNoiseSweep(
        meter,
        { minRpm: SWEEP_MIN_RPM, maxRpm: SWEEP_MAX_RPM, stepRpm: SWEEP_STEP_RPM },
        (p, collected) => {
          setProgress(p);
          setSamples([...collected]);
        },
        controller.signal,
      );
      setSamples(result);
      setAnalysis(analyzeNoiseSamples(result));
      setPhase('done');
    } catch (err) {
      if (isAbortError(err)) {
        toast.info(t('fanCurve.noiseTest.toast.cancelled'));
      } else {
        toast.error(t('fanCurve.noiseTest.toast.failed'), { description: getErrorMessage(err) });
      }
      setPhase('setup');
    } finally {
      abortRef.current = null;
      setProgress(null);
      setRestoring(true);
      try {
        await restoreFanState(snapshot);
      } catch (err) {
        toast.error(t('fanCurve.noiseTest.toast.restoreFailed'), { description: getErrorMessage(err) });
      } finally {
        setRestoring(false);
      }
    }
  }, [config, isConnected, t]);

  const cancelSweep = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    if (!nextOpen && phaseRef.current === 'running') {
      // 测试进行中先取消（触发恢复），不直接关闭
      cancelSweep();
      return;
    }
    onOpenChange(nextOpen);
  }, [cancelSweep, onOpenChange]);

  /* ── 应用结果 ── */

  const applyNoiseProfile = useCallback(async () => {
    if (!analysis) return;
    setApplyProfileLoading(true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({
        ...config.smartControl,
        noiseProfile: analysis.profile.map((p) => types.NoiseProfilePoint.createFrom(p)),
        noiseProfileUpdatedAt: Math.floor(Date.now() / 1000),
      });
      const nextConfig = types.AppConfig.createFrom({ ...config, smartControl: nextSmartControl });
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
      setProfileApplied(true);
      toast.success(t('fanCurve.noiseTest.toast.profileSaved'));
    } catch (err) {
      toast.error(t('fanCurve.noiseTest.toast.applyFailed'), { description: getErrorMessage(err) });
    } finally {
      setApplyProfileLoading(false);
    }
  }, [analysis, config, onConfigChange, t]);

  /* ── 图表数据 ── */

  const chartData = useMemo(() => {
    if (phase === 'done' && analysis) return analysis.profile;
    if (samples.length === 0) return [];
    const minDb = Math.min(...samples.map((s) => s.db));
    return samples.map((s) => ({ rpm: s.rpm, db: Math.round((s.db - minDb) * 10) / 10 }));
  }, [analysis, phase, samples]);

  const renderChart = (height: string) => (
    <div className={clsx('rounded-xl border border-border/70 bg-background/45 p-2', height)}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={chartData} margin={{ top: 10, right: 16, left: 0, bottom: 4 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
          <XAxis dataKey="rpm" type="number" domain={[SWEEP_MIN_RPM, SWEEP_MAX_RPM]} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 10 }} unit="" />
          <YAxis type="number" domain={[0, 'auto']} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 10 }} width={34} />
          <RechartsTooltip
            formatter={(value) => [`+${Number(value ?? 0).toFixed(1)} dB`, t('fanCurve.noiseTest.chart.series')]}
            labelFormatter={(v) => `${v} RPM`}
            contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', padding: '6px 10px', color: 'var(--chart-tooltip-text)' }}
            labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
            itemStyle={{ color: 'var(--chart-tooltip-text)' }}
          />
          {phase === 'done' && analysis?.resonance && (
            <ReferenceArea x1={analysis.resonance.startRpm} x2={analysis.resonance.endRpm} fill="var(--chart-primary)" fillOpacity={0.12} />
          )}
          {phase === 'done' && analysis?.kneeRpm !== null && analysis?.kneeRpm !== undefined && (
            <ReferenceLine x={analysis.kneeRpm} stroke="var(--chart-temperature-indicator)" strokeDasharray="5 4" />
          )}
          <Line type="monotone" dataKey="db" stroke="var(--chart-primary)" strokeWidth={2.5} dot={{ r: 3 }} isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );

  /* ── 渲染 ── */

  const micOptions = useMemo(
    () => mics.map((m) => ({ value: m.deviceId, label: m.label })),
    [mics],
  );

  const progressPercent = progress ? Math.round(((progress.index + (progress.phase === 'sampling' ? 0.6 : 0.15)) / progress.total) * 100) : 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AudioLines className="h-4 w-4 text-primary" />
            {t('fanCurve.noiseTest.title')}
          </DialogTitle>
          <DialogDescription>{t('fanCurve.noiseTest.description')}</DialogDescription>
        </DialogHeader>

        {phase === 'intro' && (
          <div className="space-y-3">
            <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-3">
              <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                <TriangleAlert className="h-4 w-4 text-amber-500" />
                {t('fanCurve.noiseTest.intro.envTitle')}
              </div>
              <ul className="mt-2 list-disc space-y-1 pl-5 text-xs leading-relaxed text-muted-foreground">
                <li>{t('fanCurve.noiseTest.intro.point1')}</li>
                <li>{t('fanCurve.noiseTest.intro.point2')}</li>
                <li>{t('fanCurve.noiseTest.intro.point3')}</li>
                <li>{t('fanCurve.noiseTest.intro.point4')}</li>
              </ul>
            </div>
            <div className="rounded-xl border border-border/70 bg-background/45 p-3 text-xs leading-relaxed text-muted-foreground">
              {t('fanCurve.noiseTest.intro.sweepNotice', { min: SWEEP_MIN_RPM, max: SWEEP_MAX_RPM, steps: sweepSteps.length })}
            </div>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => handleOpenChange(false)} icon={<X className="h-3.5 w-3.5" />}>
                {t('common.actions.cancel')}
              </Button>
              <Button variant="primary" size="sm" onClick={() => { setPhase('setup'); void loadMics(); }} icon={<Mic className="h-3.5 w-3.5" />}>
                {t('fanCurve.noiseTest.intro.next')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'setup' && (
          <div className="space-y-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-end">
              <div className="min-w-0 flex-1">
                <div className="mb-1 text-xs font-medium text-muted-foreground">{t('fanCurve.noiseTest.setup.micTitle')}</div>
                <Select
                  value={selectedMic}
                  onChange={handleMicChange}
                  options={micOptions}
                  disabled={micLoading || micOptions.length === 0}
                  size="sm"
                  className="w-full"
                />
              </div>
              <Button variant="secondary" size="sm" onClick={() => void loadMics()} loading={micLoading} icon={<RotateCw className="h-3.5 w-3.5" />}>
                {t('fanCurve.noiseTest.setup.refresh')}
              </Button>
            </div>

            {micError && (
              <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs leading-relaxed text-red-500">
                {micError}
              </div>
            )}

            <div className="rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                  <Volume2 className="h-3.5 w-3.5" />
                  {t('fanCurve.noiseTest.setup.levelTitle')}
                </div>
                {meterReady && (
                  <Badge variant={envStable ? 'success' : 'warning'}>
                    {envStable ? t('fanCurve.noiseTest.setup.stable') : t('fanCurve.noiseTest.setup.unstable')}
                  </Badge>
                )}
              </div>
              <div className="mt-2 h-2.5 overflow-hidden rounded-full bg-muted">
                <div
                  className={clsx('h-full rounded-full transition-all duration-150', envStable ? 'bg-green-500' : 'bg-orange-500')}
                  style={{ width: `${levelPercent(liveDb)}%` }}
                />
              </div>
              <div className="mt-2 text-xs leading-relaxed text-muted-foreground">
                {envStable ? t('fanCurve.noiseTest.setup.stableHint') : t('fanCurve.noiseTest.setup.unstableHint')}
              </div>
            </div>

            {!isConnected && (
              <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-500">
                {t('fanCurve.noiseTest.setup.deviceRequired')}
              </div>
            )}

            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => setPhase('intro')} icon={<X className="h-3.5 w-3.5" />}>
                {t('common.actions.back')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={() => void startSweep()}
                disabled={!meterReady || !isConnected || restoring}
                loading={restoring}
                icon={<Play className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.noiseTest.setup.start')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'running' && (
          <div className="space-y-3">
            <div className="rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>
                  {progress
                    ? t(progress.phase === 'settling' ? 'fanCurve.noiseTest.running.settling' : 'fanCurve.noiseTest.running.sampling', { rpm: progress.rpm })
                    : t('fanCurve.noiseTest.running.preparing')}
                </span>
                {progress && <span className="tabular-nums">{progress.index + 1}/{progress.total}</span>}
              </div>
              <div className="mt-2 h-2.5 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-primary transition-all duration-300" style={{ width: `${progressPercent}%` }} />
              </div>
              <div className="mt-2 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.noiseTest.running.keepQuiet')}</div>
            </div>
            {chartData.length >= 2 && renderChart('h-44')}
            <DialogFooter>
              <Button variant="danger" size="sm" onClick={cancelSweep} icon={<X className="h-3.5 w-3.5" />}>
                {t('fanCurve.noiseTest.running.cancel')}
              </Button>
            </DialogFooter>
          </div>
        )}

        {phase === 'done' && analysis && (
          <div className="space-y-3">
            {renderChart('h-52')}

            <div className="grid grid-cols-1 gap-2 text-xs md:grid-cols-2">
              <div className="rounded-lg border border-border/70 bg-card/70 px-3 py-2">
                <div className="text-muted-foreground">{t('fanCurve.noiseTest.done.riseTitle')}</div>
                <div className="mt-0.5 font-semibold text-foreground">+{analysis.totalRiseDb.toFixed(1)} dB</div>
              </div>
              <div className="rounded-lg border border-border/70 bg-card/70 px-3 py-2">
                <div className="text-muted-foreground">{t('fanCurve.noiseTest.done.kneeTitle')}</div>
                <div className="mt-0.5 font-semibold text-foreground">
                  {analysis.kneeRpm !== null ? t('fanCurve.noiseTest.done.kneeValue', { rpm: analysis.kneeRpm }) : t('fanCurve.noiseTest.done.noKnee')}
                </div>
              </div>
            </div>

            {analysis.resonance && (
              <div className="rounded-lg border border-orange-500/40 bg-orange-500/10 px-3 py-2 text-xs leading-relaxed text-foreground">
                {t('fanCurve.noiseTest.done.resonance', {
                  rpm: analysis.resonance.peakRpm,
                  db: analysis.resonance.prominenceDb,
                  min: analysis.resonance.startRpm,
                  max: analysis.resonance.endRpm,
                })}
              </div>
            )}

            {analysis.lowConfidence && (
              <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
                {t('fanCurve.noiseTest.done.lowConfidence')}
              </div>
            )}

            <div className="flex flex-col gap-2">
              <Button
                variant="primary"
                size="sm"
                onClick={() => void applyNoiseProfile()}
                loading={applyProfileLoading}
                disabled={profileApplied || analysis.lowConfidence}
                icon={profileApplied ? <Check className="h-3.5 w-3.5" /> : <AudioLines className="h-3.5 w-3.5" />}
              >
                {profileApplied ? t('fanCurve.noiseTest.done.profileApplied') : t('fanCurve.noiseTest.done.applyProfile')}
              </Button>
            </div>

            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => { setPhase('setup'); void loadMics(); }} icon={<RotateCw className="h-3.5 w-3.5" />}>
                {t('fanCurve.noiseTest.done.retest')}
              </Button>
              <Button variant="primary" size="sm" onClick={() => handleOpenChange(false)} icon={<Check className="h-3.5 w-3.5" />}>
                {t('fanCurve.noiseTest.done.close')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};

export default NoiseTest;

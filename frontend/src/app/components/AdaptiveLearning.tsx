'use client';

import React, { useCallback, useMemo, useState } from 'react';
import { BrainCircuit, Gauge, RotateCw, ShieldAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import clsx from 'clsx';
import { smartcontrol } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { Badge, Button, NumberInput, Slider, ToggleSwitch } from './ui/index';
import { ADAPTIVE_PREFERENCE_ANCHORS, nearestAdaptiveAnchor } from '../lib/adaptive-preference';

export const ADAPTIVE_TEMP_LIMIT_MIN = 75;
export const ADAPTIVE_TEMP_LIMIT_MAX = 100;

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

interface AdaptiveLearningProps {
  status: smartcontrol.AdaptiveStatus | null;
  onStatusChange: (status: smartcontrol.AdaptiveStatus) => void;
  /** 后端每次改动都会重写配置，父级据此把新的自动曲线同步进图表。 */
  onConfigInvalidated: () => void;
}

export default function AdaptiveLearning({ status, onStatusChange, onConfigInvalidated }: AdaptiveLearningProps) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [resetting, setResetting] = useState(false);
  // 滑块拖动期间用本地草稿，避免每一帧都往后端发请求；松手时才提交。
  const [preferenceDraft, setPreferenceDraft] = useState<number | null>(null);

  const enabled = !!status?.enabled;
  const preference = preferenceDraft ?? status?.preference ?? 50;
  const confidence = status?.confidence ?? 0;
  const samples = status?.samples ?? 0;

  const anchorLabel = useMemo(() => t(nearestAdaptiveAnchor(preference).labelKey), [preference, t]);

  // 在生成曲线上取某个温度对应的转速。用与控温环相同的线性插值，
  // 免得面板上报的数字和风扇实际会做的事对不上。
  const rpmAt = useCallback((temp: number) => {
    const curve = status?.curve ?? [];
    if (curve.length === 0) return 0;
    if (temp <= curve[0].temperature) return curve[0].rpm;
    const last = curve.length - 1;
    if (temp >= curve[last].temperature) return curve[last].rpm;
    for (let i = 0; i < last; i++) {
      if (temp < curve[i + 1].temperature) {
        const span = curve[i + 1].temperature - curve[i].temperature;
        if (span <= 0) return curve[i].rpm;
        const ratio = (temp - curve[i].temperature) / span;
        return Math.round(curve[i].rpm + ratio * (curve[i + 1].rpm - curve[i].rpm));
      }
    }
    return curve[last].rpm;
  }, [status?.curve]);

  const run = useCallback(async (
    action: () => Promise<smartcontrol.AdaptiveStatus>,
    setLoading: (value: boolean) => void,
  ) => {
    setLoading(true);
    try {
      onStatusChange(await action());
      onConfigInvalidated();
    } catch (error) {
      toast.error(t('fanCurve.adaptive.updateFailed', { message: getErrorMessage(error) }));
    } finally {
      setLoading(false);
    }
  }, [onConfigInvalidated, onStatusChange, t]);

  const handleToggle = useCallback((next: boolean) => {
    void run(() => apiService.setAdaptiveMode(next), setBusy);
  }, [run]);

  const handlePreferenceCommit = useCallback(() => {
    const value = preferenceDraft;
    setPreferenceDraft(null);
    if (value === null || value === status?.preference) return;
    void run(() => apiService.setAdaptivePreference(value), setBusy);
  }, [preferenceDraft, run, status?.preference]);

  const handleTempLimitChange = useCallback((value: number) => {
    if (value === status?.tempLimit) return;
    void run(() => apiService.setAdaptiveTempLimit(value), setBusy);
  }, [run, status?.tempLimit]);

  const handleReset = useCallback(() => {
    void run(async () => {
      const next = await apiService.resetAdaptiveModel();
      toast.success(t('fanCurve.adaptive.resetDone'));
      return next;
    }, setResetting);
  }, [run, t]);

  // 学习进度用置信度而不是样本数：样本数说明"跑了多久"，置信度说明"够不够用"。
  const progressPercent = Math.round(confidence * 100);
  const progressStage = confidence >= 0.999
    ? t('fanCurve.adaptive.progress.ready')
    : confidence > 0
      ? t('fanCurve.adaptive.progress.learning')
      : t('fanCurve.adaptive.progress.cold');

  return (
    <section className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <BrainCircuit className="h-4 w-4 text-violet-500" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <div className="text-sm font-medium text-foreground">{t('fanCurve.adaptive.title')}</div>
              <Badge variant="info">{t('fanCurve.adaptive.badge')}</Badge>
              {enabled && <Badge variant="success">{t('fanCurve.adaptive.active')}</Badge>}
            </div>
            <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.adaptive.description')}</div>
          </div>
        </div>
        <ToggleSwitch
          enabled={enabled}
          onChange={handleToggle}
          loading={busy}
          size="sm"
          color="purple"
          srLabel={t('fanCurve.adaptive.toggleAria')}
        />
      </div>

      {!enabled ? (
        <div className="mt-3 rounded-xl border border-dashed border-border/70 bg-background/45 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
          {t('fanCurve.adaptive.disabledHint')}
        </div>
      ) : (
        <div className="mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3">
          {/* ── 倾向 ── */}
          <div className="flex flex-col gap-3 rounded-xl border border-border/70 bg-card/55 p-3">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.adaptive.preferenceTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.adaptive.preferenceDescription')}</div>
              </div>
              <Badge variant="default">{anchorLabel}</Badge>
            </div>

            <Slider
              min={0}
              max={100}
              step={1}
              value={preference}
              onChange={setPreferenceDraft}
              onChangeEnd={handlePreferenceCommit}
              valueFormatter={(value) => `${value}`}
              disabled={busy}
            />
            <div className="flex justify-between text-[11px] text-muted-foreground">
              {ADAPTIVE_PREFERENCE_ANCHORS.map((anchor) => (
                <button
                  key={anchor.value}
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setPreferenceDraft(null);
                    if (anchor.value !== status?.preference) {
                      void run(() => apiService.setAdaptivePreference(anchor.value), setBusy);
                    }
                  }}
                  className={clsx(
                    'cursor-pointer rounded px-1 transition-colors hover:text-foreground',
                    Math.abs(anchor.value - preference) < 8 && 'font-semibold text-foreground',
                  )}
                >
                  {t(anchor.labelKey)}
                </button>
              ))}
            </div>
          </div>

          {/* ── 倾向到底改变了什么：直接报生成曲线在两个代表温度上的转速。
                 比"目标温度"之类的抽象参数好懂，也更诚实——2.0 没有目标温度，
                 稳态落点是权衡出来的，能拿出来看的只有曲线本身。 ── */}
          <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
            <DerivedStat label={t('fanCurve.adaptive.derived.atWarm')} value={`${rpmAt(70)} RPM`} />
            <DerivedStat label={t('fanCurve.adaptive.derived.atHot')} value={`${rpmAt(85)} RPM`} />
            <DerivedStat label={t('fanCurve.adaptive.derived.rpmRange')} value={`${status?.rpmFloor ?? 0}–${status?.rpmCeil ?? 0}`} />
            <DerivedStat
              label={t('fanCurve.adaptive.derived.model')}
              value={status?.usingPower ? t('fanCurve.adaptive.derived.modelPower') : t('fanCurve.adaptive.derived.modelBasic')}
            />
          </div>

          {/* ── 学习进度 ── */}
          <div className="flex flex-col gap-2 rounded-xl border border-border/70 bg-card/55 p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Gauge className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.adaptive.progressTitle')}</span>
                <Badge variant={confidence >= 0.999 ? 'success' : 'info'}>{progressStage}</Badge>
              </div>
              <span className="text-xs tabular-nums text-muted-foreground">
                {t('fanCurve.adaptive.progressCount', { samples })}
              </span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-violet-500 transition-[width] duration-500"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
            <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.adaptive.progressHint')}</div>
          </div>

          {/* ── 安全红线：唯一保留给用户的硬性约束 ── */}
          <div className="flex flex-col gap-3 rounded-xl border border-border/70 bg-card/55 p-3 md:flex-row md:items-center md:justify-between">
            <div className="flex min-w-0 items-start gap-2">
              <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-orange-500" />
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.adaptive.limitTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  {t('fanCurve.adaptive.limitDescription', { ceiling: status?.ceilingTemp ?? 0 })}
                </div>
              </div>
            </div>
            <div className="w-full md:w-32">
              <NumberInput
                value={status?.tempLimit ?? ADAPTIVE_TEMP_LIMIT_MAX}
                onChange={handleTempLimitChange}
                min={ADAPTIVE_TEMP_LIMIT_MIN}
                max={ADAPTIVE_TEMP_LIMIT_MAX}
                step={1}
                suffix="°C"
                disabled={busy}
              />
            </div>
          </div>

          {/* ── 重置模型 ── */}
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="min-w-0">
              <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.adaptive.resetTitle')}</div>
              <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.adaptive.resetDescription')}</div>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleReset}
              loading={resetting}
              disabled={samples === 0}
              icon={<RotateCw className="h-3.5 w-3.5" />}
            >
              {t('fanCurve.adaptive.resetButton')}
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}

function DerivedStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border/70 bg-card/70 px-3 py-2">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-sm font-semibold tabular-nums text-foreground">{value}</div>
    </div>
  );
}

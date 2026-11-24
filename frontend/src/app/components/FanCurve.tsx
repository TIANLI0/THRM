'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
// m + AppShell 里的 LazyMotion：动画特性异步加载，见 lib/motion-features.ts
import { m as motion, AnimatePresence } from 'framer-motion';
import {
  RotateCw,
  Check,
  Clock3,
  Spline,
  TriangleAlert,
  Plus,
  Trash2,
  Clipboard,
  Download,
  Sparkles,
  Gauge,
  Pencil,
  X,
  AudioLines,
} from 'lucide-react';
import type { TimelineEvent } from '../store/app-store';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useLocale } from '../lib/i18n';
import type { CurveFocusTarget } from '../store/app-store';
import * as types from '@go/types';
import { Clipboard as WailsClipboard } from '@wailsio/runtime';
import { BS1_MANUAL_GEAR_PRESETS, getManualGearLabel, getManualLevelLabel, MANUAL_GEAR_PRESETS, getEffectiveManualGearPresets, normalizeManualGearRpmMap, MANUAL_GEAR_RPM_MAX, MANUAL_GEAR_RPM_MIN, type ManualGearRpmMap } from '../lib/manualGearPresets';
import { useTranslation } from 'react-i18next';
import FanCurveProfileToolbar from './FanCurveProfileToolbar';
import { toast } from 'sonner';
import { ToggleSwitch, Button, Badge, Select, Slider, NumberInput, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './ui/index';
import clsx from 'clsx';
import { NoiseTest, CoolingBenefit } from './fan-curve/deferred-dialogs';
import {
  LOW_RPM_WARNING_DATE_KEY,
  FAN_CURVE_MIN_TEMP,
  FAN_CURVE_MAX_TEMP,
  FAN_CURVE_TEMP_STEP,
  DEFAULT_CURVE_LENGTH,
  SMART_CONTROL_TARGET_TEMP_MIN,
  SMART_CONTROL_TARGET_TEMP_MAX,
  type CurveProfile,
  type TimeCurveScheduleRuleView,
  type TimeCurveScheduleView,
  LEARNING_BIAS_OPTIONS,
  DEFAULT_SCHEDULE_RULE,
} from './fan-curve/constants';
import {
  getErrorMessage,
  normalizeLearningBias,
  constrainOffsetByLearningBias,
  normalizeTargetTemp,
  normalizeClockValue,
  normalizeWeekdayList,
  sanitizeTimeDraftInput,
  normalizeTimeDraftValue,
  ruleMatchesNow,
  syncCurveRpmAtIndex,
} from './fan-curve/curve-utils';
import { type CurveTempSource, TemperatureIndicator } from './fan-curve/TemperatureIndicator';
import { HistoryDetailsSection } from './fan-curve/HistoryDetailsSection';
import { DraggablePoint } from './fan-curve/chart-parts';

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  deviceModel: string | null;
  focusTarget: CurveFocusTarget | null;
  onFocusHandled: () => void;
  timelineEvents: TimelineEvent[];
}

/* ═══════════════════════════════════════════════════════════
   ─── Main FanCurve Component ───
   ═══════════════════════════════════════════════════════════ */

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, deviceModel, focusTarget, onFocusHandled, timelineEvents }: FanCurveProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState('');
  const [profileNameInput, setProfileNameInput] = useState('');
  const [isProfileNameComposing, setIsProfileNameComposing] = useState(false);
  const [profileOpLoading, setProfileOpLoading] = useState(false);
  const [createProfileDialogOpen, setCreateProfileDialogOpen] = useState(false);
  const [manageProfilesDialogOpen, setManageProfilesDialogOpen] = useState(false);
  const [profileSwitchDialogOpen, setProfileSwitchDialogOpen] = useState(false);
  const [deleteProfileDialogOpen, setDeleteProfileDialogOpen] = useState(false);
  const [pendingProfileId, setPendingProfileId] = useState('');
  const [pendingDeleteProfileId, setPendingDeleteProfileId] = useState('');
  const [newProfileNameInput, setNewProfileNameInput] = useState('');
  const [importCode, setImportCode] = useState('');
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [learningConfigLoading, setLearningConfigLoading] = useState(false);
  const [learningResetLoading, setLearningResetLoading] = useState(false);
  const [noiseTestOpen, setNoiseTestOpen] = useState(false);
  const [benefitOpen, setBenefitOpen] = useState(false);
  const [featureConfigLoading, setFeatureConfigLoading] = useState(false);
  const [scheduleTimeDrafts, setScheduleTimeDrafts] = useState<Record<string, string>>({});
  const [scheduleNameDrafts, setScheduleNameDrafts] = useState<Record<string, string>>({});
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [showLowRpmWarning, setShowLowRpmWarning] = useState(false);
  const chartRef = useRef<HTMLDivElement>(null);
  const curveEditorRef = useRef<HTMLDivElement>(null);
  const historyDetailsRef = useRef<HTMLElement>(null);
  const lowRpmWarnedInDragRef = useRef(false);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
  const [rpmRange, setRpmRange] = useState({ min: 0, max: 4000, ticks: [0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000] });

  const activeProfile = useMemo(() => curveProfiles.find((p) => p.id === activeProfileId) ?? null, [curveProfiles, activeProfileId]);
  const pendingDeleteProfile = useMemo(
    () => curveProfiles.find((profile) => profile.id === pendingDeleteProfileId) ?? null,
    [curveProfiles, pendingDeleteProfileId],
  );
  const externalActiveProfileId = ((config as any).activeFanCurveProfileId || '') as string;

  const shouldShowLowRpmWarningToday = useCallback(() => {
    if (typeof window === 'undefined') return false;
    const today = new Date().toISOString().slice(0, 10);
    const lastShownDate = window.localStorage.getItem(LOW_RPM_WARNING_DATE_KEY);
    if (lastShownDate === today) return false;
    window.localStorage.setItem(LOW_RPM_WARNING_DATE_KEY, today);
    return true;
  }, []);

  const temperatureRange = useMemo(() => ({
    min: FAN_CURVE_MIN_TEMP,
    max: FAN_CURVE_MAX_TEMP,
    ticks: Array.from({ length: DEFAULT_CURVE_LENGTH }, (_, i) => FAN_CURVE_MIN_TEMP + i * FAN_CURVE_TEMP_STEP),
  }), []);

  const syncConfigFromBackend = useCallback(async () => {
    try {
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch {
      /* noop */
    }
  }, [onConfigChange]);

  const loadCurveProfiles = useCallback(async () => {
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      const activeId = payload?.activeId || profiles[0]?.id || '';
      setCurveProfiles(profiles);
      setActiveProfileId(activeId);
      const current = profiles.find((p) => p.id === activeId) ?? profiles[0];
      if (current) {
        setProfileNameInput(current.name || '');
        setLocalCurve([...(current.curve || [])]);
        setHasUnsavedChanges(false);
      }
    } catch {
      /* noop */
    }
  }, []);

  const curveRpmBounds = useMemo(() => {
    const source = localCurve.length > 0 ? localCurve : (config.fanCurve ?? []);
    if (source.length === 0) {
      return { min: rpmRange.min, max: rpmRange.max };
    }
    let minCurveRPM = source[0].rpm;
    let maxCurveRPM = source[0].rpm;
    for (let i = 1; i < source.length; i++) {
      const rpm = source[i].rpm;
      if (rpm < minCurveRPM) minCurveRPM = rpm;
      if (rpm > maxCurveRPM) maxCurveRPM = rpm;
    }
    return { min: minCurveRPM, max: maxCurveRPM };
  }, [config.fanCurve, localCurve, rpmRange.max, rpmRange.min]);

  /* ── Smart control state ── */

  const smartControl = useMemo(() => {
    const curveLength = config.fanCurve?.length || localCurve.length || DEFAULT_CURVE_LENGTH;
    const defaultOffsets = Array.from({ length: curveLength }, () => 0);
    const defaultRateOffsets = Array.from({ length: 7 }, () => 0);
    const existing = config.smartControl;
    const normalizeOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, curveLength), ...defaultOffsets].slice(0, curveLength) : defaultOffsets;
    const normalizeRateOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, 7), ...defaultRateOffsets].slice(0, 7) : defaultRateOffsets;

    if (!existing) {
      return { enabled: true, learning: true, learningBias: 'balanced', filterTransientSpike: true, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 50, rampUpLimit: 220, rampDownLimit: 160, learnRate: 3, learnWindow: 8, learnDelay: 3, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, maxLearnOffset: 300, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
    }

    return {
      ...existing,
      learning: existing.learning ?? true,
      learningBias: normalizeLearningBias((existing as any).learningBias),
      filterTransientSpike: existing.filterTransientSpike ?? true,
      targetTemp: normalizeTargetTemp(existing.targetTemp ?? 68),
      hysteresis: Math.max(1, existing.hysteresis ?? 2),
      learnWindow: existing.learnWindow ?? 8, learnDelay: existing.learnDelay ?? 3,
      overheatWeight: existing.overheatWeight ?? 8, rpmDeltaWeight: existing.rpmDeltaWeight ?? 5,
      noiseWeight: existing.noiseWeight ?? 4,
      learnedOffsets: normalizeOffsets(existing.learnedOffsets),
      learnedOffsetsHeat: normalizeOffsets(existing.learnedOffsetsHeat),
      learnedOffsetsCool: normalizeOffsets(existing.learnedOffsetsCool),
      learnedRateHeat: normalizeRateOffsets(existing.learnedRateHeat),
      learnedRateCool: normalizeRateOffsets(existing.learnedRateCool),
    };
  }, [config.fanCurve, config.smartControl, localCurve.length]);

  const learningBiasOptions = useMemo(
    () => LEARNING_BIAS_OPTIONS.map((option) => ({
      value: option.value,
      label: t(option.labelKey),
      description: t(option.descriptionKey),
    })),
    [t, locale],
  );

  const noiseProfileDate = useMemo(() => {
    const updatedAt = (config.smartControl as any)?.noiseProfileUpdatedAt;
    const profile = (config.smartControl as any)?.noiseProfile;
    if (!updatedAt || !Array.isArray(profile) || profile.length < 2) return null;
    return new Date(updatedAt * 1000).toLocaleDateString(locale);
  }, [config.smartControl, locale]);

  const currentLearningBias = normalizeLearningBias((smartControl as any).learningBias);
  const currentLearningBiasOption = learningBiasOptions.find((option) => option.value === currentLearningBias) ?? learningBiasOptions[0];
  const [targetTempDraft, setTargetTempDraft] = useState(() => normalizeTargetTemp((config.smartControl as any)?.targetTemp ?? 68));
  const timeCurveSchedule = useMemo<TimeCurveScheduleView>(() => {
    const existing = (config as any).timeCurveSchedule;
    const fallbackProfileId = externalActiveProfileId || curveProfiles[0]?.id || '';
    const rules: TimeCurveScheduleRuleView[] = Array.isArray(existing?.rules)
      ? existing.rules.map((rule: any, index: number): TimeCurveScheduleRuleView => ({
        id: String(rule?.id || `schedule-${index + 1}`),
        name: String(rule?.name || t('fanCurve.schedule.defaultRuleName', { index: index + 1 })),
        enabled: rule?.enabled !== false,
        weekdays: normalizeWeekdayList(rule?.weekdays),
        startTime: normalizeClockValue(rule?.startTime, DEFAULT_SCHEDULE_RULE.startTime),
        endTime: normalizeClockValue(rule?.endTime, DEFAULT_SCHEDULE_RULE.endTime),
        curveProfileId: String(rule?.curveProfileId || fallbackProfileId),
      }))
      : [];

    return {
      enabled: existing?.enabled ?? false,
      rules,
    };
  }, [config, curveProfiles, externalActiveProfileId, t]);
  const weekdayOptions = useMemo(() => ([
    { value: 1, label: t('fanCurve.schedule.days.mon') },
    { value: 2, label: t('fanCurve.schedule.days.tue') },
    { value: 3, label: t('fanCurve.schedule.days.wed') },
    { value: 4, label: t('fanCurve.schedule.days.thu') },
    { value: 5, label: t('fanCurve.schedule.days.fri') },
    { value: 6, label: t('fanCurve.schedule.days.sat') },
    { value: 0, label: t('fanCurve.schedule.days.sun') },
  ]), [t, locale]);
  const scheduleProfileOptions = useMemo(() => curveProfiles.map((profile) => ({
    value: profile.id,
    label: profile.name,
  })), [curveProfiles]);
  const currentScheduleRule = useMemo(() => {
    if (!timeCurveSchedule.enabled) {
      return null;
    }
    return timeCurveSchedule.rules.find((rule) => ruleMatchesNow(rule, new Date())) ?? null;
  }, [timeCurveSchedule]);

  useEffect(() => {
    setScheduleTimeDrafts((prev) => {
      const next: Record<string, string> = {};
      for (const rule of timeCurveSchedule.rules) {
        const startKey = `${rule.id}:start`;
        const endKey = `${rule.id}:end`;
        if (startKey in prev) next[startKey] = prev[startKey];
        if (endKey in prev) next[endKey] = prev[endKey];
      }
      return next;
    });
  }, [timeCurveSchedule.rules]);

  useEffect(() => {
    setTargetTempDraft(normalizeTargetTemp((smartControl as any).targetTemp ?? 68));
  }, [smartControl.targetTemp]);

  useEffect(() => {
    if (!focusTarget) {
      return;
    }

    const target = focusTarget === 'history-details' ? historyDetailsRef.current : curveEditorRef.current;
    if (!target) {
      onFocusHandled();
      return;
    }

    // 首次进入曲线页时，页面切换动画（y 位移）仍在进行、且方案/历史等数据
    // 异步加载会继续改变上方内容高度，一次性 scrollIntoView 的位置会随即失效。
    // 因此逐帧对齐目标到视口顶部，直到位置连续多帧稳定（或超时）才结束。
    let cancelled = false;
    let frame = 0;
    const startedAt = performance.now();
    let lastTop = Number.NaN;
    let stableFrames = 0;

    const align = () => {
      if (cancelled) return;
      target.scrollIntoView({ block: 'start' });
      const top = target.getBoundingClientRect().top;
      if (Math.abs(top - lastTop) < 1) {
        stableFrames += 1;
      } else {
        stableFrames = 0;
      }
      lastTop = top;
      if (stableFrames >= 8 || performance.now() - startedAt > 1200) {
        onFocusHandled();
        return;
      }
      frame = window.requestAnimationFrame(align);
    };

    frame = window.requestAnimationFrame(align);

    return () => {
      cancelled = true;
      window.cancelAnimationFrame(frame);
    };
  }, [focusTarget, onFocusHandled]);

  const learnedOffsetSummary = useMemo(() => {
    const sourceCurve = localCurve.length > 0 ? localCurve : (config.fanCurve || []);
    return (smartControl.learnedOffsets || [])
      .map((value, index) => ({ value: constrainOffsetByLearningBias(typeof value === 'number' ? value : 0, currentLearningBias), index }))
      .filter((item) => item.value !== 0 && item.index < sourceCurve.length)
      .sort((left, right) => Math.abs(right.value) - Math.abs(left.value))
      .slice(0, 4)
      .map((item) => ({
        ...item,
        temperature: sourceCurve[item.index]?.temperature,
      }));
  }, [config.fanCurve, currentLearningBias, localCurve, smartControl.learnedOffsets]);

  /* ── Init ── */

  useEffect(() => {
    if (!isInitialized && config.fanCurve && config.fanCurve.length > 0) {
      setLocalCurve([...config.fanCurve]);
      setIsInitialized(true);
    }
  }, [config.fanCurve, isInitialized]);

  useEffect(() => {
    loadCurveProfiles().catch(() => {});
  }, [loadCurveProfiles]);

  useEffect(() => {
    if (externalActiveProfileId && externalActiveProfileId !== activeProfileId) {
      loadCurveProfiles().catch(() => {});
    }
  }, [activeProfileId, externalActiveProfileId, loadCurveProfiles]);

  // 入口处的一行摘要直接取配置里已存的报告，省掉一次只为徽标发起的 IPC。
  const benefitSummary = useMemo(() => {
    const analysis = (config as any).coolingBenefit?.report?.analysis;
    if (!analysis || analysis.regime === 'inconclusive') return '';
    const parts: string[] = [];
    if (analysis.tempDelta < 0) parts.push(`${analysis.tempDelta}°C`);
    if (analysis.powerDelta > 0) parts.push(`+${analysis.powerDelta}W`);
    if (parts.length === 0) return '';
    return t('fanCurve.benefit.entryBadge', { span: `${analysis.baselineRpm}→${analysis.topRpm}`, effect: parts.join(' / ') });
  }, [config, t]);
  /* ── Chart data ── */

  const chartData = useMemo(() => {
    const offsets = smartControl.learnedOffsets || [];
    return localCurve.map((point, index) => {
      const offset = constrainOffsetByLearningBias(offsets[index] ?? 0, currentLearningBias);
      return {
        temperature: point.temperature,
        rpm: point.rpm,
        coupledRpm: Math.max(curveRpmBounds.min, Math.min(curveRpmBounds.max, point.rpm + offset)),
        index,
      };
    });
  }, [curveRpmBounds.max, curveRpmBounds.min, currentLearningBias, localCurve, smartControl.learnedOffsets]);

  const hasLearnedOffsets = learnedOffsetSummary.length > 0;
  const showCoupledCurve = config.autoControl && !!smartControl.learning && hasLearnedOffsets;

  /* ── Point update + drag ── */

  const updatePoint = useCallback((index: number, newRpm: number) => {
    let didChange = false;

    setLocalCurve((prev) => {
      const nextState = syncCurveRpmAtIndex(prev, index, newRpm, rpmRange.min, rpmRange.max);

      if (nextState.hasLowRpmPoint && !lowRpmWarnedInDragRef.current) {
        lowRpmWarnedInDragRef.current = true;
        if (shouldShowLowRpmWarningToday()) {
          setShowLowRpmWarning(true);
        }
      }

      if (!nextState.changed) {
        return prev;
      }

      didChange = true;
      return nextState.curve;
    });

    if (didChange) {
      setHasUnsavedChanges(true);
    }
  }, [rpmRange, shouldShowLowRpmWarningToday]);

  const handleDragStart = useCallback((index: number) => {
    setDragIndex(index);
    setIsInteracting(true);
    lowRpmWarnedInDragRef.current = false;
    if (chartRef.current) {
      const chartArea = chartRef.current.querySelector('.recharts-cartesian-grid');
      if (chartArea) {
        const rect = chartArea.getBoundingClientRect();
        chartBoundsRef.current = { top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right, yMin: rpmRange.min, yMax: rpmRange.max };
      }
    }
  }, [rpmRange]);

  const handleDrag = useCallback((clientY: number) => {
    if (dragIndex === null || !chartBoundsRef.current) return;
    const bounds = chartBoundsRef.current;
    const relativeY = Math.max(0, Math.min(1, (bounds.bottom - clientY) / (bounds.bottom - bounds.top)));
    updatePoint(dragIndex, bounds.yMin + relativeY * (bounds.yMax - bounds.yMin));
  }, [dragIndex, updatePoint]);

  const scheduleDrag = useCallback((clientY: number) => {
    pendingDragYRef.current = clientY;
    if (dragFrameRef.current !== null) {
      return;
    }

    dragFrameRef.current = window.requestAnimationFrame(() => {
      dragFrameRef.current = null;
      const nextClientY = pendingDragYRef.current;
      pendingDragYRef.current = null;
      if (nextClientY !== null) {
        handleDrag(nextClientY);
      }
    });
  }, [handleDrag]);

  const handleDragEnd = useCallback(() => {
    if (dragFrameRef.current !== null) {
      window.cancelAnimationFrame(dragFrameRef.current);
      dragFrameRef.current = null;
    }
    pendingDragYRef.current = null;
    setDragIndex(null);
    setTimeout(() => setIsInteracting(false), 100);
  }, []);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (e: MouseEvent) => { e.preventDefault(); scheduleDrag(e.clientY); };
    const tm = (e: TouchEvent) => { if (e.touches.length > 0) scheduleDrag(e.touches[0].clientY); };
    const end = () => handleDragEnd();
    document.addEventListener('mousemove', mm);
    document.addEventListener('mouseup', end);
    document.addEventListener('touchmove', tm, { passive: false });
    document.addEventListener('touchend', end);
    return () => {
      document.removeEventListener('mousemove', mm);
      document.removeEventListener('mouseup', end);
      document.removeEventListener('touchmove', tm);
      document.removeEventListener('touchend', end);
      if (dragFrameRef.current !== null) {
        window.cancelAnimationFrame(dragFrameRef.current);
        dragFrameRef.current = null;
      }
      pendingDragYRef.current = null;
    };
  }, [dragIndex, handleDragEnd, scheduleDrag]);

  /* ── Save / Reset ── */

  const persistCurrentCurve = useCallback(async () => {
    if (isSaving) return;
    try {
      setIsSaving(true);
      const profileID = activeProfileId || (((config as any).activeFanCurveProfileId || '') as string);
      const profileName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
      await apiService.saveFanCurveProfile(profileID, profileName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setHasUnsavedChanges(false);
      return true;
    } catch (e) {
      toast.error(t('fanCurve.toast.saveCurveFailed', { error: getErrorMessage(e) }));
      return false;
    } finally {
      setIsSaving(false);
    }
  }, [activeProfile?.name, activeProfileId, config, isSaving, loadCurveProfiles, localCurve, syncConfigFromBackend, t]);

  const saveCurve = useCallback(async () => {
    await persistCurrentCurve();
  }, [persistCurrentCurve]);

  const getSafeProfileName = useCallback((input: string, fallback: string) => {
    const name = (input || '').trim() || fallback;
    const runes = Array.from(name);
    return runes.slice(0, 6).join('');
  }, []);

  const trimProfileNameToLimit = useCallback((value: string) => {
    return Array.from(value).slice(0, 6).join('');
  }, []);

  const handleProfileNameInputChange = useCallback((value: string, composing: boolean) => {
    if (composing || isProfileNameComposing) {
      setProfileNameInput(value);
      return;
    }
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [isProfileNameComposing, trimProfileNameToLimit]);

  const handleProfileNameCompositionStart = useCallback(() => {
    setIsProfileNameComposing(true);
  }, []);

  const handleProfileNameCompositionEnd = useCallback((value: string) => {
    setIsProfileNameComposing(false);
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [trimProfileNameToLimit]);

  const applyProfileSwitch = useCallback(async (id: string) => {
    try {
      setProfileOpLoading(true);
      await apiService.setActiveFanCurveProfile(id);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileSwitched'));
    } catch (e) {
      toast.error(t('fanCurve.toast.switchFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [loadCurveProfiles, syncConfigFromBackend, t]);

  const switchProfile = useCallback(async (id: string) => {
    if (!id || id === activeProfileId) return;
    if (hasUnsavedChanges) {
      setPendingProfileId(id);
      setProfileSwitchDialogOpen(true);
      return;
    }
    await applyProfileSwitch(id);
  }, [activeProfileId, applyProfileSwitch, hasUnsavedChanges]);

  const confirmProfileSwitch = useCallback(async (action: 'save' | 'discard') => {
    if (!pendingProfileId) return;
    if (action === 'save') {
      const saved = await persistCurrentCurve();
      if (!saved) return;
    }
    const nextProfileId = pendingProfileId;
    setProfileSwitchDialogOpen(false);
    setPendingProfileId('');
    await applyProfileSwitch(nextProfileId);
  }, [applyProfileSwitch, pendingProfileId, persistCurrentCurve]);

  const saveCurrentProfileName = useCallback(async () => {
    const fallbackName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
    const safeName = getSafeProfileName(profileNameInput, fallbackName);
    try {
      setProfileOpLoading(true);
      const profileCurve = activeProfile?.curve || localCurve;
      await apiService.saveFanCurveProfile(activeProfileId, safeName, profileCurve, false);
      setCurveProfiles((profiles) => profiles.map((profile) => (
        profile.id === activeProfileId ? { ...profile, name: safeName } : profile
      )));
      setProfileNameInput(safeName);
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileRenamed'));
    } catch (e) {
      toast.error(t('fanCurve.toast.renameFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.curve, activeProfile?.name, activeProfileId, getSafeProfileName, localCurve, profileNameInput, syncConfigFromBackend, t]);

  const createNewProfile = useCallback(async () => {
    const fallbackName = t('fanCurve.profiles.newCurveName');
    const safeName = getSafeProfileName(newProfileNameInput, fallbackName);
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', safeName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setNewProfileNameInput('');
      setCreateProfileDialogOpen(false);
      toast.success(t('fanCurve.toast.profileSavedAsNew'));
    } catch (e) {
      toast.error(t('fanCurve.toast.saveAsFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [getSafeProfileName, loadCurveProfiles, localCurve, newProfileNameInput, syncConfigFromBackend, t]);

  const removeProfile = useCallback(async () => {
    if (!pendingDeleteProfileId) return;
    const deletingActiveProfile = pendingDeleteProfileId === activeProfileId;
    try {
      setProfileOpLoading(true);
      await apiService.deleteFanCurveProfile(pendingDeleteProfileId);
      if (deletingActiveProfile) {
        await loadCurveProfiles();
      } else {
        setCurveProfiles((profiles) => profiles.filter((profile) => profile.id !== pendingDeleteProfileId));
      }
      await syncConfigFromBackend();
      setDeleteProfileDialogOpen(false);
      setPendingDeleteProfileId('');
      toast.success(t('fanCurve.toast.profileDeleted'));
    } catch (e) {
      toast.error(t('fanCurve.toast.deleteFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfileId, loadCurveProfiles, pendingDeleteProfileId, syncConfigFromBackend, t]);

  const exportProfiles = useCallback(async () => {
    try {
      if (hasUnsavedChanges) {
        const ok = await persistCurrentCurve();
        if (!ok) return;
      }
      const code = await apiService.exportFanCurveProfiles();
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        await WailsClipboard.SetText(code);
      }
      toast.success(t('fanCurve.toast.exportCopied'));
    } catch (e) {
      toast.error(t('fanCurve.toast.exportFailed', { error: getErrorMessage(e) }));
    }
  }, [hasUnsavedChanges, persistCurrentCurve, t]);

  const importProfiles = useCallback(async () => {
    const code = importCode.trim();
    if (!code) {
      toast.error(t('fanCurve.toast.importMissingCode'));
      return;
    }
    if (hasUnsavedChanges) {
      const saved = await persistCurrentCurve();
      if (!saved) return;
    }
    try {
      setProfileOpLoading(true);
      await apiService.importFanCurveProfiles(code);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setImportCode('');
      toast.success(t('fanCurve.toast.importSucceeded'));
    } catch (e) {
      toast.error(t('fanCurve.toast.importFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [hasUnsavedChanges, importCode, loadCurveProfiles, persistCurrentCurve, syncConfigFromBackend, t]);

  const resetCurve = useCallback(() => {
    const d: types.FanCurvePoint[] = [
      { temperature: 30, rpm: 1000 }, { temperature: 35, rpm: 1200 }, { temperature: 40, rpm: 1400 }, { temperature: 45, rpm: 1600 },
      { temperature: 50, rpm: 1800 }, { temperature: 55, rpm: 2000 }, { temperature: 60, rpm: Math.min(2300, rpmRange.max) },
      { temperature: 65, rpm: Math.min(2600, rpmRange.max) }, { temperature: 70, rpm: Math.min(2900, rpmRange.max) },
      { temperature: 75, rpm: Math.min(3200, rpmRange.max) }, { temperature: 80, rpm: Math.min(3500, rpmRange.max) },
      { temperature: 85, rpm: Math.min(3800, rpmRange.max) }, { temperature: 90, rpm: rpmRange.max }, { temperature: 95, rpm: rpmRange.max },
      { temperature: 100, rpm: rpmRange.max }, { temperature: 105, rpm: rpmRange.max }, { temperature: 110, rpm: rpmRange.max },
    ];
    setLocalCurve(d);
    setHasUnsavedChanges(true);
  }, [rpmRange.max]);

  /* ── Auto control / smart control handlers ── */

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    try { await apiService.setAutoControl(enabled); onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled })); } catch { /* noop */ }
  }, [config, onConfigChange]);

  const updateSmartControlConfig = useCallback(async (patch: Partial<types.SmartControlConfig> & { learningBias?: string }) => {
    setLearningConfigLoading(true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({ ...smartControl, ...patch });
      const nextConfig = types.AppConfig.createFrom({ ...config, smartControl: nextSmartControl });
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
    } catch (err) {
      toast.error(t('fanCurve.toast.saveLearningFailed'), { description: getErrorMessage(err) });
    } finally {
      setLearningConfigLoading(false);
    }
  }, [config, onConfigChange, smartControl, t]);

  const handleLearningToggle = useCallback((enabled: boolean) => {
    void updateSmartControlConfig({ learning: enabled });
  }, [updateSmartControlConfig]);

  const handleLearningBiasChange = useCallback((value: string) => {
    void updateSmartControlConfig({ learningBias: normalizeLearningBias(value) });
  }, [updateSmartControlConfig]);

  const commitTargetTemp = useCallback((value: number) => {
    const normalized = normalizeTargetTemp(value);
    setTargetTempDraft(normalized);
    if (normalized === normalizeTargetTemp((smartControl as any).targetTemp ?? 68)) {
      return;
    }
    void updateSmartControlConfig({ targetTemp: normalized });
  }, [smartControl.targetTemp, updateSmartControlConfig]);

  const handleTargetTempSliderChange = useCallback((value: number) => {
    setTargetTempDraft(normalizeTargetTemp(value));
  }, []);

  const handleTargetTempSliderCommit = useCallback(() => {
    commitTargetTemp(targetTempDraft);
  }, [commitTargetTemp, targetTempDraft]);

  const handleTargetTempInputChange = useCallback((value: number) => {
    commitTargetTemp(value);
  }, [commitTargetTemp]);

  const handleResetLearnedOffsets = useCallback(async () => {
    setLearningResetLoading(true);
    try {
      await apiService.resetLearnedOffsets();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.learningReset'), { description: t('fanCurve.toast.learningResetDescription'), duration: 2400 });
    } catch (err) {
      toast.error(t('fanCurve.toast.resetFailed'), { description: getErrorMessage(err) });
    } finally {
      setLearningResetLoading(false);
    }
  }, [syncConfigFromBackend, t]);

  const updateFanFeatureConfig = useCallback(async (patch: Partial<types.AppConfig>) => {
    setFeatureConfigLoading(true);
    try {
      const nextConfig = types.AppConfig.createFrom({ ...config, ...patch });
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
    } catch (err) {
      toast.error(t('fanCurve.toast.saveFeatureFailed'), { description: getErrorMessage(err) });
    } finally {
      setFeatureConfigLoading(false);
    }
  }, [config, onConfigChange, t]);

  const updateTimeCurveSchedule = useCallback((patch: Partial<types.TimeCurveScheduleConfig> & { rules?: TimeCurveScheduleRuleView[] }) => {
    const nextTimeCurveSchedule = types.TimeCurveScheduleConfig.createFrom({
      ...timeCurveSchedule,
      ...patch,
      rules: patch.rules ?? timeCurveSchedule.rules,
    });
    void updateFanFeatureConfig({ timeCurveSchedule: nextTimeCurveSchedule as any });
  }, [timeCurveSchedule, updateFanFeatureConfig]);

  const handleAddScheduleRule = useCallback(() => {
    const fallbackProfileId = activeProfileId || curveProfiles[0]?.id || '';
    const nextRule = {
      id: `schedule-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
      name: t('fanCurve.schedule.newRuleName'),
      enabled: true,
      weekdays: [...DEFAULT_SCHEDULE_RULE.weekdays],
      startTime: DEFAULT_SCHEDULE_RULE.startTime,
      endTime: DEFAULT_SCHEDULE_RULE.endTime,
      curveProfileId: fallbackProfileId,
    };
    updateTimeCurveSchedule({ rules: [...timeCurveSchedule.rules, nextRule] });
  }, [activeProfileId, curveProfiles, t, timeCurveSchedule.rules, updateTimeCurveSchedule]);

  const handleScheduleRuleChange = useCallback((ruleId: string, patch: Partial<types.TimeCurveScheduleRule>) => {
    const nextRules = timeCurveSchedule.rules.map((rule) => {
      if (rule.id !== ruleId) return rule;
      return {
        ...rule,
        ...patch,
      };
    });
    updateTimeCurveSchedule({ rules: nextRules });
  }, [timeCurveSchedule.rules, updateTimeCurveSchedule]);

  const handleScheduleRuleDelete = useCallback((ruleId: string) => {
    updateTimeCurveSchedule({ rules: timeCurveSchedule.rules.filter((rule) => rule.id !== ruleId) });
  }, [timeCurveSchedule.rules, updateTimeCurveSchedule]);

  const toggleScheduleWeekday = useCallback((ruleId: string, day: number) => {
    const nextRules = timeCurveSchedule.rules.map((rule) => {
      if (rule.id !== ruleId) return rule;
      const exists = rule.weekdays.includes(day);
      const nextWeekdays = exists
        ? rule.weekdays.filter((item) => item !== day)
        : [...rule.weekdays, day];
      return {
        ...rule,
        weekdays: normalizeWeekdayList(nextWeekdays.length > 0 ? nextWeekdays : rule.weekdays),
      };
    });
    updateTimeCurveSchedule({ rules: nextRules });
  }, [timeCurveSchedule.rules, updateTimeCurveSchedule]);

  const handleScheduleNameDraftCommit = useCallback((ruleId: string, fallback: string) => {
    const rawValue = scheduleNameDrafts[ruleId];
    setScheduleNameDrafts((prev) => {
      const next = { ...prev };
      delete next[ruleId];
      return next;
    });
    if (rawValue === undefined) return;
    const trimmed = rawValue.trim() || fallback;
    if (trimmed !== fallback) {
      handleScheduleRuleChange(ruleId, { name: trimmed } as Partial<types.TimeCurveScheduleRule>);
    }
  }, [handleScheduleRuleChange, scheduleNameDrafts]);

  const handleScheduleTimeDraftChange = useCallback((ruleId: string, field: 'startTime' | 'endTime', value: string) => {
    const draftKey = `${ruleId}:${field === 'startTime' ? 'start' : 'end'}`;
    setScheduleTimeDrafts((prev) => ({
      ...prev,
      [draftKey]: sanitizeTimeDraftInput(value),
    }));
  }, []);

  const handleScheduleTimeDraftCommit = useCallback((ruleId: string, field: 'startTime' | 'endTime', fallback: string) => {
    const draftKey = `${ruleId}:${field === 'startTime' ? 'start' : 'end'}`;
    const rawValue = scheduleTimeDrafts[draftKey] ?? fallback;
    const normalizedValue = normalizeTimeDraftValue(rawValue, fallback);
    setScheduleTimeDrafts((prev) => {
      const next = { ...prev };
      delete next[draftKey];
      return next;
    });
    handleScheduleRuleChange(ruleId, { [field]: normalizedValue } as Partial<types.TimeCurveScheduleRule>);
  }, [handleScheduleRuleChange, scheduleTimeDrafts]);

  /* ── Reference temperature (follows settings 控温温度来源: max/cpu/gpu) ──
     具体数值由 TemperatureIndicator 自己订阅，曲线页主体不随温度推送重渲染。 */
  const tempSource = (((config as any).tempSource as string) || 'max') as CurveTempSource;

  /* ── Manual gear ── */

  const isBs1 = deviceModel === 'BS1';

  const customGearRpm = useMemo(() => {
    return ((config as any).manualGearRpm ?? null) as ManualGearRpmMap | null;
  }, [config]);

  const manualGearPresets = isBs1
    ? BS1_MANUAL_GEAR_PRESETS
    : getEffectiveManualGearPresets(customGearRpm);

  const manualPoints = useMemo(() => {
    return manualGearPresets.flatMap((preset, gearIndex) => preset.levels.map((item, levelIndex) => ({
      key: `${preset.gear}-${item.level}`,
      gear: preset.gear,
      level: item.level,
      rpm: item.rpm,
      gearIndex,
      levelIndex,
      colorClass: preset.colorClass,
      borderClass: preset.borderClass,
      bgClass: preset.bgClass,
      dotActiveClass: preset.dotActiveClass,
      dotPassedClass: preset.dotPassedClass,
    })));
  }, [manualGearPresets]);

  const selectedManualPointIndex = useMemo(() => {
    // BS1 每个挡位只有一个点，配置里残留的"低/高"子级别不能让选中状态整个丢掉。
    const selected = manualPoints.findIndex((p) => p.gear === (config.manualGear || '标准') && (isBs1 || p.level === (config.manualLevel || '中')));
    return selected >= 0 ? selected : 4;
  }, [config.manualGear, config.manualLevel, isBs1, manualPoints]);

  const rememberedManualGearLevels = useMemo(() => {
    return ((config as any).manualGearLevels ?? {}) as Record<string, string>;
  }, [config]);

  const applyManualGearPreset = useCallback(async (gear: string, level: string) => {
    try {
      await apiService.setManualGear(gear, level);
      onConfigChange(types.AppConfig.createFrom({
        ...config,
        manualGear: gear,
        manualLevel: level,
        manualGearLevels: {
          ...rememberedManualGearLevels,
          [gear]: level,
        },
      }));
    } catch { /* noop */ }
  }, [config, onConfigChange, rememberedManualGearLevels]);

  const handleManualPointSelect = useCallback(async (index: number) => {
    const selected = manualPoints[index];
    if (!selected) return;
    await applyManualGearPreset(selected.gear, selected.level);
  }, [applyManualGearPreset, manualPoints]);

  const handleGearCardSelect = useCallback(async (gear: string) => {
    const rememberedLevel = rememberedManualGearLevels[gear];
    const nextLevel = rememberedLevel === '低' || rememberedLevel === '中' || rememberedLevel === '高'
      ? rememberedLevel
      : (config.manualLevel || '中');
    await applyManualGearPreset(gear, nextLevel);
  }, [applyManualGearPreset, config, rememberedManualGearLevels]);

  /* ── Manual gear RPM editor ── */

  const [gearEditOpen, setGearEditOpen] = useState(false);
  const [draftGearRpm, setDraftGearRpm] = useState<ManualGearRpmMap>({});
  const [gearRpmSaving, setGearRpmSaving] = useState(false);

  const buildDraftFrom = useCallback((source: ManualGearRpmMap | null): ManualGearRpmMap => {
    const base: ManualGearRpmMap = {};
    MANUAL_GEAR_PRESETS.forEach((preset) => {
      base[preset.gear] = {};
      preset.levels.forEach((lv) => {
        const value = source?.[preset.gear]?.[lv.level];
        base[preset.gear][lv.level] = typeof value === 'number' && value > 0 ? value : lv.rpm;
      });
    });
    return base;
  }, []);

  const openGearEditor = useCallback(() => {
    setDraftGearRpm(buildDraftFrom(customGearRpm));
    setGearEditOpen(true);
  }, [buildDraftFrom, customGearRpm]);

  const setDraftRpm = useCallback((gear: string, level: string, value: number) => {
    setDraftGearRpm((prev) => ({
      ...prev,
      [gear]: { ...(prev[gear] ?? {}), [level]: value },
    }));
  }, []);

  const saveGearRpm = useCallback(async () => {
    setGearRpmSaving(true);
    try {
      const normalized = normalizeManualGearRpmMap(draftGearRpm);
      const next = types.AppConfig.createFrom({ ...config, manualGearRpm: normalized });
      await apiService.updateConfig(next);
      onConfigChange(next);
      // 重新下发当前挡位以应用新转速
      await apiService.setManualGear(config.manualGear || '标准', config.manualLevel || '中');
      setGearEditOpen(false);
      toast.success(t('fanCurve.manualGear.rpmSaved'));
    } catch (err) {
      toast.error(t('fanCurve.manualGear.rpmSaveFailed', { error: getErrorMessage(err) }));
    } finally {
      setGearRpmSaving(false);
    }
  }, [config, draftGearRpm, onConfigChange, t]);

  /* ── Custom dot renderer ── */

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    if (cx === undefined || cy === undefined) return <g />;
    return <DraggablePoint key={`dot-${index}`} cx={cx} cy={cy} index={index} temperature={payload.temperature} rpm={payload.rpm} onDragStart={handleDragStart} isActive={dragIndex === index} />;
  }, [dragIndex, handleDragStart]);

  /* 主曲线图表整体缓存：温度/转速等高频数据每秒刷新时不重建 recharts 树，
     当前温度指示线由 TemperatureIndicator 独立覆盖层绘制 */
  const curveChart = useMemo(() => (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 20 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
        <XAxis dataKey="temperature" type="number" domain={[temperatureRange.min, temperatureRange.max]} ticks={temperatureRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} label={{ value: t('fanCurve.chart.axes.temperature'), position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }} />
        <YAxis type="number" domain={[rpmRange.min, rpmRange.max]} ticks={rpmRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} label={{ value: t('fanCurve.chart.axes.rpm'), angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }} />
        <RechartsTooltip
          formatter={(value, name) => {
            const numericValue = Number(value ?? 0);
            return name === 'coupledRpm' ? [`${numericValue} RPM`, t('fanCurve.chart.series.learned')] : [`${numericValue} RPM`, t('fanCurve.chart.series.base')];
          }}
          labelFormatter={(v) => t('fanCurve.chart.temperatureLabel', { temperature: v })}
          contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
          labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
          itemStyle={{ color: 'var(--chart-tooltip-text)' }}
        />
        <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={CustomDot} activeDot={false} isAnimationActive={false} />
        {showCoupledCurve && <Line type="monotone" dataKey="coupledRpm" stroke="var(--chart-primary)" strokeWidth={2} strokeDasharray="6 4" dot={false} activeDot={false} isAnimationActive={false} />}
      </LineChart>
    </ResponsiveContainer>
  ), [CustomDot, chartData, rpmRange, showCoupledCurve, t, temperatureRange]);

  /* ═══════════════════ RENDER ═══════════════════ */

  return (
    <div className="relative space-y-4 px-1 pb-2">
        {/* ── Header ── */}
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className="relative px-1 py-1"
        >
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <Spline className="h-4 w-4 text-primary" />
              <h2 className="text-base font-semibold text-foreground">{t('fanCurve.title')}</h2>
              {hasUnsavedChanges && <Badge variant="warning">{t('fanCurve.badges.unsaved')}</Badge>}
              {isInteracting && <Badge variant="info">{t('fanCurve.badges.editing')}</Badge>}
            </div>

            <div data-curve-profile-row className="flex min-w-0 items-center gap-3">
              <FanCurveProfileToolbar
                profiles={curveProfiles}
                activeProfileId={activeProfileId}
                onChange={switchProfile}
                loading={profileOpLoading}
                className="min-w-0 flex-1"
                onAddNew={() => {
                  setNewProfileNameInput('');
                  setCreateProfileDialogOpen(true);
                }}
                onManage={() => {
                  setManageProfilesDialogOpen(true);
                }}
                onDelete={(profileId) => {
                  setPendingDeleteProfileId(profileId);
                  setDeleteProfileDialogOpen(true);
                }}
              />
              <div className="flex shrink-0 items-center gap-3">
                <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label={t('fanCurve.actions.smartControl')} size="sm" color="blue" />
                <Button variant="secondary" size="sm" className="rounded-lg" onClick={resetCurve} icon={<RotateCw className="h-3.5 w-3.5" />}>
                  {t('fanCurve.actions.reset')}
                </Button>
                <Button variant="primary" size="sm" className="rounded-lg" onClick={saveCurve} disabled={!hasUnsavedChanges} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('common.actions.save')}
                </Button>
              </div>
            </div>
          </div>
        </motion.div>

        {/* ── Manual gear (when auto off) ── */}
        <AnimatePresence>
          {!config.autoControl && isConnected && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
              <div className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{t('fanCurve.manualGear.title')}</span>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">{isBs1 ? t('fanCurve.manualGear.bs1SliderHint') : t('fanCurve.manualGear.defaultSliderHint')}</span>
                    {!isBs1 && (
                      <Button variant="secondary" size="sm" onClick={openGearEditor} icon={<Pencil className="h-3.5 w-3.5" />}>
                        {t('fanCurve.manualGear.customize')}
                      </Button>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {manualGearPresets.map((preset) => {
                    const isActiveGear = (config.manualGear || '标准') === preset.gear;
                    const rememberedLevel = isActiveGear
                      ? (config.manualLevel || '中')
                      : rememberedManualGearLevels[preset.gear];
                    const activeLevel = preset.levels.find((l) => l.level === rememberedLevel) ?? preset.levels[0];
                    return (
                      <button
                        key={preset.gear}
                        type="button"
                        onClick={() => isBs1 ? applyManualGearPreset(preset.gear, '中') : handleGearCardSelect(preset.gear)}
                        className={clsx(
                          'relative cursor-pointer rounded-xl border px-3 py-2.5 text-left transition-all',
                          isActiveGear ? preset.activeCardClass : 'border-border/70 bg-background/40 opacity-70 hover:bg-muted/35 hover:opacity-100',
                        )}
                        aria-pressed={isActiveGear}
                      >
                        {isActiveGear && (
                          <span className={clsx('absolute right-2.5 top-2.5 flex h-4 w-4 items-center justify-center rounded-full text-white', preset.solidBgClass)}>
                            <Check className="h-3 w-3" strokeWidth={3} />
                          </span>
                        )}
                        <div className={clsx('text-lg font-bold', isActiveGear ? preset.colorClass : 'text-foreground')}>{getManualGearLabel(preset.gear)}</div>
                        {!isBs1 && <div className={clsx('mt-1 text-base font-semibold', isActiveGear ? preset.colorClass : 'text-muted-foreground')}>{activeLevel.rpm}RPM</div>}
                      </button>
                    );
                  })}
                </div>

                <div className="rounded-xl border border-border/70 bg-background/40 p-3">
                  <div className="relative mb-3 px-2">
                    <div className="absolute left-2 right-2 top-1/2 h-1 -translate-y-1/2 rounded-full bg-muted" />
                    <div className="relative flex items-center justify-between">
                      {manualPoints.map((point, index) => {
                        const isActivePoint = selectedManualPointIndex === index;
                        const isPassed = index < selectedManualPointIndex;
                        return (
                          <button
                            key={point.key}
                            type="button"
                            onClick={() => handleManualPointSelect(index)}
                            className="flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center"
                            title={t('fanCurve.manualGear.pointTooltip', { gear: getManualGearLabel(point.gear), level: getManualLevelLabel(point.level), rpm: point.rpm })}
                          >
                            <span
                              className={clsx(
                                'block h-4 w-4 rounded-full border-2 transition-all duration-150',
                                isActivePoint
                                  ? `scale-125 ${point.dotActiveClass}`
                                  : isPassed
                                    ? point.dotPassedClass
                                    : 'border-border bg-card',
                              )}
                            />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex items-start justify-between px-2 text-[11px]">
                    {manualPoints.map((point, index) => (
                      <span
                        key={`${point.key}-label`}
                        className={clsx(
                          'w-6 text-center truncate',
                          point.colorClass,
                          selectedManualPointIndex === index ? 'font-bold' : 'opacity-50',
                        )}
                      >
                        {/* BS1 的四个点就是固件的挡位 1..4，按子级别编号会全部显示成"1档" */}
                        {t('fanCurve.manualGear.pointIndex', { index: (isBs1 ? point.gearIndex : point.levelIndex) + 1 })}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* ── Manual gear RPM editor dialog ── */}
        <Dialog open={gearEditOpen} onOpenChange={setGearEditOpen}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.manualGear.editTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.manualGear.editHint', { max: MANUAL_GEAR_RPM_MAX })}</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              {MANUAL_GEAR_PRESETS.map((preset) => (
                <div key={preset.gear} className="rounded-xl border border-border/70 bg-background/40 p-3">
                  <div className={clsx('mb-2 text-sm font-semibold', preset.colorClass)}>{getManualGearLabel(preset.gear)}</div>
                  <div className="grid grid-cols-3 gap-2">
                    {preset.levels.map((lv) => (
                      <div key={lv.level} className="space-y-1">
                        <div className="text-[11px] text-muted-foreground">{getManualLevelLabel(lv.level)}</div>
                        <NumberInput
                          value={draftGearRpm[preset.gear]?.[lv.level] ?? lv.rpm}
                          onChange={(value) => setDraftRpm(preset.gear, lv.level, value)}
                          min={MANUAL_GEAR_RPM_MIN}
                          max={MANUAL_GEAR_RPM_MAX}
                          step={50}
                          suffix="RPM"
                        />
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
            <DialogFooter>
              <Button variant="secondary" size="sm" onClick={() => setDraftGearRpm(buildDraftFrom(null))} icon={<RotateCw className="h-3.5 w-3.5" />}>
                {t('fanCurve.manualGear.restoreDefault')}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setGearEditOpen(false)} icon={<X className="h-3.5 w-3.5" />}>
                {t('common.actions.cancel')}
              </Button>
              <Button variant="primary" size="sm" onClick={saveGearRpm} loading={gearRpmSaving} icon={<Check className="h-3.5 w-3.5" />}>
                {t('common.actions.save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* ── Chart ── */}
        <div ref={curveEditorRef}>
          <div
            ref={chartRef}
            className={clsx('relative rounded-3xl border bg-card p-4 shadow-sm', dragIndex !== null ? 'ring-2 ring-primary/40 border-primary/30' : 'border-border/70')}
          >
            <div className="h-80 md:h-96 relative">
              {curveChart}
              <TemperatureIndicator tempSource={tempSource} chartRef={chartRef} temperatureRange={temperatureRange} />
            </div>
          </div>
        </div>

        {/* 散热收益是一次性的实测报告，与学习/控温策略无关，因此单列一节。 */}
        <section className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Gauge className="h-4 w-4 text-emerald-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.benefit.title')}</div>
                  {benefitSummary && <Badge variant="success">{benefitSummary}</Badge>}
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.benefit.entryDescription')}</div>
              </div>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setBenefitOpen(true)}
              icon={<Gauge className="h-3.5 w-3.5" />}
            >
              {t('fanCurve.benefit.entryButton')}
            </Button>
          </div>
        </section>

        <CoolingBenefit
          open={benefitOpen}
          onOpenChange={setBenefitOpen}
          config={config}
          deviceModel={deviceModel}
          isConnected={isConnected}
        />

        <section className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Sparkles className="h-4 w-4 text-amber-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.learning.title')}</div>
                  {!smartControl.learning && <Badge variant="info">{t('fanCurve.learning.paused')}</Badge>}
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.description')}</div>
              </div>
            </div>
            <ToggleSwitch
              enabled={!!smartControl.learning}
              onChange={handleLearningToggle}
              loading={learningConfigLoading}
              size="sm"
              color="purple"
              srLabel={t('fanCurve.learning.toggleAria')}
            />
          </div>

          <div className="mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.biasTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{currentLearningBiasOption.description}</div>
              </div>
              <Select
                value={currentLearningBias}
                onChange={handleLearningBiasChange}
                options={learningBiasOptions}
                disabled={learningConfigLoading}
                size="sm"
                className="w-full md:w-44"
              />
            </div>

            <div className="flex flex-col gap-3 rounded-xl border border-border/70 bg-card/55 p-3">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.targetTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.targetDescription')}</div>
              </div>
              <div className="flex flex-col gap-3 md:flex-row md:items-center">
                <div className="min-w-0 flex-1">
                  <Slider
                    min={SMART_CONTROL_TARGET_TEMP_MIN}
                    max={SMART_CONTROL_TARGET_TEMP_MAX}
                    step={1}
                    value={targetTempDraft}
                    onChange={handleTargetTempSliderChange}
                    onChangeEnd={handleTargetTempSliderCommit}
                    valueFormatter={(value) => `${value}°C`}
                    disabled={learningConfigLoading}
                  />
                </div>
                <div className="w-full md:w-28">
                  <NumberInput
                    value={targetTempDraft}
                    onChange={handleTargetTempInputChange}
                    min={SMART_CONTROL_TARGET_TEMP_MIN}
                    max={SMART_CONTROL_TARGET_TEMP_MAX}
                    step={1}
                    suffix="°C"
                    disabled={learningConfigLoading}
                  />
                </div>
              </div>
            </div>

            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.offsetTitle')}</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.offsetDescription')}</div>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleResetLearnedOffsets}
                loading={learningResetLoading}
                disabled={!hasLearnedOffsets}
                icon={<Sparkles className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.learning.reset')}
              </Button>
            </div>

            {hasLearnedOffsets ? (
              <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground md:grid-cols-4">
                {learnedOffsetSummary.map((item) => (
                  <div key={item.index} className="rounded-lg border border-border/70 bg-card/70 px-3 py-2 tabular-nums">
                    <span>{item.temperature}°C </span>
                    <span className={clsx('font-semibold', item.value > 0 ? 'text-orange-500' : 'text-blue-500')}>
                      {item.value > 0 ? '+' : ''}{item.value} RPM
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/70 bg-card/55 px-3 py-2 text-xs text-muted-foreground">{t('fanCurve.learning.noOffsets')}</div>
            )}

            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.learning.noiseTestTitle')}</div>
                  {noiseProfileDate && <Badge variant="success">{t('fanCurve.learning.noiseTestCalibrated', { date: noiseProfileDate })}</Badge>}
                </div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.learning.noiseTestDescription')}</div>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setNoiseTestOpen(true)}
                icon={<AudioLines className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.learning.noiseTestButton')}
              </Button>
            </div>
          </div>
        </section>

        <NoiseTest
          open={noiseTestOpen}
          onOpenChange={setNoiseTestOpen}
          config={config}
          onConfigChange={onConfigChange}
          isConnected={isConnected}
        />

        <section className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Clock3 className="h-4 w-4 text-sky-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.schedule.title')}</div>
                  {currentScheduleRule && <Badge variant="info">{t('fanCurve.schedule.currentBadge')}</Badge>}
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.schedule.description')}</div>
              </div>
            </div>
            <div className="flex items-center gap-2 self-start md:self-center">
              <Button variant="secondary" size="sm" onClick={handleAddScheduleRule} disabled={featureConfigLoading || scheduleProfileOptions.length === 0} icon={<Plus className="h-3.5 w-3.5" />}>
                {t('fanCurve.schedule.addRule')}
              </Button>
              <ToggleSwitch
                enabled={!!timeCurveSchedule.enabled}
                onChange={(enabled) => updateTimeCurveSchedule({ enabled })}
                loading={featureConfigLoading}
                size="sm"
                color="blue"
                srLabel={t('fanCurve.schedule.toggleAria')}
              />
            </div>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {currentScheduleRule ? (
              <span className="rounded-full border border-sky-500/30 bg-sky-500/10 px-3 py-1 text-sky-700 dark:text-sky-300">
                {t('fanCurve.schedule.currentRule', { name: currentScheduleRule.name })}
              </span>
            ) : (
              <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1">
                {t('fanCurve.schedule.noRuleMatched')}
              </span>
            )}
            <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1">
              {t('fanCurve.schedule.autoControlHint')}
            </span>
          </div>

          {timeCurveSchedule.rules.length === 0 ? (
            <div className="mt-3 rounded-xl border border-dashed border-border/70 bg-background/35 px-4 py-6 text-center text-sm text-muted-foreground">
              {t('fanCurve.schedule.empty')}
            </div>
          ) : (
            <div className="mt-3 space-y-3">
              {timeCurveSchedule.rules.map((rule) => (
                <div key={rule.id} className="rounded-xl border border-border/70 bg-background/35 p-3">
                  <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                    <div className="grid flex-1 grid-cols-1 gap-3 md:grid-cols-4">
                      <div className="space-y-1">
                        <div className="text-[11px] font-medium text-muted-foreground">{t('fanCurve.schedule.ruleName')}</div>
                        <Input
                          value={scheduleNameDrafts[rule.id] ?? rule.name}
                          onChange={(event) => setScheduleNameDrafts((prev) => ({ ...prev, [rule.id]: event.target.value }))}
                          onBlur={() => handleScheduleNameDraftCommit(rule.id, rule.name)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.currentTarget.blur();
                            }
                          }}
                          className="h-9"
                          placeholder={t('fanCurve.schedule.ruleNamePlaceholder')}
                        />
                      </div>

                      <div className="space-y-1">
                        <div className="text-[11px] font-medium text-muted-foreground">{t('fanCurve.schedule.profileTitle')}</div>
                        <Select
                          value={rule.curveProfileId}
                          onChange={(value: string | number) => handleScheduleRuleChange(rule.id, { curveProfileId: String(value) })}
                          options={scheduleProfileOptions}
                          size="sm"
                          className="w-full"
                        />
                      </div>

                      <div className="space-y-1">
                        <div className="text-[11px] font-medium text-muted-foreground">{t('fanCurve.schedule.startTitle')}</div>
                        <Input
                          value={scheduleTimeDrafts[`${rule.id}:start`] ?? rule.startTime}
                          onChange={(event) => handleScheduleTimeDraftChange(rule.id, 'startTime', event.target.value)}
                          onBlur={() => handleScheduleTimeDraftCommit(rule.id, 'startTime', rule.startTime)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.currentTarget.blur();
                            }
                          }}
                          className="h-9"
                          placeholder="HH:mm"
                          inputMode="numeric"
                        />
                      </div>

                      <div className="space-y-1">
                        <div className="text-[11px] font-medium text-muted-foreground">{t('fanCurve.schedule.endTitle')}</div>
                        <Input
                          value={scheduleTimeDrafts[`${rule.id}:end`] ?? rule.endTime}
                          onChange={(event) => handleScheduleTimeDraftChange(rule.id, 'endTime', event.target.value)}
                          onBlur={() => handleScheduleTimeDraftCommit(rule.id, 'endTime', rule.endTime)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.currentTarget.blur();
                            }
                          }}
                          className="h-9"
                          placeholder="HH:mm"
                          inputMode="numeric"
                        />
                      </div>
                    </div>

                    <div className="flex items-center gap-2 self-start md:self-end">
                      <ToggleSwitch
                        enabled={rule.enabled}
                        onChange={(enabled) => handleScheduleRuleChange(rule.id, { enabled })}
                        loading={featureConfigLoading}
                        size="sm"
                        color="green"
                        srLabel={t('fanCurve.schedule.ruleToggleAria')}
                      />
                      <Button variant="danger" size="sm" onClick={() => handleScheduleRuleDelete(rule.id)} icon={<Trash2 className="h-3.5 w-3.5" />}>
                        {t('common.actions.delete')}
                      </Button>
                    </div>
                  </div>

                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <span className="text-[11px] font-medium text-muted-foreground">{t('fanCurve.schedule.daysTitle')}</span>
                    {weekdayOptions.map((day) => {
                      const selected = rule.weekdays.includes(day.value);
                      return (
                        <button
                          key={`${rule.id}-${day.value}`}
                          type="button"
                          onClick={() => toggleScheduleWeekday(rule.id, day.value)}
                          className={clsx(
                            'cursor-pointer rounded-full border px-2.5 py-1 text-[11px] transition-colors',
                            selected ? 'border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300' : 'border-border/70 bg-background/60 text-muted-foreground',
                          )}
                        >
                          {day.label}
                        </button>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <HistoryDetailsSection sectionRef={historyDetailsRef} timelineEvents={timelineEvents} />

        {/* ── 新增曲线方案弹窗 ── */}
        <Dialog open={createProfileDialogOpen} onOpenChange={setCreateProfileDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.createTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.createDescription')}</DialogDescription>
            </DialogHeader>
            <Input
              value={newProfileNameInput}
              onChange={(event) => setNewProfileNameInput(trimProfileNameToLimit(event.target.value))}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !profileOpLoading) void createNewProfile();
              }}
              placeholder={t('fanCurve.profiles.newNamePlaceholder')}
              className="h-10"
              autoFocus
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateProfileDialogOpen(false)} disabled={profileOpLoading}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="button" onClick={() => void createNewProfile()} loading={profileOpLoading} icon={<Plus className="h-3.5 w-3.5" />}>
                {t('fanCurve.profiles.createAction')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* ── 管理曲线方案弹窗（重命名 / 删除） ── */}
        <Dialog open={manageProfilesDialogOpen} onOpenChange={setManageProfilesDialogOpen}>
          <DialogContent className="max-h-[calc(100vh-2rem)] max-w-lg gap-3 overflow-y-auto p-5">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.manageTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.manageDescription')}</DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              <section className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground" htmlFor="curve-profile-name">
                  {t('fanCurve.profiles.renameLabel')}
                </label>
                <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                  <Input
                    id="curve-profile-name"
                    value={profileNameInput}
                    onChange={(event) => handleProfileNameInputChange(event.target.value, Boolean((event.nativeEvent as InputEvent).isComposing))}
                    onCompositionStart={handleProfileNameCompositionStart}
                    onCompositionEnd={(event) => handleProfileNameCompositionEnd(event.currentTarget.value)}
                    placeholder={t('fanCurve.profiles.namePlaceholder')}
                    className="h-9 min-w-0"
                  />
                  <Button variant="secondary" size="sm" onClick={() => void saveCurrentProfileName()} loading={profileOpLoading} icon={<Pencil className="h-3.5 w-3.5" />}>
                    {t('fanCurve.profiles.saveName')}
                  </Button>
                </div>
              </section>

              <section data-profile-transfer className="space-y-2.5 border-t border-border/70 pt-4">
                <div>
                  <span className="text-sm font-medium">{t('fanCurve.importExport.title')}</span>
                </div>
                <div className="space-y-2.5">
                  <div data-profile-export-section className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-1">
                    <div className="min-w-0">
                      <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.exportTitle')}</span>
                      <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.importExport.exportHint')}</p>
                    </div>
                    <Button variant="secondary" size="sm" onClick={() => void exportProfiles()} icon={<Clipboard className="h-3.5 w-3.5" />}>
                      {t('fanCurve.importExport.copyCode')}
                    </Button>
                  </div>

                  <div data-profile-import-section className="space-y-2 border-t border-border/60 pt-3">
                    <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.importTitle')}</span>
                    <textarea
                      value={importCode}
                      onChange={(event) => setImportCode(event.target.value)}
                      rows={2}
                      className="min-h-18 w-full resize-none rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                      placeholder={t('fanCurve.importExport.importPlaceholder')}
                    />
                    <div className="flex justify-end pt-1">
                      <Button variant="secondary" size="sm" onClick={() => void importProfiles()} loading={profileOpLoading} icon={<Download className="h-3.5 w-3.5" />}>
                        {t('fanCurve.importExport.importAction')}
                      </Button>
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </DialogContent>
        </Dialog>

        <Dialog
          open={profileSwitchDialogOpen}
          onOpenChange={(open) => {
            setProfileSwitchDialogOpen(open);
            if (!open) setPendingProfileId('');
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.unsavedSwitchTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.unsavedSwitchDescription')}</DialogDescription>
            </DialogHeader>
            <DialogFooter className="sm:justify-between">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setProfileSwitchDialogOpen(false);
                  setPendingProfileId('');
                }}
                disabled={isSaving || profileOpLoading}
              >
                {t('common.actions.cancel')}
              </Button>
              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="secondary" onClick={() => void confirmProfileSwitch('discard')} disabled={isSaving || profileOpLoading}>
                  {t('fanCurve.profiles.discardAndSwitch')}
                </Button>
                <Button type="button" onClick={() => void confirmProfileSwitch('save')} loading={isSaving || profileOpLoading} icon={<Check className="h-3.5 w-3.5" />}>
                  {t('fanCurve.profiles.saveAndSwitch')}
                </Button>
              </div>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog
          open={deleteProfileDialogOpen}
          onOpenChange={(open) => {
            setDeleteProfileDialogOpen(open);
            if (!open) setPendingDeleteProfileId('');
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t(
                  hasUnsavedChanges && pendingDeleteProfileId === activeProfileId
                    ? 'fanCurve.profiles.deleteUnsavedDescription'
                    : 'fanCurve.profiles.deleteDescription',
                  { name: pendingDeleteProfile?.name || '' },
                )}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDeleteProfileDialogOpen(false)} disabled={profileOpLoading}>
                {t('common.actions.cancel')}
              </Button>
              <Button type="button" variant="danger" onClick={() => void removeProfile()} loading={profileOpLoading} icon={<Trash2 className="h-3.5 w-3.5" />}>
                {t('common.actions.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={showLowRpmWarning} onOpenChange={setShowLowRpmWarning}>
          <DialogContent
            hideClose
            overlayClassName="bg-black/40 backdrop-blur-sm"
            className="max-w-md rounded-2xl border border-border p-0 shadow-xl"
            onPointerDownOutside={(event) => event.preventDefault()}
            onEscapeKeyDown={(event) => event.preventDefault()}
          >
            <div className="p-6">
              <DialogHeader className="items-center text-center">
                <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                  <TriangleAlert className="h-8 w-8 text-amber-600" />
                </div>
                <DialogTitle className="text-lg font-bold text-foreground">{t('fanCurve.warning.title')}</DialogTitle>
                <DialogDescription asChild>
                  <div className="mt-1 rounded-xl border border-amber-300/40 bg-amber-500/10 p-4 text-left text-sm leading-relaxed text-foreground">
                    {t('fanCurve.warning.body')}
                  </div>
                </DialogDescription>
              </DialogHeader>

              <DialogFooter className="mt-6">
                <Button variant="secondary" size="sm" onClick={() => setShowLowRpmWarning(false)}>
                  {t('fanCurve.warning.confirm')}
                </Button>
              </DialogFooter>
            </div>
          </DialogContent>
        </Dialog>
      </div>
  );
});

export default FanCurve;

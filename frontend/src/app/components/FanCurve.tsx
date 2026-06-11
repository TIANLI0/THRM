'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
import { motion, AnimatePresence } from 'framer-motion';
import {
  RotateCw,
  Check,
  Clock3,
  History,
  Info,
  Spline,
  TriangleAlert,
  Plus,
  Trash2,
  Clipboard,
  Download,
  Sparkles,
  Upload,
  Pencil,
  X,
  AudioLines,
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { useLocale } from '../lib/i18n';
import { type HistorySeriesKey } from '../lib/temperature-history';
import type { CurveFocusTarget } from '../store/app-store';
import { types } from '../../../wailsjs/go/models';
import { BS1_MANUAL_GEAR_PRESETS, getManualGearLabel, getManualLevelLabel, MANUAL_GEAR_PRESETS, getEffectiveManualGearPresets, normalizeManualGearRpmMap, MANUAL_GEAR_RPM_MAX, MANUAL_GEAR_RPM_MIN, type ManualGearRpmMap } from '../lib/manualGearPresets';
import { useTranslation } from 'react-i18next';
import FanCurveProfileSelect from './FanCurveProfileSelect';
import NoiseTest from './NoiseTest';
import { toast } from 'sonner';
import { ToggleSwitch, Button, Badge, Select, Slider, NumberInput, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './ui/index';
import clsx from 'clsx';

const LOW_RPM_WARNING_DATE_KEY = 'fanCurveLowRpmWarningDate';
const FAN_CURVE_MIN_TEMP = 30;
const FAN_CURVE_MAX_TEMP = 110;
const FAN_CURVE_TEMP_STEP = 5;
const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
const SMART_CONTROL_TARGET_TEMP_MIN = 45;
const SMART_CONTROL_TARGET_TEMP_MAX = 90;
type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };
type TimeCurveScheduleRuleView = {
  id: string;
  name: string;
  enabled: boolean;
  weekdays: number[];
  startTime: string;
  endTime: string;
  curveProfileId: string;
};

type TimeCurveScheduleView = {
  enabled: boolean;
  rules: TimeCurveScheduleRuleView[];
};

const LEARNING_BIAS_OPTIONS = [
  { value: 'balanced', labelKey: 'fanCurve.learning.biasOptions.balanced.label', descriptionKey: 'fanCurve.learning.biasOptions.balanced.description' },
  { value: 'cooling', labelKey: 'fanCurve.learning.biasOptions.cooling.label', descriptionKey: 'fanCurve.learning.biasOptions.cooling.description' },
  { value: 'quiet', labelKey: 'fanCurve.learning.biasOptions.quiet.label', descriptionKey: 'fanCurve.learning.biasOptions.quiet.description' },
];

const DEFAULT_SPEED_AVOIDANCE = {
  enabled: false,
  minRpm: 1900,
  maxRpm: 2200,
  marginRpm: 100,
  emergencyBypassTemp: 80,
};

const DEFAULT_SCHEDULE_RULE = {
  enabled: true,
  weekdays: [1, 2, 3, 4, 5, 6, 0],
  startTime: '22:00',
  endTime: '07:00',
};

const WEEKDAY_SEQUENCE = [1, 2, 3, 4, 5, 6, 0];

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function normalizeLearningBias(value: unknown): string {
  return LEARNING_BIAS_OPTIONS.some((option) => option.value === value) ? String(value) : 'balanced';
}

function constrainOffsetByLearningBias(offset: number, learningBias: string) {
  if (learningBias === 'cooling' && offset < 0) return 0;
  if (learningBias === 'quiet' && offset > 0) return 0;
  return offset;
}

function normalizeTargetTemp(value: number) {
  return Math.max(SMART_CONTROL_TARGET_TEMP_MIN, Math.min(SMART_CONTROL_TARGET_TEMP_MAX, Math.round(value)));
}

function normalizeClockValue(value: string | undefined, fallback: string) {
  if (!value || !/^\d{2}:\d{2}$/.test(value)) {
    return fallback;
  }
  return value;
}

function normalizeWeekdayList(days: number[] | undefined) {
  if (!Array.isArray(days)) {
    return [...DEFAULT_SCHEDULE_RULE.weekdays];
  }
  const unique = Array.from(new Set(days.filter((day) => Number.isInteger(day) && day >= 0 && day <= 6)));
  return unique.length > 0 ? WEEKDAY_SEQUENCE.filter((day) => unique.includes(day)) : [...DEFAULT_SCHEDULE_RULE.weekdays];
}

function sanitizeTimeDraftInput(value: string) {
  let result = '';
  let colonUsed = false;
  for (const char of value) {
    if (/\d/.test(char)) {
      result += char;
      continue;
    }
    if (char === ':' && !colonUsed) {
      result += char;
      colonUsed = true;
    }
  }
  return result.slice(0, 5);
}

function normalizeTimeDraftValue(value: string, fallback: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return fallback;
  }

  let hoursPart = '';
  let minutesPart = '';
  if (trimmed.includes(':')) {
    const [rawHours = '', rawMinutes = ''] = trimmed.split(':', 2);
    hoursPart = rawHours;
    minutesPart = rawMinutes;
  } else {
    const digitsOnly = trimmed.replace(/\D/g, '');
    if (digitsOnly.length <= 2) {
      hoursPart = digitsOnly;
      minutesPart = '00';
    } else {
      hoursPart = digitsOnly.slice(0, digitsOnly.length - 2);
      minutesPart = digitsOnly.slice(-2);
    }
  }

  if (!hoursPart) {
    return fallback;
  }

  const hours = Number(hoursPart);
  const minutes = Number(minutesPart || '0');
  if (!Number.isFinite(hours) || !Number.isFinite(minutes)) {
    return fallback;
  }
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) {
    return fallback;
  }
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
}

function ruleMatchesNow(rule: {
  enabled?: boolean;
  weekdays?: number[];
  startTime?: string;
  endTime?: string;
}, now: Date) {
  if (!rule.enabled) {
    return false;
  }

  const toMinutes = (value: string | undefined) => {
    const normalized = normalizeClockValue(value, '');
    if (!normalized) return null;
    const [hours, minutes] = normalized.split(':').map((part) => Number(part));
    if (!Number.isFinite(hours) || !Number.isFinite(minutes)) return null;
    return hours * 60 + minutes;
  };

  const startMinutes = toMinutes(rule.startTime);
  const endMinutes = toMinutes(rule.endTime);
  if (startMinutes === null || endMinutes === null) {
    return false;
  }

  const days = normalizeWeekdayList(rule.weekdays);
  const weekday = now.getDay();
  const previousWeekday = (weekday + 6) % 7;
  const currentMinutes = now.getHours() * 60 + now.getMinutes();

  if (startMinutes === endMinutes) {
    return days.includes(weekday);
  }
  if (startMinutes < endMinutes) {
    return days.includes(weekday) && currentMinutes >= startMinutes && currentMinutes < endMinutes;
  }
  if (currentMinutes >= startMinutes) {
    return days.includes(weekday);
  }
  return days.includes(previousWeekday) && currentMinutes < endMinutes;
}

function syncCurveRpmAtIndex(
  curve: types.FanCurvePoint[],
  index: number,
  targetRpm: number,
  minRpm: number,
  maxRpm: number,
) {
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve, changed: false, hasLowRpmPoint: false };
  }

  const normalizedRpm = Math.max(minRpm, Math.min(maxRpm, Math.round(targetRpm / 50) * 50));
  const nextCurve = [...curve];
  let changed = false;

  if (currentPoint.rpm !== normalizedRpm) {
    nextCurve[index] = { ...currentPoint, rpm: normalizedRpm };
    changed = true;
  }

  for (let left = index - 1; left >= 0; left -= 1) {
    if (nextCurve[left].rpm <= nextCurve[left + 1].rpm) {
      break;
    }

    nextCurve[left] = {
      ...nextCurve[left],
      rpm: nextCurve[left + 1].rpm,
    };
    changed = true;
  }

  for (let right = index + 1; right < nextCurve.length; right += 1) {
    if (nextCurve[right].rpm >= nextCurve[right - 1].rpm) {
      break;
    }

    nextCurve[right] = {
      ...nextCurve[right],
      rpm: nextCurve[right - 1].rpm,
    };
    changed = true;
  }

  return {
    curve: nextCurve,
    changed,
    hasLowRpmPoint: nextCurve.some((point) => point.rpm < 1000),
  };
}

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  deviceModel: string | null;
  focusTarget: CurveFocusTarget | null;
  onFocusHandled: () => void;
}

function formatHistoryTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatHistoryDateTime(timestamp: number, locale: string) {
  return new Date(timestamp).toLocaleTimeString(locale, {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatHistoryDuration(
  startTimestamp: number,
  endTimestamp: number,
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  const durationMs = Math.max(0, endTimestamp - startTimestamp);
  if (durationMs < 60_000) {
    return t('fanCurve.history.duration.ltOneMinute');
  }
  const totalMinutes = Math.round(durationMs / 60_000);
  if (totalMinutes < 60) {
    return t('fanCurve.history.duration.minutes', { count: totalMinutes });
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes > 0
    ? t('fanCurve.history.duration.hoursAndMinutes', { hours, minutes })
    : t('fanCurve.history.duration.hours', { hours });
}

/* ── Temperature indicator overlay (memo, doesn't re-render chart) ── */

const TemperatureIndicator = memo(function TemperatureIndicator({
  temperature,
  chartRef,
  temperatureRange,
}: {
  temperature: number | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
}) {
  const { t } = useTranslation();
  const [position, setPosition] = useState<{ x: number; top: number; height: number } | null>(null);

  useEffect(() => {
    if (temperature === null || !chartRef.current) { setPosition(null); return; }
    const updatePosition = () => {
      const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
      if (!chartArea) return;
      const rect = chartArea.getBoundingClientRect();
      const containerRect = chartRef.current!.querySelector('.recharts-responsive-container')?.getBoundingClientRect();
      if (!containerRect) return;
      const chartWidth = rect.width;
      const chartLeft = rect.left - containerRect.left;
      const tempPercent = (temperature - temperatureRange.min) / (temperatureRange.max - temperatureRange.min);
      const x = chartLeft + tempPercent * chartWidth;
      setPosition({ x, top: rect.top - containerRect.top, height: rect.height });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    return () => window.removeEventListener('resize', updatePosition);
  }, [temperature, chartRef, temperatureRange]);

  if (!position || temperature === null) return null;

  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={2} strokeDasharray="5 5" />
      <rect x={position.x - 45} y={position.top - 22} width={90} height={20} rx={4} fill="var(--chart-temperature-indicator)" />
      <text x={position.x} y={position.top - 8} textAnchor="middle" fill="white" fontSize={11} fontWeight={500}>{t('fanCurve.chart.currentTemperature', { temperature })}</text>
    </svg>
  );
});

/* ── Tooltip label helper ── */

const ConfigTooltipLabel = memo(function ConfigTooltipLabel({ label, description }: { label: string; description: string }) {
  const { t } = useTranslation();

  return (
    <span className="inline-flex items-center gap-1">
      <span>{label}</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="inline-flex cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground" aria-label={t('fanCurve.chart.tooltipDescriptionAria', { label })}>
            <Info className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent className="max-w-[260px] leading-relaxed">{description}</TooltipContent>
      </Tooltip>
    </span>
  );
});

/* ── Draggable chart point ── */

const DraggablePoint = memo(function DraggablePoint({
  cx, cy, index, rpm, onDragStart, isActive,
}: {
  cx: number; cy: number; index: number; temperature: number; rpm: number;
  onDragStart: (index: number) => void; isActive: boolean;
}) {
  const handleMouseDown = useCallback((e: React.MouseEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);
  const handleTouchStart = useCallback((e: React.TouchEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);

  return (
    <g>
      <circle cx={cx} cy={cy} r={isActive ? 14 : 10} fill="transparent" stroke="transparent" style={{ cursor: 'ns-resize' }} onMouseDown={handleMouseDown} onTouchStart={handleTouchStart} />
      <circle cx={cx} cy={cy} r={isActive ? 8 : 6} fill={isActive ? 'var(--chart-primary-active)' : 'var(--chart-primary)'} stroke="var(--card)" strokeWidth={2}
        style={{ cursor: 'ns-resize', transition: isActive ? 'none' : 'all 0.2s ease', filter: isActive ? 'drop-shadow(0 4px 8px var(--chart-primary-glow))' : 'drop-shadow(0 2px 4px var(--chart-point-shadow))' }}
        onMouseDown={handleMouseDown} onTouchStart={handleTouchStart}
      />
      {isActive && (
        <g>
          <rect x={cx - 35} y={cy - 35} width={70} height={24} rx={4} fill="var(--chart-primary-active)" opacity={0.95} />
          <text x={cx} y={cy - 19} textAnchor="middle" fill="white" fontSize={12} fontWeight={600}>{rpm} RPM</text>
        </g>
      )}
    </g>
  );
});

/* ═══════════════════════════════════════════════════════════
   ─── Main FanCurve Component ───
   ═══════════════════════════════════════════════════════════ */

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, fanData, temperature, deviceModel, focusTarget, onFocusHandled }: FanCurveProps) {
  const { t } = useTranslation();
  const { locale } = useLocale();
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState('');
  const [profileNameInput, setProfileNameInput] = useState('');
  const [isProfileNameComposing, setIsProfileNameComposing] = useState(false);
  const [profileOpLoading, setProfileOpLoading] = useState(false);
  const [exportCode, setExportCode] = useState('');
  const [importCode, setImportCode] = useState('');
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [learningConfigLoading, setLearningConfigLoading] = useState(false);
  const [learningResetLoading, setLearningResetLoading] = useState(false);
  const [noiseTestOpen, setNoiseTestOpen] = useState(false);
  const [featureConfigLoading, setFeatureConfigLoading] = useState(false);
  const [scheduleTimeDrafts, setScheduleTimeDrafts] = useState<Record<string, string>>({});
  const [profileManagerOpen, setProfileManagerOpen] = useState(false);
  const [profileCreateOpen, setProfileCreateOpen] = useState(false);
  const [newProfileName, setNewProfileName] = useState('');
  const [renameDrafts, setRenameDrafts] = useState<Record<string, string>>({});
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [showLowRpmWarning, setShowLowRpmWarning] = useState(false);
  const [historySeriesVisibility, setHistorySeriesVisibility] = useState<Record<HistorySeriesKey, boolean>>({
    cpu: true,
    gpu: true,
    fan: true,
  });
  const chartRef = useRef<HTMLDivElement>(null);
  const curveEditorRef = useRef<HTMLDivElement>(null);
  const historyDetailsRef = useRef<HTMLElement>(null);
  const lowRpmWarnedInDragRef = useRef(false);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
  const [rpmRange, setRpmRange] = useState({ min: 0, max: 4000, ticks: [0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000] });
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    saving: temperatureHistorySaving,
    setEnabled: setTemperatureHistoryEnabled,
  } = useTemperatureHistory();

  const activeProfile = useMemo(() => curveProfiles.find((p) => p.id === activeProfileId) ?? null, [curveProfiles, activeProfileId]);
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
      return { enabled: true, learning: true, learningBias: 'balanced', filterTransientSpike: true, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 50, rampUpLimit: 220, rampDownLimit: 160, learnRate: 3, learnWindow: 8, learnDelay: 3, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, trendGain: 5, maxLearnOffset: 300, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
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
      noiseWeight: existing.noiseWeight ?? 4, trendGain: existing.trendGain ?? 5,
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
  const speedAvoidance = useMemo(() => {
    const existing = (config as any).speedAvoidance;
    return {
      enabled: existing?.enabled ?? DEFAULT_SPEED_AVOIDANCE.enabled,
      minRpm: existing?.minRpm ?? DEFAULT_SPEED_AVOIDANCE.minRpm,
      maxRpm: existing?.maxRpm ?? DEFAULT_SPEED_AVOIDANCE.maxRpm,
      marginRpm: existing?.marginRpm ?? DEFAULT_SPEED_AVOIDANCE.marginRpm,
      emergencyBypassTemp: existing?.emergencyBypassTemp ?? DEFAULT_SPEED_AVOIDANCE.emergencyBypassTemp,
    };
  }, [config]);
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
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      target.scrollIntoView({ block: 'start' });
      onFocusHandled();
    });

    return () => {
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

  const detailHistoryPoints = useMemo(() => temperatureHistory.slice(-720), [temperatureHistory]);

  const historyTempDomain = useMemo<[number, number]>(() => {
    const values = detailHistoryPoints.flatMap((point) => [point.cpuTemp, point.gpuTemp]).filter((value) => value > 0);
    if (values.length === 0) {
      return [30, 90];
    }
    const min = Math.max(0, Math.floor((Math.min(...values) - 4) / 5) * 5);
    const max = Math.min(110, Math.ceil((Math.max(...values) + 4) / 5) * 5);
    return [min, Math.max(min + 10, max)];
  }, [detailHistoryPoints]);

  const historyFanMax = useMemo(() => {
    return Math.max(4000, ...detailHistoryPoints.map((point) => point.fanRpm).filter((value) => value > 0), 0);
  }, [detailHistoryPoints]);

  const historySummary = useMemo(() => {
    const latest = temperatureHistory[temperatureHistory.length - 1] ?? null;
    const first = temperatureHistory[0] ?? null;
    const cpuValues = temperatureHistory.map((point) => point.cpuTemp).filter((value) => value > 0);
    const gpuValues = temperatureHistory.map((point) => point.gpuTemp).filter((value) => value > 0);
    const fanValues = temperatureHistory.map((point) => point.fanRpm).filter((value) => value > 0);
    const average = (values: number[]) => values.length > 0 ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0;

    return {
      sampleCount: temperatureHistory.length,
      latest,
      latestLabel: latest ? formatHistoryDateTime(latest.timestamp, locale) : '--',
      durationLabel: first && latest ? formatHistoryDuration(first.timestamp, latest.timestamp, t) : '--',
      cpuPeak: cpuValues.length > 0 ? Math.max(...cpuValues) : 0,
      cpuAverage: average(cpuValues),
      gpuPeak: gpuValues.length > 0 ? Math.max(...gpuValues) : 0,
      gpuAverage: average(gpuValues),
      fanPeak: fanValues.length > 0 ? Math.max(...fanValues) : 0,
      fanAverage: average(fanValues),
    };
  }, [locale, t, temperatureHistory]);
  const historyChartData = useMemo(() => detailHistoryPoints.map((point) => ({ ...point })), [detailHistoryPoints]);

  const historySeriesMeta = useMemo(() => ([
    { key: 'cpu' as const, label: t('fanCurve.history.series.cpu'), color: '#2f6df6' },
    { key: 'gpu' as const, label: t('fanCurve.history.series.gpu'), color: '#f97316' },
    { key: 'fan' as const, label: t('fanCurve.history.series.fan'), color: '#10b981' },
  ]), [t, locale]);

  const toggleHistorySeries = useCallback((series: HistorySeriesKey) => {
    setHistorySeriesVisibility((prev) => ({
      ...prev,
      [series]: !prev[series],
    }));
  }, []);

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

  const switchProfile = useCallback(async (id: string) => {
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

  const saveCurrentProfileName = useCallback(async () => {
    const fallbackName = activeProfile?.name || t('fanCurve.profiles.currentCurveName');
    const safeName = getSafeProfileName(profileNameInput, fallbackName);
    try {
      setProfileOpLoading(true);
      const profileCurve = activeProfile?.curve || localCurve;
      await apiService.saveFanCurveProfile(activeProfileId, safeName, profileCurve, false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileRenamed'));
    } catch (e) {
      toast.error(t('fanCurve.toast.renameFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.curve, activeProfile?.name, activeProfileId, getSafeProfileName, loadCurveProfiles, localCurve, profileNameInput, syncConfigFromBackend, t]);

  const createNewProfile = useCallback(async () => {
    const rawName = (profileNameInput || '').trim();
    const activeName = (activeProfile?.name || '').trim();
    const shouldUseDefaultNewName = !rawName || rawName === activeName;
    const newProfileName = t('fanCurve.profiles.newCurveName');
    const safeName = shouldUseDefaultNewName ? newProfileName : getSafeProfileName(rawName, newProfileName);
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', safeName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setProfileNameInput('');
      toast.success(t('fanCurve.toast.profileSavedAsNew'));
    } catch (e) {
      toast.error(t('fanCurve.toast.saveAsFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.name, getSafeProfileName, loadCurveProfiles, localCurve, profileNameInput, syncConfigFromBackend, t]);

  const removeActiveProfile = useCallback(async () => {
    if (!activeProfileId) return;
    try {
      setProfileOpLoading(true);
      await apiService.deleteFanCurveProfile(activeProfileId);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileDeleted'));
    } catch (e) {
      toast.error(t('fanCurve.toast.deleteFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfileId, loadCurveProfiles, syncConfigFromBackend, t]);

  // 删除指定 ID 的曲线方案（用于"管理曲线方案"弹窗中按行删除）。
  const removeProfileById = useCallback(async (profileID: string) => {
    if (!profileID) return;
    if (curveProfiles.length <= 1) {
      toast.error(t('fanCurve.profiles.deleteLastError'));
      return;
    }
    try {
      setProfileOpLoading(true);
      await apiService.deleteFanCurveProfile(profileID);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileDeleted'));
    } catch (e) {
      toast.error(t('fanCurve.toast.deleteFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [curveProfiles.length, loadCurveProfiles, syncConfigFromBackend, t]);

  // 用新名字保存指定 ID 的曲线（不改变其曲线本体）。
  const renameProfileById = useCallback(async (profileID: string, name: string) => {
    const target = curveProfiles.find((p) => p.id === profileID);
    if (!target) return;
    const fallback = target.name || t('fanCurve.profiles.currentCurveName');
    const safeName = getSafeProfileName(name, fallback);
    if (!safeName || safeName === target.name) return;
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile(profileID, safeName, target.curve || [], false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileRenamed'));
    } catch (e) {
      toast.error(t('fanCurve.toast.renameFailed', { error: getErrorMessage(e) }));
    } finally {
      setProfileOpLoading(false);
    }
  }, [curveProfiles, getSafeProfileName, loadCurveProfiles, syncConfigFromBackend, t]);

  // 以指定名称基于"当前画布上的曲线"创建一个新方案（弹窗里"新增"流程）。
  const createProfileWithName = useCallback(async (rawName: string) => {
    const fallbackName = t('fanCurve.profiles.newCurveName');
    const safeName = getSafeProfileName(rawName, fallbackName);
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', safeName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success(t('fanCurve.toast.profileSavedAsNew'));
      return true;
    } catch (e) {
      toast.error(t('fanCurve.toast.saveAsFailed', { error: getErrorMessage(e) }));
      return false;
    } finally {
      setProfileOpLoading(false);
    }
  }, [getSafeProfileName, loadCurveProfiles, localCurve, syncConfigFromBackend, t]);

  const exportProfiles = useCallback(async () => {
    try {
      if (hasUnsavedChanges) {
        const ok = await persistCurrentCurve();
        if (!ok) {
          return;
        }
        await loadCurveProfiles();
      }
      const code = await apiService.exportFanCurveProfiles();
      setExportCode(code);
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
        toast.success(t('fanCurve.toast.exportCopied'));
      } else {
        toast.success(t('fanCurve.toast.exportGenerated'));
      }
    } catch (e) {
      toast.error(t('fanCurve.toast.exportFailed', { error: getErrorMessage(e) }));
    }
  }, [hasUnsavedChanges, loadCurveProfiles, persistCurrentCurve, t]);

  const importProfiles = useCallback(async () => {
    const code = importCode.trim();
    if (!code) {
      toast.error(t('fanCurve.toast.importMissingCode'));
      return;
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
  }, [importCode, loadCurveProfiles, syncConfigFromBackend, t]);

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
    setTargetTempDraft(normalizeTargetTemp(value));
  }, []);

  const handleTargetTempInputBlur = useCallback(() => {
    commitTargetTemp(targetTempDraft);
  }, [commitTargetTemp, targetTempDraft]);

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

  const updateSpeedAvoidance = useCallback((patch: Partial<types.SpeedAvoidanceConfig>) => {
    const nextSpeedAvoidance = types.SpeedAvoidanceConfig.createFrom({ ...speedAvoidance, ...patch });
    void updateFanFeatureConfig({ speedAvoidance: nextSpeedAvoidance as any });
  }, [speedAvoidance, updateFanFeatureConfig]);

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

  /* ── Reference temperature (follows settings 控温温度来源: max/cpu/gpu) ── */
  const referenceTemp = useMemo(() => {
    if (!temperature) return null;
    const source = (((config as any).tempSource as string) || 'max') as 'max' | 'cpu' | 'gpu';
    const cpu = temperature.cpuTemp ?? 0;
    const gpu = temperature.gpuTemp ?? 0;
    const max = temperature.maxTemp ?? 0;
    if (source === 'cpu') return cpu > 0 ? cpu : (max > 0 ? max : null);
    if (source === 'gpu') return gpu > 0 ? gpu : (max > 0 ? max : null);
    return max > 0 ? max : null;
  }, [temperature, config]);

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
    })));
  }, [manualGearPresets]);

  const selectedManualPointIndex = useMemo(() => {
    const selected = manualPoints.findIndex((p) => p.gear === (config.manualGear || '标准') && p.level === (config.manualLevel || '中'));
    return selected >= 0 ? selected : 4;
  }, [config.manualGear, config.manualLevel, manualPoints]);

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
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <Spline className="h-4 w-4 text-primary" />
              <h2 className="text-base font-semibold text-foreground">{t('fanCurve.title')}</h2>
              {hasUnsavedChanges && <Badge variant="warning">{t('fanCurve.badges.unsaved')}</Badge>}
              {isInteracting && <Badge variant="info">{t('fanCurve.badges.editing')}</Badge>}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <FanCurveProfileSelect
                profiles={curveProfiles}
                activeProfileId={activeProfileId}
                onChange={switchProfile}
                loading={profileOpLoading}
                onAddNew={() => {
                  setNewProfileName('');
                  setProfileCreateOpen(true);
                }}
                onManage={() => {
                  // 进入管理弹窗时，把当前所有曲线的名字注入草稿，便于直接编辑。
                  const drafts: Record<string, string> = {};
                  curveProfiles.forEach((profile) => { drafts[profile.id] = profile.name; });
                  setRenameDrafts(drafts);
                  setConfirmDeleteId(null);
                  setProfileManagerOpen(true);
                }}
              />
              <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label={t('fanCurve.actions.smartControl')} size="sm" color="blue" />
              <div className="flex items-center gap-2">
                <Button variant="secondary" size="sm" onClick={resetCurve} icon={<RotateCw className="h-3.5 w-3.5" />}>
                  {t('fanCurve.actions.reset')}
                </Button>
                <Button variant="primary" size="sm" onClick={saveCurve} disabled={!hasUnsavedChanges} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>
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
                          'cursor-pointer rounded-xl border px-3 py-2.5 text-left transition-colors',
                          isActiveGear ? `${preset.borderClass} ${preset.bgClass}` : 'border-border/70 bg-background/40 hover:bg-muted/35',
                        )}
                      >
                        <div className={clsx('text-lg font-bold', isActiveGear ? preset.colorClass : 'text-foreground')}>{getManualGearLabel(preset.gear)}</div>
                        {!isBs1 && <div className={clsx('mt-1 text-base font-semibold', preset.colorClass)}>{activeLevel.rpm}RPM</div>}
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
                                'block h-4 w-4 rounded-full border border-border/80 bg-card transition-transform duration-150',
                                isActivePoint ? `scale-125 ${point.borderClass} ${point.bgClass}` : '',
                                isPassed && !isActivePoint ? point.bgClass : '',
                              )}
                            />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex items-start justify-between px-2 text-[11px]">
                    {manualPoints.map((point) => (
                      <span key={`${point.key}-label`} className={clsx('w-6 text-center truncate', point.colorClass)}>
                        {t('fanCurve.manualGear.pointIndex', { index: point.levelIndex + 1 })}
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
              <TemperatureIndicator temperature={referenceTemp} chartRef={chartRef} temperatureRange={temperatureRange} />
            </div>
          </div>
        </div>

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
                    onBlur={handleTargetTempInputBlur}
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
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <TriangleAlert className="h-4 w-4 text-orange-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">{t('fanCurve.avoidance.title')}</div>
                  {speedAvoidance.enabled && <Badge variant="warning">{t('fanCurve.avoidance.badge')}</Badge>}
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">{t('fanCurve.avoidance.description')}</div>
              </div>
            </div>
            <ToggleSwitch
              enabled={!!speedAvoidance.enabled}
              onChange={(enabled) => updateSpeedAvoidance({ enabled })}
              loading={featureConfigLoading}
              size="sm"
              color="orange"
              srLabel={t('fanCurve.avoidance.toggleAria')}
            />
          </div>

          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-4">
            <div className="flex h-full flex-col rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.avoidance.minTitle')}</div>
              <div className="mt-1 min-h-10 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.avoidance.minDescription')}</div>
              <div className="mt-3">
                <NumberInput
                  value={speedAvoidance.minRpm}
                  onChange={(value) => updateSpeedAvoidance({ minRpm: value })}
                  min={800}
                  max={4500}
                  step={50}
                  suffix="RPM"
                  disabled={featureConfigLoading}
                />
              </div>
            </div>

            <div className="flex h-full flex-col rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.avoidance.maxTitle')}</div>
              <div className="mt-1 min-h-10 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.avoidance.maxDescription')}</div>
              <div className="mt-3">
                <NumberInput
                  value={speedAvoidance.maxRpm}
                  onChange={(value) => updateSpeedAvoidance({ maxRpm: value })}
                  min={800}
                  max={4500}
                  step={50}
                  suffix="RPM"
                  disabled={featureConfigLoading}
                />
              </div>
            </div>

            <div className="flex h-full flex-col rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.avoidance.marginTitle')}</div>
              <div className="mt-1 min-h-10 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.avoidance.marginDescription')}</div>
              <div className="mt-3">
                <NumberInput
                  value={speedAvoidance.marginRpm}
                  onChange={(value) => updateSpeedAvoidance({ marginRpm: value })}
                  min={50}
                  max={500}
                  step={50}
                  suffix="RPM"
                  disabled={featureConfigLoading}
                />
              </div>
            </div>

            <div className="flex h-full flex-col rounded-xl border border-border/70 bg-background/45 p-3">
              <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.avoidance.bypassTitle')}</div>
              <div className="mt-1 min-h-10 text-xs leading-relaxed text-muted-foreground">{t('fanCurve.avoidance.bypassDescription')}</div>
              <div className="mt-3">
                <NumberInput
                  value={speedAvoidance.emergencyBypassTemp}
                  onChange={(value) => updateSpeedAvoidance({ emergencyBypassTemp: value })}
                  min={60}
                  max={95}
                  step={1}
                  suffix="°C"
                  disabled={featureConfigLoading}
                />
              </div>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1">
              {t('fanCurve.avoidance.summary', {
                min: Math.min(speedAvoidance.minRpm, speedAvoidance.maxRpm),
                max: Math.max(speedAvoidance.minRpm, speedAvoidance.maxRpm),
                margin: speedAvoidance.marginRpm,
              })}
            </span>
            {speedAvoidance.enabled && fanData?.targetRpm && fanData.targetRpm >= Math.min(speedAvoidance.minRpm, speedAvoidance.maxRpm) && fanData.targetRpm <= Math.max(speedAvoidance.minRpm, speedAvoidance.maxRpm) && (
              <span className="rounded-full border border-orange-500/40 bg-orange-500/10 px-3 py-1 text-orange-600 dark:text-orange-300">
                {t('fanCurve.avoidance.activeHint', { rpm: fanData.targetRpm })}
              </span>
            )}
            {!config.autoControl && (
              <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1">
                {t('fanCurve.avoidance.autoOnlyHint')}
              </span>
            )}
          </div>
        </section>

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
                          value={rule.name}
                          onChange={(event) => handleScheduleRuleChange(rule.id, { name: event.target.value })}
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

        <section ref={historyDetailsRef} className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
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
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                {[
                  [t('fanCurve.history.summary.cpuPeak'), historySummary.cpuPeak ? `${historySummary.cpuPeak}°C` : '--', historySummary.cpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.cpuAverage }) : t('fanCurve.history.summary.noCpuTemperature')],
                  [t('fanCurve.history.summary.gpuPeak'), historySummary.gpuPeak ? `${historySummary.gpuPeak}°C` : '--', historySummary.gpuAverage ? t('fanCurve.history.summary.averageTemperature', { value: historySummary.gpuAverage }) : t('fanCurve.history.summary.noGpuTemperature')],
                  [t('fanCurve.history.summary.fanPeak'), historySummary.fanPeak ? `${historySummary.fanPeak} RPM` : '--', historySummary.fanAverage ? t('fanCurve.history.summary.averageFan', { value: historySummary.fanAverage }) : t('fanCurve.history.summary.noFanData')],
                ].map(([label, value, hint]) => (
                  <div key={label} className="rounded-xl border border-border/70 bg-background/35 p-3">
                    <div className="text-[11px] text-muted-foreground">{label}</div>
                    <div className="mt-1 text-sm font-semibold text-foreground">{value}</div>
                    <div className="mt-1 text-[11px] text-muted-foreground">{hint}</div>
                  </div>
                ))}
              </div>

              <div className="rounded-xl border border-border/70 bg-background/35 p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-xs font-medium text-muted-foreground">{t('fanCurve.history.recentTrend')}</div>
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
                  <div className="h-72">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={historyChartData} margin={{ top: 12, right: 16, left: 4, bottom: 8 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                        <XAxis
                          dataKey="timestamp"
                          type="number"
                          domain={['dataMin', 'dataMax']}
                          tickFormatter={(value) => formatHistoryTime(Number(value), locale)}
                          tickLine={false}
                          minTickGap={24}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                        />
                        <YAxis
                          yAxisId="temp"
                          type="number"
                          domain={historyTempDomain}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={40}
                        />
                        <YAxis
                          yAxisId="fan"
                          orientation="right"
                          type="number"
                          domain={[0, historyFanMax]}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={52}
                        />
                        <RechartsTooltip
                          labelFormatter={(value) => formatHistoryDateTime(Number(value), locale)}
                          formatter={(value, name) => {
                            const numericValue = Number(value ?? 0);
                            if (name === 'fanRpm') {
                              return [`${numericValue} RPM`, t('fanCurve.history.series.fan')];
                            }
                            return [`${numericValue} °C`, name === 'cpuTemp' ? t('fanCurve.history.series.cpu') : t('fanCurve.history.series.gpu')];
                          }}
                          contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
                          labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
                          itemStyle={{ color: 'var(--chart-tooltip-text)' }}
                        />
                        {historySeriesVisibility.cpu && <Line yAxisId="temp" type="monotone" dataKey="cpuTemp" stroke="#2f6df6" strokeWidth={2.3} dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                        {historySeriesVisibility.gpu && <Line yAxisId="temp" type="monotone" dataKey="gpuTemp" stroke="#f97316" strokeWidth={2.3} dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                        {historySeriesVisibility.fan && <Line yAxisId="fan" type="monotone" dataKey="fanRpm" stroke="#10b981" strokeWidth={2} dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </div>
            </>
          )}
        </section>

        <section className="rounded-2xl border border-border/70 bg-card p-4 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium">{t('fanCurve.importExport.title')}</span>
            <span className="text-xs text-muted-foreground">{t('fanCurve.importExport.description')}</span>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2 rounded-xl border border-border/70 bg-background/30 p-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.exportTitle')}</span>
                <div className="flex items-center gap-2">
                  <Button variant="secondary" size="sm" onClick={exportProfiles} icon={<Download className="h-3.5 w-3.5" />}>{t('fanCurve.importExport.generate')}</Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={async () => {
                      if (!exportCode) return;
                      if (navigator?.clipboard?.writeText) {
                        await navigator.clipboard.writeText(exportCode);
                        toast.success(t('fanCurve.toast.exportCopiedAgain'));
                      }
                    }}
                    icon={<Clipboard className="h-3.5 w-3.5" />}
                    disabled={!exportCode}
                  >
                    {t('common.actions.copy')}
                  </Button>
                </div>
              </div>
              <textarea
                value={exportCode}
                readOnly
                rows={3}
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                placeholder={t('fanCurve.importExport.exportPlaceholder')}
              />
            </div>

            <div className="space-y-2 rounded-xl border border-border/70 bg-background/30 p-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-muted-foreground">{t('fanCurve.importExport.importTitle')}</span>
                <Button variant="secondary" size="sm" onClick={importProfiles} loading={profileOpLoading} icon={<Upload className="h-3.5 w-3.5" />}>{t('common.actions.import')}</Button>
              </div>
              <textarea
                value={importCode}
                onChange={(e) => setImportCode(e.target.value)}
                rows={3}
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                placeholder={t('fanCurve.importExport.importPlaceholder')}
              />
            </div>
          </div>
        </section>

        {/* ── 新增曲线方案弹窗 ── */}
        <Dialog
          open={profileCreateOpen}
          onOpenChange={(open) => {
            setProfileCreateOpen(open);
            if (!open) setNewProfileName('');
          }}
        >
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.createTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.createDescription')}</DialogDescription>
            </DialogHeader>
            <div className="space-y-2 py-1">
              <Input
                value={newProfileName}
                onChange={(e) => setNewProfileName(trimProfileNameToLimit(e.target.value))}
                placeholder={t('fanCurve.profiles.namePlaceholder')}
                className="h-10"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    void (async () => {
                      const ok = await createProfileWithName(newProfileName);
                      if (ok) {
                        setProfileCreateOpen(false);
                        setNewProfileName('');
                      }
                    })();
                  }
                }}
              />
            </div>
            <DialogFooter>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  setProfileCreateOpen(false);
                  setNewProfileName('');
                }}
                icon={<X className="h-3.5 w-3.5" />}
              >
                {t('common.actions.cancel')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                loading={profileOpLoading}
                onClick={async () => {
                  const ok = await createProfileWithName(newProfileName);
                  if (ok) {
                    setProfileCreateOpen(false);
                    setNewProfileName('');
                  }
                }}
                icon={<Check className="h-3.5 w-3.5" />}
              >
                {t('common.actions.confirm')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* ── 管理曲线方案弹窗（重命名 / 删除） ── */}
        <Dialog
          open={profileManagerOpen}
          onOpenChange={(open) => {
            setProfileManagerOpen(open);
            if (!open) setConfirmDeleteId(null);
          }}
        >
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{t('fanCurve.profiles.manageTitle')}</DialogTitle>
              <DialogDescription>{t('fanCurve.profiles.manageDescription')}</DialogDescription>
            </DialogHeader>
            <div className="space-y-2 py-1">
              {curveProfiles.map((profile) => {
                const draft = renameDrafts[profile.id] ?? profile.name;
                const isActive = profile.id === activeProfileId;
                const isDirty = draft.trim() !== profile.name;
                const canDelete = curveProfiles.length > 1;
                const isConfirming = confirmDeleteId === profile.id;
                return (
                  <div
                    key={profile.id}
                    className={clsx(
                      'flex items-center gap-2 rounded-xl border p-2',
                      isActive ? 'border-primary/40 bg-primary/5' : 'border-border/70 bg-background/40',
                    )}
                  >
                    <div className="flex-1 min-w-0">
                      <Input
                        value={draft}
                        onChange={(e) => setRenameDrafts((prev) => ({ ...prev, [profile.id]: trimProfileNameToLimit(e.target.value) }))}
                        placeholder={t('fanCurve.profiles.namePlaceholder')}
                        className="h-9"
                      />
                    </div>
                    {isActive && <Badge variant="info">{t('fanCurve.profiles.activeBadge')}</Badge>}
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={!isDirty}
                      loading={profileOpLoading && isDirty}
                      onClick={() => renameProfileById(profile.id, draft)}
                      icon={<Pencil className="h-3.5 w-3.5" />}
                    >
                      {t('fanCurve.profiles.saveName')}
                    </Button>
                    {isConfirming ? (
                      <Button
                        variant="danger"
                        size="sm"
                        loading={profileOpLoading}
                        disabled={!canDelete}
                        onClick={async () => {
                          await removeProfileById(profile.id);
                          setConfirmDeleteId(null);
                        }}
                        icon={<Check className="h-3.5 w-3.5" />}
                      >
                        {t('common.actions.confirm')}
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={!canDelete}
                        onClick={() => setConfirmDeleteId(profile.id)}
                        icon={<Trash2 className="h-3.5 w-3.5" />}
                      />
                    )}
                  </div>
                );
              })}
            </div>
            <DialogFooter>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  setProfileCreateOpen(true);
                  setNewProfileName('');
                }}
                icon={<Plus className="h-3.5 w-3.5" />}
              >
                {t('fanCurve.profiles.addAction')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={() => setProfileManagerOpen(false)}
                icon={<Check className="h-3.5 w-3.5" />}
              >
                {t('common.actions.done')}
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

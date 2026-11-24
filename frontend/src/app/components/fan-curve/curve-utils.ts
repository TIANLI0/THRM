import * as types from '@go/types';
import {
  SMART_CONTROL_TARGET_TEMP_MIN,
  SMART_CONTROL_TARGET_TEMP_MAX,
  LEARNING_BIAS_OPTIONS,
  DEFAULT_SCHEDULE_RULE,
  WEEKDAY_SEQUENCE,
} from './constants';

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function normalizeLearningBias(value: unknown): string {
  return LEARNING_BIAS_OPTIONS.some((option) => option.value === value) ? String(value) : 'balanced';
}

export function constrainOffsetByLearningBias(offset: number, learningBias: string) {
  if (learningBias === 'cooling' && offset < 0) return 0;
  if (learningBias === 'quiet' && offset > 0) return 0;
  return offset;
}

export function normalizeTargetTemp(value: number) {
  return Math.max(SMART_CONTROL_TARGET_TEMP_MIN, Math.min(SMART_CONTROL_TARGET_TEMP_MAX, Math.round(value)));
}

export function normalizeClockValue(value: string | undefined, fallback: string) {
  if (!value || !/^\d{2}:\d{2}$/.test(value)) {
    return fallback;
  }
  return value;
}

export function normalizeWeekdayList(days: number[] | undefined) {
  if (!Array.isArray(days)) {
    return [...DEFAULT_SCHEDULE_RULE.weekdays];
  }
  const unique = Array.from(new Set(days.filter((day) => Number.isInteger(day) && day >= 0 && day <= 6)));
  return unique.length > 0 ? WEEKDAY_SEQUENCE.filter((day) => unique.includes(day)) : [...DEFAULT_SCHEDULE_RULE.weekdays];
}

export function sanitizeTimeDraftInput(value: string) {
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

export function normalizeTimeDraftValue(value: string, fallback: string) {
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

export function ruleMatchesNow(rule: {
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

export function syncCurveRpmAtIndex(
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

'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { apiService } from '../services/api';
import {
  appendHistoryPoint,
  trimHistoryPoints,
  HISTORY_SAMPLE_INTERVAL_MS,
  DEFAULT_HISTORY_RETENTION_HOURS,
  clampHistoryRetentionHours,
  type TemperatureHistoryPoint,
} from '../lib/temperature-history';
import { useAppStore } from '../store/app-store';

// 一小时可容纳的样本数（按固定采样间隔换算），用于把保留小时数换算成点数上限。
const POINTS_PER_HOUR = Math.round((60 * 60 * 1000) / HISTORY_SAMPLE_INTERVAL_MS);

export function useTemperatureHistory() {
  const sessionHistoryPoints = useAppStore((state) => state.sessionHistoryPoints);
  const [points, setPoints] = useState<TemperatureHistoryPoint[]>([]);
  const [enabled, setEnabledState] = useState(false);
  const [saving, setSaving] = useState(false);
  const [retentionHours, setRetentionHoursState] = useState(DEFAULT_HISTORY_RETENTION_HOURS);
  const enabledRef = useRef(enabled);
  const retentionRef = useRef(retentionHours);
  const sessionPointsRef = useRef(sessionHistoryPoints);

  useEffect(() => {
    enabledRef.current = enabled;
  }, [enabled]);

  useEffect(() => {
    retentionRef.current = retentionHours;
  }, [retentionHours]);

  useEffect(() => {
    sessionPointsRef.current = sessionHistoryPoints;
    if (!enabledRef.current) {
      setPoints(sessionHistoryPoints);
    }
  }, [sessionHistoryPoints]);

  const loadSnapshot = useCallback(async (activeGuard?: { active: boolean }) => {
    try {
      const payload = await apiService.getTemperatureHistory();
      if (activeGuard && !activeGuard.active) {
        return;
      }

      const nextEnabled = payload?.enabled !== false;
      const nextRetention = clampHistoryRetentionHours(payload?.retentionHours);
      setEnabledState(nextEnabled);
      setRetentionHoursState(nextRetention);
      const retentionMs = nextRetention * 60 * 60 * 1000;
      const limit = nextRetention * POINTS_PER_HOUR;
      setPoints(nextEnabled
        ? trimHistoryPoints((payload?.points || []) as TemperatureHistoryPoint[], retentionMs, limit)
        : sessionPointsRef.current);
    } catch {
      if (activeGuard && !activeGuard.active) {
        return;
      }

      setEnabledState(false);
      setPoints(sessionPointsRef.current);
    }
  }, []);

  useEffect(() => {
    const activeGuard = { active: true };
    void loadSnapshot(activeGuard);
    return () => {
      activeGuard.active = false;
    };
  }, [loadSnapshot]);

  useEffect(() => {
    return apiService.onTemperatureHistoryUpdate((point) => {
      if (!enabledRef.current) {
        return;
      }
      const retentionMs = retentionRef.current * 60 * 60 * 1000;
      const limit = retentionRef.current * POINTS_PER_HOUR;
      setPoints((prev) => appendHistoryPoint(prev, point as TemperatureHistoryPoint, { retentionMs, limit }));
    });
  }, []);

  const setEnabled = useCallback(async (nextEnabled: boolean) => {
    setSaving(true);
    try {
      await apiService.setTemperatureHistoryEnabled(nextEnabled);
      await loadSnapshot();
    } catch (error) {
      console.error('设置温度历史失败:', error);
    } finally {
      setSaving(false);
    }
  }, [loadSnapshot]);

  const setRetentionHours = useCallback(async (nextHours: number) => {
    const clamped = clampHistoryRetentionHours(nextHours);
    setSaving(true);
    try {
      await apiService.setTemperatureHistoryRetentionHours(clamped);
      await loadSnapshot();
    } catch (error) {
      console.error('设置温度历史保留时长失败:', error);
    } finally {
      setSaving(false);
    }
  }, [loadSnapshot]);

  return {
    points,
    enabled,
    saving,
    retentionHours,
    setEnabled,
    setRetentionHours,
    source: enabled ? 'core' as const : 'session' as const,
    reload: loadSnapshot,
  };
}

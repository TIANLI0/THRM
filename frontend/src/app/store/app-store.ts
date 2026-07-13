import { create } from 'zustand';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { configService } from '../services/config-service';
import { deviceService, type DeviceStatusPayload } from '../services/device-service';
import {
  appendSampledHistoryPoint,
  createLiveHistoryPoint,
  SESSION_HISTORY_LIMIT,
  SESSION_HISTORY_RETENTION_MS,
} from '../lib/temperature-history';
import type { TemperatureHistoryPoint } from '../lib/temperature-history';
import { i18n } from '../lib/i18n';
import { toast } from 'sonner';
import type { DeviceSettings } from '../types/app';

const getBridgeWarningMessage = () => i18n.t('store.bridgeWarning.default');

const getCoreServiceErrorMessage = (detail?: string) => {
  const trimmed = detail?.trim();
  if (
    trimmed?.includes(i18n.t('store.coreService.unavailable')) ||
    trimmed?.startsWith('核心服务不可用') ||
    trimmed?.startsWith('Core service is unavailable') ||
    trimmed?.startsWith('Core サービスを利用できません')
  ) {
    return trimmed;
  }
  return trimmed
    ? i18n.t('store.coreService.unavailableWithDetail', { detail: trimmed })
    : i18n.t('store.coreService.unavailable');
};

const isCoreServiceFailureDetail = (detail?: string) => {
  const normalized = detail?.toLowerCase() ?? '';
  return normalized.includes('core') ||
    normalized.includes('核心服务') ||
    normalized.includes('ipc') ||
    normalized.includes('服务器') ||
    normalized.includes('服务');
};

type ActiveTab = 'status' | 'curve' | 'control' | 'about';
export type CurveFocusTarget = 'curve-editor' | 'history-details';
export interface TimelineEvent { timestamp: number; type: 'mode' | 'disconnect' | 'resume' | 'profile'; label: string }

interface AppStore {
  isConnected: boolean;
  deviceProductId: string | null;
  deviceModel: string | null;
  deviceSettings: DeviceSettings | null;
  config: types.AppConfig | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  legionFnQSupported: boolean;
  bridgeWarning: string | null;
  coreServiceError: string | null;
  isLoading: boolean;
  error: string | null;
  activeTab: ActiveTab;
  curveFocusTarget: CurveFocusTarget | null;
  sessionHistoryPoints: TemperatureHistoryPoint[];
  timelineEvents: TimelineEvent[];

  setActiveTab: (tab: ActiveTab) => void;
  openCurveTab: (target: CurveFocusTarget) => void;
  clearCurveFocusTarget: () => void;
  clearBridgeWarning: () => void;
  handleTemperaturePayload: (data: types.TemperatureData | null) => void;
  appendSessionHistoryPoint: (data: types.TemperatureData | null) => void;

  initializeApp: () => Promise<void>;
  connectDevice: () => Promise<void>;
  disconnectDevice: () => Promise<void>;
  updateConfig: (config: types.AppConfig) => Promise<void>;

  startEventListeners: () => () => void;
}

export const useAppStore = create<AppStore>((set, get) => ({
  isConnected: false,
  deviceProductId: null,
  deviceModel: null,
  deviceSettings: null,
  config: null,
  fanData: null,
  temperature: null,
  legionFnQSupported: false,
  bridgeWarning: null,
  coreServiceError: null,
  isLoading: true,
  error: null,
  activeTab: 'status',
  curveFocusTarget: null,
  sessionHistoryPoints: [],
  timelineEvents: [],

  setActiveTab: (tab) => set({ activeTab: tab, curveFocusTarget: null }),

  openCurveTab: (target) => set({ activeTab: 'curve', curveFocusTarget: target }),

  clearCurveFocusTarget: () => set({ curveFocusTarget: null }),

  clearBridgeWarning: () => set({ bridgeWarning: null }),

  handleTemperaturePayload: (data) => {
    const bridgeMessage = data?.bridgeMessage?.trim() ?? '';
    set({
      temperature: data,
      bridgeWarning: data?.bridgeOk === false ? bridgeMessage || getBridgeWarningMessage() : null,
    });
  },

  appendSessionHistoryPoint: (data) => {
    if (!data) return;

    const point = createLiveHistoryPoint({
      updateTime: data.updateTime,
      cpuTemp: data.cpuTemp,
      gpuTemp: data.gpuTemp,
      cpuPower: data.cpuPower,
      gpuPower: data.gpuPower,
    }, Number(get().fanData?.currentRpm || 0));

    if (!point) return;

    set((state) => ({
      sessionHistoryPoints: appendSampledHistoryPoint(state.sessionHistoryPoints, point, {
        retentionMs: SESSION_HISTORY_RETENTION_MS,
        limit: SESSION_HISTORY_LIMIT,
      }),
    }));
  },

  initializeApp: async () => {
    try {
      set({ isLoading: true });

      const [appConfig, deviceStatus, debugInfo] = await Promise.all([
        configService.getConfig(),
        deviceService.getStatus() as Promise<DeviceStatusPayload>,
        apiService.getDebugInfo().catch(() => null),
      ]);
      const coreServiceError = deviceStatus.error ? getCoreServiceErrorMessage(deviceStatus.error) : null;

      set({
        config: appConfig,
        isConnected: deviceStatus.connected || false,
        deviceProductId: deviceStatus.productId || null,
        deviceModel: deviceStatus.model || null,
        deviceSettings: deviceStatus.deviceSettings || null,
        fanData: deviceStatus.currentData || null,
        legionFnQSupported: debugInfo?.legionFnQSupported === true,
        coreServiceError,
        error: coreServiceError,
      });

      get().handleTemperaturePayload(deviceStatus.temperature || null);
    } catch (error) {
      console.error('初始化失败:', error);
      const detail = error instanceof Error ? error.message : undefined;
      const coreServiceError = isCoreServiceFailureDetail(detail) ? getCoreServiceErrorMessage(detail) : null;
      set({ error: coreServiceError || i18n.t('store.errors.initializeApp'), coreServiceError });
    } finally {
      set({ isLoading: false });
    }
  },

  connectDevice: async () => {
    try {
      const success = await deviceService.connect();
      if (success) {
        const status = await deviceService.getStatus().catch(() => null);
        const coreServiceError = status?.error ? getCoreServiceErrorMessage(status.error) : null;
        set({
          isConnected: true,
          deviceSettings: status?.deviceSettings || null,
          deviceProductId: status?.productId || get().deviceProductId,
          deviceModel: status?.model || get().deviceModel,
          coreServiceError,
          error: coreServiceError,
        });
      }
    } catch (error) {
      console.error('连接失败:', error);
      set({ error: i18n.t('store.errors.connectDevice') });
    }
  },

  disconnectDevice: async () => {
    try {
      await deviceService.disconnect();
      set({ isConnected: false, deviceProductId: null, deviceModel: null, deviceSettings: null, fanData: null });
    } catch (error) {
      console.error('断开连接失败:', error);
    }
  },

  updateConfig: async (config) => {
    try {
      await configService.updateConfig(config);
      set({ config, error: null });
    } catch (error) {
      console.error('配置更新失败:', error);
      set({ error: i18n.t('store.errors.saveConfig') });
      toast.error(i18n.t('store.errors.saveConfig'));
      throw error;
    }
  },

  startEventListeners: () => {
    if (typeof window === 'undefined' || !(window as any).runtime?.EventsOnMultiple) {
      return () => {};
    }
    const unsubscribers: Array<() => void> = [];
    let telemetryTimer: number | null = null;
    let pendingFanData: types.FanData | null | undefined;
    let pendingTemperature: types.TemperatureData | null | undefined;
    const flushTelemetry = () => {
      telemetryTimer = null;
      if (pendingFanData !== undefined) {
        set({ fanData: pendingFanData });
        pendingFanData = undefined;
      }
      if (pendingTemperature !== undefined) {
        const data = pendingTemperature;
        pendingTemperature = undefined;
        get().handleTemperaturePayload(data);
        get().appendSessionHistoryPoint(data);
      }
    };
    const scheduleTelemetryFlush = () => {
      if (telemetryTimer !== null) return;
      telemetryTimer = window.setTimeout(flushTelemetry, document.hidden ? 1000 : 200);
    };
    const addTimelineEvent = (event: TimelineEvent) => set((state) => ({
      timelineEvents: [...state.timelineEvents, event].slice(-100),
    }));
    let lastHeartbeat = 0;

    unsubscribers.push(
      apiService.onCoreServiceError((message) => {
        const coreServiceError = getCoreServiceErrorMessage(message);
        set({
          coreServiceError,
          error: coreServiceError,
          isConnected: false,
          deviceProductId: null,
          deviceModel: null,
          deviceSettings: null,
          fanData: null,
        });
      })
    );

    unsubscribers.push(
      apiService.onCoreServiceOK(() => {
        set((state) => ({
          coreServiceError: null,
          error: state.coreServiceError && state.error === state.coreServiceError ? null : state.error,
        }));
      })
    );

    unsubscribers.push(
      deviceService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        const info = deviceInfo as { productId?: string; model?: string };
        const settings = (deviceInfo as { deviceSettings?: DeviceSettings | null })?.deviceSettings || null;
        set({
          isConnected: true,
          deviceProductId: info.productId || null,
          deviceModel: info.model || null,
          deviceSettings: settings,
          coreServiceError: null,
          error: null,
        });
        addTimelineEvent({ timestamp: Date.now(), type: 'mode', label: '设备已连接' });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceDisconnected(() => {
        console.log('设备已断开');
        set({ isConnected: false, deviceProductId: null, deviceModel: null, deviceSettings: null, fanData: null });
        addTimelineEvent({ timestamp: Date.now(), type: 'disconnect', label: '设备断开' });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceSettingsUpdate((settings) => {
        set({ deviceSettings: settings || null });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceError((errorMsg) => {
        console.error('设备错误:', errorMsg);
        set({ error: errorMsg });
      })
    );

    unsubscribers.push(
      deviceService.onFanDataUpdate((data) => {
        pendingFanData = data;
        scheduleTelemetryFlush();
      })
    );

    unsubscribers.push(
      deviceService.onTemperatureUpdate((data) => {
        pendingTemperature = data;
        scheduleTelemetryFlush();
      })
    );

    unsubscribers.push(
      configService.onConfigUpdate((updatedConfig) => {
        const previous = get().config;
        set({ config: updatedConfig });
        if (previous && previous.autoControl !== updatedConfig.autoControl) {
          addTimelineEvent({ timestamp: Date.now(), type: 'mode', label: updatedConfig.autoControl ? '切换为智能控温' : '退出智能控温' });
        }
        const previousProfile = (previous as any)?.activeFanCurveProfileId;
        const nextProfile = (updatedConfig as any)?.activeFanCurveProfileId;
        if (previousProfile && nextProfile && previousProfile !== nextProfile) {
          addTimelineEvent({ timestamp: Date.now(), type: 'profile', label: '切换风扇曲线' });
        }
      })
    );

    unsubscribers.push(apiService.onHeartbeat((timestamp) => {
      const now = Number(timestamp || Date.now());
      if (lastHeartbeat > 0 && now - lastHeartbeat > 20_000) {
        addTimelineEvent({ timestamp: now, type: 'resume', label: '系统睡眠唤醒' });
      }
      lastHeartbeat = now;
    }));

    unsubscribers.push(
      deviceService.onHotkeyTriggered((payload) => {
        const message = typeof payload?.message === 'string' ? payload.message : '';
        if (!message) return;
        const ok = payload?.success !== false;
        if (ok) {
          toast.success(i18n.t('store.hotkey.successTitle'), { description: message, duration: 2600 });
        } else {
          toast.error(i18n.t('store.hotkey.failureTitle'), { description: message, duration: 3200 });
        }
      })
    );

    unsubscribers.push(
      deviceService.onLegionPowerModeUpdate((payload) => {
        const mode = typeof payload?.mode === 'string' ? payload.mode : '';
        if (!mode) return;
        const modeLabel: Record<string, string> = {
          Quiet: i18n.t('store.legionModes.Quiet'),
          Balance: i18n.t('store.legionModes.Balance'),
          Performance: i18n.t('store.legionModes.Performance'),
          Extreme: i18n.t('store.legionModes.Extreme'),
          GodMode: i18n.t('store.legionModes.GodMode'),
        };
        toast.success(i18n.t('store.legionFnQ.modeChangedTitle'), {
          description: i18n.t('store.legionFnQ.modeDescription', { mode: modeLabel[mode] || mode }),
          duration: 2600,
        });
      })
    );

    unsubscribers.push(
      apiService.onLegionFnQSupportUpdate((payload) => {
        set({ legionFnQSupported: payload?.supported === true });
      })
    );

    return () => {
      if (telemetryTimer !== null) window.clearTimeout(telemetryTimer);
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  },
}));

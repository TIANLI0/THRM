import { create } from 'zustand';
import { types } from '../../../wailsjs/go/models';
import { configService } from '../services/config-service';
import { deviceService, type DeviceStatusPayload } from '../services/device-service';
import { toast } from 'sonner';

const BRIDGE_WARNING_MESSAGE = 'CPU/GPU 温度读取失败，请检查Pawnio是否安装成功，或升级最新版。';

type ActiveTab = 'status' | 'curve' | 'control';

interface AppStore {
  isConnected: boolean;
  deviceProductId: string | null;
  deviceModel: string | null;
  config: types.AppConfig | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  bridgeWarning: string | null;
  isLoading: boolean;
  error: string | null;
  activeTab: ActiveTab;

  setActiveTab: (tab: ActiveTab) => void;
  clearBridgeWarning: () => void;
  handleTemperaturePayload: (data: types.TemperatureData | null) => void;

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
  config: null,
  fanData: null,
  temperature: null,
  bridgeWarning: null,
  isLoading: true,
  error: null,
  activeTab: 'status',

  setActiveTab: (tab) => set({ activeTab: tab }),

  clearBridgeWarning: () => set({ bridgeWarning: null }),

  handleTemperaturePayload: (data) => {
    const bridgeMessage = data?.bridgeMessage?.trim() ?? '';
    set({
      temperature: data,
      bridgeWarning: data?.bridgeOk === false ? bridgeMessage || BRIDGE_WARNING_MESSAGE : null,
    });
  },

  initializeApp: async () => {
    try {
      set({ isLoading: true });

      const appConfig = await configService.getConfig();
      const deviceStatus = (await deviceService.getStatus()) as DeviceStatusPayload;

      set({
        config: appConfig,
        isConnected: deviceStatus.connected || false,
        deviceProductId: deviceStatus.productId || null,
        deviceModel: deviceStatus.model || null,
        fanData: deviceStatus.currentData || null,
        error: null,
      });

      get().handleTemperaturePayload(deviceStatus.temperature || null);
    } catch (error) {
      console.error('初始化失败:', error);
      set({ error: '应用初始化失败' });
    } finally {
      set({ isLoading: false });
    }
  },

  connectDevice: async () => {
    try {
      const success = await deviceService.connect();
      if (success) {
        set({ isConnected: true, error: null });
      }
    } catch (error) {
      console.error('连接失败:', error);
      set({ error: '设备连接失败' });
    }
  },

  disconnectDevice: async () => {
    try {
      await deviceService.disconnect();
      set({ isConnected: false, deviceProductId: null, deviceModel: null, fanData: null });
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
      set({ error: '配置保存失败' });
    }
  },

  startEventListeners: () => {
    const unsubscribers: Array<() => void> = [];

    unsubscribers.push(
      deviceService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        const info = deviceInfo as { productId?: string; model?: string };
        set({
          isConnected: true,
          deviceProductId: info.productId || null,
          deviceModel: info.model || null,
          error: null,
        });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceDisconnected(() => {
        console.log('设备已断开');
        set({ isConnected: false, deviceProductId: null, deviceModel: null, fanData: null });
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
        set({ fanData: data });
      })
    );

    unsubscribers.push(
      deviceService.onTemperatureUpdate((data) => {
        get().handleTemperaturePayload(data);
      })
    );

    unsubscribers.push(
      configService.onConfigUpdate((updatedConfig) => {
        set({ config: updatedConfig });
      })
    );

    unsubscribers.push(
      deviceService.onHotkeyTriggered((payload) => {
        const message = typeof payload?.message === 'string' ? payload.message : '';
        if (!message) return;
        const ok = payload?.success !== false;
        if (ok) {
          toast.success('功能变动', { description: message, duration: 2600 });
        } else {
          toast.error('操作失败', { description: message, duration: 3200 });
        }
      })
    );

    unsubscribers.push(
      deviceService.onLegionPowerModeUpdate((payload) => {
        const mode = typeof payload?.mode === 'string' ? payload.mode : '';
        if (!mode) return;
        const modeLabel: Record<string, string> = {
          Quiet: '安静模式',
          Balance: '均衡模式',
          Performance: '野兽模式',
          Extreme: '超能模式',
          GodMode: '自定义模式',
        };
        toast.success('拯救者性能模式变化', {
          description: `当前模式：${modeLabel[mode] || mode}`,
          duration: 2600,
        });
      })
    );

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  },
}));

import { types } from '../../../wailsjs/go/models';
import { apiService } from './api';
import type { LegionPowerModePayload } from '../types/app';

export interface DeviceStatusPayload {
  connected?: boolean;
  currentData?: types.FanData | null;
  temperature?: types.TemperatureData | null;
  productId?: string;
  model?: string;
}

class DeviceService {
  async connect() {
    return apiService.connectDevice();
  }

  async disconnect() {
    return apiService.disconnectDevice();
  }

  async getStatus() {
    return (await apiService.getDeviceStatus()) as DeviceStatusPayload;
  }

  onDeviceConnected(callback: (data: unknown) => void) {
    return apiService.onDeviceConnected(callback as never);
  }

  onDeviceDisconnected(callback: () => void) {
    return apiService.onDeviceDisconnected(callback);
  }

  onDeviceError(callback: (error: string) => void) {
    return apiService.onDeviceError(callback);
  }

  onFanDataUpdate(callback: (data: types.FanData) => void) {
    return apiService.onFanDataUpdate(callback);
  }

  onTemperatureUpdate(callback: (data: types.TemperatureData) => void) {
    return apiService.onTemperatureUpdate(callback);
  }

  onHotkeyTriggered(callback: (payload: { action: string; shortcut: string; success: boolean; message: string }) => void) {
    return apiService.onHotkeyTriggered(callback);
  }

  onLegionPowerModeUpdate(callback: (payload: LegionPowerModePayload) => void) {
    return apiService.onLegionPowerModeUpdate(callback);
  }
}

export const deviceService = new DeviceService();

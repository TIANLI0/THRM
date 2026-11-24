import React, { memo, useMemo } from 'react';
import * as types from '@go/types';
import { useAppStore } from '../../store/app-store';
import { Select, MultiSelect } from '../ui/index';
import { useTranslation } from 'react-i18next';
import { translateControlSource } from './helpers';

/* ── 温度基准 ──

传感器结构体带着实时读数，温度每次推送都是新数组。设置页主体只关心清单本身（key、名称、
显卡归属）和型号这类慢变化字段：序列化成签名来订阅，签名不变就不重渲染这个三千多行的组件。
实时读数、温度文字交给下面几个小组件各自订阅 store。 */

type AppState = ReturnType<typeof useAppStore.getState>;
type SensorRef = { key: string; name: string };
type GpuDeviceRef = { key: string; name: string; vendor: string; sensors: SensorRef[] };
export interface ControlTemperatureMeta {
  cpuModel?: string;
  gpuModel?: string;
  selectedGpuDevice?: string;
  cpuSensors: SensorRef[];
  gpuSensors: SensorRef[];
  gpuDevices: GpuDeviceRef[];
}

const EMPTY_SENSORS: types.TemperatureSensor[] = [];

const toSensorRefs = (sensors: unknown): SensorRef[] =>
  Array.isArray(sensors)
    ? (sensors as types.TemperatureSensor[]).map((sensor) => ({ key: sensor.key, name: sensor.name }))
    : [];

// 选择器在每次 store 变化（含每秒数次的转速推送）时都会执行，同一帧温度只序列化一次。
let temperatureSignatureSource: types.TemperatureData | null | undefined;
let temperatureSignature: string | null = null;
export const selectControlTemperatureSignature = (state: AppState): string | null => {
  const temperature = state.temperature;
  if (temperature === temperatureSignatureSource) return temperatureSignature;
  temperatureSignatureSource = temperature;
  if (!temperature) {
    temperatureSignature = null;
    return null;
  }
  const meta: ControlTemperatureMeta = {
    cpuModel: temperature.cpuModel,
    gpuModel: temperature.gpuModel,
    selectedGpuDevice: temperature.selectedGpuDevice,
    cpuSensors: toSensorRefs(temperature.cpuSensors),
    gpuSensors: toSensorRefs(temperature.gpuSensors),
    gpuDevices: Array.isArray(temperature.gpuDevices)
      ? temperature.gpuDevices.map((device) => ({
        key: device.key,
        name: device.name,
        vendor: device.vendor,
        sensors: toSensorRefs(device.sensors),
      }))
      : [],
  };
  temperatureSignature = JSON.stringify(meta);
  return temperatureSignature;
};

// 与主体里 effectiveGpuSensors 同一规则，只是取带实时读数的原始清单（store 复用未变的引用）。
const selectEffectiveGpuSensors = (state: AppState, activeGpuDeviceKey: string): types.TemperatureSensor[] => {
  const temperature = state.temperature;
  const devices = temperature?.gpuDevices;
  const device = Array.isArray(devices) ? devices.find((item) => item.key === activeGpuDeviceKey) : undefined;
  if (device && Array.isArray(device.sensors) && device.sensors.length > 0) return device.sensors;
  return Array.isArray(temperature?.gpuSensors) ? temperature.gpuSensors : EMPTY_SENSORS;
};

export const LiveCpuSensorMultiSelect = memo(function LiveCpuSensorMultiSelect(
  props: Omit<React.ComponentProps<typeof MultiSelect>, 'options'>,
) {
  const sensors = useAppStore((state) => state.temperature?.cpuSensors);
  const options = useMemo(
    () => (Array.isArray(sensors) ? sensors : EMPTY_SENSORS).map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
    [sensors],
  );
  return <MultiSelect {...props} options={options} />;
});

export const LiveGpuSensorSelect = memo(function LiveGpuSensorSelect({
  activeGpuDeviceKey,
  value,
  onChange,
  disabled,
}: {
  activeGpuDeviceKey: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
}) {
  const { t, i18n } = useTranslation();
  const sensors = useAppStore((state) => selectEffectiveGpuSensors(state, activeGpuDeviceKey));
  const options = useMemo(() => [
    { value: 'auto', label: sensors.length > 0 ? t('controlPanel.options.sensor.autoRecommended') : t('controlPanel.options.sensor.auto') },
    ...sensors.map((sensor) => ({ value: sensor.key, label: `${sensor.name} (${sensor.value}°C)` })),
  ], [sensors, t, i18n.language]);
  return (
    <Select
      value={value}
      onChange={onChange}
      options={options}
      size="sm"
      className="w-full min-w-0"
      disabled={disabled}
    />
  );
});

export const LiveBaselineTemperatureText = memo(function LiveBaselineTemperatureText({ channel }: { channel: 'cpu' | 'gpu' }) {
  const { t } = useTranslation();
  const temp = useAppStore((state) => (channel === 'cpu' ? state.temperature?.cpuTemp : state.temperature?.gpuTemp));
  if (temp && temp > 0) return <>{t('controlPanel.fan.currentBaselineTemperature', { temperature: temp })}</>;
  return <>{channel === 'cpu' ? t('controlPanel.fan.noCpuTemperatureData') : t('controlPanel.fan.noGpuTemperatureData')}</>;
});

export const LiveControlSourceText = memo(function LiveControlSourceText() {
  const { t } = useTranslation();
  const controlTemp = useAppStore((state) => state.temperature?.controlTemp);
  const controlSource = useAppStore((state) => state.temperature?.controlSource);
  return (
    <>
      {controlTemp && controlTemp > 0
        ? t('controlPanel.fan.currentControlSource', { source: translateControlSource(controlSource, t), temperature: controlTemp })
        : t('controlPanel.fan.noControlTemperature')}
    </>
  );
});

export const LiveSmartTempText = memo(function LiveSmartTempText() {
  const { t } = useTranslation();
  const current = useAppStore((state) => Math.max(state.temperature?.cpuTemp || 0, state.temperature?.gpuTemp || 0));
  return <>{current > 0 ? t('controlPanel.light.smartTemp.liveTemp', { temp: current }) : t('controlPanel.light.smartTemp.noTemp')}</>;
});

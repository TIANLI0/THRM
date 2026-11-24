import * as types from '@go/types';
import {
  SMART_TEMP_PRESET_MAX,
  SMART_TEMP_PRESET_MIN,
  rgbToCss,
  smartTempPresetBaseColor,
} from '../../lib/lightProgram';

/* ── 智能温控灯效 ──

固件自己读不到电脑温度，温度分区完全由主机驱动：主机按当前最高温决定用哪个
原生预设，再用 0x44 下发。这里只用原生预设，不上传自定义帧——固件的自定义帧
提交命令 0x43 每次都会擦写一页数据闪存，而分区切换是随负载持续发生的。
*/

export const SMART_TEMP_MAX_BANDS = 5;
export const SMART_TEMP_MAX_TEMP = 110;
export const SMART_TEMP_MAX_HYSTERESIS = 10;

/**
 * 固件原生预设 1..5。颜色表与速度都是从固件生成器里还原出来的，
 * 见 lib/lightProgram.ts 顶部的说明。预设 5 的表随设备的运行配置字节切换。
 */
export function smartTempPresetOptions(runtimeProfile: number | null) {
  return Array.from({ length: SMART_TEMP_PRESET_MAX - SMART_TEMP_PRESET_MIN + 1 }, (_, i) => {
    const value = SMART_TEMP_PRESET_MIN + i;
    const color = smartTempPresetBaseColor(value, runtimeProfile);
    return {
      value,
      swatch: color ? rgbToCss(color) : null,
      labelKey: `controlPanel.light.smartTemp.preset${value}`,
    };
  });
}

function getDefaultSmartTempBands(): types.SmartTempLightBand[] {
  return [
    types.SmartTempLightBand.createFrom({ minTemp: 0, preset: 1 }),
    types.SmartTempLightBand.createFrom({ minTemp: 71, preset: 2 }),
    types.SmartTempLightBand.createFrom({ minTemp: 81, preset: 3 }),
  ];
}

/** 与 Go 侧 types.NormalizeSmartTempLightBands 保持一致：下限升序、首段为 0、预设 1..5。 */
export function normalizeSmartTempBands(raw: unknown): types.SmartTempLightBand[] {
  if (!Array.isArray(raw) || raw.length === 0) return getDefaultSmartTempBands();

  const result: types.SmartTempLightBand[] = [];
  let previousMin = -1;
  for (const [index, item] of raw.slice(0, SMART_TEMP_MAX_BANDS).entries()) {
    const source = (item || {}) as { minTemp?: number; preset?: number };
    let preset = Number(source.preset);
    if (!Number.isFinite(preset) || preset < SMART_TEMP_PRESET_MIN || preset > SMART_TEMP_PRESET_MAX) preset = SMART_TEMP_PRESET_MIN;

    let minTemp = index === 0 ? 0 : Number(source.minTemp);
    if (!Number.isFinite(minTemp)) minTemp = previousMin + 1;
    if (index > 0) {
      minTemp = Math.min(Math.round(minTemp), SMART_TEMP_MAX_TEMP);
      if (minTemp <= previousMin) minTemp = previousMin + 1;
    }
    if (minTemp > SMART_TEMP_MAX_TEMP) break;

    result.push(types.SmartTempLightBand.createFrom({ minTemp, preset }));
    previousMin = minTemp;
  }
  return result.length > 0 ? result : getDefaultSmartTempBands();
}

/**
 * 与 Go 侧 types.SelectSmartTempLightBand 保持一致的区间判定。
 * currentIndex 为负表示首次判定，直接按阈值取值，不套用回差。
 */
export function selectSmartTempBand(
  bands: types.SmartTempLightBand[],
  hysteresis: number,
  temperature: number,
  currentIndex: number,
): number {
  if (bands.length === 0) return -1;
  const gap = Math.min(Math.max(hysteresis, 0), SMART_TEMP_MAX_HYSTERESIS);

  if (currentIndex < 0 || currentIndex >= bands.length) {
    let index = 0;
    bands.forEach((band, i) => {
      if (temperature >= band.minTemp) index = i;
    });
    return index;
  }

  let index = currentIndex;
  while (index + 1 < bands.length && temperature >= bands[index + 1].minTemp + gap) index++;
  while (index > 0 && temperature < bands[index].minTemp - gap) index--;
  return index;
}

function getDefaultLightStripConfig(): types.LightStripConfig {
  return types.LightStripConfig.createFrom({
    mode: 'smart_temp',
    speed: 'medium',
    brightness: 100,
    colors: [
      { r: 255, g: 0, b: 0 },
      { r: 0, g: 255, b: 0 },
      { r: 0, g: 128, b: 255 },
    ],
    smartTempBands: getDefaultSmartTempBands(),
    smartTempHysteresis: 2,
  });
}

export function normalizeLightStripConfig(config: types.AppConfig): types.LightStripConfig {
  const defaults = getDefaultLightStripConfig();
  const raw = (config as any).lightStrip;
  if (!raw) return defaults;

  const normalized = types.LightStripConfig.createFrom({
    mode: raw.mode || defaults.mode,
    speed: raw.speed || defaults.speed,
    brightness: typeof raw.brightness === 'number' ? Math.max(0, Math.min(100, raw.brightness)) : defaults.brightness,
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : defaults.colors,
    smartTempBands: normalizeSmartTempBands(raw.smartTempBands),
    smartTempHysteresis:
      typeof raw.smartTempHysteresis === 'number'
        ? Math.max(0, Math.min(SMART_TEMP_MAX_HYSTERESIS, Math.round(raw.smartTempHysteresis)))
        : defaults.smartTempHysteresis,
  });

  if ((normalized.colors || []).length < 3) {
    const merged = [...(normalized.colors || [])];
    while (merged.length < 3) merged.push(defaults.colors[merged.length]);
    normalized.colors = merged;
  }
  return normalized;
}

export function rgbToHex(color: types.RGBColor): string {
  const h = (v: number) => v.toString(16).padStart(2, '0');
  return `#${h(color.r || 0)}${h(color.g || 0)}${h(color.b || 0)}`;
}

export function hexToRgb(hex: string): types.RGBColor {
  const n = Number.parseInt(hex.replace('#', ''), 16);
  return types.RGBColor.createFrom({ r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 });
}

export function getRequiredColorCount(mode: string): number {
  switch (mode) {
    case 'static_single': return 1;
    case 'off': case 'smart_temp': case 'flowing': return 0;
    case 'static_multi': return 3;
    default: return 3;
  }
}

export const LIGHT_MODE_OPTIONS = [
  { value: 'off', labelKey: 'controlPanel.options.lightMode.off.label', descriptionKey: 'controlPanel.options.lightMode.off.description' },
  { value: 'smart_temp', labelKey: 'controlPanel.options.lightMode.smart_temp.label', descriptionKey: 'controlPanel.options.lightMode.smart_temp.description' },
  { value: 'static_single', labelKey: 'controlPanel.options.lightMode.static_single.label', descriptionKey: 'controlPanel.options.lightMode.static_single.description' },
  { value: 'static_multi', labelKey: 'controlPanel.options.lightMode.static_multi.label', descriptionKey: 'controlPanel.options.lightMode.static_multi.description' },
  { value: 'rotation', labelKey: 'controlPanel.options.lightMode.rotation.label', descriptionKey: 'controlPanel.options.lightMode.rotation.description' },
  { value: 'flowing', labelKey: 'controlPanel.options.lightMode.flowing.label', descriptionKey: 'controlPanel.options.lightMode.flowing.description' },
  { value: 'breathing', labelKey: 'controlPanel.options.lightMode.breathing.label', descriptionKey: 'controlPanel.options.lightMode.breathing.description' },
];

export const LIGHT_SPEED_OPTIONS = [
  { value: 'fast', labelKey: 'controlPanel.options.lightSpeed.fast' },
  { value: 'medium', labelKey: 'controlPanel.options.lightSpeed.medium' },
  { value: 'slow', labelKey: 'controlPanel.options.lightSpeed.slow' },
];

export const LIGHT_COLOR_PRESETS = [
  { nameKey: 'controlPanel.options.lightPresets.neon', colors: [{ r: 255, g: 0, b: 128 }, { r: 0, g: 255, b: 255 }, { r: 128, g: 0, b: 255 }] },
  { nameKey: 'controlPanel.options.lightPresets.forest', colors: [{ r: 86, g: 169, b: 84 }, { r: 161, g: 210, b: 106 }, { r: 44, g: 120, b: 115 }] },
  { nameKey: 'controlPanel.options.lightPresets.glacier', colors: [{ r: 80, g: 170, b: 255 }, { r: 116, g: 214, b: 255 }, { r: 200, g: 240, b: 255 }] },
];

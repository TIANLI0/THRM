import { i18n } from './i18n';

export interface ManualGearPresetLevel {
  level: string;
  rpm: number;
}

export interface ManualGearPreset {
  gear: string;
  colorClass: string;
  borderClass: string;
  bgClass: string;
  solidBgClass: string;
  // 选中卡片：实色边框 + 外圈 ring，避免与未选中卡片难以区分
  activeCardClass: string;
  // 滑杆圆点：选中为实心 + 光圈，已经过的为半透明实心
  dotActiveClass: string;
  dotPassedClass: string;
  levels: ManualGearPresetLevel[];
}

const MANUAL_GEAR_LABEL_KEYS: Record<string, string> = {
  '静音': 'manualGear.gears.quiet',
  '标准': 'manualGear.gears.standard',
  '强劲': 'manualGear.gears.strong',
  '超频': 'manualGear.gears.overclock',
};

const MANUAL_LEVEL_LABEL_KEYS: Record<string, string> = {
  '低': 'manualGear.levels.low',
  '中': 'manualGear.levels.medium',
  '高': 'manualGear.levels.high',
};

export const MANUAL_GEAR_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    solidBgClass: 'bg-emerald-500',
    activeCardClass: 'border-emerald-500 bg-emerald-500/15 ring-2 ring-emerald-500/30 shadow-sm',
    dotActiveClass: 'border-emerald-500 bg-emerald-500 ring-4 ring-emerald-500/25',
    dotPassedClass: 'border-emerald-500/70 bg-emerald-500/60',
    levels: [
      { level: '低', rpm: 1300 },
      { level: '中', rpm: 1700 },
      { level: '高', rpm: 1900 },
    ],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    solidBgClass: 'bg-blue-500',
    activeCardClass: 'border-blue-500 bg-blue-500/15 ring-2 ring-blue-500/30 shadow-sm',
    dotActiveClass: 'border-blue-500 bg-blue-500 ring-4 ring-blue-500/25',
    dotPassedClass: 'border-blue-500/70 bg-blue-500/60',
    levels: [
      { level: '低', rpm: 2100 },
      { level: '中', rpm: 2400 },
      { level: '高', rpm: 2700 },
    ],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    solidBgClass: 'bg-purple-500',
    activeCardClass: 'border-purple-500 bg-purple-500/15 ring-2 ring-purple-500/30 shadow-sm',
    dotActiveClass: 'border-purple-500 bg-purple-500 ring-4 ring-purple-500/25',
    dotPassedClass: 'border-purple-500/70 bg-purple-500/60',
    levels: [
      { level: '低', rpm: 2800 },
      { level: '中', rpm: 3000 },
      { level: '高', rpm: 3300 },
    ],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    solidBgClass: 'bg-orange-500',
    activeCardClass: 'border-orange-500 bg-orange-500/15 ring-2 ring-orange-500/30 shadow-sm',
    dotActiveClass: 'border-orange-500 bg-orange-500 ring-4 ring-orange-500/25',
    dotPassedClass: 'border-orange-500/70 bg-orange-500/60',
    levels: [
      { level: '低', rpm: 3500 },
      { level: '中', rpm: 3700 },
      { level: '高', rpm: 4000 },
    ],
  },
];

// BS1 挡位预设（只有4个固定挡位，无子级别）
export const BS1_MANUAL_GEAR_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    solidBgClass: 'bg-emerald-500',
    activeCardClass: 'border-emerald-500 bg-emerald-500/15 ring-2 ring-emerald-500/30 shadow-sm',
    dotActiveClass: 'border-emerald-500 bg-emerald-500 ring-4 ring-emerald-500/25',
    dotPassedClass: 'border-emerald-500/70 bg-emerald-500/60',
    levels: [{ level: '中', rpm: 1300 }],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    solidBgClass: 'bg-blue-500',
    activeCardClass: 'border-blue-500 bg-blue-500/15 ring-2 ring-blue-500/30 shadow-sm',
    dotActiveClass: 'border-blue-500 bg-blue-500 ring-4 ring-blue-500/25',
    dotPassedClass: 'border-blue-500/70 bg-blue-500/60',
    levels: [{ level: '中', rpm: 2100 }],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    solidBgClass: 'bg-purple-500',
    activeCardClass: 'border-purple-500 bg-purple-500/15 ring-2 ring-purple-500/30 shadow-sm',
    dotActiveClass: 'border-purple-500 bg-purple-500 ring-4 ring-purple-500/25',
    dotPassedClass: 'border-purple-500/70 bg-purple-500/60',
    levels: [{ level: '中', rpm: 2800 }],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    solidBgClass: 'bg-orange-500',
    activeCardClass: 'border-orange-500 bg-orange-500/15 ring-2 ring-orange-500/30 shadow-sm',
    dotActiveClass: 'border-orange-500 bg-orange-500 ring-4 ring-orange-500/25',
    dotPassedClass: 'border-orange-500/70 bg-orange-500/60',
    levels: [{ level: '中', rpm: 3500 }],
  },
];

export const getManualGearHighLevelRpm = (gear?: string | null): number | undefined => {
  if (!gear) return undefined;
  const preset = MANUAL_GEAR_PRESETS.find((item) => item.gear === gear);
  return preset?.levels.find((level) => level.level === '高')?.rpm;
};

// 自定义挡位转速约束（与后端 types.ManualGearMinRPM/MaxRPM 保持一致）
export const MANUAL_GEAR_RPM_MIN = 800;
export const MANUAL_GEAR_RPM_MAX = 5000;

export type ManualGearRpmMap = Record<string, Record<string, number>>;

// 根据用户自定义转速生成有效挡位预设（缺省回退到出厂默认）
export const getEffectiveManualGearPresets = (
  custom?: ManualGearRpmMap | null,
): ManualGearPreset[] => {
  if (!custom) return MANUAL_GEAR_PRESETS;
  return MANUAL_GEAR_PRESETS.map((preset) => ({
    ...preset,
    levels: preset.levels.map((lv) => {
      const value = custom[preset.gear]?.[lv.level];
      return {
        ...lv,
        rpm: typeof value === 'number' && value > 0 ? value : lv.rpm,
      };
    }),
  }));
};

// 校验并补全 12 个自定义转速：限制在 [MIN, MAX] 且按从低到高强制非递减
export const normalizeManualGearRpmMap = (custom?: ManualGearRpmMap | null): ManualGearRpmMap => {
  const out: ManualGearRpmMap = {};
  let prev = 0;
  for (const preset of MANUAL_GEAR_PRESETS) {
    out[preset.gear] = {};
    for (const lv of preset.levels) {
      let v = Math.round(Number(custom?.[preset.gear]?.[lv.level] ?? lv.rpm));
      if (!Number.isFinite(v) || v <= 0) v = lv.rpm;
      if (v < MANUAL_GEAR_RPM_MIN) v = MANUAL_GEAR_RPM_MIN;
      if (v > MANUAL_GEAR_RPM_MAX) v = MANUAL_GEAR_RPM_MAX;
      if (v < prev) v = prev;
      out[preset.gear][lv.level] = v;
      prev = v;
    }
  }
  return out;
};

export const getManualGearLabel = (gear?: string | null): string => {
  if (!gear) return '';
  return i18n.t(MANUAL_GEAR_LABEL_KEYS[gear] || gear);
};

export const getManualLevelLabel = (level?: string | null): string => {
  if (!level) return '';
  return i18n.t(MANUAL_LEVEL_LABEL_KEYS[level] || level);
};

const MAX_GEAR_CODE_TO_RPM: Record<number, number> = {
  // Legacy max-gear codes observed in HID reports. 
  0x2: 2760,
  0x3: 2760,
  0x4: 3300,
  0x6: 4000,
  // Compatibility for firmware variants that use full gear codes.
  0xA: 2760,
  0xC: 3300,
  0xE: 4000,
};

export interface ReportedMaxRpmInfo {
  rpm?: number;
  codeHex?: string;
  source: 'gearSettings' | 'maxGearText' | 'unknown';
}

export const getReportedMaxRpm = (
  gearSettings?: number | null,
  maxGearText?: string | null,
): ReportedMaxRpmInfo => {
  if (typeof gearSettings === 'number') {
    const maxGearCode = (gearSettings >> 4) & 0x0f;
    const mapped = MAX_GEAR_CODE_TO_RPM[maxGearCode];
    if (mapped) {
      return { rpm: mapped, source: 'gearSettings' };
    }
    return { codeHex: `0x${maxGearCode.toString(16).toUpperCase()}`, source: 'gearSettings' };
  }

  const textMapped = getManualGearHighLevelRpm(maxGearText);
  if (textMapped) {
    return { rpm: textMapped, source: 'maxGearText' };
  }

  return { source: 'unknown' };
};

export const LEGION_POWER_MODE_VALUES = ['Quiet', 'Balance', 'Performance', 'Extreme', 'GodMode'] as const;
export const FAN_GEAR_VALUES = ['静音', '标准', '强劲', '超频'] as const;
export const FAN_LEVEL_VALUES = ['低', '中', '高'] as const;

export function getDefaultLegionFnQConfig() {
  return {
    enabled: false,
    takeOverFan: false,
    modeMapping: {
      Quiet: { gear: '静音', level: '中' },
      Balance: { gear: '标准', level: '中' },
      Performance: { gear: '强劲', level: '中' },
      Extreme: { gear: '超频', level: '中' },
      GodMode: { gear: '超频', level: '高' },
    },
  };
}

export function normalizeLegionFnQConfig(raw: any) {
  const defaults = getDefaultLegionFnQConfig();
  const mapping = { ...defaults.modeMapping, ...(raw?.modeMapping || {}) };
  return {
    enabled: !!raw?.enabled,
    takeOverFan: !!raw?.takeOverFan,
    modeMapping: mapping,
  };
}

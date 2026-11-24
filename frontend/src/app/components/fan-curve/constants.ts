import * as types from '@go/types';

export const LOW_RPM_WARNING_DATE_KEY = 'fanCurveLowRpmWarningDate';
export const FAN_CURVE_MIN_TEMP = 30;
export const FAN_CURVE_MAX_TEMP = 110;
export const FAN_CURVE_TEMP_STEP = 5;
export const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
export const SMART_CONTROL_TARGET_TEMP_MIN = 45;
export const SMART_CONTROL_TARGET_TEMP_MAX = 90;
export type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };
export type TimeCurveScheduleRuleView = {
  id: string;
  name: string;
  enabled: boolean;
  weekdays: number[];
  startTime: string;
  endTime: string;
  curveProfileId: string;
};

export type TimeCurveScheduleView = {
  enabled: boolean;
  rules: TimeCurveScheduleRuleView[];
};

export const LEARNING_BIAS_OPTIONS = [
  { value: 'balanced', labelKey: 'fanCurve.learning.biasOptions.balanced.label', descriptionKey: 'fanCurve.learning.biasOptions.balanced.description' },
  { value: 'cooling', labelKey: 'fanCurve.learning.biasOptions.cooling.label', descriptionKey: 'fanCurve.learning.biasOptions.cooling.description' },
  { value: 'quiet', labelKey: 'fanCurve.learning.biasOptions.quiet.label', descriptionKey: 'fanCurve.learning.biasOptions.quiet.description' },
];

export const DEFAULT_SCHEDULE_RULE = {
  enabled: true,
  weekdays: [1, 2, 3, 4, 5, 6, 0],
  startTime: '22:00',
  endTime: '07:00',
};

export const WEEKDAY_SEQUENCE = [1, 2, 3, 4, 5, 6, 0];

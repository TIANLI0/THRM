export const RTSS_UPDATE_INTERVALS = [250, 500, 1000, 2000] as const;
export const RTSS_POSITION_MIN = -1000;
export const RTSS_POSITION_MAX = 1000;
export const RTSS_POSITION_COMMIT_DELAY = 250;
export const RTSS_POSITION_PREVIEW_INTERVAL = 40;
export const RTSS_POSITION_HOLD_DELAY = 350;
export const RTSS_POSITION_CLICK_STEP = 2;
export const RTSS_POSITION_HOLD_INITIAL_SPEED = 28;
export const RTSS_POSITION_HOLD_MAX_SPEED = 56;
export const RTSS_POSITION_HOLD_ACCELERATION_MS = 1600;

export type RTSSPosition = { x: number; y: number };

export function clampRTSSPosition(value: number): number {
  return Math.max(RTSS_POSITION_MIN, Math.min(RTSS_POSITION_MAX, value));
}

/**
 * 自适应学习 2.0 的倾向锚点。
 *
 * 倾向滑块本身是连续的 0..100，锚点只是给它几个说得出口的位置，免得用户面对
 * 一条裸滑块无从下手。曲线页的面板和首页的状态卡都要显示当前落在哪一档，
 * 放在这里共用一份，避免两处各自内置一套阈值、改了一处忘了另一处。
 */
export const ADAPTIVE_PREFERENCE_ANCHORS = [
  { value: 0, labelKey: 'fanCurve.adaptive.anchors.silent' },
  { value: 35, labelKey: 'fanCurve.adaptive.anchors.balanced' },
  { value: 70, labelKey: 'fanCurve.adaptive.anchors.performance' },
  { value: 100, labelKey: 'fanCurve.adaptive.anchors.extreme' },
] as const;

/** 倾向值最接近的档位。 */
export function nearestAdaptiveAnchor(preference: number) {
  return ADAPTIVE_PREFERENCE_ANCHORS.reduce((best, anchor) =>
    Math.abs(anchor.value - preference) < Math.abs(best.value - preference) ? anchor : best,
  );
}

/** 倾向值对应的档位文案 key。 */
export function adaptivePreferenceLabelKey(preference: number) {
  return nearestAdaptiveAnchor(preference).labelKey;
}

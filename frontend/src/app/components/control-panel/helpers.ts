import type { TFunction } from 'i18next';

/* ── Helpers ── */

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function translateWorkMode(
  workMode: string | null | undefined,
  t: TFunction,
) {
  switch (workMode) {
    case '挡位工作模式':
      return t('controlPanel.overview.workModes.manual');
    case '自动模式(实时转速)':
      return t('controlPanel.overview.workModes.auto');
    default:
      return workMode || '--';
  }
}

export function translateControlSource(
  source: string | null | undefined,
  t: (key: string) => string,
) {
  switch (source) {
    case 'cpu':
      return t('controlPanel.options.tempSource.cpu');
    case 'gpu':
      return t('controlPanel.options.tempSource.gpu');
    default:
      return t('controlPanel.options.tempSource.max');
  }
}

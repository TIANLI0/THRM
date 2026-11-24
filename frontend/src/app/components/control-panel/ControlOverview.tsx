import { Settings } from 'lucide-react';
import { getManualGearLabel } from '../../lib/manualGearPresets';
import { useAppStore } from '../../store/app-store';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { translateWorkMode } from './helpers';
import { Section } from './settings-layout';

/* ── 实时概览 ── */

// 概览自己订阅遥测：转速每秒最多推送 4 次，只刷新这三张卡片，不带动整个设置页重渲染。
export function ControlOverview({ isBs1 }: { isBs1: boolean }) {
  const { t } = useTranslation();
  const fanData = useAppStore((state) => state.fanData);
  const temperature = useAppStore((state) => state.temperature);

  return (
    <Section title={t('controlPanel.overview.title')} icon={Settings}>
      <div className="grid grid-cols-1 gap-4 p-5 md:grid-cols-3">
        <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
          <div className="text-sm text-muted-foreground">{t('controlPanel.overview.currentTemperature')}</div>
          <div className={clsx(
            'mt-1 text-2xl font-semibold tabular-nums',
            (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' : (temperature?.maxTemp ?? 0) > 70 ? 'text-amber-500' : 'text-primary'
          )}>
            {temperature?.maxTemp ?? '--'}°C
          </div>
          <div className="mt-1 text-xs text-muted-foreground">{t('controlPanel.overview.cpuGpuTemperature', { cpu: temperature?.cpuTemp ?? '--', gpu: temperature?.gpuTemp ?? '--' })}</div>
        </div>
        <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
          <div className="text-sm text-muted-foreground">{t('controlPanel.overview.currentRpm')}</div>
          <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{fanData?.currentRpm ?? '--'} RPM</div>
          <div className="mt-1 text-xs text-muted-foreground">{translateWorkMode(fanData?.workMode, t)}</div>
        </div>
        <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
          <div className="text-sm text-muted-foreground">{isBs1 ? t('controlPanel.overview.currentGear') : t('controlPanel.overview.targetRpm')}</div>
          <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{isBs1 ? (getManualGearLabel(fanData?.setGear) || '--') : `${fanData?.targetRpm ?? '--'} RPM`}</div>
          <div className="mt-1 text-xs text-muted-foreground">{isBs1 ? translateWorkMode(fanData?.workMode, t) : t('controlPanel.overview.gearValue', { gear: getManualGearLabel(fanData?.setGear) || '--' })}</div>
        </div>
      </div>
    </Section>
  );
}

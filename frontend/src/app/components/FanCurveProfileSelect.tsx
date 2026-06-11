'use client';

import { useMemo } from 'react';
import clsx from 'clsx';
import { Plus, Settings2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Select } from './ui/index';

export type FanCurveProfileOption = {
  id: string;
  name: string;
};

interface FanCurveProfileSelectProps {
  profiles: FanCurveProfileOption[];
  activeProfileId: string;
  onChange: (profileId: string) => void;
  loading?: boolean;
  className?: string;
  placeholder?: string;
  /** 点击 "+" 按钮时打开新增曲线方案弹窗 */
  onAddNew?: () => void;
  /** 点击"管理"按钮时打开管理曲线方案弹窗 */
  onManage?: () => void;
}

const EMPTY_PROFILE_SENTINEL = '__no_curve_profile__';

export default function FanCurveProfileSelect({
  profiles,
  activeProfileId,
  onChange,
  loading = false,
  className,
  placeholder,
  onAddNew,
  onManage,
}: FanCurveProfileSelectProps) {
  const { t } = useTranslation();
  const options = useMemo(
    () => profiles.map((profile) => ({ value: profile.id, label: profile.name })),
    [profiles]
  );

  const resolvedPlaceholder = placeholder || t('fanCurveProfileSelect.placeholder');

  const selectedValue = activeProfileId || options[0]?.value || EMPTY_PROFILE_SENTINEL;

  return (
    <div className={clsx('flex items-center gap-1', className)}>
      <div className="w-[172px]">
        <Select
          value={selectedValue}
          onChange={(v: string | number) => {
            const id = String(v);
            if (!id || id === activeProfileId || id === EMPTY_PROFILE_SENTINEL) return;
            onChange(id);
          }}
          options={
            options.length > 0
              ? options
              : [{ value: EMPTY_PROFILE_SENTINEL, label: t('fanCurveProfileSelect.empty'), disabled: true }]
          }
          size="sm"
          placeholder={resolvedPlaceholder}
          disabled={loading || options.length === 0}
          triggerClassName="h-9 rounded-xl border-border/70 bg-background/45 text-[13px]"
        />
      </div>
      {onAddNew && (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={onAddNew}
              disabled={loading}
              aria-label={t('fanCurveProfileSelect.addNewAria')}
              className={clsx(
                'flex h-9 w-9 cursor-pointer items-center justify-center rounded-xl border border-border/70 bg-background/45 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary',
                loading && 'cursor-not-allowed opacity-50',
              )}
            >
              <Plus className="h-4 w-4" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{t('fanCurveProfileSelect.addNewTooltip')}</TooltipContent>
        </Tooltip>
      )}
      {onManage && (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={onManage}
              disabled={loading}
              aria-label={t('fanCurveProfileSelect.manageAria')}
              className={clsx(
                'flex h-9 w-9 cursor-pointer items-center justify-center rounded-xl border border-border/70 bg-background/45 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary',
                loading && 'cursor-not-allowed opacity-50',
              )}
            >
              <Settings2 className="h-4 w-4" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{t('fanCurveProfileSelect.manageTooltip')}</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

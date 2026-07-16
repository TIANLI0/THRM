'use client';

import clsx from 'clsx';
import { Plus, Settings2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from './ui/index';

type FanCurveProfileOption = {
  id: string;
  name: string;
};

interface FanCurveProfileToolbarProps {
  profiles: FanCurveProfileOption[];
  activeProfileId: string;
  onChange: (profileId: string) => void;
  onAddNew: () => void;
  onManage: () => void;
  onDelete: (profileId: string) => void;
  loading?: boolean;
  className?: string;
}

export default function FanCurveProfileToolbar({
  profiles,
  activeProfileId,
  onChange,
  onAddNew,
  onManage,
  onDelete,
  loading = false,
  className,
}: FanCurveProfileToolbarProps) {
  const { t } = useTranslation();

  return (
    <div
      data-curve-profile-toolbar
      className={clsx('flex min-w-0 items-center gap-1.5 rounded-xl border border-border/70 bg-card/70 p-1.5 shadow-sm shadow-black/5', className)}
    >
      <div data-curve-profile-list className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto px-0.5 py-0.5 scrollbar-none [&::-webkit-scrollbar]:hidden">
        {profiles.map((profile) => {
          const isActive = profile.id === activeProfileId;
          const deletable = profiles.length > 1;
          return (
            <div key={profile.id} className="group relative flex shrink-0">
              <button
                type="button"
                onClick={() => onChange(profile.id)}
                disabled={loading}
                className={clsx(
                  'h-9 cursor-pointer truncate whitespace-nowrap rounded-full border text-center text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60',
                  deletable ? 'pl-4 pr-8' : 'px-4',
                  isActive
                    ? 'border-primary/40 bg-primary/10 text-primary'
                    : 'border-border/70 bg-background/55 text-muted-foreground hover:border-border hover:bg-muted/65 hover:text-foreground',
                )}
                aria-current={isActive ? 'true' : undefined}
              >
                {profile.name}
              </button>
              {deletable && (
                <button
                  type="button"
                  onClick={() => onDelete(profile.id)}
                  disabled={loading}
                  className={clsx(
                    'absolute right-1 top-1/2 flex h-5 w-5 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full transition-[color,background-color,opacity] focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed',
                    isActive
                      ? 'text-primary/70 opacity-100 hover:bg-primary/15 hover:text-destructive'
                      : 'text-muted-foreground opacity-0 hover:bg-muted hover:text-destructive group-hover:opacity-100 group-focus-within:opacity-100',
                  )}
                  aria-label={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                  title={t('fanCurve.profiles.deleteProfileLabel', { name: profile.name })}
                >
                  <X className="h-3 w-3" />
                </button>
              )}
            </div>
          );
        })}
      </div>
      <Button variant="secondary" size="sm" className="shrink-0 rounded-lg" onClick={onAddNew} disabled={loading} icon={<Plus className="h-3.5 w-3.5" />}>
        {t('fanCurve.profiles.add')}
      </Button>
      <Button variant="outline" size="sm" className="shrink-0 rounded-lg" onClick={onManage} disabled={loading || profiles.length === 0} icon={<Settings2 className="h-3.5 w-3.5" />}>
        {t('fanCurve.profiles.manage')}
      </Button>
    </div>
  );
}

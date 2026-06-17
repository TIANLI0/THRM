'use client';

import React, { forwardRef, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import { Loader2, ChevronDown, Check } from 'lucide-react';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button as ShadcnButton } from '@/components/ui/button';
import { Card as ShadcnCard } from '@/components/ui/card';
import { Badge as ShadcnBadge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Slider as ShadcnSlider } from '@/components/ui/slider';
import { ScrollArea as ShadcnScrollArea } from '@/components/ui/scroll-area';
import {
  Select as ShadcnSelect,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { RadioGroup as ShadcnRadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { i18n } from '../../lib/i18n';

interface ToggleSwitchProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  loading?: boolean;
  size?: 'sm' | 'md' | 'lg';
  color?: 'blue' | 'green' | 'purple' | 'orange';
  label?: string;
  srLabel?: string;
}

const toggleColorClasses: Record<NonNullable<ToggleSwitchProps['color']>, string> = {
  blue: 'data-[state=checked]:!bg-primary',
  green: 'data-[state=checked]:!bg-green-600',
  purple: 'data-[state=checked]:!bg-primary',
  orange: 'data-[state=checked]:!bg-orange-600',
};

const toggleSizeClasses: Record<NonNullable<ToggleSwitchProps['size']>, string> = {
  sm: 'h-5 w-9 [&>span]:h-4 [&>span]:w-4 data-[state=checked]:[&>span]:translate-x-4',
  md: 'h-6 w-11 [&>span]:h-5 [&>span]:w-5 data-[state=checked]:[&>span]:translate-x-5',
  lg: 'h-7 w-14 [&>span]:h-6 [&>span]:w-6 data-[state=checked]:[&>span]:translate-x-7',
};

export const ToggleSwitch = forwardRef<HTMLButtonElement, ToggleSwitchProps>(
  ({ enabled, onChange, disabled = false, loading = false, size = 'md', color = 'blue', label, srLabel }, ref) => {
    const isDisabled = disabled || loading;

    return (
      <div className="inline-flex min-h-6 items-center gap-3 align-middle">
        {label && <span className="inline-flex items-center text-sm leading-none font-medium text-muted-foreground">{label}</span>}
        <Switch
          ref={ref}
          checked={enabled}
          onCheckedChange={onChange}
          disabled={isDisabled}
          aria-label={srLabel || label || i18n.t('ui.toggle.defaultAriaLabel')}
          className={clsx('self-center', toggleColorClasses[color], toggleSizeClasses[size], loading && 'animate-pulse')}
        />
      </div>
    );
  }
);
ToggleSwitch.displayName = 'ToggleSwitch';

interface SelectOption<T = string> {
  value: T;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface SelectProps<T = string> {
  value: T;
  onChange: (value: T) => void;
  options: SelectOption<T>[];
  disabled?: boolean;
  placeholder?: string;
  label?: string;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  triggerClassName?: string;
}

const selectTriggerSize: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'h-10 text-sm',
  md: 'h-11 text-sm',
  lg: 'h-12 text-base',
};

export function Select<T extends string | number>({
  value,
  onChange,
  options,
  disabled = false,
  placeholder = i18n.t('ui.select.placeholder'),
  label,
  size = 'md',
  className,
  triggerClassName,
}: SelectProps<T>) {
  const isNumberValue = typeof value === 'number';

  return (
    <div className={clsx('min-w-[120px]', className)}>
      {label && <Label className="mb-1 block">{label}</Label>}
      <ShadcnSelect
        value={String(value)}
        onValueChange={(raw) => onChange((isNumberValue ? Number(raw) : raw) as T)}
        disabled={disabled}
      >
        <SelectTrigger
          className={clsx(selectTriggerSize[size], '[&>span]:truncate', triggerClassName)}
        >
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={String(option.value)} value={String(option.value)} disabled={option.disabled}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </ShadcnSelect>
    </div>
  );
}

interface MultiSelectProps {
  values: string[];
  onChange: (values: string[]) => void;
  options: SelectOption<string>[];
  /** 顶部“自动/全部”项的文案：点击后清空所有选择。 */
  autoOptionLabel?: string;
  /** 未选择任何项时触发器显示的文案（通常与 autoOptionLabel 相同）。 */
  emptyLabel?: string;
  disabled?: boolean;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

/**
 * MultiSelect 是与 Select 视觉一致的多选下拉框：触发器显示已选项摘要，
 * 展开后以勾选列表方式多选。未选择任何项即视为“自动”。
 */
export function MultiSelect({
  values,
  onChange,
  options,
  autoOptionLabel,
  emptyLabel,
  disabled = false,
  size = 'md',
  className,
}: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handlePointer = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', handlePointer);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('mousedown', handlePointer);
      document.removeEventListener('keydown', handleKey);
    };
  }, [open]);

  const selectedLabels = options.filter((option) => values.includes(option.value)).map((option) => option.label);
  const triggerText = selectedLabels.length > 0 ? selectedLabels.join('、') : (emptyLabel ?? autoOptionLabel ?? '');

  const toggle = (value: string) => {
    onChange(values.includes(value) ? values.filter((item) => item !== value) : [...values, value]);
  };

  const itemClass = 'flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-foreground outline-none hover:bg-accent hover:text-accent-foreground';

  return (
    <div ref={containerRef} className={clsx('relative min-w-[120px]', className)}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((prev) => !prev)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className={clsx(
          'flex w-full items-center justify-between gap-2 rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
          selectTriggerSize[size],
        )}
      >
        <span className="truncate text-left">{triggerText}</span>
        <ChevronDown className={clsx('h-4 w-4 shrink-0 opacity-70 transition-transform', open && 'rotate-180')} />
      </button>

      {open && (
        <div className="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md">
          {autoOptionLabel && (
            <button type="button" className={itemClass} onClick={() => onChange([])}>
              <Check className={clsx('h-4 w-4 shrink-0', values.length === 0 ? 'opacity-100' : 'opacity-0')} />
              <span className="truncate">{autoOptionLabel}</span>
            </button>
          )}
          {options.map((option) => {
            const checked = values.includes(option.value);
            return (
              <button
                key={option.value}
                type="button"
                disabled={option.disabled}
                className={clsx(itemClass, option.disabled && 'pointer-events-none opacity-50')}
                onClick={() => toggle(option.value)}
              >
                <Check className={clsx('h-4 w-4 shrink-0', checked ? 'opacity-100' : 'opacity-0')} />
                <span className="truncate">{option.label}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

interface RadioOption<T = string> {
  value: T;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface RadioGroupProps<T = string> {
  value: T;
  onChange: (value: T) => void;
  options: RadioOption<T>[];
  disabled?: boolean;
  label?: string;
  orientation?: 'horizontal' | 'vertical';
}

export function RadioGroup<T extends string | number>({
  value,
  onChange,
  options,
  disabled = false,
  label,
  orientation = 'vertical',
}: RadioGroupProps<T>) {
  const isNumberValue = typeof value === 'number';

  return (
    <div className="w-full">
      {label && <div className="mb-2 text-sm font-medium text-muted-foreground">{label}</div>}
      <ShadcnRadioGroup
        value={String(value)}
        onValueChange={(raw) => onChange((isNumberValue ? Number(raw) : raw) as T)}
        className={clsx('gap-2', orientation === 'horizontal' ? 'grid-flow-col auto-cols-fr' : 'grid-cols-1')}
        disabled={disabled}
      >
        {options.map((option) => {
          const selected = option.value === value;
          const itemDisabled = disabled || option.disabled;
          return (
            <label
              key={String(option.value)}
              className={clsx(
                'flex cursor-pointer items-center rounded-lg border-2 px-4 py-3 transition-all',
                selected
                  ? 'border-primary/50 bg-primary/10'
                  : 'border-border hover:border-primary/30 hover:bg-muted/70',
                itemDisabled && 'cursor-not-allowed opacity-50'
              )}
            >
              <RadioGroupItem value={String(option.value)} disabled={itemDisabled} className="mr-3" />
              <div className="min-w-0 flex-1">
                <div className={clsx('text-sm font-medium', selected ? 'text-primary' : 'text-foreground')}>
                  {option.label}
                </div>
                {option.description && (
                  <div className={clsx('mt-0.5 text-xs', selected ? 'text-primary/80' : 'text-muted-foreground')}>
                    {option.description}
                  </div>
                )}
              </div>
            </label>
          );
        })}
      </ShadcnRadioGroup>
    </div>
  );
}

interface SliderProps {
  value: number;
  onChange: (value: number) => void;
  min: number;
  max: number;
  step?: number;
  disabled?: boolean;
  label?: string;
  showValue?: boolean;
  valueFormatter?: (value: number) => string;
  onChangeStart?: () => void;
  onChangeEnd?: () => void;
}

export const Slider = forwardRef<React.ElementRef<typeof ShadcnSlider>, SliderProps>(
  ({
    value,
    onChange,
    min,
    max,
    step = 1,
    disabled = false,
    label,
    showValue = true,
    valueFormatter = (v) => String(v),
    onChangeStart,
    onChangeEnd,
  }, ref) => {
    return (
      <div className="w-full">
        {(label || showValue) && (
          <div className="mb-2 flex items-center justify-between">
            {label && <span className="text-sm font-medium text-muted-foreground">{label}</span>}
            {showValue && <span className="text-sm font-semibold text-primary">{valueFormatter(value)}</span>}
          </div>
        )}
        <ShadcnSlider
          ref={ref}
          min={min}
          max={max}
          step={step}
          value={[value]}
          onValueChange={(next) => onChange(next[0] ?? value)}
          onPointerDown={onChangeStart}
          onPointerUp={onChangeEnd}
          disabled={disabled}
          className={clsx(
            'w-full',
            disabled && 'opacity-50'
          )}
        />
      </div>
    );
  }
);
Slider.displayName = 'Slider';

interface ScrollAreaProps extends React.ComponentPropsWithoutRef<typeof ShadcnScrollArea> {
  children: React.ReactNode;
}

export function ScrollArea({ children, className, ...props }: ScrollAreaProps) {
  return (
    <ShadcnScrollArea className={className} {...props}>
      {children}
    </ShadcnScrollArea>
  );
}

interface NumberInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  label?: string;
  suffix?: string;
  onFocus?: () => void;
  onBlur?: () => void;
}

export const NumberInput = forwardRef<HTMLInputElement, NumberInputProps>(
  ({ value, onChange, min, max, step = 1, disabled = false, label, suffix, onFocus, onBlur }, ref) => {
    // 输入期间只维护本地草稿，失焦/回车时才钳制范围并提交，
    // 避免每次按键都被 min/max 钳制、且触发整页配置写入与重绘。
    const [draft, setDraft] = React.useState<string | null>(null);
    const focusedRef = React.useRef(false);

    const commit = () => {
      if (draft === null) return;
      let nextValue = Number(draft);
      if (Number.isNaN(nextValue) || draft.trim() === '') nextValue = value;
      if (min !== undefined) nextValue = Math.max(min, nextValue);
      if (max !== undefined) nextValue = Math.min(max, nextValue);
      setDraft(null);
      if (nextValue !== value) onChange(nextValue);
    };

    return (
      <div className="w-full">
        {label && <Label className="mb-1 block">{label}</Label>}
        <div className="relative flex items-center">
          <Input
            ref={ref}
            type="number"
            value={focusedRef.current && draft !== null ? draft : value}
            onChange={(e) => setDraft(e.target.value)}
            onFocus={() => {
              focusedRef.current = true;
              onFocus?.();
            }}
            onBlur={() => {
              focusedRef.current = false;
              commit();
              onBlur?.();
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') e.currentTarget.blur();
            }}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            className={clsx(suffix && 'pr-12')}
          />
          {suffix && <span className="pointer-events-none absolute right-3 text-sm text-muted-foreground">{suffix}</span>}
        </div>
      </div>
    );
  }
);
NumberInput.displayName = 'NumberInput';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  padding?: 'none' | 'sm' | 'md' | 'lg';
  hover?: boolean;
}

const cardPaddingVariants = {
  none: '',
  sm: 'p-3',
  md: 'p-4',
  lg: 'p-6',
};

export function Card({ children, className, padding = 'md', hover = false }: CardProps) {
  return (
    <ShadcnCard
      className={clsx(
        cardPaddingVariants[padding],
        hover && 'transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md',
        className
      )}
    >
      {children}
    </ShadcnCard>
  );
}

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  size?: 'sm' | 'md';
}

export function Badge({ children, variant = 'default', size = 'sm' }: BadgeProps) {
  return (
    <ShadcnBadge variant={variant} size={size}>
      {children}
    </ShadcnBadge>
  );
}

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: React.ReactNode;
}

const buttonVariantMap: Record<NonNullable<ButtonProps['variant']>, 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive'> = {
  primary: 'default',
  secondary: 'secondary',
  outline: 'outline',
  ghost: 'ghost',
  danger: 'destructive',
};

const buttonSizeMap: Record<NonNullable<ButtonProps['size']>, 'sm' | 'default' | 'lg'> = {
  sm: 'sm',
  md: 'default',
  lg: 'lg',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading = false, icon, className, children, disabled, ...props }, ref) => {
    return (
      <ShadcnButton
        ref={ref}
        variant={buttonVariantMap[variant]}
        size={buttonSizeMap[size]}
        disabled={disabled || loading}
        className={clsx('cursor-pointer disabled:cursor-not-allowed', className)}
        {...props}
      >
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : icon ? <span>{icon}</span> : null}
        {children}
      </ShadcnButton>
    );
  }
);
Button.displayName = 'Button';

export { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
export { Skeleton } from '@/components/ui/skeleton';

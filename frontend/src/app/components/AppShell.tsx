'use client';

import type { CSSProperties, KeyboardEvent, ReactNode } from 'react';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { motion, AnimatePresence, type Variants } from 'framer-motion';
import {
  Copy,
  LineChart,
  LayoutGrid,
  Minus,
  Settings2,
  Square,
  TriangleAlert,
  BluetoothConnected,
  BluetoothOff,
  X,
  Fan,
  Thermometer,
  Sparkles,
  Info,
  PanelLeftOpen,
  PanelLeftClose,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { BrowserOpenURL, Environment, Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../../wailsjs/runtime/runtime';
import { WindowBlurEnabled } from '../../../wailsjs/go/main/App';
import { types } from '../../../wailsjs/go/models';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

const MAIN_TAB_ITEMS = [
  { id: 'status', titleKey: 'appShell.tabs.status', icon: LayoutGrid },
  { id: 'curve', titleKey: 'appShell.tabs.curve', icon: LineChart },
  { id: 'control', titleKey: 'appShell.tabs.control', icon: Settings2 },
] as const;

const ABOUT_TAB = { id: 'about', titleKey: 'appShell.tabs.about', icon: Info } as const;

type ActiveTab = (typeof MAIN_TAB_ITEMS)[number]['id'] | typeof ABOUT_TAB.id;

const SIDEBAR_COLLAPSED_WIDTH = 64; // w-16
const SIDEBAR_EXPANDED_WIDTH = 216;
const SIDEBAR_EXPANDED_STORAGE_KEY = 'thrm.sidebar.expanded';

function readStoredSidebarExpanded(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_EXPANDED_STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

const TAB_TRANSITION_ORDER: ActiveTab[] = [...MAIN_TAB_ITEMS.map((tab) => tab.id), ABOUT_TAB.id];

function getTabTransitionDirection(fromTab: ActiveTab, toTab: ActiveTab) {
  const fromIndex = TAB_TRANSITION_ORDER.indexOf(fromTab);
  const toIndex = TAB_TRANSITION_ORDER.indexOf(toTab);
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
    return 0;
  }
  return toIndex > fromIndex ? 1 : -1;
}

const TAB_CONTENT_VARIANTS: Variants = {
  enter: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? 8 : direction * 18,
  }),
  center: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.19,
      ease: [0.22, 1, 0.36, 1],
    },
  },
  exit: (direction: number) => ({
    opacity: 0,
    y: direction === 0 ? -6 : direction * -14,
    transition: {
      duration: 0.16,
      ease: [0.22, 1, 0.36, 1],
    },
  }),
};

interface AppShellProps {
  activeTab: ActiveTab;
  onTabChange: (tab: ActiveTab) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  autoControl: boolean;
  error: string | null;
  bridgeWarning: string | null;
  onDismissBridgeWarning: () => void;
  statusContent: ReactNode;
  curveContent: ReactNode;
  controlContent: ReactNode;
  aboutContent: ReactNode;
}

function getTempColor(temp?: number) {
  if (!temp) return 'text-muted-foreground';
  if (temp > 80) return 'text-red-500';
  if (temp > 70) return 'text-amber-500';
  return 'text-primary';
}

function getFanSpinDuration(rpm?: number) {
  if (!rpm || rpm <= 0) return 0;
  if (rpm >= 4200) return 0.48;
  if (rpm >= 3200) return 0.72;
  if (rpm >= 2200) return 1;
  return 1.35;
}

type WailsDragStyle = CSSProperties & { ['--wails-draggable']?: 'drag' | 'no-drag' };

const DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'drag' };
const NO_DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'no-drag' };

/* ──────────────────────────────────────────────────────────────
 * TitleBar — slim, fixed at the very top of the window.
 * Outside the scroll viewport, so window controls never scroll.
 * ────────────────────────────────────────────────────────────── */

function TitleBarButton({
  icon,
  label,
  onClick,
  danger = false,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      style={NO_DRAG_STYLE}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={clsx(
        'flex h-8 w-10 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors',
        danger
          ? 'hover:bg-red-500 hover:text-white'
          : 'hover:bg-foreground/10 hover:text-foreground',
      )}
    >
      {icon}
    </button>
  );
}

function TitleBar({
  minimizeLabel,
  maximizeLabel,
  restoreLabel,
  closeLabel,
  isMaximised,
  leftSlot,
  leftOffset,
  onMinimise,
  onToggleMaximise,
  onClose,
}: {
  minimizeLabel: string;
  maximizeLabel: string;
  restoreLabel: string;
  closeLabel: string;
  isMaximised: boolean;
  leftSlot?: ReactNode;
  leftOffset: number;
  onMinimise: () => void;
  onToggleMaximise: () => void;
  onClose: () => void;
}) {
  return (
    <div
      // z-[9999]：保证最小化/最大化/关闭三键始终高于 Dialog 遮罩（z-50）等弹层并可交互
      className="glacier-titlebar pointer-events-auto absolute right-0 top-0 z-[9999] flex h-10 items-center justify-between bg-background transition-[left] duration-300 ease-out"
      style={{ ...DRAG_STYLE, left: leftOffset }}
      onDoubleClick={onToggleMaximise}
    >
      <div className="flex h-full min-w-0 flex-1 items-center px-3 pt-1">
        {leftSlot}
      </div>
      <div className="flex h-full items-center gap-0.5 pr-1" style={NO_DRAG_STYLE}>
        <TitleBarButton icon={<Minus className="h-3.5 w-3.5" />} label={minimizeLabel} onClick={onMinimise} />
        <TitleBarButton
          icon={isMaximised ? <Copy className="h-3 w-3" /> : <Square className="h-3 w-3" />}
          label={isMaximised ? restoreLabel : maximizeLabel}
          onClick={onToggleMaximise}
        />
        <TitleBarButton icon={<X className="h-3.5 w-3.5" />} label={closeLabel} onClick={onClose} danger />
      </div>
    </div>
  );
}

function StatusBadges({
  isConnected,
  fanData,
  temperature,
  autoControl,
  compact = false,
}: {
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  autoControl: boolean;
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);
  const baseClass = compact
    ? 'inline-flex h-6 items-center gap-1.5 rounded-full border px-2.5 text-[11px] font-medium'
    : 'inline-flex h-8 items-center gap-1.5 rounded-xl border px-3 text-[13px] font-medium';
  const fanSpinStyle = fanSpinDuration ? { animationDuration: `${fanSpinDuration}s` } : undefined;

  return (
    <div
      className={clsx(
        'flex min-w-0 items-center gap-2 text-[13px] tabular-nums',
        compact && 'translate-y-px overflow-hidden whitespace-nowrap',
      )}
    >
      <span
        className={clsx(
          baseClass,
          'glacier-status-chip',
          isConnected
            ? 'glacier-status-chip--tint border-primary/20 bg-primary/10 text-primary'
            : 'border-border bg-card text-muted-foreground',
        )}
      >
        {isConnected ? <BluetoothConnected className="h-3.5 w-3.5" /> : <BluetoothOff className="h-3.5 w-3.5" />}
        {isConnected ? t('appShell.status.connected') : t('appShell.status.offline')}
      </span>

      <span
        className={clsx(
          baseClass,
          'glacier-status-chip',
          autoControl ? 'glacier-status-chip--tint border-primary/20 bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground',
        )}
      >
        <Sparkles className="h-3.5 w-3.5" />
        {autoControl ? t('appShell.status.smartControl') : t('appShell.status.manualMode')}
      </span>

      {isConnected && (
        <>
          <span className={clsx(baseClass, 'glacier-status-chip border-border bg-card font-semibold shadow-sm shadow-black/5')}>
            <Thermometer className={clsx('h-3.5 w-3.5', getTempColor(temperature?.maxTemp))} />
            <span className={clsx(getTempColor(temperature?.maxTemp))}>
              {temperature?.maxTemp ?? '--'}°C
            </span>
          </span>
          <span className={clsx(baseClass, 'glacier-status-chip border-border bg-card font-semibold text-primary shadow-sm shadow-black/5')}>
            <span className={clsx('inline-flex', fanSpinDuration && 'animate-spin')} style={fanSpinStyle}>
              <Fan className="h-3.5 w-3.5" />
            </span>
            {fanData?.currentRpm ?? '--'} RPM
          </span>
        </>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
 * OverlayScrollbar — floating thumb, never reserves width.
 * Native scrollbar is hidden via .app-scroll-root--hide-native.
 * ────────────────────────────────────────────────────────────── */

function OverlayScrollbar({
  scrollRef,
  topOffset = 6,
}: {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  topOffset?: number;
}) {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const thumbRef = useRef<HTMLDivElement | null>(null);
  const hideTimerRef = useRef<number | null>(null);
  const draggingRef = useRef<{ startY: number; startScroll: number } | null>(null);
  const [visible, setVisible] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);

  const updateThumb = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;

    const { scrollTop, scrollHeight, clientHeight } = el;
    const overflow = scrollHeight - clientHeight;
    if (overflow <= 1) {
      setHasOverflow(false);
      setVisible(false);
      return;
    }
    setHasOverflow(true);

    const thumb = thumbRef.current;
    const track = trackRef.current;
    if (!thumb || !track) return;

    const trackHeight = track.clientHeight;
    const ratio = clientHeight / scrollHeight;
    const thumbHeight = Math.max(28, trackHeight * ratio);
    const maxThumbTop = trackHeight - thumbHeight;
    const top = (scrollTop / overflow) * maxThumbTop;
    thumb.style.height = `${thumbHeight}px`;
    thumb.style.transform = `translateY(${top}px)`;
  }, [scrollRef]);

  const flashVisible = useCallback(() => {
    setVisible(true);
    if (hideTimerRef.current) {
      window.clearTimeout(hideTimerRef.current);
    }
    hideTimerRef.current = window.setTimeout(() => {
      if (!draggingRef.current) {
        setVisible(false);
      }
    }, 1400);
  }, []);

  useLayoutEffect(() => {
    updateThumb();
  }, [hasOverflow, updateThumb]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const onActivity = () => {
      updateThumb();
      flashVisible();
    };

    el.addEventListener('scroll', onActivity, { passive: true });
    el.addEventListener('mouseenter', onActivity);
    el.addEventListener('wheel', onActivity, { passive: true });
    el.addEventListener('touchstart', onActivity, { passive: true });

    const ro = new ResizeObserver(() => updateThumb());
    ro.observe(el);
    const content = el.firstElementChild;
    if (content instanceof HTMLElement) {
      ro.observe(content);
    }

    updateThumb();
    if (el.scrollHeight - el.clientHeight > 1) {
      flashVisible();
    }

    return () => {
      el.removeEventListener('scroll', onActivity);
      el.removeEventListener('mouseenter', onActivity);
      el.removeEventListener('wheel', onActivity);
      el.removeEventListener('touchstart', onActivity);
      ro.disconnect();
      if (hideTimerRef.current) window.clearTimeout(hideTimerRef.current);
    };
  }, [scrollRef, updateThumb, flashVisible]);

  const handleThumbPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const el = scrollRef.current;
      if (!el) return;
      event.preventDefault();
      (event.target as HTMLElement).setPointerCapture(event.pointerId);
      draggingRef.current = { startY: event.clientY, startScroll: el.scrollTop };
      setVisible(true);
    },
    [scrollRef],
  );

  const handleThumbPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = draggingRef.current;
      const el = scrollRef.current;
      const track = trackRef.current;
      const thumb = thumbRef.current;
      if (!drag || !el || !track || !thumb) return;
      const dy = event.clientY - drag.startY;
      const trackHeight = track.clientHeight;
      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = trackHeight - thumbHeight;
      if (maxThumbTop <= 0) return;
      const overflow = el.scrollHeight - el.clientHeight;
      const scrollDelta = (dy / maxThumbTop) * overflow;
      el.scrollTop = drag.startScroll + scrollDelta;
    },
    [scrollRef],
  );

  const handleThumbPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      draggingRef.current = null;
      try {
        (event.target as HTMLElement).releasePointerCapture(event.pointerId);
      } catch {
        /* noop */
      }
      flashVisible();
    },
    [flashVisible],
  );

  if (!hasOverflow) return null;

  return (
    <div
      ref={trackRef}
      className={clsx('app-overlay-scrollbar', visible && 'is-visible')}
      style={{ top: topOffset }}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={flashVisible}
    >
      <div
        ref={thumbRef}
        className="app-overlay-scrollbar-thumb"
        onPointerDown={handleThumbPointerDown}
        onPointerMove={handleThumbPointerMove}
        onPointerUp={handleThumbPointerUp}
        onPointerCancel={handleThumbPointerUp}
      />
    </div>
  );
}

function SidebarNavButton({
  icon: Icon,
  label,
  isActive,
  expanded,
  onClick,
  role,
}: {
  icon: LucideIcon;
  label: string;
  isActive: boolean;
  expanded: boolean;
  onClick: () => void;
  role?: 'tab';
}) {
  // 布局在收起/展开两态下完全一致：图标固定居中于最左 64px 槽位、始终不移动；
  // 文字标签为 flex-1 + truncate，宽度随侧边栏宽度过渡从 0 平滑展开/收起并自动裁剪。
  // 这样收起时图标不会横向扫动、文字也不会突然消失。
  const button = (
    <button
      type="button"
      role={role}
      aria-label={label}
      aria-selected={isActive}
      onClick={onClick}
      className={clsx(
        'group/nav relative flex h-11 w-full cursor-pointer items-center rounded-xl text-left transition-colors duration-200',
        isActive ? 'text-primary' : 'text-sidebar-foreground/62 hover:text-sidebar-foreground',
      )}
    >
      <span
        className={clsx(
          'pointer-events-none absolute inset-y-0 left-2.5 right-2.5 rounded-xl transition-colors duration-200',
          isActive ? 'border border-primary/15 bg-primary/10' : 'bg-transparent group-hover/nav:bg-sidebar-accent',
        )}
      />
      <span className="relative z-10 flex w-16 shrink-0 items-center justify-center">
        <Icon className="h-4.5 w-4.5" />
      </span>
      <span className="relative z-10 min-w-0 flex-1 truncate pr-3 text-sm font-medium">{label}</span>
    </button>
  );

  if (expanded) {
    return button;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  );
}

/* ──────────────────────────────────────────────────────────────
 * AppShell — layout
 * ────────────────────────────────────────────────────────────── */

export default function AppShell({
  activeTab,
  onTabChange,
  isConnected,
  fanData,
  temperature,
  autoControl,
  error,
  bridgeWarning,
  onDismissBridgeWarning,
  statusContent,
  curveContent,
  controlContent,
  aboutContent,
}: AppShellProps) {
  const { t } = useTranslation();
  const [isWindowsChrome, setIsWindowsChrome] = useState(false);
  // 是否启用系统模糊材质(云母)：关闭时窗口为不透明，前端需回退为不透明背景。
  const [nativeBackdrop, setNativeBackdrop] = useState(false);
  const [isMaximised, setIsMaximised] = useState(false);
  const [sidebarExpanded, setSidebarExpanded] = useState(readStoredSidebarExpanded);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const previousActiveTabRef = useRef<ActiveTab>(activeTab);

  const syncWindowState = useCallback(async () => {
    try {
      setIsMaximised(await WindowIsMaximised());
    } catch {
      setIsMaximised(false);
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    let cleanup = () => {};

    const initializeWindowChrome = async () => {
      try {
        const env = await Environment();
        if (disposed) return;
        const isWindows = env.platform === 'windows';
        setIsWindowsChrome(isWindows);
        if (!isWindows) {
          setNativeBackdrop(false);
          setIsMaximised(false);
          return;
        }
        try {
          setNativeBackdrop(await WindowBlurEnabled());
        } catch {
          setNativeBackdrop(true);
        }
        const handleResize = () => void syncWindowState();
        window.addEventListener('resize', handleResize);
        cleanup = () => window.removeEventListener('resize', handleResize);
        await syncWindowState();
      } catch {
        if (!disposed) {
          setIsWindowsChrome(false);
          setIsMaximised(false);
        }
      }
    };

    void initializeWindowChrome();

    return () => {
      disposed = true;
      cleanup();
    };
  }, [syncWindowState]);

  const scheduleWindowStateSync = useCallback(() => {
    window.setTimeout(() => void syncWindowState(), 80);
  }, [syncWindowState]);

  const handleToggleMaximise = useCallback(() => {
    WindowToggleMaximise();
    scheduleWindowStateSync();
  }, [scheduleWindowStateSync]);

  const handleOpenRepository = useCallback(() => {
    try {
      BrowserOpenURL(BRAND.repositoryUrl);
    } catch {
      window.open(BRAND.repositoryUrl, '_blank', 'noopener,noreferrer');
    }
  }, []);

  const handleLogoKeyDown = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      handleOpenRepository();
    }
  }, [handleOpenRepository]);

  const handleToggleSidebar = useCallback(() => {
    setSidebarExpanded((prev) => {
      const next = !prev;
      try {
        window.localStorage.setItem(SIDEBAR_EXPANDED_STORAGE_KEY, next ? '1' : '0');
      } catch {
        /* 持久化失败不影响交互 */
      }
      return next;
    });
  }, []);

  const sidebarWidth = sidebarExpanded ? SIDEBAR_EXPANDED_WIDTH : SIDEBAR_COLLAPSED_WIDTH;

  const handleTabChange = (tab: ActiveTab) => {
    if (tab === activeTab) return;
    onTabChange(tab);
  };

  const contentMap: Record<ActiveTab, ReactNode> = {
    status: statusContent,
    curve: curveContent,
    control: controlContent,
    about: aboutContent,
  };
  const transitionDirection = getTabTransitionDirection(previousActiveTabRef.current, activeTab);

  useEffect(() => {
    if (previousActiveTabRef.current === activeTab) {
      return;
    }
    const scrollElement = scrollRef.current;
    if (scrollElement) {
      scrollElement.scrollTop = 0;
      scrollElement.scrollLeft = 0;
    }
    previousActiveTabRef.current = activeTab;
  }, [activeTab]);

  return (
    <div
      className={clsx(
        'glacier-shell relative flex h-dvh w-full overflow-hidden bg-background text-foreground',
        activeTab === 'status' && 'app-shell--hide-scrollbar',
        isWindowsChrome && nativeBackdrop && 'glacier-native-backdrop',
      )}
    >
      {isWindowsChrome && (
        <TitleBar
          minimizeLabel={t('appShell.titleBar.minimize')}
          maximizeLabel={t('appShell.titleBar.maximize')}
          restoreLabel={t('appShell.titleBar.restore')}
          closeLabel={t('appShell.titleBar.close')}
          isMaximised={isMaximised}
          leftOffset={sidebarWidth}
          leftSlot={<StatusBadges isConnected={isConnected} fanData={fanData} temperature={temperature} autoControl={autoControl} compact />}
          onMinimise={() => WindowMinimise()}
          onToggleMaximise={handleToggleMaximise}
          onClose={() => Quit()}
        />
      )}

      <aside
        className="glacier-sidebar flex shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar text-sidebar-foreground shadow-[1px_0_0_rgba(15,23,42,0.04)] transition-[width] duration-300 ease-out dark:shadow-[1px_0_0_rgba(255,255,255,0.04)]"
        style={{ width: sidebarWidth }}
      >
        <div className="flex h-[76px] items-center pl-2" style={DRAG_STYLE}>
          <Tooltip>
            <TooltipTrigger asChild>
              <div
                aria-label={t('appShell.repository.openAria', { name: BRAND.name })}
                role="link"
                tabIndex={0}
                onClick={handleOpenRepository}
                onKeyDown={handleLogoKeyDown}
                className="group flex cursor-pointer items-center outline-none"
                style={NO_DRAG_STYLE}
              >
                <img
                  src="/brand/wordmark-light.png"
                  alt={BRAND.name}
                  draggable={false}
                  className={clsx(
                    'h-auto origin-left object-contain transition-all duration-300 ease-out group-hover:scale-[1.03] dark:hidden',
                    sidebarExpanded ? 'w-[104px]' : 'w-[48px]',
                  )}
                />
                <img
                  src="/brand/wordmark-dark.png"
                  alt={BRAND.name}
                  draggable={false}
                  className={clsx(
                    'hidden h-auto origin-left object-contain transition-all duration-300 ease-out group-hover:scale-[1.03] dark:block',
                    sidebarExpanded ? 'w-[104px]' : 'w-[48px]',
                  )}
                />
              </div>
            </TooltipTrigger>
            <TooltipContent side="right">{t('appShell.repository.openTooltip')}</TooltipContent>
          </Tooltip>
        </div>

        <nav className="flex flex-1 flex-col gap-1" role="tablist" style={NO_DRAG_STYLE}>
          {MAIN_TAB_ITEMS.map((tab) => (
            <SidebarNavButton
              key={tab.id}
              icon={tab.icon}
              label={t(tab.titleKey)}
              isActive={activeTab === tab.id}
              expanded={sidebarExpanded}
              onClick={() => handleTabChange(tab.id)}
              role="tab"
            />
          ))}
        </nav>

        <div className="flex flex-col gap-1 pb-5" style={NO_DRAG_STYLE}>
          <SidebarNavButton
            icon={ABOUT_TAB.icon}
            label={t(ABOUT_TAB.titleKey)}
            isActive={activeTab === ABOUT_TAB.id}
            expanded={sidebarExpanded}
            onClick={() => handleTabChange(ABOUT_TAB.id)}
          />

          {(() => {
            const toggleLabel = sidebarExpanded ? t('appShell.sidebar.collapse') : t('appShell.sidebar.expand');
            const ToggleIcon = sidebarExpanded ? PanelLeftClose : PanelLeftOpen;
            const toggleButton = (
              <button
                type="button"
                aria-label={toggleLabel}
                aria-expanded={sidebarExpanded}
                onClick={handleToggleSidebar}
                className="group/nav relative flex h-11 w-full cursor-pointer items-center rounded-xl text-left text-sidebar-foreground/62 transition-colors duration-200 hover:text-sidebar-foreground"
              >
                <span className="pointer-events-none absolute inset-y-0 left-2.5 right-2.5 rounded-xl bg-transparent transition-colors duration-200 group-hover/nav:bg-sidebar-accent" />
                <span className="relative z-10 flex w-16 shrink-0 items-center justify-center">
                  <ToggleIcon className="h-4.5 w-4.5" />
                </span>
                <span className="relative z-10 min-w-0 flex-1 truncate pr-3 text-sm font-medium">{toggleLabel}</span>
              </button>
            );
            return sidebarExpanded ? (
              toggleButton
            ) : (
              <Tooltip>
                <TooltipTrigger asChild>{toggleButton}</TooltipTrigger>
                <TooltipContent side="right">{toggleLabel}</TooltipContent>
              </Tooltip>
            );
          })()}
        </div>
      </aside>

      <section className="glacier-content relative flex min-w-0 flex-1 flex-col overflow-hidden">
        {!isWindowsChrome && (
          <header
            className="shrink-0 border-b border-border/65 bg-background/92 px-4 pb-3 pt-3 backdrop-blur-xl sm:px-5 lg:px-6"
            style={DRAG_STYLE}
          >
            <div className="mx-auto flex min-h-9 max-w-[1120px] min-[1536px]:max-w-[1280px] min-[1800px]:max-w-[1440px] min-[2400px]:max-w-[1560px] items-center justify-start gap-3" style={NO_DRAG_STYLE}>
              <StatusBadges isConnected={isConnected} fanData={fanData} temperature={temperature} autoControl={autoControl} />
            </div>
          </header>
        )}

        <div className="glacier-content-panel relative min-h-0 flex-1 overflow-hidden">
          <div
            ref={scrollRef}
            className="app-scroll-root app-scroll-root--hide-native h-full"
            style={NO_DRAG_STYLE}
          >
            <div className="min-h-full px-4 pb-6 pt-4 sm:px-5 lg:px-6">

          {/* Alerts */}
          <div className="mx-auto max-w-[1120px] min-[1536px]:max-w-[1280px] min-[1800px]:max-w-[1440px] min-[2400px]:max-w-[1560px]">
            <AnimatePresence>
              {error && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mb-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-2.5 text-sm text-destructive">
                    {error}
                  </div>
                </motion.div>
              )}

              {bridgeWarning && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mb-3 flex items-start gap-3 rounded-lg border border-amber-300/50 bg-amber-50/80 px-4 py-2.5 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/15 dark:text-amber-200">
                    <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                    <p className="flex-1 text-sm leading-relaxed">{bridgeWarning}</p>
                    <button
                      type="button"
                      aria-label={t('appShell.bridgeWarning.closeAria')}
                      onClick={onDismissBridgeWarning}
                      className="cursor-pointer rounded p-0.5 transition hover:bg-amber-200/60 dark:hover:bg-amber-800/40"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          {/* Tab content */}
          <main className="mx-auto w-full max-w-[1120px] min-[1536px]:max-w-[1280px] min-[1800px]:max-w-[1440px] min-[2400px]:max-w-[1560px] min-w-0 overflow-hidden">
            <AnimatePresence mode="wait" initial={false} custom={transitionDirection}>
              <motion.div
                key={activeTab}
                custom={transitionDirection}
                variants={TAB_CONTENT_VARIANTS}
                initial="enter"
                animate="center"
                exit="exit"
                data-page-reveal="cards"
                className="w-full min-w-0 px-1 pb-2 will-change-transform"
              >
                {contentMap[activeTab]}
              </motion.div>
            </AnimatePresence>
          </main>
          </div>
        </div>

        {/* Floating overlay scrollbar — never reserves width */}
        {/* 内容面板在 Windows 下已经从顶栏下沿开始，滚动条无需再避让顶栏 */}
        <OverlayScrollbar scrollRef={scrollRef} topOffset={6} />
        </div>
      </section>
    </div>
  );
}

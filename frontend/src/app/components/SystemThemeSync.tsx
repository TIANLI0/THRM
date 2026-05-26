'use client';

import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';

type ThemeMode = 'system' | 'light' | 'dark' | 'thrm';

function normalizeThemeMode(mode: unknown): ThemeMode {
  if (mode === 'light' || mode === 'dark' || mode === 'thrm') return mode;
  return 'system';
}

function applyTheme(mode: ThemeMode) {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const isDark = mode === 'dark' || (mode === 'system' && media.matches);
  document.documentElement.classList.toggle('dark', isDark);
  document.documentElement.classList.toggle('theme-thrm', mode === 'thrm');
}

/**
 * 在 <html> 上写入 data-os：CSS 用 [data-os="mac"] / [data-os="win"]
 * 精确分流字体渲染策略 — 因为 Windows ClearType 子像素和 macOS 灰阶反锯齿
 * 对同一段 CSS 的诉求是反的。
 */
function detectOs(): 'win' | 'mac' | 'linux' | 'other' {
  if (typeof navigator === 'undefined') return 'other';
  const ua = navigator.userAgent || '';
  const platform = (navigator as Navigator & { userAgentData?: { platform?: string } }).userAgentData?.platform || '';
  const probe = `${ua} ${platform}`.toLowerCase();
  if (probe.includes('windows') || probe.includes('win32') || probe.includes('win64')) return 'win';
  if (probe.includes('mac') || probe.includes('darwin')) return 'mac';
  if (probe.includes('linux')) return 'linux';
  return 'other';
}

export default function SystemThemeSync() {
  const themeMode = useAppStore((state) => normalizeThemeMode((state.config as any)?.themeMode));

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    applyTheme(themeMode);

    const handleChange = (event: MediaQueryListEvent) => {
      if (themeMode !== 'system') {
        return;
      }
      document.documentElement.classList.toggle('dark', event.matches);
      document.documentElement.classList.remove('theme-thrm');
    };

    media.addEventListener('change', handleChange);
    return () => media.removeEventListener('change', handleChange);
  }, [themeMode]);

  // 仅首挂载时打 data-os；UA / 平台不会运行时变化
  useEffect(() => {
    document.documentElement.dataset.os = detectOs();
  }, []);

  return null;
}

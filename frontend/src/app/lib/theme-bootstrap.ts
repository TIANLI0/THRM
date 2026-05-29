export const CUSTOM_STYLE_ID = 'thrm-custom-theme-style';
export const THEME_BOOTSTRAP_STORAGE_KEY = 'thrm.theme-bootstrap';

const THEME_BOOTSTRAP_VERSION = 1;
const BUILTIN_THEME_MODES = ['system', 'light', 'dark'] as const;

export type BuiltinThemeMode = (typeof BUILTIN_THEME_MODES)[number];
export type CustomThemeBase = 'light' | 'dark';

export type ThemeBootstrapSnapshot = {
  version: typeof THEME_BOOTSTRAP_VERSION;
  mode: string;
  base?: CustomThemeBase;
  css?: string;
};

export function isBuiltinMode(mode: string): mode is BuiltinThemeMode {
  return (BUILTIN_THEME_MODES as readonly string[]).includes(mode);
}

function normalizeMode(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  return trimmed ? trimmed : null;
}

export function parseThemeBootstrapSnapshot(raw: string | null | undefined): ThemeBootstrapSnapshot | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<ThemeBootstrapSnapshot> | null;
    const mode = normalizeMode(parsed?.mode);
    if (!parsed || parsed.version !== THEME_BOOTSTRAP_VERSION || !mode) {
      return null;
    }

    if (isBuiltinMode(mode)) {
      return {
        version: THEME_BOOTSTRAP_VERSION,
        mode,
      };
    }

    return {
      version: THEME_BOOTSTRAP_VERSION,
      mode,
      base: parsed.base === 'dark' ? 'dark' : 'light',
      css: typeof parsed.css === 'string' ? parsed.css : '',
    };
  } catch {
    return null;
  }
}

export function serializeThemeBootstrapSnapshot(snapshot: ThemeBootstrapSnapshot): string {
  return JSON.stringify(snapshot);
}

export function createBuiltinThemeSnapshot(mode: BuiltinThemeMode): ThemeBootstrapSnapshot {
  return {
    version: THEME_BOOTSTRAP_VERSION,
    mode,
  };
}

export function createCustomThemeSnapshot(mode: string, base: CustomThemeBase, css: string): ThemeBootstrapSnapshot {
  return {
    version: THEME_BOOTSTRAP_VERSION,
    mode,
    base,
    css,
  };
}

export function getThemeBootstrapScript(): string {
  return `
(() => {
  const STORAGE_KEY = ${JSON.stringify(THEME_BOOTSTRAP_STORAGE_KEY)};
  const STYLE_ID = ${JSON.stringify(CUSTOM_STYLE_ID)};
  const BUILTIN_MODES = new Set(${JSON.stringify([...BUILTIN_THEME_MODES])});
  const root = document.documentElement;

  const applyBaseTheme = (isDark) => {
    const styleEl = document.getElementById(STYLE_ID);
    if (styleEl) styleEl.remove();
    delete root.dataset.theme;
    root.classList.toggle('dark', !!isDark);
  };

  const detectOs = () => {
    const ua = navigator.userAgent || '';
    const platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    const probe = (ua + ' ' + platform).toLowerCase();
    if (probe.includes('windows') || probe.includes('win32') || probe.includes('win64')) return 'win';
    if (probe.includes('mac') || probe.includes('darwin')) return 'mac';
    if (probe.includes('linux')) return 'linux';
    return 'other';
  };

  try {
    root.dataset.os = detectOs();
  } catch {
    // noop
  }

  let snapshot = null;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    snapshot = raw ? JSON.parse(raw) : null;
  } catch {
    snapshot = null;
  }

  if (!snapshot || snapshot.version !== ${JSON.stringify(THEME_BOOTSTRAP_VERSION)} || typeof snapshot.mode !== 'string' || !snapshot.mode) {
    return;
  }

  const prefersDark = !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
  if (BUILTIN_MODES.has(snapshot.mode)) {
    applyBaseTheme(snapshot.mode === 'dark' || (snapshot.mode === 'system' && prefersDark));
    return;
  }

  const css = typeof snapshot.css === 'string' ? snapshot.css : '';
  const base = snapshot.base === 'dark' ? 'dark' : 'light';
  if (!css) {
    applyBaseTheme(base === 'dark');
    return;
  }

  let styleEl = document.getElementById(STYLE_ID);
  if (!styleEl) {
    styleEl = document.createElement('style');
    styleEl.id = STYLE_ID;
    document.head.appendChild(styleEl);
  }
  styleEl.textContent = css;
  root.classList.toggle('dark', base === 'dark');
  root.dataset.theme = snapshot.mode;
})();
`.trim();
}
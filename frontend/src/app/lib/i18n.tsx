"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import i18n, { type Resource } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
// 只有默认语言（同时是 fallbackLng）随主包静态打包，其它语言按需加载，
// 避免每次冷启动都解析全部语言的完整词条。
import zhCN from "../locales/zh-CN/translation.json";

export const SUPPORTED_LOCALES = ["zh-CN", "en-US", "ja-JP", "ko-KR"] as const;
export type AppLocale = (typeof SUPPORTED_LOCALES)[number];

const DEFAULT_LOCALE: AppLocale = "zh-CN";
const LOCALE_STORAGE_KEY = "thrm.locale";

const resources: Resource = {
  "zh-CN": { translation: zhCN },
};

type LocaleBundle = Record<string, unknown>;

const LOCALE_LOADERS: Partial<Record<AppLocale, () => Promise<{ default: LocaleBundle }>>> = {
  "en-US": () => import("../locales/en-US/translation.json"),
  "ja-JP": () => import("../locales/ja-JP/translation.json"),
  "ko-KR": () => import("../locales/ko-KR/translation.json"),
};

const pendingLocaleLoads = new Map<AppLocale, Promise<void>>();

/**
 * 确保某个语言的词条已注册到 i18next。已就绪时返回 null（可同步切换），
 * 否则返回加载中的 Promise；加载失败时 reject，调用方照常切换即可——
 * 缺失的词条会按 fallbackLng 回退到默认语言，与原先缺词时的表现一致。
 */
function ensureLocaleLoaded(locale: AppLocale): Promise<void> | null {
  const loader = LOCALE_LOADERS[locale];
  if (!loader || i18n.hasResourceBundle(locale, "translation")) {
    return null;
  }

  let pending = pendingLocaleLoads.get(locale);
  if (!pending) {
    pending = loader().then(
      (mod) => {
        i18n.addResourceBundle(locale, "translation", mod.default, true, true);
      },
      (error: unknown) => {
        pendingLocaleLoads.delete(locale);
        throw error;
      },
    );
    pendingLocaleLoads.set(locale, pending);
  }
  return pending;
}

function normalizeLocale(value?: string | null): AppLocale {
  if (!value) {
    return DEFAULT_LOCALE;
  }

  if (SUPPORTED_LOCALES.includes(value as AppLocale)) {
    return value as AppLocale;
  }

  const lowered = value.toLowerCase();
  if (lowered.startsWith("zh")) {
    return "zh-CN";
  }
  if (lowered.startsWith("ja")) {
    return "ja-JP";
  }
  if (lowered.startsWith("ko")) {
    return "ko-KR";
  }
  if (lowered.startsWith("en")) {
    return "en-US";
  }

  return DEFAULT_LOCALE;
}

function readPreferredLocale(): AppLocale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (stored) {
    return normalizeLocale(stored);
  }

  return normalizeLocale(window.navigator.language);
}

function syncDocumentLocale(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.lang = locale;
  document.documentElement.dataset.locale = locale;
}

function syncDocumentMetadata(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  const t = i18n.getFixedT(locale);
  document.title = t("common.appName");

  const description = t("common.metaDescription");
  const descriptionMeta = document.querySelector('meta[name="description"]');
  if (descriptionMeta) {
    descriptionMeta.setAttribute("content", description);
  }
}

if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources,
    lng: DEFAULT_LOCALE,
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: [...SUPPORTED_LOCALES],
    defaultNS: "translation",
    ns: ["translation"],
    interpolation: {
      escapeValue: false,
    },
    returnNull: false,
    initAsync: false,
  });
}

// 模块一求值就开始拉取用户偏好的语言包，与页面水合、核心初始化并行，
// 等界面需要显示文字时基本已就绪。
if (typeof window !== "undefined") {
  ensureLocaleLoaded(readPreferredLocale())?.catch(() => {});
}

type LocaleContextValue = {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => void;
  supportedLocales: readonly AppLocale[];
  /** 启动时偏好语言的词条已加载并生效；之后运行时切换语言不会再变回 false。 */
  localeReady: boolean;
};

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function AppI18nProvider({ children }: { children: React.ReactNode }) {
  // locale 是已经生效的语言；requestedLocale 是想切到的语言（词条可能还在加载）。
  const [locale, setLocaleState] = useState<AppLocale>(DEFAULT_LOCALE);
  const [requestedLocale, setRequestedLocale] = useState<AppLocale | null>(null);
  const [localeReady, setLocaleReady] = useState(false);

  useEffect(() => {
    setRequestedLocale(readPreferredLocale());
  }, []);

  useEffect(() => {
    if (!requestedLocale) {
      return;
    }

    window.localStorage.setItem(LOCALE_STORAGE_KEY, requestedLocale);

    let cancelled = false;
    const apply = () => {
      if (cancelled) {
        return;
      }
      void i18n.changeLanguage(requestedLocale);
      syncDocumentLocale(requestedLocale);
      syncDocumentMetadata(requestedLocale);
      setLocaleState(requestedLocale);
      setLocaleReady(true);
    };

    const pending = ensureLocaleLoaded(requestedLocale);
    if (pending) {
      pending.then(apply, apply);
    } else {
      apply();
    }

    return () => {
      cancelled = true;
    };
  }, [requestedLocale]);

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale: (nextLocale) => setRequestedLocale(normalizeLocale(nextLocale)),
      supportedLocales: SUPPORTED_LOCALES,
      localeReady,
    }),
    [locale, localeReady],
  );

  return (
    <I18nextProvider i18n={i18n}>
      <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
    </I18nextProvider>
  );
}

export function useLocale() {
  const context = useContext(LocaleContext);

  if (!context) {
    throw new Error("useLocale must be used within AppI18nProvider");
  }

  return context;
}

export { i18n };

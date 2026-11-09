// 把界面语言同步给核心服务，让托盘菜单、系统通知与快捷键提示跟随界面语言。
//
// 只依赖 i18next 单例的 languageChanged 事件，不依赖语言资源如何加载，
// 因此与 i18n.tsx 的实现细节解耦。
//
// 上报策略：
// - 去抖：启动时 i18n 会先初始化为默认语言再切到用户偏好，只上报最终结果；
// - 去重：核心确认过的语言不再重复上报；核心侧语言未变时也不会写配置；
// - 补发：核心启动晚于界面或中途重启时，借 core-service-ok / core-resynced 再试一次，
//   失败不在这里轮询重试。
import i18n from 'i18next';
import { Events } from '@wailsio/runtime';
import { App } from '@go/guiapp';

const CORE_LANGUAGES = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR'];
const SYNC_DEBOUNCE_MS = 400;

function currentLanguage(): string | null {
  const lang = i18n.resolvedLanguage || i18n.language;
  return lang && CORE_LANGUAGES.includes(lang) ? lang : null;
}

function subscribe(eventName: keyof Events.CustomEvents, callback: () => void): () => void {
  try {
    return Events.On(eventName, () => callback());
  } catch {
    // 非 Wails 环境（浏览器预览）没有运行时
    return () => {};
  }
}

export function startCoreLanguageSync(): () => void {
  if (typeof window === 'undefined') {
    return () => {};
  }

  let acknowledged: string | null = null;
  let inFlight: string | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;

  const flush = async () => {
    timer = undefined;
    const lang = currentLanguage();
    if (disposed || !lang || lang === acknowledged || lang === inFlight) {
      return;
    }

    inFlight = lang;
    try {
      acknowledged = (await App.SetUILanguage(lang)) || lang;
    } catch {
      // 核心暂不可达：等 core-service-ok / core-resynced 时再补发
    } finally {
      inFlight = null;
    }

    // 上报期间用户又切换了语言
    const latest = currentLanguage();
    if (!disposed && latest && latest !== lang) {
      schedule();
    }
  };

  const schedule = () => {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
    timer = setTimeout(() => {
      void flush();
    }, SYNC_DEBOUNCE_MS);
  };

  const onCoreResynced = () => {
    // 核心重启过，不再假定它记得上次的确认
    acknowledged = null;
    schedule();
  };

  i18n.on('languageChanged', schedule);
  const offServiceOK = subscribe('core-service-ok', schedule);
  const offResynced = subscribe('core-resynced', onCoreResynced);
  schedule();

  return () => {
    disposed = true;
    if (timer !== undefined) {
      clearTimeout(timer);
    }
    i18n.off('languageChanged', schedule);
    offServiceOK();
    offResynced();
  };
}

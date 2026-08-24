'use client';

// 更新下载/安装进度弹窗。由 AppShell 常驻渲染，因此在任何标签页都能看到，
// 也不会因为离开"关于"页而丢掉进度与事件订阅。
import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import { Download, ExternalLink, Rocket } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { BRAND } from '../lib/brand';
import { useUpdateStore } from '../store/update-store';
import { Button } from './ui/index';

export default function UpdateProgress() {
  const { t } = useTranslation();
  const stage = useUpdateStore((state) => state.stage);
  const percent = useUpdateStore((state) => state.percent);
  const error = useUpdateStore((state) => state.error);
  const releaseUrl = useUpdateStore((state) => state.releaseUrl);
  const subscribeProgress = useUpdateStore((state) => state.subscribeProgress);
  const startDownloadInstall = useUpdateStore((state) => state.startDownloadInstall);
  const dismiss = useUpdateStore((state) => state.dismiss);

  useEffect(() => {
    subscribeProgress();
  }, [subscribeProgress]);

  // 仍然走 portal：外层有 transform/backdrop-filter 的祖先，fixed 会被它们变成
  // 相对定位的容器块，弹窗位置就会跟着页面内容跑。
  if (stage === 'idle' || typeof document === 'undefined') return null;

  const retry = () => {
    void startDownloadInstall({
      windowTitle: t('aboutPanel.version.updaterWindowTitle'),
      windowBody: t('aboutPanel.version.updaterWindowBody'),
      windowRestarting: t('aboutPanel.version.updaterWindowRestarting'),
    });
  };

  return createPortal(
    <div className="fixed bottom-4 left-4 right-4 z-90 rounded-2xl border border-border/80 bg-popover/98 p-4 shadow-xl shadow-black/10 backdrop-blur-xl animate-in fade-in-0 slide-in-from-bottom-2 sm:bottom-6 sm:left-auto sm:right-6 sm:w-88">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          {stage === 'error' ? (
            <Rocket className="h-4 w-4 text-amber-500" />
          ) : (
            <Download className="h-4 w-4" />
          )}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <div className="text-sm font-semibold text-foreground">
              {stage === 'downloading'
                ? t('aboutPanel.version.floatDownloadingTitle')
                : stage === 'installing'
                  ? t('aboutPanel.version.floatInstallingTitle')
                  : stage === 'done'
                    ? t('aboutPanel.version.floatDoneTitle')
                    : t('aboutPanel.version.floatErrorTitle')}
            </div>

            {(stage === 'error' || stage === 'done') && (
              <button
                type="button"
                onClick={dismiss}
                className="shrink-0 cursor-pointer rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition hover:bg-muted hover:text-foreground"
                aria-label={t('common.actions.close')}
              >
                ✕
              </button>
            )}
          </div>

          {stage === 'error' ? (
            <p className="mt-1 text-xs leading-relaxed text-amber-700 dark:text-amber-300">
              {error
                ? t('aboutPanel.version.installFailed', { error })
                : t('aboutPanel.version.floatErrorTitle')}
            </p>
          ) : stage === 'done' ? (
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              {t('aboutPanel.version.floatDoneHint')}
            </p>
          ) : (
            <>
              <div className="mt-2.5 h-1.5 w-full overflow-hidden rounded-full bg-border/60">
                <div
                  className={`h-full rounded-full bg-primary transition-[width] duration-200 ${
                    stage !== 'downloading' ? 'animate-pulse' : ''
                  }`}
                  style={{ width: `${stage === 'downloading' ? percent : 100}%` }}
                />
              </div>
              <div className="mt-1.5 flex items-center justify-between gap-3 text-xs text-muted-foreground">
                <span className="min-w-0">
                  {stage === 'downloading'
                    ? t('aboutPanel.version.downloading', { percent })
                    : stage === 'installing'
                      ? t('aboutPanel.version.installingHint')
                      : t('aboutPanel.version.installStarted')}
                </span>
                {stage === 'downloading' && (
                  <span className="shrink-0 tabular-nums">{percent}%</span>
                )}
              </div>
            </>
          )}

          {stage === 'error' && (
            <div className="mt-3 flex flex-wrap gap-2">
              <Button
                variant="primary"
                size="sm"
                onClick={retry}
                icon={<Download className="h-3.5 w-3.5" />}
              >
                {t('aboutPanel.version.floatRetry')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => BrowserOpenURL(releaseUrl || BRAND.latestReleaseUrl)}
                icon={<ExternalLink className="h-3.5 w-3.5" />}
              >
                {t('aboutPanel.version.openReleasePage')}
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}

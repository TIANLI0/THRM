'use client';

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { Download, Heart, Mail, MessageCircleMore, RefreshCw, Rocket, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { apiService } from '../services/api';
import { Badge, Button, ScrollArea } from './ui/index';

type ReleaseChannel = 'stable' | 'prerelease';

type GithubReleaseAsset = {
  name?: string;
  browser_download_url?: string;
};

type GithubRelease = {
  tag_name?: string;
  html_url?: string;
  body?: string;
  prerelease?: boolean;
  draft?: boolean;
  assets?: GithubReleaseAsset[];
};

const INSTALLER_ASSET_NAME = 'THRM-amd64-installer.exe';

function findInstallerAsset(assets: GithubReleaseAsset[] | undefined): string {
  if (!Array.isArray(assets)) return '';
  const exact = assets.find((asset) => asset?.name === INSTALLER_ASSET_NAME);
  if (exact?.browser_download_url) return exact.browser_download_url;
  const fuzzy = assets.find(
    (asset) => typeof asset?.name === 'string' && /installer\.exe$/i.test(asset.name) && !!asset.browser_download_url,
  );
  return fuzzy?.browser_download_url || '';
}

type UpdateStage = 'idle' | 'downloading' | 'installing' | 'done' | 'error';

function openUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

function isLatestVersion(currentVersion: string, latestVersion: string) {
  const normalize = (value: string) => value.trim().replace(/^v/i, '').toLowerCase();
  const currentRaw = normalize(currentVersion);
  const latestRaw = normalize(latestVersion);
  if (!currentRaw || !latestRaw) return true;
  if (currentRaw === latestRaw) return true;

  const parseNightly = (value: string): number | null => {
    const match = value.match(/^nightly[-.]?(\d{8})$/i);
    return match ? Number(match[1]) : null;
  };

  const parseSemverParts = (value: string): number[] | null => {
    const base = value.split('-')[0].split('+')[0];
    if (!/^\d+(\.\d+){0,3}$/.test(base)) return null;
    return base.split('.').map((part) => Number(part));
  };

  const currentNightly = parseNightly(currentRaw);
  const latestNightly = parseNightly(latestRaw);
  if (currentNightly !== null && latestNightly !== null) {
    return latestNightly <= currentNightly;
  }

  const currentSemver = parseSemverParts(currentRaw);
  const latestSemver = parseSemverParts(latestRaw);
  if (!currentSemver || !latestSemver) {
    return false;
  }

  const current = currentSemver;
  const latest = latestSemver;
  const length = Math.max(current.length, latest.length);

  for (let index = 0; index < length; index += 1) {
    const currentPart = current[index] ?? 0;
    const latestPart = latest[index] ?? 0;
    if (latestPart > currentPart) return false;
    if (latestPart < currentPart) return true;
  }

  return true;
}

const ABOUT_CARD_CLASS = 'min-w-0 rounded-3xl border border-border/70 bg-card p-5';

export default function AboutPanel() {
  const { t } = useTranslation();
  const [appVersion, setAppVersion] = useState('');
  const [releaseChannel, setReleaseChannel] = useState<ReleaseChannel>('stable');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState<string>(BRAND.latestReleaseUrl);
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [latestReleaseIsPrerelease, setLatestReleaseIsPrerelease] = useState(false);
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const [installerUrl, setInstallerUrl] = useState('');
  const [updateStage, setUpdateStage] = useState<UpdateStage>('idle');
  const [updatePercent, setUpdatePercent] = useState(0);
  const [updateError, setUpdateError] = useState('');
  const [isSponsorHovered, setIsSponsorHovered] = useState(false);
  const [isSponsorPinned, setIsSponsorPinned] = useState(false);
  const [sponsorPopupStyle, setSponsorPopupStyle] = useState<{ top: number; left: number; placement: 'top' | 'bottom' } | null>(null);
  const sponsorRef = useRef<HTMLDivElement | null>(null);
  const sponsorPopupRef = useRef<HTMLDivElement | null>(null);
  const sponsorHoverTimerRef = useRef<number | null>(null);

  const supportMethods = useMemo(
    () => [
      {
        label: t('aboutPanel.sponsor.methods.alipay'),
        image: '/support/alipay.jpg',
      },
      {
        label: t('aboutPanel.sponsor.methods.wechat'),
        image: '/support/wechat.png',
      },
    ],
    [t],
  );

  const faqItems = useMemo(
    () => t('aboutPanel.faq.items', { returnObjects: true }) as Array<{ question: string; answer: string }>,
    [t],
  );

  const checkLatestRelease = useCallback(async (channel: ReleaseChannel = releaseChannel) => {
    setReleaseLoading(true);
    setReleaseError('');
    setInstallerUrl('');

    const headers = { Accept: 'application/vnd.github+json' };

    try {
      let targetRelease: GithubRelease | null = null;

      if (channel === 'prerelease') {
        const response = await fetch(`${BRAND.releasesApiUrl}?per_page=30`, { headers });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);

        const releases = (await response.json()) as GithubRelease[];
        targetRelease = (Array.isArray(releases) ? releases : []).find((item) => !item?.draft && !!item?.prerelease) || null;

        if (!targetRelease) {
          setLatestReleaseTag('');
          setLatestReleaseUrl(BRAND.latestReleaseUrl);
          setLatestReleaseBody('');
          setLatestReleaseIsPrerelease(false);
          setReleaseError(t('aboutPanel.version.noPrereleaseFound'));
          return;
        }
      } else {
        const response = await fetch(BRAND.latestReleaseApiUrl, { headers });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        targetRelease = (await response.json()) as GithubRelease;
      }

      setLatestReleaseTag(targetRelease?.tag_name || '');
      setLatestReleaseUrl(targetRelease?.html_url || BRAND.latestReleaseUrl);
      setLatestReleaseBody(typeof targetRelease?.body === 'string' ? targetRelease.body.trim() : '');
      setLatestReleaseIsPrerelease(!!targetRelease?.prerelease);
      setInstallerUrl(findInstallerAsset(targetRelease?.assets));
    } catch {
      setLatestReleaseTag('');
      setLatestReleaseUrl(BRAND.latestReleaseUrl);
      setLatestReleaseBody('');
      setLatestReleaseIsPrerelease(false);
      setReleaseError(t('aboutPanel.version.checkFailed'));
    } finally {
      setReleaseLoading(false);
    }
  }, [releaseChannel, t]);

  useEffect(() => {
    let disposed = false;
    apiService.getAppVersion()
      .then((value) => {
        if (!disposed) setAppVersion(value || '');
      })
      .catch(() => {
        if (!disposed) setAppVersion('');
      });
    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    void checkLatestRelease(releaseChannel);
  }, [checkLatestRelease, releaseChannel]);

  useEffect(() => {
    const dispose = apiService.onUpdateDownloadProgress((payload) => {
      const stage = payload?.stage;
      if (stage === 'downloading') {
        setUpdateStage('downloading');
        setUpdatePercent(typeof payload.percent === 'number' && payload.percent >= 0 ? payload.percent : 0);
      } else if (stage === 'installing') {
        setUpdateStage('installing');
        setUpdatePercent(100);
      } else if (stage === 'done') {
        setUpdateStage('done');
        setUpdatePercent(100);
      } else if (stage === 'error') {
        setUpdateStage('error');
        setUpdateError(payload?.message || '');
      }
    });
    return () => {
      dispose?.();
    };
  }, []);

  const startDownloadInstall = useCallback(async () => {
    if (!installerUrl) {
      return;
    }
    setUpdateStage('downloading');
    setUpdatePercent(0);
    setUpdateError('');
    try {
      await apiService.downloadAndInstallUpdate(
        installerUrl,
        t('aboutPanel.version.updaterWindowTitle'),
        t('aboutPanel.version.updaterWindowBody'),
        t('aboutPanel.version.updaterWindowRestarting'),
      );
    } catch (error) {
      setUpdateStage('error');
      const message = error instanceof Error ? error.message : String(error);
      setUpdateError(message);
      toast.error(t('aboutPanel.version.installFailed', { error: message }));
    }
  }, [installerUrl, t]);

  const clearSponsorHoverTimer = useCallback(() => {
    if (sponsorHoverTimerRef.current !== null) {
      window.clearTimeout(sponsorHoverTimerRef.current);
      sponsorHoverTimerRef.current = null;
    }
  }, []);

  const handleSponsorHoverEnter = useCallback(() => {
    clearSponsorHoverTimer();
    setIsSponsorHovered(true);
  }, [clearSponsorHoverTimer]);

  const handleSponsorHoverLeave = useCallback(() => {
    clearSponsorHoverTimer();
    sponsorHoverTimerRef.current = window.setTimeout(() => {
      setIsSponsorHovered(false);
      sponsorHoverTimerRef.current = null;
    }, 120);
  }, [clearSponsorHoverTimer]);

  useEffect(() => {
    if (!isSponsorPinned) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (!sponsorRef.current?.contains(target)) {
        setIsSponsorPinned(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsSponsorPinned(false);
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isSponsorPinned]);

  useEffect(() => {
    return () => {
      clearSponsorHoverTimer();
    };
  }, [clearSponsorHoverTimer]);

  const hasNewVersion = useMemo(() => {
    return !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);
  }, [appVersion, latestReleaseTag]);

  const isSponsorOpen = isSponsorHovered || isSponsorPinned;

  const updateSponsorPopupPosition = useCallback(() => {
    const trigger = sponsorRef.current;
    const popup = sponsorPopupRef.current;
    if (!trigger || !popup) {
      return;
    }

    const gap = 12;
    const viewportPadding = 16;
    const triggerRect = trigger.getBoundingClientRect();
    const popupRect = popup.getBoundingClientRect();
    const width = popupRect.width || 544;
    const height = popupRect.height || 0;

    const horizontalAnchor = triggerRect.left + (triggerRect.width / 2) - (width * 0.38);
    let left = horizontalAnchor;
    left = Math.max(viewportPadding, Math.min(left, window.innerWidth - width - viewportPadding));

    let top = triggerRect.bottom + gap;
    let placement: 'top' | 'bottom' = 'bottom';

    if (top + height > window.innerHeight - viewportPadding && triggerRect.top - gap - height >= viewportPadding) {
      top = triggerRect.top - height - gap;
      placement = 'top';
    }

    setSponsorPopupStyle({ top, left, placement });
  }, []);

  useLayoutEffect(() => {
    if (!isSponsorOpen) {
      setSponsorPopupStyle(null);
      return;
    }

    const handlePositionChange = () => updateSponsorPopupPosition();
    handlePositionChange();

    window.addEventListener('resize', handlePositionChange);
    window.addEventListener('scroll', handlePositionChange, true);

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => handlePositionChange());
      if (sponsorRef.current) {
        resizeObserver.observe(sponsorRef.current);
      }
      if (sponsorPopupRef.current) {
        resizeObserver.observe(sponsorPopupRef.current);
      }
    }

    return () => {
      window.removeEventListener('resize', handlePositionChange);
      window.removeEventListener('scroll', handlePositionChange, true);
      resizeObserver?.disconnect();
    };
  }, [isSponsorOpen, updateSponsorPopupPosition]);

  return (
    <div className="mx-auto max-w-[860px] space-y-4">
      <section className="rounded-[28px] border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border/60 px-5 py-4">
          <Rocket className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">{t('aboutPanel.title', { name: BRAND.name })}</h3>
        </div>

        <div className="grid gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className={`${ABOUT_CARD_CLASS} flex h-full flex-col`}>
            <div className="flex flex-1 flex-col justify-between gap-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
                <img src="/brand/appicon.png" alt={t('aboutPanel.images.logoAlt', { name: BRAND.name })} className="h-20 w-20 shrink-0 object-contain" draggable={false} />

                <div className="min-w-0 flex-1">
                  <div>
                    <img src="/brand/wordmark-light.png" alt={t('aboutPanel.images.wordmarkAlt', { name: BRAND.name })} className="h-auto w-[220px] object-contain dark:hidden" draggable={false} />
                    <img src="/brand/wordmark-dark.png" alt={t('aboutPanel.images.wordmarkAlt', { name: BRAND.name })} className="hidden h-auto w-[220px] object-contain dark:block" draggable={false} />
                  </div>

                  <p className="mt-4 max-w-[36rem] text-sm leading-relaxed text-muted-foreground">
                    {t('aboutPanel.description', { name: BRAND.name })}
                  </p>
                </div>
              </div>

              <div className="rounded-2xl border border-border/70 bg-background/70 p-4">
                <div className="flex flex-wrap gap-2">
                  <span className="relative inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {t('aboutPanel.version.current', { version: appVersion ? `v${appVersion}` : '--' })}
                    {hasNewVersion && (
                      <span
                        className="pointer-events-none absolute -right-1 -top-1 size-2 shrink-0 rounded-full bg-red-500"
                        aria-label="有新版本"
                        title="有新版本"
                      />
                    )}
                  </span>
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {t('aboutPanel.version.latest', { version: releaseLoading ? t('aboutPanel.version.checkingShort') : latestReleaseTag || '--' })}
                  </span>
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {releaseChannel === 'prerelease' ? t('aboutPanel.version.channelPrerelease') : t('aboutPanel.version.channelStable')}
                  </span>
                  {latestReleaseIsPrerelease && <Badge variant="info">{t('aboutPanel.version.prereleaseBadge')}</Badge>}
                </div>

                <div className="mt-3 inline-flex rounded-xl border border-border/70 bg-background/70 p-1">
                  <button
                    type="button"
                    className={`rounded-lg px-3 py-1 text-xs transition ${releaseChannel === 'stable' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                    onClick={() => setReleaseChannel('stable')}
                    disabled={releaseLoading}
                  >
                    {t('aboutPanel.version.channelStable')}
                  </button>
                  <button
                    type="button"
                    className={`rounded-lg px-3 py-1 text-xs transition ${releaseChannel === 'prerelease' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                    onClick={() => setReleaseChannel('prerelease')}
                    disabled={releaseLoading}
                  >
                    {t('aboutPanel.version.channelPrerelease')}
                  </button>
                </div>

                <div className="mt-4 flex flex-wrap gap-2">
                  <Button
                    variant="primary"
                    size="sm"
                    loading={releaseLoading}
                    onClick={() => {
                      void checkLatestRelease(releaseChannel);
                    }}
                    icon={<RefreshCw className="h-3.5 w-3.5" />}
                  >
                    {releaseLoading ? t('aboutPanel.version.checkingButton') : t('aboutPanel.version.checkUpdate')}
                  </Button>
                  {hasNewVersion && installerUrl ? (
                    <div className="inline-flex items-stretch overflow-hidden rounded-lg shadow-sm">
                      <button
                        type="button"
                        disabled={updateStage === 'downloading' || updateStage === 'installing' || updateStage === 'done'}
                        onClick={() => {
                          void startDownloadInstall();
                        }}
                        className="inline-flex cursor-pointer items-center gap-1.5 bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <Download className="h-3.5 w-3.5" />
                        {updateStage === 'downloading'
                          ? t('aboutPanel.version.downloading', { percent: updatePercent })
                          : updateStage === 'installing'
                            ? t('aboutPanel.version.installing')
                            : updateStage === 'done'
                              ? t('aboutPanel.version.installStarted')
                              : t('aboutPanel.version.downloadAndInstall')}
                      </button>
                      <button
                        type="button"
                        onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                        className="inline-flex cursor-pointer items-center gap-1.5 border-l border-primary-foreground/25 bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground transition hover:bg-primary/90"
                      >
                        <Rocket className="h-3.5 w-3.5" />
                        {t('aboutPanel.version.openReleasePage')}
                      </button>
                    </div>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                      icon={<Rocket className="h-3.5 w-3.5" />}
                    >
                      {t('aboutPanel.version.openReleasePage')}
                    </Button>
                  )}
                  <div
                    ref={sponsorRef}
                    className="relative"
                    onPointerEnter={handleSponsorHoverEnter}
                    onPointerLeave={handleSponsorHoverLeave}
                  >
                    <Button
                      variant={isSponsorPinned ? 'secondary' : 'outline'}
                      size="sm"
                      icon={<Heart className="h-3.5 w-3.5" />}
                      aria-expanded={isSponsorOpen}
                      aria-pressed={isSponsorPinned}
                      onClick={() => {
                        clearSponsorHoverTimer();
                        setIsSponsorHovered(true);
                        setIsSponsorPinned((value) => !value);
                      }}
                    >
                      {t('aboutPanel.sponsor.button')}
                    </Button>
                  </div>
                </div>

                {releaseError && <div className="mt-3 text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}

                {hasNewVersion && !installerUrl && (
                  <div className="mt-3 text-xs text-muted-foreground">{t('aboutPanel.version.noInstallerHint')}</div>
                )}
                {hasNewVersion && installerUrl && updateStage === 'idle' && (
                  <div className="mt-2 text-xs text-muted-foreground">{t('aboutPanel.version.autoInstallHint')}</div>
                )}
              </div>
            </div>

          </div>

          <div className={ABOUT_CARD_CLASS}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Rocket className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.contact.title')}</span>
            </div>

            <div className="mt-4 flex items-center gap-3 rounded-2xl border border-border/70 bg-background/70 p-3">
              <img
                src="https://q1.qlogo.cn/g?b=qq&nk=507249007&s=640"
                alt={t('aboutPanel.images.avatarAlt')}
                className="h-14 w-14 rounded-2xl border border-border object-cover"
                referrerPolicy="no-referrer"
              />
              <div className="min-w-0 flex-1">
                <div className="text-base font-semibold text-foreground">Tianli</div>
                <div className="mt-1 text-sm text-muted-foreground">{t('aboutPanel.contact.tagline')}</div>
              </div>
            </div>

            <div className="mt-4 space-y-2">
              <button
                type="button"
                onClick={() => openUrl('mailto:wutianli@tianli0.top')}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.email')}
                </span>
                <span className="text-xs text-muted-foreground">wutianli@tianli0.top</span>
              </button>

              <button
                type="button"
                onClick={() => openUrl('https://qm.qq.com/q/2lEOycrLjq')}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <MessageCircleMore className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.feedbackGroup')}
                </span>
                <span className="text-xs text-muted-foreground">{t('aboutPanel.contact.feedbackGroupEntry')}</span>
              </button>

              <button
                type="button"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <Rocket className="h-4 w-4 text-muted-foreground" />
                  {t('aboutPanel.contact.repository')}
                </span>
                <span className="text-xs text-muted-foreground">{t('aboutPanel.contact.repositoryPlatform')}</span>
              </button>
            </div>
          </div>

          {hasNewVersion && (
            <div className={`${ABOUT_CARD_CLASS} lg:col-span-2`}>
              <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-foreground">
                <Rocket className="h-4 w-4 text-primary" />
                <span>{t('aboutPanel.version.newVersionFound', { version: latestReleaseTag })}</span>
                {latestReleaseIsPrerelease && <Badge variant="info">{t('aboutPanel.version.prereleaseBadge')}</Badge>}
              </div>

              <div className="mt-3 rounded-2xl border border-border/70 bg-background/70 p-3">
                {latestReleaseBody ? (
                  <ScrollArea className="h-56 pr-2">
                    <div className="flex flex-col gap-2 text-xs leading-relaxed text-foreground/90">
                      {latestReleaseBody.split(/\r?\n/).map((line, index) => {
                        const trimmed = line.trim();
                        if (!trimmed) {
                          return <div key={`release-line-${index}`} className="h-1" />;
                        }

                        if (/^#{1,6}\s+/.test(trimmed)) {
                          return (
                            <div key={`release-line-${index}`} className="pt-1 text-sm font-semibold text-foreground">
                              {trimmed.replace(/^#{1,6}\s+/, '')}
                            </div>
                          );
                        }

                        if (/^[-*]\s+/.test(trimmed) || /^\d+\.\s+/.test(trimmed)) {
                          const content = trimmed.replace(/^[-*]\s+/, '').replace(/^\d+\.\s+/, '');
                          return (
                            <div key={`release-line-${index}`} className="flex items-start gap-2 text-foreground/90">
                              <span className="mt-[1px] text-muted-foreground">-</span>
                              <span>{content}</span>
                            </div>
                          );
                        }

                        return (
                          <p key={`release-line-${index}`} className="text-foreground/85">
                            {trimmed}
                          </p>
                        );
                      })}
                    </div>
                  </ScrollArea>
                ) : (
                  <p className="text-xs text-muted-foreground">{t('aboutPanel.version.emptyReleaseNotes')}</p>
                )}
              </div>
            </div>
          )}

          <div className={`${ABOUT_CARD_CLASS} lg:col-span-2`}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Sparkles className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.faq.title')}</span>
            </div>

            <div className="mt-4 divide-y divide-border/60 rounded-2xl border border-border/70 bg-background/70">
              {faqItems.map((item) => (
                <div key={item.question} className="px-4 py-3">
                  <div className="text-sm font-medium text-foreground">{item.question}</div>
                  <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{item.answer}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {isSponsorOpen && typeof document !== 'undefined' && createPortal(
        <div
          ref={sponsorPopupRef}
          onPointerEnter={handleSponsorHoverEnter}
          onPointerLeave={handleSponsorHoverLeave}
          className="fixed z-[80] w-[34rem] max-w-[calc(100vw-2rem)] rounded-3xl border border-border/80 bg-popover/98 p-4 backdrop-blur-xl animate-in fade-in-0 zoom-in-95"
          style={sponsorPopupStyle ? { top: sponsorPopupStyle.top, left: sponsorPopupStyle.left } : { top: 0, left: 0, visibility: 'hidden' }}
        >
          <div className="mb-3 flex items-center justify-between px-1">
            <div className="text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">{t('aboutPanel.sponsor.title')}</div>
            {isSponsorPinned && <Badge variant="info">{t('aboutPanel.sponsor.pinned')}</Badge>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            {supportMethods.map((item) => (
              <div key={item.label} className="rounded-2xl border border-border/70 bg-background/80 p-3">
                <img
                  src={item.image}
                  alt={t('aboutPanel.images.supportQrAlt', { label: item.label })}
                  className="aspect-square w-full rounded-xl object-contain"
                  draggable={false}
                />
                <div className="mt-3 text-center text-sm font-medium text-foreground">{item.label}</div>
              </div>
            ))}
          </div>
        </div>,
        document.body,
      )}

      {updateStage !== 'idle' && typeof document !== 'undefined' && createPortal(
        <div className="fixed bottom-6 right-6 z-[90] w-[22rem] max-w-[calc(100vw-2rem)] rounded-2xl border border-border/80 bg-popover/98 p-4 shadow-xl backdrop-blur-xl animate-in fade-in-0 slide-in-from-bottom-2">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              {updateStage === 'error' ? <Rocket className="h-4 w-4 text-amber-500" /> : <Download className="h-4 w-4" />}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <div className="text-sm font-semibold text-foreground">
                  {updateStage === 'downloading'
                    ? t('aboutPanel.version.floatDownloadingTitle')
                    : updateStage === 'installing'
                      ? t('aboutPanel.version.floatInstallingTitle')
                      : updateStage === 'done'
                        ? t('aboutPanel.version.floatDoneTitle')
                        : t('aboutPanel.version.floatErrorTitle')}
                </div>
                {(updateStage === 'error' || updateStage === 'done') && (
                  <button
                    type="button"
                    onClick={() => {
                      setUpdateStage('idle');
                      setUpdateError('');
                    }}
                    className="shrink-0 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition hover:bg-muted hover:text-foreground"
                    aria-label={t('common.actions.close')}
                  >
                    ✕
                  </button>
                )}
              </div>

              {updateStage === 'error' ? (
                <p className="mt-1 text-xs leading-relaxed text-amber-600 dark:text-amber-300">
                  {updateError ? t('aboutPanel.version.installFailed', { error: updateError }) : t('aboutPanel.version.floatErrorTitle')}
                </p>
              ) : updateStage === 'done' ? (
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  {t('aboutPanel.version.floatDoneHint')}
                </p>
              ) : (
                <>
                  <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-border/60">
                    <div
                      className={`h-full rounded-full bg-primary transition-[width] duration-200 ${updateStage !== 'downloading' ? 'animate-pulse' : ''}`}
                      style={{ width: `${updateStage === 'downloading' ? updatePercent : 100}%` }}
                    />
                  </div>
                  <div className="mt-1.5 flex items-center justify-between text-xs text-muted-foreground">
                    <span>
                      {updateStage === 'downloading'
                        ? t('aboutPanel.version.downloading', { percent: updatePercent })
                        : updateStage === 'installing'
                          ? t('aboutPanel.version.installingHint')
                          : t('aboutPanel.version.installStarted')}
                    </span>
                    {updateStage === 'downloading' && <span className="tabular-nums">{updatePercent}%</span>}
                  </div>
                </>
              )}

              {updateStage === 'error' && (
                <div className="mt-3 flex flex-wrap gap-2">
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => {
                      void startDownloadInstall();
                    }}
                    icon={<Download className="h-3.5 w-3.5" />}
                  >
                    {t('aboutPanel.version.floatRetry')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                    icon={<Rocket className="h-3.5 w-3.5" />}
                  >
                    {t('aboutPanel.version.openReleasePage')}
                  </Button>
                </div>
              )}
            </div>
          </div>
        </div>,
        document.body,
      )}
    </div>
  );
}
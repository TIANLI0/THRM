'use client';

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import {
  ChevronDown,
  Code2,
  Download,
  ExternalLink,
  Heart,
  Mail,
  MessageCircleMore,
  Monitor,
  RefreshCw,
  Rocket,
  ShieldCheck,
  Sparkles,
  Users,
  Wifi,
} from 'lucide-react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { BRAND } from '../lib/brand';
import { apiService } from '../services/api';
import { SiGithub } from 'react-icons/si';
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
    (asset) =>
      typeof asset?.name === 'string' &&
      /installer\.exe$/i.test(asset.name) &&
      !!asset.browser_download_url,
  );
  return fuzzy?.browser_download_url || '';
}

type UpdateStage = 'idle' | 'downloading' | 'installing' | 'done' | 'error';

type CreditContributor = {
  login?: string;
  name?: string;
  url?: string;
  avatar?: string;
  role?: string;
};

type CreditSponsor = {
  name?: string;
  url?: string;
  avatar?: string;
  note?: string;
  amount?: number;
};

type CreditsData = {
  contributors: CreditContributor[];
  sponsors: CreditSponsor[];
};

function openUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

function isLatestVersion(currentVersion: string, latestVersion: string) {
  const normalize = (value: string) =>
    value.trim().replace(/^v/i, '').toLowerCase();
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

const PANEL_CLASS =
  'min-w-0 rounded-[26px] border border-border/70 bg-card/90 shadow-sm shadow-black/[0.025]';
const LINK_ROW_CLASS =
  'group flex w-full cursor-pointer items-center gap-3 rounded-2xl px-3 py-3 text-left transition-colors hover:bg-muted/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40';

type VersionValueProps = {
  value: string;
  canCopy: boolean;
  onCopy: () => void;
  copyLabel: string;
};

/**
 * 版本号展示：单行、无滚动条；当文字超出容器宽度时以“来回播放”的方式
 * 自动滚动展示完整内容，点击可复制。
 */
function VersionValue({ value, canCopy, onCopy, copyLabel }: VersionValueProps) {
  const boxRef = useRef<HTMLSpanElement | null>(null);
  const textRef = useRef<HTMLSpanElement | null>(null);

  useEffect(() => {
    const box = boxRef.current;
    const text = textRef.current;
    if (!box || !text) return;

    let animation: Animation | null = null;

    const setup = () => {
      animation?.cancel();
      animation = null;
      const overflow = text.scrollWidth - box.clientWidth;
      if (overflow <= 1) {
        text.style.transform = 'translateX(0)';
        return;
      }
      // 停顿→滚到末尾→停顿→滚回起点，循环播放。速度约 40px/秒。
      const travel = Math.round((overflow / 40) * 1000);
      animation = text.animate(
        [
          { transform: 'translateX(0)' },
          { transform: 'translateX(0)', offset: 0.15 },
          { transform: `translateX(-${overflow}px)`, offset: 0.5 },
          { transform: `translateX(-${overflow}px)`, offset: 0.65 },
          { transform: 'translateX(0)', offset: 1 },
        ],
        { duration: travel * 2 + 2400, iterations: Infinity, easing: 'linear' },
      );
    };

    setup();
    const observer = new ResizeObserver(setup);
    observer.observe(box);
    observer.observe(text);

    return () => {
      animation?.cancel();
      observer.disconnect();
    };
  }, [value]);

  return (
    <button
      type="button"
      disabled={!canCopy}
      onClick={onCopy}
      title={canCopy ? copyLabel : undefined}
      className="mt-2 block w-full overflow-hidden text-left text-lg font-semibold leading-tight text-foreground transition-colors enabled:cursor-pointer enabled:hover:text-primary disabled:cursor-default"
    >
      <span ref={boxRef} className="block overflow-hidden">
        <span
          ref={textRef}
          className="inline-block whitespace-nowrap tabular-nums will-change-transform"
        >
          {value}
        </span>
      </span>
    </button>
  );
}

export default function AboutPanel() {
  const { t } = useTranslation();
  const [appVersion, setAppVersion] = useState('');
  const [releaseChannel, setReleaseChannel] =
    useState<ReleaseChannel>('stable');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState<string>(
    BRAND.latestReleaseUrl,
  );
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [, setLatestReleaseIsPrerelease] = useState(false);
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const [installerUrl, setInstallerUrl] = useState('');
  const [updateStage, setUpdateStage] = useState<UpdateStage>('idle');
  const [updatePercent, setUpdatePercent] = useState(0);
  const [updateError, setUpdateError] = useState('');
  const [credits, setCredits] = useState<CreditsData | null>(null);
  const [creditsLoading, setCreditsLoading] = useState(false);
  const [creditsError, setCreditsError] = useState(false);
  const [isSponsorHovered, setIsSponsorHovered] = useState(false);
  const [isSponsorPinned, setIsSponsorPinned] = useState(false);
  const [sponsorPopupStyle, setSponsorPopupStyle] = useState<{
    top: number;
    left: number;
    placement: 'top' | 'bottom';
  } | null>(null);
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
    () =>
      t('aboutPanel.faq.items', { returnObjects: true }) as Array<{
        question: string;
        answer: string;
      }>,
    [t],
  );

  const checkLatestRelease = useCallback(
    async (channel: ReleaseChannel = releaseChannel) => {
      setReleaseLoading(true);
      setReleaseError('');
      setInstallerUrl('');

      const headers = { Accept: 'application/vnd.github+json' };

      try {
        let targetRelease: GithubRelease | null = null;

        if (channel === 'prerelease') {
          const response = await fetch(`${BRAND.releasesApiUrl}?per_page=30`, {
            headers,
          });
          if (!response.ok) throw new Error(`HTTP ${response.status}`);

          const releases = (await response.json()) as GithubRelease[];
          targetRelease =
            (Array.isArray(releases) ? releases : []).find(
              (item) => !item?.draft && !!item?.prerelease,
            ) || null;

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
        setLatestReleaseBody(
          typeof targetRelease?.body === 'string'
            ? targetRelease.body.trim()
            : '',
        );
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
    },
    [releaseChannel, t],
  );

  useEffect(() => {
    let disposed = false;
    apiService
      .getAppVersion()
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
    let disposed = false;
    setCreditsLoading(true);
    setCreditsError(false);
    fetch(BRAND.creditsUrl, { cache: 'no-cache' })
      .then((response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return response.json();
      })
      .then((data: Partial<CreditsData>) => {
        if (disposed) return;
        setCredits({
          contributors: Array.isArray(data?.contributors)
            ? data.contributors
            : [],
          sponsors: Array.isArray(data?.sponsors) ? data.sponsors : [],
        });
      })
      .catch(() => {
        if (!disposed) setCreditsError(true);
      })
      .finally(() => {
        if (!disposed) setCreditsLoading(false);
      });
    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    const dispose = apiService.onUpdateDownloadProgress((payload) => {
      const stage = payload?.stage;
      if (stage === 'downloading') {
        setUpdateStage('downloading');
        setUpdatePercent(
          typeof payload.percent === 'number' && payload.percent >= 0
            ? payload.percent
            : 0,
        );
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
    return (
      !!appVersion &&
      !!latestReleaseTag &&
      !isLatestVersion(appVersion, latestReleaseTag)
    );
  }, [appVersion, latestReleaseTag]);

  const sortedSponsors = useMemo(() => {
    const list = credits?.sponsors ?? [];
    return [...list].sort((a, b) => (b.amount ?? 0) - (a.amount ?? 0));
  }, [credits]);

  const contributors = credits?.contributors ?? [];

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

    const horizontalAnchor =
      triggerRect.left + triggerRect.width / 2 - width * 0.38;
    let left = horizontalAnchor;
    left = Math.max(
      viewportPadding,
      Math.min(left, window.innerWidth - width - viewportPadding),
    );

    let top = triggerRect.bottom + gap;
    let placement: 'top' | 'bottom' = 'bottom';

    if (
      top + height > window.innerHeight - viewportPadding &&
      triggerRect.top - gap - height >= viewportPadding
    ) {
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

  const issuesUrl = `${BRAND.repositoryUrl.replace(/\/$/, '')}/issues`;

  const copyVersion = useCallback(
    (version: string) => {
      const value = version.trim();
      if (!value || value === '--') return;
      void navigator.clipboard?.writeText(value).then(
        () => toast.success(t('aboutPanel.version.copied', { version: value })),
        () => toast.error(t('aboutPanel.version.copyFailed')),
      );
    },
    [t],
  );

  return (
    <div className="mx-auto w-full max-w-270 space-y-5 pb-8">
      <section className="relative overflow-hidden rounded-[32px] border border-border/70 bg-card shadow-sm shadow-black/3">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-20 -top-28 h-72 w-72 rounded-full bg-primary/[0.07] blur-3xl"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -bottom-32 left-[28%] h-64 w-64 rounded-full bg-primary/4 blur-3xl"
        />

        <div className="relative grid lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="flex min-w-0 flex-col px-6 pt-7 pb-5 sm:px-8 sm:pt-8 sm:pb-6 lg:px-10 lg:pt-8 lg:pb-6">
            <h1 className="sr-only">
              {t('aboutPanel.title', { name: BRAND.name })}
            </h1>

            <div className="flex flex-col gap-6 sm:flex-row sm:items-start">
              <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-[24px] border border-border/60 bg-background/75 p-2.5 shadow-sm backdrop-blur">
                <img
                  src="/brand/appicon.png"
                  alt={t('aboutPanel.images.logoAlt', { name: BRAND.name })}
                  className="h-full w-full object-contain"
                  draggable={false}
                />
              </div>

              <div className="min-w-0 flex-1 pt-0.5">
                <img
                  src="/brand/wordmark-light.png"
                  alt={t('aboutPanel.images.wordmarkAlt', { name: BRAND.name })}
                  className="h-auto w-57.5 max-w-full object-contain dark:hidden sm:w-65"
                  draggable={false}
                />
                <img
                  src="/brand/wordmark-dark.png"
                  alt={t('aboutPanel.images.wordmarkAlt', { name: BRAND.name })}
                  className="hidden h-auto w-57.5 max-w-full object-contain dark:block sm:w-65"
                  draggable={false}
                />

                <p className="mt-5 max-w-172 text-[15px] leading-7 text-muted-foreground">
                  {t('aboutPanel.description', { name: BRAND.name })}
                </p>
              </div>
            </div>

            <div className="mt-7 flex flex-wrap gap-2.5">
              <Button
                variant="primary"
                size="sm"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                icon={<SiGithub className="h-4 w-4" />}
              >
                {t('aboutPanel.contact.repository')}
              </Button>

              <Button
                variant="outline"
                size="sm"
                onClick={() => openUrl(issuesUrl)}
                icon={<MessageCircleMore className="h-4 w-4" />}
              >
                GitHub Issues
              </Button>

              <div
                ref={sponsorRef}
                className="relative"
                onPointerEnter={handleSponsorHoverEnter}
                onPointerLeave={handleSponsorHoverLeave}
              >
                <Button
                  variant={isSponsorPinned ? 'secondary' : 'outline'}
                  size="sm"
                  icon={<Heart className="h-4 w-4" />}
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

            <div className="mt-auto grid gap-0 pt-6 sm:grid-cols-3">
              <div className="flex items-center gap-3 border-t border-border/60 py-3 sm:border-r sm:pr-5">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <ShieldCheck className="h-4 w-4" />
                </div>
                <div className="text-sm font-semibold text-foreground">
                  MIT License
                </div>
              </div>

              <div className="flex items-center gap-3 border-t border-border/60 py-3 sm:border-r sm:px-5">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Monitor className="h-4 w-4" />
                </div>
                <div className="text-sm font-semibold text-foreground">
                  Windows / Linux
                </div>
              </div>

              <div className="flex items-center gap-3 border-t border-border/60 py-3 sm:pl-5">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Wifi className="h-4 w-4" />
                </div>
                <div className="text-sm font-semibold text-foreground">
                  BS1 — BS3 Pro
                </div>
              </div>
            </div>
          </div>

          <aside className="border-t border-border/60 bg-background/45 p-5 backdrop-blur-sm sm:p-6 lg:border-l lg:border-t-0 lg:p-7">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <RefreshCw className="h-4 w-4 text-primary" />
                <span>{t('aboutPanel.version.checkUpdate')}</span>
              </div>

              {hasNewVersion && (
                <Badge variant="info">
                  {t('aboutPanel.version.updatable')}
                </Badge>
              )}
            </div>

            <div className="mt-6 grid grid-cols-2 gap-3">
              <div className="rounded-2xl border border-border/60 bg-card/80 p-4">
                <div className="text-[11px] font-medium uppercase tracking-[0.13em] text-muted-foreground">
                  {t('aboutPanel.version.current', { version: '' }).trim()}
                </div>
                {(() => {
                  const value = appVersion ? `v${appVersion}` : '--';
                  const canCopy = value !== '--';
                  return (
                    <VersionValue
                      value={value}
                      canCopy={canCopy}
                      onCopy={() => copyVersion(value)}
                      copyLabel={t('aboutPanel.version.copyTooltip')}
                    />
                  );
                })()}
              </div>

              <div className="relative rounded-2xl border border-border/60 bg-card/80 p-4">
                {hasNewVersion && (
                  <span className="absolute right-3 top-3 h-2 w-2 rounded-full bg-red-500" />
                )}
                <div className="text-[11px] font-medium uppercase tracking-[0.13em] text-muted-foreground">
                  {t('aboutPanel.version.latest', { version: '' }).trim()}
                </div>
                {(() => {
                  const value = releaseLoading
                    ? t('aboutPanel.version.checkingShort')
                    : latestReleaseTag || '--';
                  const canCopy = !releaseLoading && !!latestReleaseTag;
                  return (
                    <VersionValue
                      value={value}
                      canCopy={canCopy}
                      onCopy={() => copyVersion(value)}
                      copyLabel={t('aboutPanel.version.copyTooltip')}
                    />
                  );
                })()}
              </div>
            </div>

            <div className="mt-4 grid grid-cols-2 rounded-xl border border-border/60 bg-card/70 p-1">
              <button
                type="button"
                aria-pressed={releaseChannel === 'stable'}
                className={`cursor-pointer rounded-lg px-3 py-2 text-xs font-medium transition ${
                  releaseChannel === 'stable'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                }`}
                onClick={() => setReleaseChannel('stable')}
                disabled={releaseLoading}
              >
                {t('aboutPanel.version.channelStable')}
              </button>
              <button
                type="button"
                aria-pressed={releaseChannel === 'prerelease'}
                className={`cursor-pointer rounded-lg px-3 py-2 text-xs font-medium transition ${
                  releaseChannel === 'prerelease'
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                }`}
                onClick={() => setReleaseChannel('prerelease')}
                disabled={releaseLoading}
              >
                {t('aboutPanel.version.channelPrerelease')}
              </button>
            </div>

            {releaseError && (
              <div className="mt-4 rounded-xl border border-amber-500/25 bg-amber-500/6 px-3 py-2.5 text-xs leading-relaxed text-amber-700 dark:text-amber-300">
                {releaseError}
              </div>
            )}

            {hasNewVersion && !installerUrl && (
              <p className="mt-4 text-xs leading-relaxed text-muted-foreground">
                {t('aboutPanel.version.noInstallerHint')}
              </p>
            )}

            <div className="mt-5 flex flex-col gap-2.5">
              {hasNewVersion && installerUrl ? (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    void startDownloadInstall();
                  }}
                  loading={
                    updateStage === 'downloading' ||
                    updateStage === 'installing'
                  }
                  icon={<Download className="h-4 w-4" />}
                >
                  {updateStage === 'downloading'
                    ? t('aboutPanel.version.downloading', {
                        percent: updatePercent,
                      })
                    : updateStage === 'installing'
                      ? t('aboutPanel.version.installing')
                      : updateStage === 'done'
                        ? t('aboutPanel.version.installStarted')
                        : t('aboutPanel.version.downloadAndInstall')}
                </Button>
              ) : (
                <Button
                  variant="primary"
                  size="sm"
                  loading={releaseLoading}
                  onClick={() => {
                    void checkLatestRelease(releaseChannel);
                  }}
                  icon={<RefreshCw className="h-4 w-4" />}
                >
                  {releaseLoading
                    ? t('aboutPanel.version.checkingButton')
                    : t('aboutPanel.version.checkUpdate')}
                </Button>
              )}

              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)
                }
                icon={<ExternalLink className="h-4 w-4" />}
              >
                {t('aboutPanel.version.openReleasePage')}
              </Button>
            </div>
          </aside>
        </div>
      </section>

      {hasNewVersion && (
        <section className={`${PANEL_CLASS} overflow-hidden`}>
          <div className="flex flex-col gap-3 border-b border-border/60 px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Rocket className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold text-foreground">
                  {t('aboutPanel.version.newVersionFound', {
                    version: latestReleaseTag,
                  })}
                </div>
                <div className="mt-0.5 text-xs text-muted-foreground">
                  {releaseChannel === 'prerelease'
                    ? t('aboutPanel.version.channelPrerelease')
                    : t('aboutPanel.version.channelStable')}
                </div>
              </div>
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)
              }
              icon={<ExternalLink className="h-3.5 w-3.5" />}
            >
              {t('aboutPanel.version.openReleasePage')}
            </Button>
          </div>

          <div className="p-5 sm:p-6">
            {latestReleaseBody ? (
              <ScrollArea className="h-64 pr-3">
                <div className="flex flex-col gap-2.5 text-sm leading-6 text-foreground/85">
                  {latestReleaseBody.split(/\r?\n/).map((line, index) => {
                    const trimmed = line.trim();
                    if (!trimmed) {
                      return (
                        <div key={`release-line-${index}`} className="h-1" />
                      );
                    }

                    if (/^#{1,6}\s+/.test(trimmed)) {
                      return (
                        <div
                          key={`release-line-${index}`}
                          className="pt-2 text-[15px] font-semibold text-foreground first:pt-0"
                        >
                          {trimmed.replace(/^#{1,6}\s+/, '')}
                        </div>
                      );
                    }

                    if (/^[-*]\s+/.test(trimmed) || /^\d+\.\s+/.test(trimmed)) {
                      const content = trimmed
                        .replace(/^[-*]\s+/, '')
                        .replace(/^\d+\.\s+/, '');

                      return (
                        <div
                          key={`release-line-${index}`}
                          className="grid grid-cols-[12px_minmax(0,1fr)] items-start gap-2"
                        >
                          <span className="mt-px text-primary">•</span>
                          <span>{content}</span>
                        </div>
                      );
                    }

                    return <p key={`release-line-${index}`}>{trimmed}</p>;
                  })}
                </div>
              </ScrollArea>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t('aboutPanel.version.emptyReleaseNotes')}
              </p>
            )}
          </div>
        </section>
      )}

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
        <main className="min-w-0 space-y-5">
          <section className={`${PANEL_CLASS} p-5 sm:p-6`}>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <div className="flex items-center gap-2 text-base font-semibold text-foreground">
                  <Users className="h-4 w-4 text-primary" />
                  <span>{t('aboutPanel.credits.title')}</span>
                </div>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
                  {t('aboutPanel.credits.description')}
                </p>
              </div>

              {!creditsLoading && credits && (
                <div className="flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
                  <span>
                    {contributors.length}{' '}
                    {t('aboutPanel.credits.contributorsTitle')}
                  </span>
                  <span className="h-3 w-px bg-border" />
                  <span>
                    {sortedSponsors.length}{' '}
                    {t('aboutPanel.credits.sponsorsTitle')}
                  </span>
                </div>
              )}
            </div>

            {creditsLoading && !credits ? (
              <div className="mt-6 flex items-center justify-center gap-2 rounded-2xl bg-muted/45 px-4 py-12 text-sm text-muted-foreground">
                <RefreshCw className="h-4 w-4 animate-spin" />
                {t('aboutPanel.credits.loading')}
              </div>
            ) : creditsError && !credits ? (
              <div className="mt-6 rounded-2xl border border-amber-500/25 bg-amber-500/6 px-4 py-10 text-center text-sm text-amber-700 dark:text-amber-300">
                {t('aboutPanel.credits.error')}
              </div>
            ) : (
              <div className="mt-6 space-y-7">
                <div>
                  <div className="flex items-center justify-between border-b border-border/60 pb-3">
                    <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.13em] text-muted-foreground">
                      <Sparkles className="h-3.5 w-3.5 text-primary" />
                      {t('aboutPanel.credits.contributorsTitle')}
                    </div>
                    {contributors.length > 0 && (
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {contributors.length}
                      </span>
                    )}
                  </div>

                  {contributors.length > 0 ? (
                    <div className="mt-4 grid gap-2 sm:grid-cols-2">
                      {contributors.map((person, index) => {
                        const displayName =
                          person.name ||
                          person.login ||
                          t('aboutPanel.credits.anonymous');
                        const key =
                          person.login ||
                          person.url ||
                          `${displayName}-${index}`;

                        const content = (
                          <>
                            {person.avatar ? (
                              <img
                                src={person.avatar}
                                alt={displayName}
                                className="h-9 w-9 shrink-0 rounded-xl border border-border/70 object-cover"
                                referrerPolicy="no-referrer"
                                draggable={false}
                              />
                            ) : (
                              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-border/70 bg-muted text-xs font-semibold uppercase text-muted-foreground">
                                {displayName.slice(0, 1)}
                              </span>
                            )}

                            <span className="min-w-0 flex-1">
                              <span className="flex items-center gap-2">
                                <span className="truncate text-sm font-medium text-foreground">
                                  {displayName}
                                </span>
                                {person.role === 'author' && (
                                  <Badge variant="info">
                                    {t('aboutPanel.credits.authorBadge')}
                                  </Badge>
                                )}
                              </span>
                              {person.login && person.name && (
                                <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                                  @{person.login}
                                </span>
                              )}
                            </span>

                            {person.url && (
                              <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
                            )}
                          </>
                        );

                        return person.url ? (
                          <button
                            key={key}
                            type="button"
                            onClick={() => openUrl(person.url as string)}
                            className="group flex min-w-0 cursor-pointer items-center gap-3 rounded-2xl border border-border/60 bg-background/55 p-3 text-left transition hover:border-primary/25 hover:bg-primary/[0.035]"
                          >
                            {content}
                          </button>
                        ) : (
                          <div
                            key={key}
                            className="flex min-w-0 items-center gap-3 rounded-2xl border border-border/60 bg-background/55 p-3"
                          >
                            {content}
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="mt-4 text-sm text-muted-foreground">
                      {t('aboutPanel.credits.contributorsEmpty')}
                    </div>
                  )}
                </div>

                <div>
                  <div className="flex items-center justify-between border-b border-border/60 pb-3">
                    <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.13em] text-muted-foreground">
                      <Heart className="h-3.5 w-3.5 text-rose-500" />
                      {t('aboutPanel.credits.sponsorsTitle')}
                    </div>
                    {sortedSponsors.length > 0 && (
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {sortedSponsors.length}
                      </span>
                    )}
                  </div>

                  {sortedSponsors.length > 0 ? (
                    <div className="mt-4 flex flex-wrap gap-2">
                      {sortedSponsors.map((sponsor, index) => {
                        const displayName =
                          sponsor.name || t('aboutPanel.credits.anonymous');
                        const key = sponsor.url || `${displayName}-${index}`;
                        const title = sponsor.note
                          ? `${displayName}：${sponsor.note}`
                          : displayName;

                        const content = (
                          <>
                            {sponsor.avatar ? (
                              <img
                                src={sponsor.avatar}
                                alt={displayName}
                                className="h-6 w-6 shrink-0 rounded-full border border-border/70 object-cover"
                                referrerPolicy="no-referrer"
                                draggable={false}
                              />
                            ) : (
                              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-rose-500/20 bg-rose-500/10 text-rose-500">
                                <Heart className="h-3 w-3" />
                              </span>
                            )}

                            <span className="max-w-40 truncate text-xs font-medium text-foreground">
                              {displayName}
                            </span>

                            {typeof sponsor.amount === 'number' &&
                              sponsor.amount > 0 && (
                                <span className="rounded-full bg-rose-500/10 px-1.5 py-0.5 text-[10px] font-semibold tabular-nums text-rose-500">
                                  ¥{sponsor.amount.toFixed(2)}
                                </span>
                              )}

                            {sponsor.url && (
                              <ExternalLink className="h-3 w-3 shrink-0 text-muted-foreground transition-colors group-hover:text-rose-500" />
                            )}
                          </>
                        );

                        const chipBase =
                          'group inline-flex max-w-full items-center gap-1.5 rounded-full border border-border/60 bg-background/55 py-1 pl-1 pr-2.5 text-left transition';

                        return sponsor.url ? (
                          <button
                            key={key}
                            type="button"
                            title={title}
                            onClick={() => openUrl(sponsor.url as string)}
                            className={`${chipBase} cursor-pointer hover:border-rose-400/30 hover:bg-rose-500/6`}
                          >
                            {content}
                          </button>
                        ) : (
                          <div key={key} title={title} className={chipBase}>
                            {content}
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="mt-4 text-sm text-muted-foreground">
                      {t('aboutPanel.credits.sponsorsEmpty')}
                    </div>
                  )}

                  <p className="mt-4 text-center text-xs leading-5 text-muted-foreground">
                    {t('aboutPanel.credits.thanks')}
                  </p>
                </div>
              </div>
            )}
          </section>

          <section className={`${PANEL_CLASS} p-5 sm:p-6`}>
            <div className="flex items-center gap-2 text-base font-semibold text-foreground">
              <Sparkles className="h-4 w-4 text-primary" />
              <span>{t('aboutPanel.faq.title')}</span>
            </div>

            <div className="mt-5 divide-y divide-border/60 border-y border-border/60">
              {faqItems.map((item, index) => (
                <details
                  key={item.question}
                  className="group"
                  open={index === 0}
                >
                  <summary className="flex cursor-pointer list-none items-center gap-4 py-4 text-left [&::-webkit-details-marker]:hidden">
                    <span className="min-w-0 flex-1 text-sm font-medium leading-6 text-foreground">
                      {item.question}
                    </span>
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-muted/70 text-muted-foreground transition group-open:rotate-180 group-open:text-foreground">
                      <ChevronDown className="h-4 w-4" />
                    </span>
                  </summary>
                  <p className="-mt-1 max-w-3xl pb-5 pr-10 text-sm leading-6 text-muted-foreground">
                    {item.answer}
                  </p>
                </details>
              ))}
            </div>
          </section>
        </main>

        <aside className="min-w-0 space-y-5 lg:sticky lg:top-5">
          <section className={`${PANEL_CLASS} overflow-hidden`}>
            <div className="border-b border-border/60 p-5">
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <Rocket className="h-4 w-4 text-primary" />
                <span>{t('aboutPanel.contact.title')}</span>
              </div>

              <div className="mt-5 flex items-center gap-3">
                <img
                  src="https://q1.qlogo.cn/g?b=qq&nk=507249007&s=640"
                  alt={t('aboutPanel.images.avatarAlt')}
                  className="h-14 w-14 shrink-0 rounded-2xl border border-border/70 object-cover shadow-sm"
                  referrerPolicy="no-referrer"
                  draggable={false}
                />
                <div className="min-w-0">
                  <div className="text-base font-semibold text-foreground">
                    Tianli
                  </div>
                  <div className="mt-1 truncate text-xs text-muted-foreground">
                    @TIANLI0 · THRM
                  </div>
                </div>
              </div>
            </div>

            <div className="p-2">
              <button
                type="button"
                onClick={() => openUrl('mailto:wutianli@tianli0.top')}
                className={LINK_ROW_CLASS}
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground transition-colors group-hover:text-primary">
                  <Mail className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-foreground">
                    {t('aboutPanel.contact.email')}
                  </span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    wutianli@tianli0.top
                  </span>
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
              </button>

              <button
                type="button"
                onClick={() => openUrl('https://qm.qq.com/q/2lEOycrLjq')}
                className={LINK_ROW_CLASS}
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground transition-colors group-hover:text-primary">
                  <MessageCircleMore className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-foreground">
                    {t('aboutPanel.contact.feedbackGroup')}
                  </span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    {t('aboutPanel.contact.feedbackGroupEntry')}
                  </span>
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
              </button>

              <button
                type="button"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                className={LINK_ROW_CLASS}
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground transition-colors group-hover:text-primary">
                  <SiGithub className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-foreground">
                    {t('aboutPanel.contact.repository')}
                  </span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    github.com/TIANLI0/THRM
                  </span>
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
              </button>

              <button
                type="button"
                onClick={() => openUrl(issuesUrl)}
                className={LINK_ROW_CLASS}
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-muted/70 text-muted-foreground transition-colors group-hover:text-primary">
                  <MessageCircleMore className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-foreground">
                    GitHub Issues
                  </span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    github.com/TIANLI0/THRM/issues
                  </span>
                </span>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
              </button>
            </div>
          </section>

          <section className={`${PANEL_CLASS} p-5`}>
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <Code2 className="h-4 w-4 text-primary" />
              <span>{BRAND.name}</span>
            </div>

            <div className="mt-4 divide-y divide-border/60">
              <div className="flex items-center gap-3 py-3 first:pt-0">
                <ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="text-xs font-medium text-foreground">
                  MIT License
                </span>
              </div>
              <div className="flex items-center gap-3 py-3">
                <Monitor className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="text-xs font-medium text-foreground">
                  Windows · Linux
                </span>
              </div>
              <div className="flex items-start gap-3 py-3">
                <Wifi className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="text-xs font-medium leading-5 text-foreground">
                  BS1 / BS2 / BS2 Pro / BS3 / BS3 Pro
                </span>
              </div>
              <div className="flex items-center gap-3 py-3 last:pb-0">
                <Code2 className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="text-xs font-medium text-foreground">
                  中文 · English · 日本語
                </span>
              </div>
            </div>
          </section>
        </aside>
      </div>

      {isSponsorOpen &&
        typeof document !== 'undefined' &&
        createPortal(
          <div
            ref={sponsorPopupRef}
            onPointerEnter={handleSponsorHoverEnter}
            onPointerLeave={handleSponsorHoverLeave}
            className="fixed z-80 max-h-[calc(100vh-2rem)] w-136 max-w-[calc(100vw-2rem)] overflow-y-auto rounded-[26px] border border-border/80 bg-popover/98 p-4 shadow-2xl shadow-black/15 backdrop-blur-xl animate-in fade-in-0 zoom-in-95"
            style={
              sponsorPopupStyle
                ? { top: sponsorPopupStyle.top, left: sponsorPopupStyle.left }
                : { top: 0, left: 0, visibility: 'hidden' }
            }
          >
            <div className="mb-4 flex items-center justify-between gap-3 px-1">
              <div>
                <div className="text-sm font-semibold text-foreground">
                  {t('aboutPanel.sponsor.title')}
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {t('aboutPanel.credits.thanks')}
                </div>
              </div>
              {isSponsorPinned && (
                <Badge variant="info">{t('aboutPanel.sponsor.pinned')}</Badge>
              )}
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              {supportMethods.map((item) => (
                <div
                  key={item.label}
                  className="rounded-2xl border border-border/70 bg-background/75 p-3"
                >
                  <img
                    src={item.image}
                    alt={t('aboutPanel.images.supportQrAlt', {
                      label: item.label,
                    })}
                    className="aspect-square w-full rounded-xl bg-white object-contain"
                    draggable={false}
                  />
                  <div className="mt-3 text-center text-sm font-medium text-foreground">
                    {item.label}
                  </div>
                </div>
              ))}
            </div>
          </div>,
          document.body,
        )}

      {updateStage !== 'idle' &&
        typeof document !== 'undefined' &&
        createPortal(
          <div className="fixed bottom-4 left-4 right-4 z-90 rounded-2xl border border-border/80 bg-popover/98 p-4 shadow-xl shadow-black/10 backdrop-blur-xl animate-in fade-in-0 slide-in-from-bottom-2 sm:bottom-6 sm:left-auto sm:right-6 sm:w-88">
            <div className="flex items-start gap-3">
              <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                {updateStage === 'error' ? (
                  <Rocket className="h-4 w-4 text-amber-500" />
                ) : (
                  <Download className="h-4 w-4" />
                )}
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
                      className="shrink-0 cursor-pointer rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition hover:bg-muted hover:text-foreground"
                      aria-label={t('common.actions.close')}
                    >
                      ✕
                    </button>
                  )}
                </div>

                {updateStage === 'error' ? (
                  <p className="mt-1 text-xs leading-relaxed text-amber-700 dark:text-amber-300">
                    {updateError
                      ? t('aboutPanel.version.installFailed', {
                          error: updateError,
                        })
                      : t('aboutPanel.version.floatErrorTitle')}
                  </p>
                ) : updateStage === 'done' ? (
                  <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                    {t('aboutPanel.version.floatDoneHint')}
                  </p>
                ) : (
                  <>
                    <div className="mt-2.5 h-1.5 w-full overflow-hidden rounded-full bg-border/60">
                      <div
                        className={`h-full rounded-full bg-primary transition-[width] duration-200 ${
                          updateStage !== 'downloading' ? 'animate-pulse' : ''
                        }`}
                        style={{
                          width: `${
                            updateStage === 'downloading' ? updatePercent : 100
                          }%`,
                        }}
                      />
                    </div>
                    <div className="mt-1.5 flex items-center justify-between gap-3 text-xs text-muted-foreground">
                      <span className="min-w-0">
                        {updateStage === 'downloading'
                          ? t('aboutPanel.version.downloading', {
                              percent: updatePercent,
                            })
                          : updateStage === 'installing'
                            ? t('aboutPanel.version.installingHint')
                            : t('aboutPanel.version.installStarted')}
                      </span>
                      {updateStage === 'downloading' && (
                        <span className="shrink-0 tabular-nums">
                          {updatePercent}%
                        </span>
                      )}
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
                      onClick={() =>
                        openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)
                      }
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
        )}
    </div>
  );
}

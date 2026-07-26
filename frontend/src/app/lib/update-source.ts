import { create } from 'zustand';
import { BRAND } from './brand';

export type UpdateSourceId = 'gitcode' | 'github';
export type ReleaseChannel = 'stable' | 'prerelease';

// 默认走 GitCode 国内镜像：GitHub 的 API 与附件下载在国内经常超时，
// 而镜像仓库由同一条发布流水线同步，版本与产物完全一致。
export const DEFAULT_UPDATE_SOURCE: UpdateSourceId = 'gitcode';

export const UPDATE_SOURCE_IDS: readonly UpdateSourceId[] = ['gitcode', 'github'];

const STORAGE_KEY = 'thrm.update.source';

const INSTALLER_ASSET_NAME = 'THRM-amd64-installer.exe';
const CHECKSUMS_ASSET_NAME = 'SHA256SUMS';

/**
 * GitCode 镜像只同步 Windows 安装包与校验清单——跨境上传实测约 15 KiB/s，
 * 同步整批产物要半小时以上。因此非 Windows 平台上镜像没有任何可用产物，
 * 只能走 GitHub 源。
 */
export function isMirrorSupportedPlatform(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /windows/i.test(navigator.userAgent);
}

/** 各更新源统一后的发布信息，UI 只消费这一份结构。 */
export type NormalizedRelease = {
  tag: string;
  pageUrl: string;
  body: string;
  prerelease: boolean;
  /** 安装包的资产名，用于在校验清单里查表；没有安装包时为空串。 */
  installerName: string;
  /** 可直接交给后端下载安装的安装包地址；没有安装包时为空串。 */
  installerUrl: string;
};

type ReleaseAsset = {
  name?: string;
  browser_download_url?: string;
};

type GithubRelease = {
  tag_name?: string;
  html_url?: string;
  body?: string;
  prerelease?: boolean;
  draft?: boolean;
  assets?: ReleaseAsset[];
};

type GitcodeRelease = {
  tag_name?: string;
  body?: string;
  prerelease?: boolean;
  release_status?: string;
  assets?: ReleaseAsset[];
};

export function isUpdateSourceId(value: unknown): value is UpdateSourceId {
  return value === 'gitcode' || value === 'github';
}

export function releasesPageUrl(source: UpdateSourceId): string {
  return source === 'gitcode' ? BRAND.gitcodeReleasesUrl : BRAND.latestReleaseUrl;
}

function findAssetName(assets: ReleaseAsset[] | undefined): string {
  if (!Array.isArray(assets)) return '';
  const exact = assets.find((asset) => asset?.name === INSTALLER_ASSET_NAME);
  if (exact?.name) return exact.name;
  const fuzzy = assets.find(
    (asset) => typeof asset?.name === 'string' && /installer\.exe$/i.test(asset.name),
  );
  return fuzzy?.name || '';
}

function findGithubInstallerUrl(assets: ReleaseAsset[] | undefined): string {
  if (!Array.isArray(assets)) return '';
  const name = findAssetName(assets);
  if (!name) return '';
  return assets.find((asset) => asset?.name === name)?.browser_download_url || '';
}

// GitCode 附件的 browser_download_url 指向华为云 OBS，域名不固定；
// 这里改用带 tag 与文件名的固定下载接口，宿主端只需放行 api.gitcode.com。
function gitcodeInstallerUrl(tag: string, assets: ReleaseAsset[] | undefined): string {
  const name = findAssetName(assets);
  if (!name || !tag) return '';
  return gitcodeAttachmentUrl(tag, name);
}

async function fetchJson(url: string, headers: Record<string, string>): Promise<unknown> {
  const response = await fetch(url, { headers, cache: 'no-cache' });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.json();
}

async function fetchGithubRelease(channel: ReleaseChannel): Promise<NormalizedRelease | null> {
  const headers = { Accept: 'application/vnd.github+json' };
  let release: GithubRelease | null = null;

  if (channel === 'prerelease') {
    const payload = (await fetchJson(`${BRAND.releasesApiUrl}?per_page=30`, headers)) as GithubRelease[];
    release =
      (Array.isArray(payload) ? payload : []).find((item) => !item?.draft && !!item?.prerelease) || null;
  } else {
    release = (await fetchJson(BRAND.latestReleaseApiUrl, headers)) as GithubRelease;
  }

  if (!release) return null;

  return {
    tag: release.tag_name || '',
    pageUrl: release.html_url || BRAND.latestReleaseUrl,
    body: typeof release.body === 'string' ? release.body.trim() : '',
    prerelease: !!release.prerelease,
    installerName: findAssetName(release.assets),
    installerUrl: findGithubInstallerUrl(release.assets),
  };
}

function isGitcodePrerelease(release: GitcodeRelease): boolean {
  return release?.prerelease === true || release?.release_status === 'pre';
}

async function fetchGitcodeRelease(channel: ReleaseChannel): Promise<NormalizedRelease | null> {
  const headers = { Accept: 'application/json' };
  let release: GitcodeRelease | null = null;

  if (channel === 'prerelease') {
    const payload = (await fetchJson(`${BRAND.gitcodeApiBaseUrl}/releases`, headers)) as GitcodeRelease[];
    release = (Array.isArray(payload) ? payload : []).find(isGitcodePrerelease) || null;
  } else {
    const response = await fetch(`${BRAND.gitcodeApiBaseUrl}/releases/latest`, {
      headers,
      cache: 'no-cache',
    });
    // 镜像仓库还没有任何发行版时接口返回 400「未找到 release」，
    // 这是"没有可用版本"而不是请求失败，不能报成检查更新出错。
    if (response.status === 400 || response.status === 404) return null;
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const payload = (await response.json()) as GitcodeRelease;
    release = payload && typeof payload === 'object' && payload.tag_name ? payload : null;
  }

  if (!release) return null;

  const tag = release.tag_name || '';
  return {
    tag,
    pageUrl: BRAND.gitcodeReleasesUrl,
    body: typeof release.body === 'string' ? release.body.trim() : '',
    prerelease: isGitcodePrerelease(release),
    installerName: findAssetName(release.assets),
    installerUrl: gitcodeInstallerUrl(tag, release.assets),
  };
}

function gitcodeAttachmentUrl(tag: string, name: string): string {
  return `${BRAND.gitcodeApiBaseUrl}/releases/${encodeURIComponent(tag)}/attach_files/${encodeURIComponent(name)}/download`;
}

/** 解析 `<hex>  <name>` 形式的校验清单；GNU sha256sum 二进制模式会在名字前多一个 *。 */
function parseChecksums(text: string): Map<string, string> {
  const table = new Map<string, string>();
  for (const line of text.split(/\r?\n/)) {
    const match = line.trim().match(/^([0-9a-f]{64})\s+\*?(.+)$/i);
    if (match) {
      table.set(match[2].trim(), match[1].toLowerCase());
    }
  }
  return table;
}

async function fetchChecksums(url: string): Promise<Map<string, string> | null> {
  try {
    const response = await fetch(url, { cache: 'no-cache' });
    if (!response.ok) return null;
    const table = parseChecksums(await response.text());
    return table.size > 0 ? table : null;
  } catch {
    return null;
  }
}

/**
 * 取安装包的预期 SHA-256。拿不到返回空串，调用方据此中止自动更新——
 * 后端会拒绝无校验值的安装请求。
 *
 * 优先从 GitHub 取清单：它与镜像是两条独立的链路和信任源，这样即便镜像被
 * 篡改也能被发现。GitHub 取不到时退回镜像自带的那份，此时只能防传输损坏，
 * 防不住"镜像本身连文件带清单一起被换掉"——但这好过完全不校验。
 */
export async function fetchInstallerChecksum(
  source: UpdateSourceId,
  release: NormalizedRelease,
): Promise<string> {
  if (!release.tag || !release.installerName) return '';

  const candidates = [
    `${BRAND.repositoryUrl}/releases/download/${encodeURIComponent(release.tag)}/${CHECKSUMS_ASSET_NAME}`,
  ];
  if (source === 'gitcode') {
    candidates.push(gitcodeAttachmentUrl(release.tag, CHECKSUMS_ASSET_NAME));
  }

  for (const url of candidates) {
    const table = await fetchChecksums(url);
    const digest = table?.get(release.installerName);
    if (digest) return digest;
  }
  return '';
}

/**
 * 按更新源与通道拉取发布信息。找不到符合通道的版本时返回 null，
 * 网络或接口错误则抛出，由调用方区分"没有预发布"和"检查失败"。
 */
export function fetchLatestRelease(
  source: UpdateSourceId,
  channel: ReleaseChannel,
): Promise<NormalizedRelease | null> {
  return source === 'gitcode' ? fetchGitcodeRelease(channel) : fetchGithubRelease(channel);
}

function readStoredSource(): UpdateSourceId {
  if (typeof window === 'undefined') return DEFAULT_UPDATE_SOURCE;
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isUpdateSourceId(stored) ? stored : DEFAULT_UPDATE_SOURCE;
  } catch {
    return DEFAULT_UPDATE_SOURCE;
  }
}

interface UpdateSourceStore {
  /** 用户在设置里选的偏好。非 Windows 上不生效，见 mirrorSupported。 */
  source: UpdateSourceId;
  /** 镜像是否对当前平台有意义。false 时一律走 GitHub。 */
  mirrorSupported: boolean;
  setSource: (source: UpdateSourceId) => void;
}

export const useUpdateSourceStore = create<UpdateSourceStore>((set) => ({
  source: readStoredSource(),
  mirrorSupported: isMirrorSupportedPlatform(),
  setSource: (source) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, source);
    } catch {
      // 隐私模式等场景下写入失败时只保留本次会话的选择。
    }
    set({ source });
  },
}));

/**
 * 实际生效的更新源。非 Windows 上镜像没有可用产物，无视用户偏好回落到 GitHub，
 * 否则 Linux 用户点"打开发布页"会进到一个只有 exe 的镜像仓库。
 */
export function useEffectiveUpdateSource(): UpdateSourceId {
  return useUpdateSourceStore((state) =>
    state.mirrorSupported ? state.source : 'github',
  );
}

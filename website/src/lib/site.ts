const repo = "https://github.com/TIANLI0/THRM";

export const SITE = {
  name: "THRM",
  // 对外域名（反代后的国内域名），构建时写进 Open Graph 地址和页面里的命令示例兜底值。
  url: process.env.NEXT_PUBLIC_SITE_URL || "http://localhost:3000",
  // 拉不到 Release（反代未配置、网络问题）时显示的版本号，发版时随 build/config.yml 一起更新。
  fallbackVersion: "3.8.1",
  repoSlug: "TIANLI0/THRM",
  repo,
  issues: `${repo}/issues`,
  privacy: `${repo}/blob/main/PRIVACY.md`,
  signing: `${repo}/blob/main/CODE_SIGNING_POLICY.md`,
  license: `${repo}/blob/main/LICENSE`,
} as const;

/** 各安装包的识别规则：以后换文件名时只改这里。fallback 是文件名固定、可直接走 latest 下载的包。 */
export const ASSET_MATCH = {
  winInstaller: { fallback: "THRM-amd64-installer.exe", test: (n: string) => /installer\.exe$/i.test(n) },
  winPortable: { fallback: "THRM-windows-portable.zip", test: (n: string) => /windows-portable\.zip$/i.test(n) },
  deb: { fallback: null, test: (n: string) => /_amd64\.deb$/i.test(n) },
  arch: { fallback: null, test: (n: string) => /\.pkg\.tar\.zst$/i.test(n) },
  linuxPortable: { fallback: "THRM-linux-amd64-portable.tar.gz", test: (n: string) => /linux.*portable.*\.tar\.gz$/i.test(n) },
} as const;

export type AssetKey = keyof typeof ASSET_MATCH;

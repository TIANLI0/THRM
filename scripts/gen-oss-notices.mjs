// 生成「关于」页的开源声明清单：frontend/src/app/lib/open-source-notices.ts
//
// 依赖变了就重跑一次： node scripts/gen-oss-notices.mjs
//
// 数据来源：
//   - Go：go list -deps 取真正被链接进二进制的模块，再读模块缓存里的 LICENSE 判型
//   - 前端：package.json 的 dependencies（devDependencies 是构建期工具，不随产物分发）
//   - 原生/内嵌组件：无法从包管理器推导，见下面的 NATIVE 常量
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const OUT = join(ROOT, 'frontend/src/app/lib/open-source-notices.ts');

/** 从许可证正文判型，避免手写清单时把协议记错。 */
function detectLicense(text) {
  const t = text.replace(/\s+/g, ' ').toLowerCase();
  if (t.includes('mozilla public license') && t.includes('2.0')) return 'MPL-2.0';
  if (t.includes('apache license') && t.includes('version 2.0')) return 'Apache-2.0';
  if (t.includes('gnu general public license') && t.includes('version 3')) return 'GPL-3.0';
  if (t.includes('permission is hereby granted, free of charge')) return 'MIT';
  if (t.includes('redistribution and use in source and binary forms')) {
    return t.includes('neither the name') ? 'BSD-3-Clause' : 'BSD-2-Clause';
  }
  if (t.includes('free and unencumbered software released into the public domain')) return 'Unlicense';
  if (t.includes('permission to use, copy, modify, and/or distribute')) return 'ISC';
  if (t.includes('sil open font license')) return 'OFL-1.1';
  return 'UNKNOWN';
}

function licenseIn(dir) {
  if (!existsSync(dir)) return null;
  const names = ['LICENSE', 'LICENSE.txt', 'LICENSE.md', 'COPYING', 'LICENCE'];
  const direct = names.map((n) => join(dir, n)).find(existsSync);
  const file =
    direct ??
    readdirSync(dir)
      .filter((n) => /^(licen[sc]e|copying)/i.test(n))
      .map((n) => join(dir, n))[0];
  return file ? detectLicense(readFileSync(file, 'utf8')) : null;
}

/** 模块路径不一定能直接当主页打开，这里补上真实仓库地址。 */
const GO_HOMEPAGE = {
  'fyne.io/systray': 'https://github.com/fyne-io/systray',
  'go.uber.org/zap': 'https://github.com/uber-go/zap',
  'go.uber.org/multierr': 'https://github.com/uber-go/multierr',
  'golang.design/x/hotkey': 'https://github.com/golang-design/hotkey',
  'golang.org/x/sys': 'https://pkg.go.dev/golang.org/x/sys',
  'gopkg.in/natefinch/lumberjack.v2': 'https://github.com/natefinch/lumberjack',
  'tinygo.org/x/bluetooth': 'https://github.com/tinygo-org/bluetooth',
};

function goHomepage(path) {
  if (GO_HOMEPAGE[path]) return GO_HOMEPAGE[path];
  if (path.startsWith('github.com/') || path.startsWith('git.sr.ht/')) {
    return 'https://' + path.replace(/\/v\d+$/, '');
  }
  return 'https://pkg.go.dev/' + path;
}

/** 分组；没列到的按默认值归类（Go 归 backend，npm 归 frontend）。 */
const GROUP_OF = {
  'github.com/wailsapp/wails/v2': 'framework',
  'github.com/wailsapp/go-webview2': 'framework',
  'github.com/sstallion/go-hid': 'device',
  'tinygo.org/x/bluetooth': 'device',
  'github.com/saltosystems/winrt-go': 'device',
  'github.com/shirou/gopsutil/v4': 'device',
  'github.com/yusufpapurcu/wmi': 'device',
  'github.com/go-ole/go-ole': 'device',
  next: 'framework',
  react: 'framework',
  'react-dom': 'framework',
};

const goModules = execFileSync(
  'go',
  ['list', '-deps', '-f', '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}', './...'],
  { cwd: ROOT, encoding: 'utf8' },
)
  .split('\n')
  .map((line) => line.trim())
  .filter(Boolean)
  .filter((line, index, all) => all.indexOf(line) === index)
  .map((line) => line.split('|'))
  .filter(([path]) => !path.startsWith('github.com/TIANLI0'))
  .map(([path, version, dir]) => ({
    name: path,
    version,
    license: licenseIn(dir) ?? 'UNKNOWN',
    url: goHomepage(path),
    group: GROUP_OF[path] ?? 'backend',
  }));

const pkg = JSON.parse(readFileSync(join(ROOT, 'frontend/package.json'), 'utf8'));
const npmPackages = Object.keys(pkg.dependencies ?? {})
  .sort()
  .map((name) => {
    const dir = join(ROOT, 'frontend/node_modules', ...name.split('/'));
    const manifestPath = join(dir, 'package.json');
    const manifest = existsSync(manifestPath) ? JSON.parse(readFileSync(manifestPath, 'utf8')) : {};
    const declared = Array.isArray(manifest.licenses)
      ? manifest.licenses.map((entry) => entry.type).join('/')
      : manifest.license;
    return {
      name,
      version: manifest.version ?? String(pkg.dependencies[name]).replace(/^[\^~]/, ''),
      license: declared || licenseIn(dir) || 'UNKNOWN',
      url: 'https://www.npmjs.com/package/' + name,
      group: GROUP_OF[name] ?? (name.startsWith('@fontsource') ? 'fonts' : 'frontend'),
    };
  });

/**
 * 原生与内嵌组件：不经包管理器引入，只能手写。
 * 改这一段时请同步核对对应项目的 LICENSE 文件。
 *
 * HIDAPI 由 go-hid 以 C 源码内嵌编译，上游是 GPL-3.0 / BSD-3-Clause / 原始 HIDAPI
 * 许可三选一，本项目按 BSD-3-Clause 使用。
 * LibreHardwareMonitorLib 为 MPL-2.0：以未修改的二进制形式分发，源码见其仓库。
 */
const NATIVE = [
  {
    name: 'HIDAPI',
    version: 'bundled in go-hid',
    license: 'BSD-3-Clause',
    url: 'https://github.com/libusb/hidapi',
    group: 'device',
  },
  {
    name: 'LibreHardwareMonitorLib',
    version: '0.9.6',
    license: 'MPL-2.0',
    url: 'https://github.com/LibreHardwareMonitor/LibreHardwareMonitor',
    group: 'device',
  },
  {
    name: 'Newtonsoft.Json',
    version: '13.0.3',
    license: 'MIT',
    url: 'https://github.com/JamesNK/Newtonsoft.Json',
    group: 'backend',
  },
  {
    name: 'Go',
    version: (readFileSync(join(ROOT, 'go.mod'), 'utf8').match(/^go (\S+)/m) ?? [])[1] ?? '',
    license: 'BSD-3-Clause',
    url: 'https://go.dev',
    group: 'framework',
  },
];

const all = [...goModules, ...npmPackages, ...NATIVE];
const unknown = all.filter((item) => item.license === 'UNKNOWN');
if (unknown.length > 0) {
  console.error('无法判定许可证，请手工补充：');
  for (const item of unknown) console.error('  ' + item.name);
  process.exit(1);
}

const GROUPS = ['framework', 'frontend', 'device', 'backend', 'fonts'];
const quote = (value) => "'" + String(value).replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "'";

const body = GROUPS.map((group) => {
  const rows = all
    .filter((item) => item.group === group)
    .sort((a, b) => a.name.localeCompare(b.name))
    .map(
      (item) =>
        '    { name: ' + quote(item.name) +
        ', version: ' + quote(item.version) +
        ', license: ' + quote(item.license) +
        ', url: ' + quote(item.url) + ' },',
    )
    .join('\n');
  return '  {\n    id: ' + quote(group) + ',\n    items: [\n' + rows + '\n    ],\n  },';
}).join('\n');

const header = [
  '// 由 scripts/gen-oss-notices.mjs 生成，请勿手改。',
  '// 依赖变动后重新运行： node scripts/gen-oss-notices.mjs',
  '//',
  '// 只收录随发行产物一起分发的组件：Go 侧取真正链接进二进制的模块，前端取',
  '// dependencies。仅在构建期使用、不进产物的工具链（TypeScript、类型声明包等）不列入。',
  '',
  'export type OpenSourceGroupId = ' + GROUPS.map(quote).join(' | ') + ';',
  '',
  'export interface OpenSourceNotice {',
  '  name: string;',
  '  version: string;',
  '  /** SPDX 标识，由各组件自带的 LICENSE 正文判定。 */',
  '  license: string;',
  '  url: string;',
  '}',
  '',
  'export interface OpenSourceGroup {',
  '  id: OpenSourceGroupId;',
  '  items: OpenSourceNotice[];',
  '}',
  '',
  'export const OPEN_SOURCE_GROUPS: OpenSourceGroup[] = [',
].join('\n');

const footer = [
  '];',
  '',
  'export const OPEN_SOURCE_TOTAL = OPEN_SOURCE_GROUPS.reduce((sum, group) => sum + group.items.length, 0);',
  '',
].join('\n');

writeFileSync(OUT, header + '\n' + body + '\n' + footer, 'utf8');
console.log('已写入 ' + OUT + '（' + all.length + ' 项）');

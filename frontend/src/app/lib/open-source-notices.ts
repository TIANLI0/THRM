// 由 scripts/gen-oss-notices.mjs 生成，请勿手改。
// 依赖变动后重新运行： node scripts/gen-oss-notices.mjs
//
// 只收录随发行产物一起分发的组件：Go 侧取真正链接进二进制的模块，前端取
// dependencies。仅在构建期使用、不进产物的工具链（TypeScript、类型声明包等）不列入。

export type OpenSourceGroupId = 'framework' | 'frontend' | 'device' | 'backend' | 'fonts';

export interface OpenSourceNotice {
  name: string;
  version: string;
  /** SPDX 标识，由各组件自带的 LICENSE 正文判定。 */
  license: string;
  url: string;
}

export interface OpenSourceGroup {
  id: OpenSourceGroupId;
  items: OpenSourceNotice[];
}

export const OPEN_SOURCE_GROUPS: OpenSourceGroup[] = [
  {
    id: 'framework',
    items: [
    { name: 'github.com/wailsapp/go-webview2', version: 'v1.0.22', license: 'MIT', url: 'https://github.com/wailsapp/go-webview2' },
    { name: 'github.com/wailsapp/wails/v2', version: 'v2.15.0', license: 'MIT', url: 'https://github.com/wailsapp/wails' },
    { name: 'Go', version: '1.27.0', license: 'BSD-3-Clause', url: 'https://go.dev' },
    { name: 'next', version: '16.3.2', license: 'MIT', url: 'https://www.npmjs.com/package/next' },
    { name: 'react', version: '19.2.8', license: 'MIT', url: 'https://www.npmjs.com/package/react' },
    { name: 'react-dom', version: '19.2.8', license: 'MIT', url: 'https://www.npmjs.com/package/react-dom' },
    ],
  },
  {
    id: 'frontend',
    items: [
    { name: '@radix-ui/react-collapsible', version: '1.1.20', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-collapsible' },
    { name: '@radix-ui/react-context-menu', version: '2.3.7', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-context-menu' },
    { name: '@radix-ui/react-dialog', version: '1.1.23', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-dialog' },
    { name: '@radix-ui/react-label', version: '2.1.15', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-label' },
    { name: '@radix-ui/react-radio-group', version: '1.4.7', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-radio-group' },
    { name: '@radix-ui/react-scroll-area', version: '1.2.18', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-scroll-area' },
    { name: '@radix-ui/react-select', version: '2.3.7', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-select' },
    { name: '@radix-ui/react-slider', version: '1.4.7', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-slider' },
    { name: '@radix-ui/react-switch', version: '1.3.7', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-switch' },
    { name: '@radix-ui/react-tabs', version: '1.1.21', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-tabs' },
    { name: '@radix-ui/react-tooltip', version: '1.2.16', license: 'MIT', url: 'https://www.npmjs.com/package/@radix-ui/react-tooltip' },
    { name: 'class-variance-authority', version: '0.7.1', license: 'Apache-2.0', url: 'https://www.npmjs.com/package/class-variance-authority' },
    { name: 'clsx', version: '2.1.1', license: 'MIT', url: 'https://www.npmjs.com/package/clsx' },
    { name: 'framer-motion', version: '13.1.1', license: 'MIT', url: 'https://www.npmjs.com/package/framer-motion' },
    { name: 'i18next', version: '26.4.0', license: 'MIT', url: 'https://www.npmjs.com/package/i18next' },
    { name: 'lucide-react', version: '1.33.0', license: 'ISC', url: 'https://www.npmjs.com/package/lucide-react' },
    { name: 'next-themes', version: '0.4.6', license: 'MIT', url: 'https://www.npmjs.com/package/next-themes' },
    { name: 'radix-ui', version: '1.6.7', license: 'MIT', url: 'https://www.npmjs.com/package/radix-ui' },
    { name: 'react-i18next', version: '17.0.12', license: 'MIT', url: 'https://www.npmjs.com/package/react-i18next' },
    { name: 'react-icons', version: '5.7.0', license: 'MIT', url: 'https://www.npmjs.com/package/react-icons' },
    { name: 'recharts', version: '3.10.1', license: 'MIT', url: 'https://www.npmjs.com/package/recharts' },
    { name: 'shadcn', version: '4.19.0', license: 'MIT', url: 'https://www.npmjs.com/package/shadcn' },
    { name: 'sonner', version: '2.0.8', license: 'MIT', url: 'https://www.npmjs.com/package/sonner' },
    { name: 'sortablejs', version: '1.15.7', license: 'MIT', url: 'https://www.npmjs.com/package/sortablejs' },
    { name: 'tailwind-merge', version: '3.6.0', license: 'MIT', url: 'https://www.npmjs.com/package/tailwind-merge' },
    { name: 'tw-animate-css', version: '1.4.0', license: 'MIT', url: 'https://www.npmjs.com/package/tw-animate-css' },
    { name: 'zustand', version: '5.0.15', license: 'MIT', url: 'https://www.npmjs.com/package/zustand' },
    ],
  },
  {
    id: 'device',
    items: [
    { name: 'github.com/go-ole/go-ole', version: 'v1.3.0', license: 'MIT', url: 'https://github.com/go-ole/go-ole' },
    { name: 'github.com/saltosystems/winrt-go', version: 'v0.0.0-20260317170058-9c2fec580d96', license: 'MIT', url: 'https://github.com/saltosystems/winrt-go' },
    { name: 'github.com/shirou/gopsutil/v4', version: 'v4.26.7', license: 'BSD-3-Clause', url: 'https://github.com/shirou/gopsutil' },
    { name: 'github.com/sstallion/go-hid', version: 'v0.15.0', license: 'BSD-2-Clause', url: 'https://github.com/sstallion/go-hid' },
    { name: 'github.com/yusufpapurcu/wmi', version: 'v1.2.4', license: 'MIT', url: 'https://github.com/yusufpapurcu/wmi' },
    { name: 'HIDAPI', version: 'bundled in go-hid', license: 'BSD-3-Clause', url: 'https://github.com/libusb/hidapi' },
    { name: 'LibreHardwareMonitorLib', version: '0.9.6', license: 'MPL-2.0', url: 'https://github.com/LibreHardwareMonitor/LibreHardwareMonitor' },
    { name: 'tinygo.org/x/bluetooth', version: 'v0.15.0', license: 'BSD-3-Clause', url: 'https://github.com/tinygo-org/bluetooth' },
    ],
  },
  {
    id: 'backend',
    items: [
    { name: 'fyne.io/systray', version: 'v1.12.2', license: 'Apache-2.0', url: 'https://github.com/fyne-io/systray' },
    { name: 'git.sr.ht/~jackmordaunt/go-toast', version: 'v1.1.2', license: 'MIT', url: 'https://git.sr.ht/~jackmordaunt/go-toast' },
    { name: 'github.com/gen2brain/beeep', version: 'v0.11.2', license: 'BSD-2-Clause', url: 'https://github.com/gen2brain/beeep' },
    { name: 'github.com/godbus/dbus/v5', version: 'v5.2.2', license: 'BSD-2-Clause', url: 'https://github.com/godbus/dbus' },
    { name: 'github.com/leaanthony/go-ansi-parser', version: 'v1.6.1', license: 'MIT', url: 'https://github.com/leaanthony/go-ansi-parser' },
    { name: 'github.com/leaanthony/slicer', version: 'v1.6.0', license: 'MIT', url: 'https://github.com/leaanthony/slicer' },
    { name: 'github.com/leaanthony/u', version: 'v1.1.1', license: 'MIT', url: 'https://github.com/leaanthony/u' },
    { name: 'github.com/Microsoft/go-winio', version: 'v0.6.2', license: 'MIT', url: 'https://github.com/Microsoft/go-winio' },
    { name: 'github.com/pkg/errors', version: 'v0.9.1', license: 'BSD-2-Clause', url: 'https://github.com/pkg/errors' },
    { name: 'github.com/rivo/uniseg', version: 'v0.4.7', license: 'MIT', url: 'https://github.com/rivo/uniseg' },
    { name: 'github.com/sergeymakinen/go-bmp', version: 'v1.0.0', license: 'BSD-3-Clause', url: 'https://github.com/sergeymakinen/go-bmp' },
    { name: 'github.com/sergeymakinen/go-ico', version: 'v1.0.0-beta.0', license: 'BSD-3-Clause', url: 'https://github.com/sergeymakinen/go-ico' },
    { name: 'github.com/tadvi/systray', version: 'v0.0.0-20190226123456-11a2b8fa57af', license: 'MIT', url: 'https://github.com/tadvi/systray' },
    { name: 'go.uber.org/multierr', version: 'v1.10.0', license: 'MIT', url: 'https://github.com/uber-go/multierr' },
    { name: 'go.uber.org/zap', version: 'v1.28.0', license: 'MIT', url: 'https://github.com/uber-go/zap' },
    { name: 'golang.design/x/hotkey', version: 'v0.6.1', license: 'MIT', url: 'https://github.com/golang-design/hotkey' },
    { name: 'golang.org/x/sys', version: 'v0.47.0', license: 'BSD-3-Clause', url: 'https://pkg.go.dev/golang.org/x/sys' },
    { name: 'gopkg.in/natefinch/lumberjack.v2', version: 'v2.2.1', license: 'MIT', url: 'https://github.com/natefinch/lumberjack' },
    { name: 'Newtonsoft.Json', version: '13.0.3', license: 'MIT', url: 'https://github.com/JamesNK/Newtonsoft.Json' },
    ],
  },
  {
    id: 'fonts',
    items: [
    { name: '@fontsource-variable/geist-mono', version: '5.3.0', license: 'OFL-1.1', url: 'https://www.npmjs.com/package/@fontsource-variable/geist-mono' },
    { name: '@fontsource-variable/manrope', version: '5.3.0', license: 'OFL-1.1', url: 'https://www.npmjs.com/package/@fontsource-variable/manrope' },
    ],
  },
];

export const OPEN_SOURCE_TOTAL = OPEN_SOURCE_GROUPS.reduce((sum, group) => sum + group.items.length, 0);

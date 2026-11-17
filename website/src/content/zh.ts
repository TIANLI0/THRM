import type { Dict } from "./types";

export const zh: Dict = {
  locale: "zh-CN",
  htmlLang: "zh-CN",
  meta: {
    title: "THRM · 飞智 BS 系列散热器控制中心",
    description:
      "THRM 是飞智 BS 系列笔记本压风散热器的第三方开源控制中心：可编辑风扇曲线、SmartControl 智能控温、噪音校准、RGB 灯效与硬件监控，支持 Windows 与 Linux。",
  },
  nav: {
    features: "功能",
    devices: "设备",
    download: "下载",
    faq: "常见问题",
    changelog: "更新日志",
    github: "GitHub",
    downloadCta: "下载",
    openMenu: "打开菜单",
    closeMenu: "关闭菜单",
    switchLanguage: "Switch to English",
    languageLabel: "EN",
    theme: { system: "跟随系统", light: "浅色", dark: "深色", label: "切换主题，当前：" },
  },
  hero: {
    release: "v{version} 已发布",
    title: ["轻量", "易用", "自定义"],
    lead: "为游戏本而生的第三方散热驱动。THRM 按 CPU、GPU 实时温度调节飞智 BS 系列散热器的转速，关掉窗口也会在托盘里继续控温。",
    downloadWindows: "下载 Windows 版",
    downloadLinux: "下载 Linux 版",
    source: "查看源码",
    platforms: ["Windows 10 / 11", "Linux x86_64", "MIT 开源，免费使用"],
    demoHint: "拖动曲线上的红线，看看转速怎么跟着变",
  },
  window: {
    sidebar: { status: "状态", curve: "曲线", control: "设置", about: "关于", expand: "展开侧边栏" },
    connected: "已连接",
    smartControl: "智能变频",
    modeLine: "智能控制 · 根据实时温度自动调节转速",
    disconnect: "断开",
    cpuTemperature: "CPU 温度",
    gpuTemperature: "GPU 温度",
    fanRpm: "风扇转速",
    tempStatus: { overheat: "过热", high: "偏高", normal: "正常", good: "良好" },
    targetSummary: "目标 {target} · {gear}",
    gears: ["静音", "标准", "强劲", "超频"],
    controlProtection: "控制与保护",
    stats: {
      controlMode: "控制模式",
      controlModeValue: "智能控制（默认）",
      maxRpm: "最高转速",
      workMode: "工作模式",
      workModeValue: "自动模式(实时转速)",
      cpuPower: "CPU 功耗",
      gpuPower: "GPU 功耗",
    },
    curveTitle: "风扇转速曲线",
    curveSlider: "模拟温度",
    historyTitle: "温度与风扇历史",
    historyBadge: "后台记录",
  },
  pipeline: {
    title: "从一次温度读数，到一次转速变化",
    lead: "THRM Core 作为独立的后台服务运行，每次采样都会走完这四步。主窗口开着还是关着，都不影响控温。",
    steps: [
      {
        title: "读取温度",
        body: "按 CPU、GPU 或两者取高来控温。可以指定传感器，多个 CPU 传感器还能取平均值。",
      },
      {
        title: "查询曲线",
        body: "在你编辑的温度—转速曲线上插值。到了设定的星期和时段，自动换成对应方案。",
      },
      {
        title: "智能修正",
        body: "SmartControl 过滤孤立的温度尖峰，按滞回和升降速限制平滑响应，功耗上来时提前加速。",
      },
      {
        title: "写入设备",
        body: "通过 HID 或 BLE 把转速发给散热器。HID 接口重新出现后会自动恢复连接。",
      },
    ],
  },
  features: {
    title: "固定挡位之外，能调的都交给你",
    lead: "基础控制方式只给几个挡位。THRM 把曲线、学习、噪音和灯效都开放出来，并且全部在本机完成。",
    items: {
      curve: {
        title: "风扇曲线与方案",
        body: "可视化编辑温度—转速曲线，保存多套方案，用方案码分享给别人，或者按星期和时段自动切换。",
      },
      smart: {
        title: "SmartControl 智能控温",
        body: "根据长期运行结果自动修正曲线。选一种学习倾向即可，不满意随时一键清除学习结果。",
      },
      noise: {
        title: "噪音校准",
        body: "用麦克风测出每个转速下的相对噪音，标出噪音拐点和疑似共振区间，静音学习会主动避开。",
      },
      monitor: {
        title: "硬件监控",
        body: "CPU、GPU 温度与功耗，散热器当前转速和目标转速，都有历史趋势。Windows 下还能把转速显示在 RTSS 游戏内叠加层。",
      },
      lighting: {
        title: "RGB 灯效",
        body: "六种灯效，亮度和速度可调。系统睡眠或休眠时自动停转关灯。",
      },
      desktop: {
        title: "桌面体验",
        body: "像系统自带的工具一样安静地待在后台。",
      },
    },
    schedule: {
      days: ["一", "二", "三", "四", "五", "六", "日"],
      profiles: ["静音", "均衡", "游戏"],
    },
    smart: {
      tendencies: ["均衡", "散热优先", "静音优先"],
      base: "基础曲线",
      learned: "学习后",
    },
    noise: { resonance: "疑似共振", axis: "转速 RPM" },
    monitor: { cpu: "CPU 温度", gpu: "GPU 温度", fan: "风扇转速", overlay: "游戏内叠加层" },
    lighting: { modes: ["智能温控", "单色", "多色", "旋转", "流光", "呼吸"] },
    desktop: [
      "系统托盘常驻，开机自启动",
      "全局快捷键切换挡位、智能变频和方案",
      "浅色、深色、跟随系统与自定义主题",
      "Windows 亚克力、云母和云母 Alt 材质",
      "简体中文、English、日本語",
      "应用内更新，一键导出诊断包",
    ],
  },
  devices: {
    title: "支持的设备",
    lead: "不同固件和硬件版本的最高转速、灯效能力不同，THRM 会按设备实际上报的能力来控制。",
    pairFirst: "先在系统蓝牙设置中完成配对",
    autoScan: "通电后由 THRM 自动扫描并连接",
  },
  quickstart: {
    title: "四步开始控温",
    steps: [
      { title: "给散热器供电", body: "接上电源，确认设备已经正常工作。" },
      {
        title: "连接设备",
        body: "BS1 保持蓝牙广播即可。BS2、BS2 Pro、BS3、BS3 Pro 先在系统蓝牙设置中完成配对。",
      },
      { title: "启动 THRM", body: "在状态页确认型号、温度和风扇转速都已经显示出来。" },
      {
        title: "选择控制方式",
        body: "打开「智能变频」，或者在曲线页选一套方案。关闭窗口后，THRM 会继续在托盘里控温。",
      },
    ],
  },
  download: {
    title: "下载 THRM",
    tablist: "选择系统",
    windowsReq: "Windows 10 / 11 x64",
    linuxReq: "x86_64，推荐 Debian、Ubuntu 及其衍生发行版",
    recommended: "推荐",
    installer: {
      name: "安装程序",
      body: "大多数用户选这个。后台核心服务和温度桥接组件会一并装好并自动运行。",
    },
    portable: {
      name: "便携版",
      body: "解压全部文件后运行 THRM.exe。不要单独移动或删除 THRM Core.exe 和 bridge 目录。",
    },
    linux: [
      {
        key: "deb",
        label: "Debian / Ubuntu",
        body: "同时安装应用、后台核心服务、桌面入口和飞智 HID 设备的 udev 规则。",
        install: "sudo apt install ./{file}",
      },
      {
        key: "arch",
        label: "Arch / CachyOS / Manjaro",
        body: "安装 CI 已构建好的二进制，不需要本地安装 Go 或 Bun。",
        install: "sudo pacman -U ./{file}",
      },
      {
        key: "linuxPortable",
        label: "便携安装包",
        body: "默认安装到 ~/.local/bin。",
        install: "tar -xzf {file}\ncd THRM-linux-amd64\nchmod +x install.sh && ./install.sh",
      },
    ],
    linuxDeps: "需要 GTK 3、WebKit2GTK 4.1、hidapi 和 udev。使用 BS1 时还需要 BlueZ 蓝牙服务正常运行。",
    signing:
      "Windows 发布包使用 Certum 代码签名证书签名，由 GitHub Actions 从公开源码构建后自动签名。请只从 GitHub Releases 下载。",
    releases: "全部版本",
    changelog: "更新日志",
    copy: "复制命令",
    copied: "已复制",
    get: "下载",
  },
  changelog: {
    title: "更新日志",
    lead: "每个版本的改动与安装包。文件由本站转发，不需要访问 GitHub。",
    latest: "最新",
    prerelease: "预发布",
    publishedOn: "发布于 {date}",
    downloads: "安装包",
    noNotes: "这个版本没有填写更新说明。",
    unavailable: "暂时拿不到版本列表，请稍后刷新再试。",
    viewOnGithub: "在 GitHub 查看",
  },
  faq: {
    title: "常见问题",
    items: [
      {
        q: "设备连不上怎么办？",
        a: [
          "先确认散热器已经正常供电，型号和连接方式对得上。BS2、BS2 Pro、BS3、BS3 Pro 需要先在系统蓝牙设置中配对。",
          "关闭飞智官方软件或其他可能占用设备的控制工具；仍然不行时，删除系统里的蓝牙配对记录重新配对，或断电几秒再接上。BS1 扫描不到时，长按设备按键重新进入蓝牙广播状态。",
        ],
      },
      {
        q: "CPU 或 GPU 温度显示为 0？",
        a: [
          "Windows 下依次尝试：在设置页重新初始化温度监控，关闭其他硬件监控或风扇控制软件，重新安装或更新 PawnIO，重启电脑，以管理员身份运行 THRM。",
          "混合显卡笔记本如果只用 CPU 温度控温，可以关闭 GPU 监测，避免独立显卡被后台轮询唤醒。",
        ],
      },
      {
        q: "关闭窗口后程序还在运行？",
        a: ["这是为了继续自动控温。THRM 默认最小化到系统托盘，要完全退出，在托盘图标菜单里选择「退出」。"],
      },
      {
        q: "能在游戏里显示散热器转速吗？",
        a: [
          "可以，仅限 Windows，需要先安装 RivaTuner Statistics Server（RTSS），不需要 MSI Afterburner。",
          "在 THRM 的「系统设置」中开启「RTSS 屏幕显示」，叠加层会显示 Cooler Fan: <转速> RPM。刷新频率可选 250 毫秒到 2 秒，不影响控温响应速度。",
        ],
      },
      {
        q: "THRM 和飞智官方有关系吗？",
        a: [
          "没有。THRM 是社区开发的第三方开源项目，与飞智官方无关。",
          "自定义转速、非官方最低转速和实验性功能可能带来额外风险，请根据设备实际情况谨慎使用。",
        ],
      },
      {
        q: "怎样反馈问题？",
        a: [
          "到 GitHub Issues 提交，并附上 THRM 版本、系统版本、散热器型号、复现步骤，以及设置页导出的诊断包。",
          "诊断包可能包含设备和系统硬件信息，上传前请自行检查。",
        ],
      },
    ],
  },
  footer: {
    disclaimer: "THRM 是社区开发的第三方开源项目，与飞智官方无关。",
    links: {
      github: "源代码",
      releases: "版本发布",
      issues: "问题反馈",
      privacy: "隐私政策",
      signing: "代码签名策略",
      license: "MIT 许可证",
    },
    copyright: "© 2026 TIANLI0",
  },
};

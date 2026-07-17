<div align="center">

<img src="docs/assets/thrm-poster-light.png" alt="THRM" width="960">

# THRM

**飞智 BS 系列笔记本压风散热器的第三方控制中心**

*A lightweight, open-source desktop controller for Flydigi BS-series laptop coolers.*

[![Release](https://img.shields.io/github/v/release/TIANLI0/THRM?include_prereleases\&sort=semver\&label=release)](https://github.com/TIANLI0/THRM/releases)
[![Downloads](https://img.shields.io/github/downloads/TIANLI0/THRM/total?label=downloads)](https://github.com/TIANLI0/THRM/releases)
[![Build](https://github.com/TIANLI0/THRM/actions/workflows/build-and-release.yml/badge.svg)](https://github.com/TIANLI0/THRM/actions/workflows/build-and-release.yml)
[![License](https://img.shields.io/github/license/TIANLI0/THRM)](LICENSE)
[![Stars](https://img.shields.io/github/stars/TIANLI0/THRM?style=flat)](https://github.com/TIANLI0/THRM/stargazers)

[下载](#下载与安装) ·
[功能](#主要功能) ·
[设备兼容性](#设备兼容性) ·
[常见问题](#常见问题) ·
[开发与构建](#开发与构建) ·
[参与贡献](#参与贡献)

</div>

> [!IMPORTANT]
> 普通用户只需要从 [Releases](https://github.com/TIANLI0/THRM/releases/latest) 下载对应安装包，不需要安装 Go、Bun、Wails、Node.js 或 .NET SDK。

> [!WARNING]
> THRM 是由社区开发的第三方开源项目，与飞智官方无关。自定义转速、非官方最低转速和实验性功能可能带来额外风险，请根据设备实际情况谨慎使用。

## THRM 是什么

THRM 是一款面向飞智 BS 系列笔记本压风式散热器的第三方桌面控制工具。

它由独立运行的后台核心服务负责设备通信、温度监控与风扇控制，即使关闭主窗口，也可以继续在系统托盘中保持控温。

相比只提供固定挡位的基础控制方式，THRM 提供了可编辑风扇曲线、自适应学习、功耗预测前馈、分时方案、噪音校准、RGB 灯效、温度历史与诊断导出等增强能力。

## 设备兼容性

| 设备         | 通信方式 | 支持状态 | 连接说明              |
| ---------- | ---- | :--: | ----------------- |
| 飞智 BS1     | BLE  |   ✅  | 设备通电后由 THRM 扫描并连接 |
| 飞智 BS2     | HID  |   ✅  | 先在系统蓝牙设置中完成配对     |
| 飞智 BS2 Pro | HID  |   ✅  | 先在系统蓝牙设置中完成配对     |
| 飞智 BS3     | HID  |   ✅  | 先在系统蓝牙设置中完成配对     |
| 飞智 BS3 Pro | HID  |   ✅  | 先在系统蓝牙设置中完成配对     |

BS2、BS2 Pro、BS3 和 BS3 Pro 在完成系统蓝牙配对后，会以 HID 设备形式与 THRM 通信。

不同设备固件、供电能力和硬件版本可能具有不同的最高转速及灯效能力，THRM 会根据设备实际上报的能力进行控制。

## 主要功能

### 风扇控制

* 根据 CPU、GPU 或两者最高温度自动调节转速
* 可视化编辑温度—转速曲线
* 固定转速与手动挡位控制
* 自定义各挡位的具体 RPM
* 多套曲线方案创建、重命名与切换
* 使用方案码导入或分享曲线
* 按星期和时间段自动切换曲线方案
* 噪音敏感转速区间自动避让
* 高温状态下自动旁路避噪限制，优先保证散热

### SmartControl 智能控温

* 根据长期运行结果自动修正曲线
* 均衡、散热优先和静音优先三种学习倾向
* 独立的功耗预测前馈开关
* 根据 CPU/GPU 功耗突增提前提升转速
* 过滤孤立温度尖峰，减少不必要的转速波动
* 结合笔记本内置风扇负载限制快速降速
* 可配置目标温度、滞回、升降速限制和响应强度
* 支持一键清除学习结果并恢复基础曲线

### 噪音校准

* 通过麦克风测量不同 RPM 下的相对噪音
* 自动生成转速—噪音档案
* 检测明显的噪音拐点与疑似共振区间
* 将测量结果用于静音学习优化
* 可根据测试结果快速建立避噪转速区间

### 硬件监控

* CPU、GPU 实时温度
* CPU、GPU 实时功耗
* 散热器当前转速和目标转速
* 温度、功耗与风扇转速历史趋势
* 图表时间范围缩放与详细统计
* 自定义 CPU/GPU 设备和传感器
* 支持多个 CPU 传感器取平均值
* 可停用 GPU 监测，避免混合显卡笔记本低负载时持续唤醒独立显卡
* 部分华硕、联想拯救者和机械革命机型可读取笔记本内置 CPU/GPU 风扇转速

> 笔记本内置风扇读取依赖具体厂商接口，不支持的机型会自动隐藏相关数据，不影响散热器控制。

### 设备与灯效

* 设备状态、工作模式和最大转速能力读取
* 智能启停
* 通电自启动
* 挡位灯控制
* RGB 亮度与动画速度调节
* 智能温控、单色、多色、旋转、流光和呼吸灯效
* 系统睡眠或休眠时自动停止风扇并关闭灯光
* HID 接口重新出现后自动恢复连接

### 桌面端体验

* 系统托盘后台运行
* 开机自启动
* 全局快捷键切换手动挡位、智能变频和曲线方案
* 记忆窗口位置、大小与最大化状态
* 浅色、深色、跟随系统及自定义主题
* Windows 亚克力、云母和云母 Alt 窗口材质
* 简体中文、English、日本語
* 应用内检查、下载并安装更新
* 调试日志和一键导出诊断包
* 部分联想拯救者机型支持 Fn+Q 性能模式联动

## 下载与安装

前往：

**[GitHub Releases](https://github.com/TIANLI0/THRM/releases/latest)**

### Windows

支持 Windows 10 / Windows 11 x64。

| 文件                          | 用途            |
| --------------------------- | ------------- |
| `THRM-amd64-installer.exe`  | 推荐，大多数用户选择此版本 |
| `THRM-windows-portable.zip` | 便携版本，可解压后运行   |

推荐使用安装程序。安装完成后启动 THRM，后台核心服务和温度桥接组件会自动运行。

便携版用户解压全部文件后，应直接运行 `THRM.exe`，不要单独移动或删除以下组件：

```text
THRM.exe
THRM Core.exe
PawnIO_setup.exe
bridge/
```

Windows 温度监控使用 LibreHardwareMonitor 和 PawnIO。安装程序会处理所需组件；如果便携版无法读取 CPU 温度，可以手动运行目录中的 `PawnIO_setup.exe`。

### Linux

Linux 版本目前主要面向 x86_64 桌面发行版，推荐使用 Debian、Ubuntu 或其衍生发行版。

Linux 版本不使用 Windows 专用的 PawnIO 和 TempBridge，而是通过系统传感器接口与 `nvidia-smi` 等原生方式读取温度。

#### Debian 安装包

下载对应的 `.deb` 文件后执行：

```bash
sudo apt install ./thrm_<version>_amd64.deb
```

安装包会同时安装应用、后台核心服务、桌面入口和飞智 HID 设备的 udev 规则。

#### 便携安装包

```bash
tar -xzf THRM-linux-amd64-portable.tar.gz
cd THRM-linux-amd64

chmod +x install.sh
./install.sh

thrm
```

默认安装到：

```text
~/.local/bin/thrm
~/.local/bin/thrm-core
```

系统需要 GTK 3、WebKit2GTK 4.1、hidapi 和 udev。使用 BS1 时还需要正常运行的 BlueZ 蓝牙服务。

> [!NOTE]
> Linux 支持仍在持续完善。Windows 专属的 PawnIO、云母窗口材质、静默更新以及部分厂商硬件联动在 Linux 上不可用。

## 快速开始

1. 为散热器正常供电。
2. 根据型号完成连接：

   * BS1：保持设备蓝牙广播开启，THRM 会自动扫描。
   * BS2 / BS2 Pro / BS3 / BS3 Pro：先在系统蓝牙设置中完成配对。
3. 启动 THRM。
4. 在状态页确认设备型号、温度和风扇转速均已显示。
5. 打开“智能变频”，或在曲线页选择适合自己的控制方案。
6. 关闭主窗口后，THRM 默认继续在系统托盘后台运行。

## 配置与数据

默认配置文件：

```text
~/.thrm/config.json
```

在 Windows 中对应：

```text
%USERPROFILE%\.thrm\config.json
```

如果用户目录不可写，THRM 会尝试使用：

```text
<安装目录>/config/config.json
```

旧版本的 `.bs2pro-controller` 配置会在首次运行时自动迁移。

运行日志默认保存在安装目录的 `logs` 文件夹中。遇到问题时，更推荐直接在设置页使用“导出诊断包”，诊断包会包含必要的应用信息、配置和近期日志。

## 常见问题

### 设备无法连接

1. 确认散热器已经正常供电。
2. 确认设备型号与连接方式匹配。
3. BS2、BS2 Pro、BS3 和 BS3 Pro 需要先在系统蓝牙设置中完成配对。
4. 关闭可能占用设备的飞智官方软件或其他第三方控制工具。
5. 删除系统中原有的蓝牙配对记录，重新配对后再启动 THRM。
6. 断开设备供电，等待数秒后重新连接。

BS1 无法扫描时，可以尝试长按设备按键，重新进入蓝牙广播状态。

### CPU 或 GPU 温度显示为 0

Windows 用户可以依次尝试：

1. 在设置页重新初始化温度监控。
2. 关闭其他硬件监控或风扇控制软件。
3. 重新安装或更新 PawnIO。
4. 重启计算机。
5. 以管理员身份运行 THRM。

如果混合显卡笔记本只需要使用 CPU 温度控温，可以关闭 GPU 监测，避免独立显卡被后台轮询唤醒。

### 核心服务不可用

确认以下文件仍位于安装目录：

```text
THRM Core.exe
```

安全软件可能会隔离未签名或较新的开源程序。请确认安装包来自本仓库 Releases 页面，再检查安全软件的隔离区。

恢复文件后重新启动 THRM；问题仍然存在时，建议重新安装。

### Linux 无权访问设备

确认 udev 规则已安装：

```bash
sudo cp scripts/99-flydigi-fan.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger
```

完成后重新连接设备，必要时注销并重新登录桌面会话。

### 关闭窗口后程序仍然运行

THRM 默认最小化到系统托盘，以便继续自动控温。

如需完全退出，请在托盘图标菜单中选择“退出”。

### 如何反馈问题

提交 Issue 前，请准备：

* THRM 版本
* 操作系统及版本
* 散热器型号
* 问题复现步骤
* 设置页导出的诊断包
* 必要的截图或录屏

反馈入口：

**[GitHub Issues](https://github.com/TIANLI0/THRM/issues)**

诊断包可能包含设备和系统硬件信息。上传前请自行检查其中内容，不要公开包含隐私的信息。

<details>
<summary><strong>English overview</strong></summary>

THRM is an open-source, third-party desktop controller for Flydigi BS-series laptop coolers.

### Supported devices

* Flydigi BS1 — BLE
* Flydigi BS2 — HID
* Flydigi BS2 Pro — HID
* Flydigi BS3 — HID
* Flydigi BS3 Pro — HID

BS2, BS2 Pro, BS3 and BS3 Pro must first be paired through the operating system's Bluetooth settings.

### Highlights

* Automatic temperature-based fan control
* Editable fan curves and multiple profiles
* Fixed RPM and manual gear control
* Adaptive curve learning
* CPU/GPU power-based predictive boost
* Temperature, power and fan history
* Optional GPU monitoring disable switch
* Noise calibration and resonance avoidance
* Scheduled profile switching
* RGB lighting controls
* Tray mode, autostart and global hotkeys
* Automatic device reconnection
* In-app updates and diagnostics export
* Windows and Linux builds
* Simplified Chinese, English and Japanese interfaces

Download the latest installer or portable package from the [Releases page](https://github.com/TIANLI0/THRM/releases/latest).

THRM is an independent community project and is not affiliated with or endorsed by Flydigi.

</details>

<details>
<summary><strong>开发与构建</strong></summary>

## 技术架构

```text
┌───────────────────────────────────────────────┐
│                  THRM GUI                     │
│        Wails + Next.js + React WebView        │
└──────────────────────┬────────────────────────┘
                       │ IPC
                       ▼
┌───────────────────────────────────────────────┐
│                 THRM Core                     │
│                                               │
│  ├─ HID / BLE 设备通信                        │
│  ├─ SmartControl 智能控温                     │
│  ├─ 曲线、计划、快捷键和托盘                  │
│  ├─ 配置、日志与诊断                          │
│  └─ 温度与硬件监控                            │
└───────────────┬───────────────────┬───────────┘
                │                   │
                ▼                   ▼
       Windows TempBridge     Linux 原生传感器
       LibreHardwareMonitor   hwmon / gopsutil
       PawnIO                 nvidia-smi
```

GUI 与 Core 采用独立进程设计。关闭 GUI 后，Core 可以继续维持设备连接、温度采样和自动控温。

Windows 通过命名管道通信，Linux 使用 Unix Domain Socket。

## 技术栈

| 模块           | 技术                                      |
| ------------ | --------------------------------------- |
| 后端           | Go 1.26.5                               |
| 桌面框架         | Wails v2                                |
| 前端           | Next.js 16、React 19、TypeScript          |
| UI           | Tailwind CSS 4、Radix UI、shadcn/ui       |
| 动画与图表        | Framer Motion、Recharts                  |
| 状态管理         | Zustand                                 |
| Windows 温度桥接 | C#、.NET 8、LibreHardwareMonitor、PawnIO   |
| HID          | `github.com/sstallion/go-hid`           |
| BLE          | `tinygo.org/x/bluetooth`                |
| 系统托盘         | `fyne.io/systray`                       |
| IPC          | Windows Named Pipe / Unix Domain Socket |
| 日志           | Zap、Lumberjack                          |

## 目录结构

```text
THRM/
├── main.go                         # GUI 入口
├── app.go                          # Wails 绑定入口
├── cmd/
│   └── core/                       # 后台核心服务
├── internal/
│   ├── bridge/                     # Windows TempBridge 管理
│   ├── config/                     # 配置读写与迁移
│   ├── coreapp/                    # Core 生命周期和业务编排
│   ├── curveprofiles/              # 曲线方案管理
│   ├── device/                     # HID / BLE 设备通信
│   ├── deviceproto/                # 飞智设备协议
│   ├── guiapp/                     # GUI 对外 API
│   ├── ipc/                        # 跨进程通信
│   ├── smartcontrol/               # 智能控温算法
│   ├── temperature/                # 温度和功耗读取
│   ├── theme/                      # 主题系统
│   └── tray/                       # 系统托盘
├── bridge/
│   └── TempBridge/                 # C# 温度桥接程序
├── frontend/                       # Next.js 前端
├── scripts/                        # 安装、验证和设备脚本
├── build/
│   └── windows/installer/          # NSIS 安装程序
└── .github/workflows/              # CI、构建与发布
```

## 开发环境

### 通用依赖

* Go 1.26.5+
* Bun
* Git
* Wails CLI

安装 Wails：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Windows 额外依赖

* .NET SDK 8.0+
* go-winres
* NSIS 3.x，可选，用于生成安装程序

```bash
go install github.com/tc-hib/go-winres@latest
```

### Linux 额外依赖

Ubuntu / Debian：

```bash
sudo apt update
sudo apt install \
  build-essential \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  libhidapi-dev \
  libudev-dev
```

使用 BS1 时还需要 BlueZ。

## 本地开发

```bash
go mod download

cd frontend
bun install
cd ..

wails dev
```

## Windows 构建

首先构建 TempBridge：

```bat
build_bridge.bat
```

随后构建 THRM：

```bat
build.bat
```

主要输出：

```text
build/bin/THRM.exe
build/bin/THRM Core.exe
build/bin/bridge/THRM TempBridge.exe
build/bin/THRM-amd64-installer.exe
```

`build_bridge.bat` 会同步 LibreHardwareMonitor 源码、构建 TempBridge，并准备 PawnIO 安装程序。

## Linux 构建

```bash
chmod +x build.sh
./build.sh
```

输出：

```text
build/thrm
build/thrm-core
```

## 测试与检查

```bash
go test ./...

cd frontend
bun run build
```

Pull Request 和推送到主要分支时，GitHub Actions 会分别构建 Windows 与 Linux 版本。

推送 `v*` 或 `V*` 标签后，发布流程会自动生成：

```text
THRM-amd64-installer.exe
THRM-windows-portable.zip
THRM-linux-amd64-portable.tar.gz
thrm_<version>_amd64.deb
```

</details>

## 参与贡献

欢迎提交 Issue 和 Pull Request。

1. Fork 本仓库。
2. 从 `dev` 或目标基础分支创建功能分支。
3. 保持修改范围清晰，并补充必要测试。
4. 确认 Go 测试和前端构建通过。
5. 在 Pull Request 中说明改动目的、实现方式和验证结果。

建议分支名称：

```text
feat/feature-name
fix/problem-name
docs/readme-update
```

建议提交信息：

```text
feat: add ...
fix: resolve ...
docs: update ...
refactor: simplify ...
```

涉及新设备协议时，请避免在公开 Issue 中上传包含个人标识信息的完整蓝牙抓包；可以先联系维护者确认安全的提交方式。

## 作者与联系

* **TIANLI0**
* GitHub：[@TIANLI0](https://github.com/TIANLI0)
* Email：[wutianli@tianli0.top](mailto:wutianli@tianli0.top)
* 问题反馈：[GitHub Issues](https://github.com/TIANLI0/THRM/issues)

## 致谢

感谢以下项目及所有参与测试、反馈和贡献的社区成员：

* [Wails](https://wails.io/)
* [Next.js](https://nextjs.org/)
* [LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor)
* [PawnIO](https://pawnio.eu/)
* [项目贡献者](https://github.com/TIANLI0/THRM/graphs/contributors)
* 飞智 BS 系列设备用户与测试者

## 免责声明

THRM 是独立开发的第三方开源软件，与飞智官方不存在从属、授权或合作关系。

使用本项目提供的自定义转速、灯效、设备调试和实验性功能所造成的设备异常、数据损失或其他问题，由使用者自行承担。

请勿在不了解设备能力的情况下长期使用低于官方设计范围的转速，或尝试超出设备实际能力的参数。

## 开源许可

本项目使用 [MIT License](LICENSE) 开源。

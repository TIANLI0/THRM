# FanControlLinux — 项目 Linux 移植计划

> **目标**：基于 THRM 和 FanControlPortable 两个 Windows 参考项目，将飞智 BS 系列散热器及多品牌风扇控制功能从 Windows 移植到 Ubuntu Linux。本项目仅需支持 NVIDIA GPU，不考虑 AMD GPU 温度读取。

---

## 一、项目概况

### 1.1 源项目

两个参考项目位于 `references/` 目录下，关系如下：

- **THRM**（仓库：`github.com/TIANLI0/THRM`，许可证 MIT，版本 v3.3.1）：**原始项目**，专注控制飞智 BS1/BS2/BS2PRO/BS3/BS3PRO 系列散热器。经过多版本迭代，架构成熟，支持 RGB 灯控、智能温控、插件系统等。
- **FanControlPortable**（v2.3.2）：**基于 THRM 早期版本的 fork**，在原有飞智设备支持的基础上，扩展了 WiFi（HTTP REST API）、Serial（虚拟串口）和通用 BLE Profile 等传输层，使之能够适配更多品牌的外接风扇。

> 两者的关系是：THRM（原版）→ FanControlPortable（fork 旧版 THRM，增加多品牌支持）。本项目以 **THRM** 为核心参考代码源（架构更成熟），同时吸收 FanControlPortable 的多品牌适配方案。

### 1.2 技术栈

| 层级 | 技术 | 用途 |
|------|------|------|
| 桌面框架 | Wails v2.12.0 | Go 后端 + WebView 前端 |
| 后端语言 | Go 1.26.2 | 设备通信、温度监控、IPC |
| 前端 | Next.js 16 + React 19 + TypeScript | 用户界面 |
| 样式 | Tailwind CSS 4 + shadcn/ui (Radix) | UI 组件 |
| 图表 | Recharts 3.8 | 温度历史曲线 |
| 前端包管理 | Bun | JS 依赖管理和构建 |
| 温度桥接 | C# .NET Framework 4.7.2 + LibreHardwareMonitor | **Windows 专有，需替代** |
| HID 通信 | `github.com/sstallion/go-hid` v0.15 | USB HID（基于 hidapi） |
| BLE 通信 | `tinygo.org/x/bluetooth` v0.15 | 蓝牙 LE |
| 系统托盘 | `fyne.io/systray` v1.12 | 托盘图标 |
| 全局热键 | `golang.design/x/hotkey` v0.6 | 快捷键 |
| IPC | Windows 命名管道 / Unix 域套接字 | Core ↔ GUI 通信 |
| 日志 | `go.uber.org/zap` + `lumberjack` | 结构化日志 + 轮转 |
| 系统信息 | `shirou/gopsutil` v4 | 传感器读取 |
| 通知 | `gen2brain/beeep` | 桌面通知 |

### 1.3 架构（三进程模型）

```
┌──────────────────────────────────────────────────────────┐
│ THRM GUI (Wails/WebView)                                 │
│ ┌──────────────┐     IPC (JSON-line)      ┌────────────┐ │
│ │ React 前端   │◄────────────────────────►│ THRM Core  │ │
│ │ (Next.js)    │   Wails bindings         │ (守护进程) │ │
│ └──────────────┘                          └─────┬──────┘ │
│                                                 │        │
│                    ┌─────────────────────────────┤        │
│                    ▼                             ▼        │
│            ┌──────────────┐          ┌──────────────────┐ │
│            │ TempBridge   │          │  Device Manager   │ │
│            │ (C# .NET)    │          │  ├ HID (USB)      │ │
│            │ CPU/GPU 温度 │          │  └ BLE (蓝牙)     │ │
│            └──────────────┘          └──────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

**GUI 进程**（`thrm`）：Wails WebView 窗口，只负责 UI 渲染，通过 IPC 与 Core 通信。
**Core 进程**（`thrm-core`）：后台守护进程，管理设备通信、温度监控、风扇曲线控制、配置持久化。
**TempBridge 进程**：C# 温度采集子进程，**在 Linux 上完全移除，改为 Go 原生实现**。

### 1.4 已有跨平台基础设施

THRM 项目已通过 Go 构建标签实现了部分平台分离，**15 个 `*_other.go` 文件**为非 Windows 平台提供了桩实现：

| 已就绪的 Linux 支持 | 状态 |
|---------------------|------|
| IPC 传输层（Unix 域套接字） | ✅ 已实现 (`transport_other.go`) |
| CPU 温度读取（hwmon/thermal） | ✅ 已实现 (`cpu_linux.go`) |
| `go-hid` 库 Linux 后端（hidraw） | ✅ 库本身支持 |
| `tinygo.org/x/bluetooth` Linux 后端（BlueZ） | ✅ 库本身支持 |
| `golang.design/x/hotkey` Linux 后端（X11） | ✅ 库本身支持 |
| 进程管理 no-op | ✅ 已实现 |
| 窗口背景效果 no-op | ✅ 已实现 |
| 系统托盘 shell 等待 no-op | ✅ 已实现 |
| 温度命令执行（无 HideWindow） | ✅ 已实现 |

### 1.5 源项目目录结构

```
THRM/
├── main.go                          # GUI 入口 (Wails)
├── app.go                           # Wails 绑定表面
├── cmd/core/main.go                 # 核心服务入口
├── wails.json                       # Wails 配置
├── themes_embed.go                  # 主题嵌入
├── go.mod / go.sum                  # Go 模块
│
├── internal/
│   ├── appmeta/meta.go              # 应用元数据（名称、路径常量）
│   │                                  ⚠ .exe 后缀需改为 Linux 命名
│   ├── autostart/
│   │   ├── autostart.go             # Windows 自启动（注册表/计划任务）
│   │   └── autostart_other.go       # ❌ 空桩，需 Linux 实现 (XDG .desktop)
│   ├── bridge/
│   │   ├── bridge.go                # 温度桥接管理器（C# 子进程管理）
│   │   ├── supported.go             # 桥接支持分发
│   │   ├── supported_windows.go     # Windows：始终返回 true
│   │   ├── supported_other.go       # 非 Windows：返回 false（保持）
│   │   ├── process_other.go         # ✅ Unix Signal(0) 进程探测
│   │   └── spawn_other.go           # ✅ no-op
│   ├── config/
│   │   ├── config.go                # ✅ JSON 配置管理（跨平台）
│   │   └── fan_feature_defaults.go  # ✅ 风扇功能默认值
│   ├── coreapp/
│   │   ├── app.go                   # CoreApp 编排（子系统注入）
│   │   ├── platform_windows.go      # PawnIO 驱动 + launchGUI (Win)
│   │   ├── platform_other.go        # ❌ 返回错误，需实现 launchGUI
│   │   ├── ipc.go                   # IPC 请求处理
│   │   ├── config_control.go        # 配置变更处理
│   │   ├── monitoring*.go           # 温度/风扇监控循环
│   │   ├── system_device.go         # 系统休眠/唤醒处理
│   │   └── ...                      # 其他控制逻辑文件
│   ├── curveprofiles/               # ✅ 风扇曲线配置管理（纯逻辑）
│   ├── device/
│   │   ├── device.go                # ✅ HID 设备管理器（go-hid，跨平台）
│   │   ├── ble.go                   # ✅ BLE 管理器（需 BlueZ）
│   │   ├── rgb.go                   # ✅ RGB 灯控
│   │   ├── query.go                 # ✅ 设备设置查询
│   │   ├── debug.go                 # ✅ 调试帧捕获
│   │   └── flydigi_hid_winapi_*.go  # ❌ 不复制（Windows 原生 API）
│   ├── deviceproto/
│   │   ├── protocol.go              # ✅ 5A A5 协议帧构建/解析（纯 Go）
│   │   └── commands.go              # ✅ 命令定义和解码（纯 Go）
│   ├── deviceprofileexec/           # ⚠️ 部分复制（DIY 设备配置执行器）
│   ├── guiapp/
│   │   ├── app.go                   # ✅ GUI App 结构体
│   │   ├── app_init.go              # ✅ 初始化
│   │   ├── runtime.go               # ⚠️ 需修改 EnsureCoreServiceRunning
│   │   ├── control_api.go           # ✅ 风扇控制 API
│   │   ├── config_api.go            # ✅ 配置 API
│   │   ├── device_api.go            # ✅ 设备 API
│   │   ├── temperature_api.go       # ✅ 温度 API
│   │   ├── fan_curve_api.go         # ✅ 风扇曲线 API
│   │   ├── theme_api.go             # ✅ 主题 API（已处理 Linux xdg-open）
│   │   ├── bridge_api.go            # ⚠️ PawnIO 方法需返回"不支持"
│   │   ├── autostart_api.go         # ⚠️ 转发到 Linux 实现
│   │   ├── ipc_client.go            # ✅ IPC 客户端+事件处理
│   │   ├── window_backdrop_other.go # ✅ 返回 false
│   │   └── process_other.go         # ✅ no-op
│   ├── hotkey/
│   │   ├── manager.go               # Windows 热键（//go:build windows）
│   │   └── manager_other.go         # ❌ 仅验证语法，需激活 Linux 实现
│   ├── ipc/
│   │   ├── ipc.go                   # ✅ JSON-line RPC 协议（跨平台）
│   │   ├── transport_windows.go     # ❌ 不复制（命名管道）
│   │   └── transport_other.go       # ✅ Unix 域套接字已实现
│   ├── logger/                      # ✅ 日志（zap + lumberjack）
│   ├── notifier/                    # ✅ 桌面通知（beeep，跨平台）
│   ├── plugins/
│   │   ├── plugin.go                # ✅ 插件接口
│   │   ├── manager.go               # ✅ 插件管理器
│   │   └── fnqpowermode/
│   │       ├── plugin_other.go      # ✅ 空桩（联想 FN+Q Linux 不实现）
│   │       └── support_other.go     # ✅ 返回 false
│   ├── powernotify/                 # ❌ 仅 Windows 实现，需 Linux 实现
│   ├── smartcontrol/                # ✅ 智能温控算法（纯逻辑）
│   ├── temperature/
│   │   ├── temperature.go           # ✅ 温度读取器主逻辑（fallback 已就绪）
│   │   ├── cpu_linux.go             # ✅ CPU 温度 (hwmon/thermal zones)
│   │   ├── cpu_windows.go           # ❌ 不复制（WMIC）
│   │   ├── cpu_other.go             # ✅ 非 Linux/Windows 回退桩
│   │   ├── exec_other.go            # ✅ 命令执行（无 HideWindow）
│   │   ├── exec_windows.go          # ❌ 不复制
│   │   └── history.go               # ✅ 温度历史记录
│   ├── theme/                       # ✅ 主题管理
│   ├── tray/
│   │   ├── shell_other.go           # ✅ 托盘 shell 等待（直接返回 true）
│   │   └── ...                      # ⚠️ 需检查 icon 格式
│   ├── types/
│   │   ├── types.go                 # ✅ 所有共享数据结构
│   │   ├── speed.go                 # ✅ 速度类型
│   │   ├── device_profile.go        # ✅ 设备配置类型
│   │   ├── flydigi_profiles.go      # ✅ 飞智内置配置
│   │   ├── flydigi_capability.go    # ✅ 运行时性能上限
│   │   ├── fan_features.go          # ✅ 风扇功能类型
│   │   └── diagnostics.go           # ✅ 诊断类型
│   └── version/                     # ✅ 版本信息
│
├── frontend/                        # ✅ Next.js 前端（完全复用）
│   ├── package.json
│   ├── next.config.ts
│   └── src/
│       ├── app/                     # UI 页面和组件
│       ├── components/ui/           # shadcn/ui 组件
│       ├── locales/                 # ✅ 国际化翻译（zh-CN, en-US, ja-JP）
│       └── wailsjs/                 # Wails JS 运行时绑定
│
├── bridge/TempBridge/Program.cs     # ❌ C# 温度桥接（需完全替代）
├── themes/                          # ✅ 主题文件
├── ota/                             # ⚠️ BS2PRO 固件（按需保留）
├── docs/
│   └── bs2pro-ota-ble-commands.md   # ✅ 协议参考文档
├── scripts/                         # ⚠️ 含 HID/BLE 抓包数据（参考用）
├── build/                           # ❌ Windows 构建产物
├── build.bat                        # ❌ Windows 批处理
├── build_bridge.bat                 # ❌ Windows 桥接构建
├── BS2PRO-Controller.sln            # ❌ VS 解决方案
└── lib/                             # ❌ LibreHardwareMonitorLib.dll
```

图例：✅ 可直接复用 | ⚠️ 需小幅修改 | ❌ 需实现或替换

---

## 二、总体迁移策略

### 核心原则

1. **以 THRM 为主要参考**（比 FanControlPortable 更成熟完整）
2. **保持前端 API 兼容**——不做任何前端代码改动
3. **利用现有 `_other.go` 桩文件**，将桩替换为真正的 Linux 实现
4. **将 C# 温度桥接改为 Go 原生实现**（去除 .NET 依赖）
5. **仅支持 NVIDIA GPU**——忽略 AMD/Intel GPU 温度

### 迁移阶段概览

```
Phase 0  环境准备          (创建项目骨架 + 安装依赖)
    ↓
Phase 1  平台无关代码迁移   (protocol、types、config、frontend 等)
    ↓
Phase 2  设备通信适配       (HID/BLE + udev 规则)
    ↓
Phase 3  温度监控替代       (替代 C# 桥接 + Go 原生实现)
    ↓
Phase 4  桌面集成           (IPC、托盘、自启动、热键、电源通知)
    ↓
Phase 5  组装与构建         (Core + GUI 组装、构建脚本、打包)
```

---

## 三、各阶段详细任务

---

## Phase 0 — 环境准备

**目标**：创建项目骨架，安装所有编译和运行依赖。

### 任务 0.1：创建 Go 模块

- 创建项目目录 `FanControlLinux/`
- 初始化 Go 模块：`go mod init github.com/<your-org>/FanControlLinux`
- Go 版本要求：Go 1.23+

### 任务 0.2：创建项目目录结构

```
FanControlLinux/
├── cmd/core/                        # 核心服务入口
├── internal/
│   ├── appmeta/
│   ├── autostart/
│   ├── bridge/
│   ├── config/
│   ├── coreapp/
│   ├── curveprofiles/
│   ├── device/
│   ├── deviceproto/
│   ├── deviceprofileexec/
│   ├── guiapp/
│   ├── hotkey/
│   ├── ipc/
│   ├── logger/
│   ├── notifier/
│   ├── plugins/
│   │   └── fnqpowermode/
│   ├── powernotify/
│   ├── smartcontrol/
│   ├── temperature/
│   ├── theme/
│   ├── tray/
│   ├── types/
│   └── version/
├── frontend/                        # 从 THRM 复制
├── themes/                          # 从 THRM 复制
├── docs/
└── build/                           # 构建输出
```

### 任务 0.3：安装系统依赖

```bash
# 编译依赖
sudo apt install build-essential gcc pkg-config

# HID 设备访问 (go-hid 的 hidapi 后端)
sudo apt install libhidapi-dev

# 蓝牙 BLE (BlueZ)
sudo apt install bluez

# Wails v2 Linux 前端 (WebKit2GTK)
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

# Node.js / Bun（用于构建前端）
curl -fsSL https://bun.sh/install | bash

# NVIDIA GPU 温度读取（nvidia-smi 随驱动安装）
nvidia-smi --query-gpu=temperature.gpu --format=csv,noheader
```

---

## Phase 1 — 平台无关代码迁移

**目标**：将所有无需修改的代码直接复制到新项目，验证编译通过。

### 任务 1.1：通信协议层

| 源文件 | 目标位置 | 备注 |
|--------|---------|------|
| `THRM/internal/deviceproto/protocol.go` | `internal/deviceproto/` | 5A A5 帧构建/解析 |
| `THRM/internal/deviceproto/commands.go` | `internal/deviceproto/` | 命令定义和解码 |

约 420 行纯 Go 代码，**零修改**。

### 任务 1.2：类型定义

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/types/types.go` | `internal/types/` |
| `THRM/internal/types/speed.go` | `internal/types/` |
| `THRM/internal/types/device_profile.go` | `internal/types/` |
| `THRM/internal/types/flydigi_profiles.go` | `internal/types/` |
| `THRM/internal/types/flydigi_capability.go` | `internal/types/` |
| `THRM/internal/types/diagnostics.go` | `internal/types/` |
| `THRM/internal/types/fan_features.go` | `internal/types/` |

### 任务 1.3：配置管理

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/config/config.go` | `internal/config/` |
| `THRM/internal/config/fan_feature_defaults.go` | `internal/config/` |

配置路径使用 `os.UserHomeDir()`，Linux 下自动解析为 `~/.thrm/config.json`。**零修改**。

### 任务 1.4：前端

- 将 `THRM/frontend/` 完整复制到新项目
- 运行 `bun install && bun run build` 生成静态文件到 `frontend/dist/`
- 在 `main.go` 中通过 `//go:embed all:frontend/dist` 嵌入
- **零修改**——前端是静态 Web 内容，完全平台无关

### 任务 1.5：主题系统

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/theme/` 全部文件 | `internal/theme/` |
| `THRM/themes/` 目录 | `themes/` |
| `THRM/themes_embed.go` | 项目根目录 |

### 任务 1.6：日志系统

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/logger/` 全部文件 | `internal/logger/` |

zap + lumberjack 均为纯 Go 跨平台库。

### 任务 1.7：风扇曲线和智能控制

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/curveprofiles/` 全部文件 | `internal/curveprofiles/` |
| `THRM/internal/smartcontrol/` 全部文件 | `internal/smartcontrol/` |

纯算法逻辑，无任何平台依赖。

### 任务 1.8：插件系统

| 源文件 | 目标位置 | 备注 |
|--------|---------|------|
| `THRM/internal/plugins/plugin.go` | `internal/plugins/` | 插件接口 |
| `THRM/internal/plugins/manager.go` | `internal/plugins/` | 插件生命周期管理 |
| `THRM/internal/plugins/fnqpowermode/plugin_other.go` | `internal/plugins/fnqpowermode/` | 联想 FN+Q 空桩 |
| `THRM/internal/plugins/fnqpowermode/support_other.go` | `internal/plugins/fnqpowermode/` | 返回 false |

**不复制** `*_windows.go` 文件。

### 任务 1.9：桌面通知

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/notifier/` 全部文件 | `internal/notifier/` |

beeep 在 Linux 上通过 D-Bus 通知工作，**直接可用**。

### 任务 1.10：版本信息和其他

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/version/` 全部文件 | `internal/version/` |
| `THRM/internal/device/runtime_profile.go` (按需) | `internal/device/` |
| `THRM/internal/device/wifi.go` (按需) | `internal/device/` |
| `THRM/internal/device/wifi_discovery.go` (按需) | `internal/device/` |
| `THRM/internal/device/manager_wifi.go` (按需) | `internal/device/` |

### 任务 1.11：更新 go.mod 依赖

从 THRM 的 `go.mod` 提取并添加以下**跨平台依赖**：

```
require (
    github.com/sstallion/go-hid v0.15.0
    tinygo.org/x/bluetooth v0.15.0
    fyne.io/systray v1.12.2
    golang.design/x/hotkey v0.6.1
    github.com/shirou/gopsutil/v4 v4.26.5
    go.uber.org/zap v1.28.0
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
    github.com/gen2brain/beeep v0.11.2
    github.com/wailsapp/wails/v2 v2.12.0
)
```

**必须排除的 Windows 专有依赖**（不让 go mod 引入）：

| 排除的依赖 | 原因 |
|-----------|------|
| `github.com/Microsoft/go-winio` | Windows 命名管道（仅 `transport_windows.go` 使用） |
| `github.com/wailsapp/go-webview2` | Windows WebView2（Linux 用 WebKit2GTK） |
| `github.com/jchv/go-winloader` | Windows PE 加载器 |
| `github.com/saltosystems/winrt-go` | Windows Runtime 绑定 |
| `github.com/go-ole/go-ole` | Windows OLE/COM |
| `github.com/yusufpapurcu/wmi` | Windows WMI |

---

## Phase 2 — 设备通信适配

**目标**：使 HID（USB）和 BLE（蓝牙）通信在 Linux 上正常工作。这是最早可验证功能的时间点——连接到设备后即可测试基本控制。

### 任务 2.1：HID 设备通信层

**源文件参考**：`THRM/internal/device/device.go`（约 960 行）

**Go 代码层面无需修改**。`github.com/sstallion/go-hid` 库在 Linux 上使用 hidraw 后端，API 完全一致：

- `hid.OpenFirst(vid, pid)` — 打开设备
- `device.SetNonblock(true)` — 设置非阻塞模式
- `device.ReadWithTimeout(buf, 500*time.Millisecond)` — 带超时读取
- `device.Write(buf)` — 写入 HID 报告

**需要复制的文件**：

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/device/device.go` | `internal/device/` |
| `THRM/internal/device/query.go` | `internal/device/` |
| `THRM/internal/device/rgb.go` | `internal/device/` |
| `THRM/internal/device/debug.go` | `internal/device/` |
| `THRM/internal/device/debug_buffer.go` | `internal/device/` |
| `THRM/internal/device/send_retry.go` | `internal/device/` |
| `THRM/internal/device/auto_native.go` | `internal/device/` |
| `THRM/internal/device/auto_native_legacy.go` | `internal/device/` |

**不复制**：

| 文件 | 原因 |
|------|------|
| `THRM/internal/device/flydigi_hid_winapi_windows.go` | Windows 原生 HID API (`kernel32.dll`) |
| `THRM/internal/device/flydigi_hid_windows.go` | Windows HID 路径 |
| `THRM/internal/device/flydigi_hid_hidapi_windows.go` | 如果存在，使用 Windows 特定 HIDAPI 模式 |

**潜在问题**：

1. **HID 报告长度**：`hidControlReportLen = 23` 常量在 Linux 下可能不同，需要通过实际测试验证
2. **写入回退策略**：Windows 版有 `Write()` → `SendOutputReport()` → padded write 的回退链，Linux 下可能不需要

### 任务 2.2：创建 udev 规则

Linux 默认不允许普通用户访问 HID 设备。需创建 udev 规则授权飞智设备访问。

**文件**：`scripts/99-flydigi-fan.rules`

```udev
# 飞智 BS2 / BS2PRO / BS3 / BS3PRO 散热器
# VID: 0x37D7

# USB 设备权限
SUBSYSTEM=="usb", ATTRS{idVendor}=="37d7", ATTRS{idProduct}=="1001", MODE="0666"
SUBSYSTEM=="usb", ATTRS{idVendor}=="37d7", ATTRS{idProduct}=="1002", MODE="0666"
SUBSYSTEM=="usb", ATTRS{idVendor}=="37d7", ATTRS{idProduct}=="1003", MODE="0666"
SUBSYSTEM=="usb", ATTRS{idVendor}=="37d7", ATTRS{idProduct}=="1004", MODE="0666"

# hidraw 设备权限
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="37d7", MODE="0666"
```

安装方法：
```bash
sudo cp scripts/99-flydigi-fan.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger
```

### 任务 2.3：BLE 蓝牙通信

**源文件参考**：`THRM/internal/device/ble.go`（约 516 行）

**Go 代码层面无需修改**。`tinygo.org/x/bluetooth` 库在 Linux 上通过 BlueZ D-Bus 工作：

- `adapter.Enable()` — 启用蓝牙适配器
- `adapter.Scan(callback)` — 扫描设备
- `device.DiscoverServices(uuids)` — 发现 GATT 服务
- `characteristic.Write(p)` / `WriteWithoutResponse(p)` — 写入特征值
- `characteristic.EnableNotifications(callback)` — 启用通知

**需要复制的文件**（除 `device.go` 已包含的都复制）：

| 源文件 | 目标位置 |
|--------|---------|
| `THRM/internal/device/ble.go` | `internal/device/` |

**系统要求**：
- BlueZ ≥ 5.48（`bluetoothctl --version` 检查）
- 用户需在 `bluetooth` 组中或配置 polkit 策略

**备选方案**：如果 `tinygo.org/x/bluetooth` 在标准 Go（非 TinyGo）编译出现问题，可换用 `github.com/go-ble/ble` 或直接使用 BlueZ D-Bus API（`github.com/godbus/dbus`）。

---

## Phase 3 — 温度监控替代

**目标**：完全移除 C# 温度桥接，用 Go 原生实现温度读取。这是迁移中**逻辑最复杂的部分**。

### 设计决策

THRM 的温度读取有两级回退：

```
bridgeManager.IsSupported() == true
  → 调用外部 C# TempBridge.exe (stdin/stdout JSON 协议)
    → 失败或返回空 → 回退到内置 Go 读取

bridgeManager.IsSupported() == false
  → 直接使用内置 Go 读取
      CPU: gopsutil → readPlatformCPUTemp() → (platform specific)
      GPU: nvidia-smi
```

**Linux 策略**：保持 `isBridgeSupported()` 返回 `false`（`supported_other.go` 不改），让温度读取直接走 fallback 路径。这样完全避免了外部子进程的复杂性。

### 任务 3.1：CPU 温度 — 已有完整实现

**源文件**：`THRM/internal/temperature/cpu_linux.go`（约 148 行）

已实现功能：
- 扫描 `/sys/class/hwmon/hwmon*/`，过滤 `coretemp`、`k10temp`、`zenpower` 驱动
- 读取 `temp*_input` 文件（milli-Celsius → Celsius 自动转换）
- 标签优先级评分算法：`package`(100) > `tdie`(90) > `tctl`(80) > `cpu`(70) > `core`(60)
- 回退到 `/sys/class/thermal/thermal_zone*/`，按类型评分：`x86_pkg_temp`(100) > `pkg`(90) > `cpu`(80)

**零修改，直接复制使用。**

### 任务 3.2：NVIDIA GPU 温度 — 已有实现

**源自**：`THRM/internal/temperature/temperature.go`

项目已实现 `readNvidiaGPUTemp()` 函数：

```go
func readNvidiaGPUTemp() (float64, error) {
    output, err := execHelperCommand(helperCommandTimeout, "nvidia-smi",
        "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
    // 解析返回的数值
}
```

`nvidia-smi` 在 Linux 上完全相同，**无需修改**。

### 任务 3.3：温度采集层文件清单

| 源文件 | 目标位置 | 改动 |
|--------|---------|------|
| `THRM/internal/temperature/temperature.go` | `internal/temperature/` | 无需改动——fallback 路径已处理非 Windows 场景 |
| `THRM/internal/temperature/cpu_linux.go` | `internal/temperature/` | **无需改动** |
| `THRM/internal/temperature/cpu_windows.go` | **不复制** | Windows WMI 专有 |
| `THRM/internal/temperature/cpu_other.go` | `internal/temperature/` | 保留作为非 Linux 回退 |
| `THRM/internal/temperature/exec_other.go` | `internal/temperature/` | Linux 使用此版本（无 HideWindow） |
| `THRM/internal/temperature/exec_windows.go` | **不复制** | |
| `THRM/internal/temperature/history.go` | `internal/temperature/` | 温度历史记录（纯 Go） |

### 任务 3.4：Bridge 支撑文件（保持桩）

| 源文件 | 目标位置 | 改动 |
|--------|---------|------|
| `THRM/internal/bridge/bridge.go` | `internal/bridge/` | 复制——子进程逻辑在 Linux 上不会被执行 |
| `THRM/internal/bridge/supported.go` | `internal/bridge/` | 无需改动 |
| `THRM/internal/bridge/supported_windows.go` | **不复制** | |
| `THRM/internal/bridge/supported_other.go` | `internal/bridge/` | **保持返回 false**——触发 fallback 路径 |
| `THRM/internal/bridge/process_other.go` | `internal/bridge/` | 已实现 Unix `Signal(0)` 探测 |
| `THRM/internal/bridge/spawn_other.go` | `internal/bridge/` | 已实现 no-op |

### Linux 温度读取架构总结

```
Read()
  ├─ bridgeManager.IsSupported() → false
  │   └─ readCPUTemperature()
  │       ├─ gopsutil.SensorsTemperatures() → 尝试
  │       └─ readPlatformCPUTemp()
  │           └─ (linux) → /sys/class/hwmon/hwmon*/temp*_input
  │                      → /sys/class/thermal/thermal_zone*/temp
  └─ readGPUTemperature()
      └─ detectGPUVendor() → nvidia-smi --version
          └─ readNvidiaGPUTemp() → nvidia-smi --query-gpu=temperature.gpu
```

---

## Phase 4 — 桌面集成

**目标**：使应用在 Linux 桌面上正常交互。需实现或激活以下功能。

### 任务 4.1：IPC 传输层

**现状**：`transport_other.go` 已完整实现 Unix 域套接字。

| 文件 | 目标位置 | 改动 |
|------|---------|------|
| `THRM/internal/ipc/ipc.go` | `internal/ipc/` | 复制——JSON-line 协议层 |
| `THRM/internal/ipc/transport_other.go` | `internal/ipc/` | 复制——Unix 域套接字 (`/tmp/THRM-IPC.sock`) |
| `THRM/internal/ipc/transport_windows.go` | **不复制** | |

**验证点**：
- 套接字路径为 `/tmp/THRM-IPC.sock`，权限 `0600`
- `CheckCoreServiceRunning()` 尝试拨号到套接字
- `ipc.go` 中硬编码的 `PipePath = "\\.\pipe\THRM-IPC"` 在 Linux 构建中不会被使用（`transport_other.go` 使用 `ipcEndpointFromName()` 而非 `PipePath`）

### 任务 4.2：系统托盘

**源文件**：

| 文件 | 目标位置 | 改动 |
|------|---------|------|
| `THRM/internal/tray/` 中非 Windows 文件 | `internal/tray/` | |
| `THRM/internal/tray/shell_other.go` | `internal/tray/` | ✅ 已正确实现（直接返回 true） |

**注意事项**：

1. **图标格式**：THRM 嵌入的是 `icon.ico`（Windows 格式）。Linux 需要 PNG 格式的图标。
   - 准备一个 256x256 的 PNG 图标
   - 修改 `cmd/core/main.go` 中的 embed 路径：`//go:embed icon.png`
   - 修改 `main.go` 中的 embed（如果 GUI 也需要）

2. `fyne.io/systray` 在 Linux 上通过 AppIndicator 或 XEmbed 工作，通常无需额外配置。

### 任务 4.3：自启动（需完整实现）

**现状**：`THRM/internal/autostart/autostart_other.go` 是完全的空桩，所有方法返回零值。

**需新建**：`internal/autostart/autostart_linux.go`（`//go:build linux`）

**实现方案**——XDG Autostart 规范：

需实现的接口（参考 `autostart_other.go` 中的方法签名）：

```go
//go:build linux

package autostart

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "github.com/TIANLI0/THRM/internal/types"
)

type Manager struct {
    logger types.Logger
}

func NewManager(logger types.Logger) *Manager {
    return &Manager{logger: logger}
}

// 自启动 .desktop 文件路径
func autostartDesktopPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "autostart", "thrm.desktop")
}

// IsRunningAsAdmin Linux 下检查是否为 root
func (m *Manager) IsRunningAsAdmin() bool {
    return os.Geteuid() == 0
}

// SetWindowsAutoStart 创建或删除 XDG autostart 文件
func (m *Manager) SetWindowsAutoStart(enable bool) error {
    return m.SetAutoStartWithMethod(enable, "desktop")
}

// GetAutoStartMethod 返回当前的启动方式
func (m *Manager) GetAutoStartMethod() string {
    if _, err := os.Stat(autostartDesktopPath()); err == nil {
        return "desktop"
    }
    return "none"
}

// SetAutoStartWithMethod 设置自启动
func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
    desktopPath := autostartDesktopPath()
    if enable {
        // 确保目录存在
        os.MkdirAll(filepath.Dir(desktopPath), 0755)
        // 找到当前可执行文件路径
        exePath, _ := os.Executable()
        content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=THRM Fan Control
Comment=Flydigi BS Series Fan Controller
Exec=%s --autostart
Terminal=false
Hidden=false
X-GNOME-Autostart-enabled=true
`, exePath)
        return os.WriteFile(desktopPath, []byte(content), 0644)
    }
    // 删除文件或标记为 Hidden
    os.Remove(desktopPath)
    return nil
}

// CheckWindowsAutoStart 检查自启动是否已启用
func (m *Manager) CheckWindowsAutoStart() bool {
    _, err := os.Stat(autostartDesktopPath())
    return err == nil
}

// DetectAutoStartLaunch 检查是否由自启动触发
func DetectAutoStartLaunch(args []string) bool {
    for _, arg := range args {
        if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
            return true
        }
    }
    return false
}
```

**注意**：前端 API 方法名包含 "Windows"（如 `SetWindowsAutoStart`），但为保持前端兼容性，**不要改名**——在 Linux 实现中保留相同的方法名，内部执行 XDG autostart 逻辑。

### 任务 4.4：全局热键（需激活实现）

**现状**：`THRM/internal/hotkey/manager_other.go` 只验证快捷键字符串语法，不实际注册热键。

**需新建**：`internal/hotkey/manager_linux.go`（`//go:build linux`）

**关键依赖**：`golang.design/x/hotkey` v0.6.1 在 Linux 上通过 X11 `XGrabKey` 实现。

**重要限制**：
- **仅支持 X11**，Wayland 协议不支持全局热键
- 需在运行时检测 `$XDG_SESSION_TYPE`，若为 `wayland` 则优雅降级

**实现参考**：

```go
//go:build linux

package hotkey

import (
    "fmt"
    "os"
    "strings"
    "sync"
    "github.com/TIANLI0/THRM/internal/types"
    hotkeylib "golang.design/x/hotkey"
)

type Action string

const (
    ActionToggleManualGear   Action = "toggle-manual-gear"
    ActionToggleAutoMode     Action = "toggle-auto-control"
    ActionToggleCurveProfile Action = "toggle-curve-profile"
)

type hotkeyBinding struct {
    hk       *hotkeylib.Hotkey
    action   Action
    shortcut string
    stopCh   chan struct{}
}

type Manager struct {
    logger    types.Logger
    onAction  func(action Action, shortcut string)
    mutex     sync.Mutex
    bindings  []*hotkeyBinding
    closed    bool
    isWayland bool
}

func NewManager(logger types.Logger, onAction func(action Action, shortcut string)) *Manager {
    isWayland := os.Getenv("XDG_SESSION_TYPE") == "wayland" ||
        os.Getenv("WAYLAND_DISPLAY") != ""
    return &Manager{
        logger:    logger,
        onAction:  onAction,
        isWayland: isWayland,
    }
}

func (m *Manager) UpdateBindings(manualGearShortcut, autoControlShortcut, curveProfileShortcut string) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    if m.closed {
        return fmt.Errorf("hotkey manager already stopped")
    }

    // 取消所有旧绑定
    for _, b := range m.bindings {
        b.hk.Unregister()
        close(b.stopCh)
    }
    m.bindings = nil

    // 在 Wayland 下只做验证，不实际注册
    if m.isWayland {
        m.logger.Info("Wayland detected: hotkey registration skipped (not supported)")
        // 仍验证语法
        actions := []struct{ shortcut, label string }{
            {manualGearShortcut, "manual gear"},
            {autoControlShortcut, "auto control"},
            {curveProfileShortcut, "curve profile"},
        }
        for _, a := range actions {
            if a.shortcut != "" {
                if _, _, err := ParseShortcut(a.shortcut); err != nil {
                    return fmt.Errorf("invalid %s shortcut: %w", a.label, err)
                }
            }
        }
        return nil
    }

    // X11 下注册热键
    actions := []struct {
        shortcut string
        action   Action
    }{
        {manualGearShortcut, ActionToggleManualGear},
        {autoControlShortcut, ActionToggleAutoMode},
        {curveProfileShortcut, ActionToggleCurveProfile},
    }

    for _, a := range actions {
        if a.shortcut == "" {
            continue
        }
        mods, key, err := ParseShortcut(a.shortcut)
        if err != nil {
            return fmt.Errorf("invalid shortcut %q: %w", a.shortcut, err)
        }

        hk := hotkeylib.New(mods, key)
        if err := hk.Register(); err != nil {
            return fmt.Errorf("failed to register hotkey %q: %w", a.shortcut, err)
        }

        binding := &hotkeyBinding{
            hk:       hk,
            action:   a.action,
            shortcut: a.shortcut,
            stopCh:   make(chan struct{}),
        }

        go func(b *hotkeyBinding) {
            for {
                select {
                case <-b.hk.Keydown():
                    m.onAction(b.action, b.shortcut)
                case <-b.stopCh:
                    return
                }
            }
        }(binding)

        m.bindings = append(m.bindings, binding)
    }

    return nil
}

func (m *Manager) Stop() {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    m.closed = true
    for _, b := range m.bindings {
        b.hk.Unregister()
        close(b.stopCh)
    }
    m.bindings = nil
}
```

**ParseShortcut** 函数：直接从 Windows 版 `manager.go` 复制 `ParseShortcut`、`normalizeShortcut`、`parseModifier`、`parseKey` 的逻辑，这些是纯字符串处理，平台无关。

### 任务 4.5：电源事件通知（需新实现）

**现状**：THRM 的 `powernotify` 包仅在 Windows 上实现（`RegisterPowerSettingNotification`）。

**需新建**：`internal/powernotify/powernotify_linux.go`（`//go:build linux`）

**实现方案**——通过 D-Bus 监听 systemd-logind：

```go
//go:build linux

package powernotify

import (
    "github.com/godbus/dbus/v5"
    "go.uber.org/zap"
)

type Manager struct {
    logger    *zap.SugaredLogger
    conn      *dbus.Conn
    onSleep   func()
    onResume  func()
    stopCh    chan struct{}
}

func NewManager(logger *zap.SugaredLogger, onSleep, onResume func()) (*Manager, error) {
    conn, err := dbus.SystemBus()
    if err != nil {
        return nil, err
    }

    // 监听 PrepareForSleep 信号
    err = conn.AddMatchSignal(
        dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
        dbus.WithMatchMember("PrepareForSleep"),
        dbus.WithMatchObjectPath("/org/freedesktop/login1"),
    )
    if err != nil {
        conn.Close()
        return nil, err
    }

    m := &Manager{
        logger:   logger,
        conn:     conn,
        onSleep:  onSleep,
        onResume: onResume,
        stopCh:   make(chan struct{}),
    }
    go m.listen()
    return m, nil
}

func (m *Manager) listen() {
    ch := make(chan *dbus.Signal, 10)
    m.conn.Signal(ch)
    for {
        select {
        case sig := <-ch:
            if len(sig.Body) >= 1 {
                if starting, ok := sig.Body[0].(bool); ok {
                    if starting {
                        m.logger.Info("System preparing for sleep")
                        m.onSleep()
                    } else {
                        m.logger.Info("System resumed from sleep")
                        m.onResume()
                    }
                }
            }
        case <-m.stopCh:
            return
        }
    }
}

func (m *Manager) Stop() {
    close(m.stopCh)
    m.conn.Close()
}
```

注意：`godbus/dbus/v5` 是 `beeep` 的间接依赖，已在 `go.mod` 中间接引入。

### 任务 4.6：桌面通知

`notifier` 包使用 `gen2brain/beeep`，通过 D-Bus 发送桌面通知。THRM 的 `notifier/` 代码直接复制即可。**无需修改**。

---

## Phase 5 — 组装与构建系统

**目标**：将所有子系统连接起来，创建可运行的 Linux 二进制文件。

### 任务 5.1：核心服务入口

**参考**：`THRM/cmd/core/main.go`

**改动**：

1. **图标格式**：将 `//go:embed icon.ico` 改为 `//go:embed icon.png`（准备一个 PNG 图标）
2. **崩溃日志**：需新建 `cmd/core/fatal_log_linux.go`

```go
//go:build linux

package main

import (
    "os"
    "path/filepath"
    "time"
)

func setupFatalOutput() (func(), string) {
    // 获取可执行文件目录
    exePath, _ := os.Executable()
    logDir := filepath.Join(filepath.Dir(exePath), "logs")
    os.MkdirAll(logDir, 0755)

    logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+"-core.log")
    f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return func() {}, ""
    }

    // 重定向 stderr 和 stdout
    oldStderr := os.Stderr
    oldStdout := os.Stdout
    os.Stderr = f
    os.Stdout = f

    return func() {
        os.Stderr = oldStderr
        os.Stdout = oldStdout
        f.Close()
    }, logFile
}
```

3. **信号处理**：`syscall.SIGINT, syscall.SIGTERM` 在 Linux 上不变，无需修改。

### 任务 5.2：应用元数据文件拆分

**现状**：`THRM/internal/appmeta/meta.go` 中包含大量 Windows 专有常量（`.exe` 后缀、`PawnIO` 路径等）。

**方案**：拆分为三个文件：

| 文件 | 构建标签 | 内容 |
|------|---------|------|
| `internal/appmeta/meta.go` | *(无)* | **共享常量**：`AppName`、`IPCPipeName`、`ConfigDirName`、`ProtocolVersion`、`RepositoryURL`、`UserConfigDir()`、`FirstExistingPath()` |
| `internal/appmeta/meta_windows.go` | `windows` | `.exe` 后缀的 `ExecutableName` 等，Windows 专有的候选路径函数 |
| `internal/appmeta/meta_linux.go` | `linux` | Linux 命名和路径 |

**`meta_linux.go` 需包含的常量**：

```go
//go:build linux

package appmeta

import (
    "os"
    "path/filepath"
)

const (
    ExecutableName       = "thrm"
    LegacyExecutableName = "bs2pro-controller"
    CoreName             = "THRM Core"
    CoreExecutableName   = "thrm-core"
    LegacyCoreExecutable = "bs2pro-core"
    // Bridge 不使用外部进程
    BridgeName           = ""
    BridgeExecutableName = ""
    // PawnIO 不存在于 Linux
    PawnIOInstallerName  = ""
)

func CoreExecutableCandidates(baseDir string) []string {
    // 搜索可能的 core 二进制位置
    candidates := []string{
        filepath.Join(baseDir, CoreExecutableName),
        filepath.Join(baseDir, LegacyCoreExecutable),
    }
    // 也在同目录和 ../core/ 下搜索
    if exePath, err := os.Executable(); err == nil {
        exeDir := filepath.Dir(exePath)
        candidates = append(candidates,
            filepath.Join(exeDir, CoreExecutableName),
            filepath.Join(exeDir, LegacyCoreExecutable),
            filepath.Join(exeDir, "..", "core", CoreExecutableName),
            filepath.Join(exeDir, "..", "core", LegacyCoreExecutable),
        )
    }
    return candidates
}

func GUIExecutableCandidates(baseDir string) []string { ... }
func BridgeExecutableCandidates(baseDir string) []string { return nil }
func PawnIOInstallerPath(baseDir string) string { return "" }
func PawnIOInstallerCandidates(baseDir string) []string { return nil }
```

**`meta.go` 保留的共享常量**：

```go
const (
    AppName              = "THRM"
    LegacyAppName        = "BS2PRO Controller"
    IPCPipeName          = "THRM-IPC"
    LegacyIPCPipeName    = "BS2PRO-Controller-IPC"
    BridgePipeName       = "THRM_TempBridge"
    LegacyBridgePipeName = "BS2PRO_TempBridge"
    ConfigDirName        = ".thrm"
    LegacyConfigDirName  = ".bs2pro-controller"
    NotificationCacheDir = "THRM"
    LegacyNotifyCacheDir = "BS2PRO-Controller"
    ProtocolVersion      = "3.0"
    RepositoryURL        = "https://github.com/TIANLI0/THRM"
    LatestReleaseURL     = RepositoryURL + "/releases/latest"
)

func IPCPipeCandidates() []string {
    return []string{IPCPipeName, LegacyIPCPipeName}
}

func FirstExistingPath(paths []string) string { ... }
func UserConfigDir(homeDir string) string { ... }
func LegacyUserConfigDir(homeDir string) string { ... }
```

### 任务 5.3：CoreApp 平台文件

**需新建**：`internal/coreapp/platform_linux.go`（替代 `platform_other.go`）

```go
//go:build linux

package coreapp

import (
    "os"
    "os/exec"
    "github.com/TIANLI0/THRM/internal/appmeta"
)

func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
    return map[string]any{
        "success": false,
        "message": "PawnIO is not supported on Linux",
    }, nil
}

func launchGUI() error {
    candidates := appmeta.GUIExecutableCandidates("")
    for _, path := range candidates {
        if _, err := os.Stat(path); err == nil {
            cmd := exec.Command(path)
            return cmd.Start()
        }
    }
    // 如果按名称找不到，尝试在 PATH 中搜索
    if path, err := exec.LookPath(appmeta.ExecutableName); err == nil {
        cmd := exec.Command(path)
        return cmd.Start()
    }
    return fmt.Errorf("GUI executable not found")
}
```

### 任务 5.4：GUI 入口

**参考**：`THRM/main.go`

**改动**：

1. `guiapp.EnsureCoreServiceRunning()` — 修改为在 Linux 上查找并启动 `thrm-core`
2. `guiapp.DefaultFrameless()` — 在非 Windows 上已返回 `false`（OK）
3. `guiapp.ResolveWindowsOptions()` — 返回空 Windows options
4. `guiapp.OnSecondInstanceLaunch` — 需要调用 Wails 运行时的窗口显示方法

**需要检查的 `runtime.go` 中的 `EnsureCoreServiceRunning()`**：

```go
func EnsureCoreServiceRunning() bool {
    // 如果是 Wails bind 生成模式，跳过
    if strings.HasPrefix(os.Args[0], os.TempDir()) {
        return true
    }

    // 检查 Core 是否已在运行
    if ipc.CheckCoreServiceRunning() {
        return true
    }

    // 查找 Core 二进制
    exePath, err := os.Executable()
    if err != nil {
        return false
    }
    exeDir := filepath.Dir(exePath)

    for _, name := range appmeta.CoreExecutableCandidates(exeDir) {
        if _, err := os.Stat(name); err == nil {
            cmd := exec.Command(name)
            configureCoreCommand(cmd) // Linux 上是 no-op
            if err := cmd.Start(); err != nil {
                continue
            }
            // 等待 Core 就绪
            for i := 0; i < 100; i++ {
                if ipc.CheckCoreServiceRunning() {
                    return true
                }
                time.Sleep(100 * time.Millisecond)
            }
            return false
        }
    }
    return false
}
```

### 任务 5.5：Wails 配置文件

**文件**：`wails.json`

```json
{
    "name": "thrm",
    "outputfilename": "thrm",
    "frontend:install": "bun install",
    "frontend:build": "bun run build",
    "frontend:dev:watcher": "bun run dev",
    "frontend:dev:serverUrl": "http://localhost:5173",
    "author": {
        "name": "",
        "email": ""
    },
    "info": {
        "productVersion": "4.0.0-linux",
        "companyName": "",
        "productName": "THRM Fan Control",
        "comments": "Linux port of THRM"
    }
}
```

**移除**：`"nsisType": "multiple"`（NSIS 是 Windows 安装包工具）。

### 任务 5.6：构建脚本

**需新建**：`build.sh`

```bash
#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

echo "=== Building FanControlLinux ==="

# 1. 构建前端
echo "--- Building frontend ---"
if [ ! -d "frontend/dist" ]; then
    cd frontend
    bun install
    bun run build
    cd "$PROJECT_ROOT"
fi

# 2. 构建核心服务 (需要 cgo for go-hid)
echo "--- Building core service ---"
CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -o "$BUILD_DIR/thrm-core" \
    ./cmd/core/

# 3. 构建 GUI (需要 cgo for Wails/WebKit2GTK)
echo "--- Building GUI ---"
CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -o "$BUILD_DIR/thrm" \
    .

echo "=== Build complete ==="
echo "Binaries:"
ls -lh "$BUILD_DIR/thrm" "$BUILD_DIR/thrm-core"
```

### 任务 5.7：安装脚本

**需新建**：`scripts/install.sh`

```bash
#!/bin/bash
set -e

INSTALL_DIR="${1:-$HOME/.local/bin}"
DESKTOP_DIR="$HOME/.local/share/applications"
UDEV_RULES_DIR="/etc/udev/rules.d"

echo "Installing FanControlLinux..."

# 安装二进制文件
install -Dm755 build/thrm "$INSTALL_DIR/thrm"
install -Dm755 build/thrm-core "$INSTALL_DIR/thrm-core"

# 创建 .desktop 文件
mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/thrm.desktop" << 'EOF'
[Desktop Entry]
Type=Application
Name=THRM Fan Control
Comment=Flydigi BS Series Fan Controller
Exec=thrm
Terminal=false
Categories=Utility;
EOF

# 安装 udev 规则（需 sudo）
echo "Installing udev rules (requires sudo)..."
sudo cp scripts/99-flydigi-fan.rules "$UDEV_RULES_DIR/"
sudo udevadm control --reload-rules
sudo udevadm trigger

echo "Installation complete."
echo "You can now run 'thrm' from your application menu or terminal."
```

---

## 四、设备通信协议参考

### 飞智 5A A5 协议概要

```
HID 帧格式（USB）：
  字节: [ReportID: 0x02] [Magic: 0x5A 0xA5] [Command] [Length] [Payload...] [Checksum]

BLE 帧格式（蓝牙，无 ReportID 前缀）：
  字节: [Magic: 0x5A 0xA5] [Command] [Length] [Payload...] [Checksum]

校验和 = (Command + Length + sum(Payload)) & 0xFF
```

**飞智设备 VID/PID**：

| 设备 | VID | PID |
|------|-----|-----|
| BS2 | 0x37D7 | 0x1001 |
| BS2 PRO | 0x37D7 | 0x1002 |
| BS3 | 0x37D7 | 0x1003 |
| BS3 PRO | 0x37D7 | 0x1004 |

**BS1（蓝牙）GATT UUID**：

| 属性 | UUID |
|------|------|
| Service | `0000FFF0-0000-1000-8000-00805F9B34FB` |
| Write Characteristic | `0000FFF2-0000-1000-8000-00805F9B34FB` |
| Notify Characteristic | `0000FFF1-0000-1000-8000-00805F9B34FB` |

**核心命令**（详见 `references/THRM/docs/bs2pro-ota-ble-commands.md`）：

| 命令 | 方向 | 功能 |
|------|------|------|
| `0x01` | 设备→主机 | 查询设备信息 |
| `0x08` | 主机→设备 | 设置固定档位 |
| `0x0C` | 主机→设备 | 上电启动设置 |
| `0x0D` | 主机→设备 | 智能启停设置 |
| `0x21` | 主机→设备 | 设置实时 RPM 目标 |
| `0x23` | 主机→设备 | 进入实时 RPM 模式 |
| `0x24` | 主机→设备 | 退出实时 RPM 模式 |
| `0x25` | 主机→设备 | 查询工作模式 |
| `0x26` | 主机→设备 | 设置档位 RPM |
| `0x27` | 主机→设备 | 查询档位 RPM 表 |
| `0x45` | 双向 | RGB 状态查询/设置 |
| `0x46` | 主机→设备 | RGB 启用 |
| `0x48` | 主机→设备 | 档位灯开关 |
| `0xEF` | 设备→主机 | 设备状态通知（定期推送） |

**设备状态通知帧 (0xEF)**：

```
[0x5A] [0xA5] [0xEF] [0x0B] [gearSettings] [workMode] [reserved] [currentRPM_LE 2bytes] [targetRPM_LE 2bytes]
```

---

## 五、关键约束和注意事项

### 5.1 前端 API 兼容性

前端通过 Wails 绑定调用 Go 方法。以下方法名含 "Windows" 但**不要改名**，以保持前端兼容：

- `SetWindowsAutoStart()`
- `CheckWindowsAutoStart()`
- `SetAutoStartWithMethod()`
- `GetAutoStartMethod()`
- `IsRunningAsAdmin()`

这些方法在 Linux 实现中执行对应逻辑（XDG autostart），方法签名不变。

### 5.2 构建标签规则

| Go 文件名模式 | 构建标签 | 适用场景 |
|-------------|---------|---------|
| `*_linux.go` | `//go:build linux` | Linux 专有实现 |
| `*_windows.go` | `//go:build windows` | Windows 专有实现 |
| `*_other.go` | `//go:build !windows` | 非 Windows 平台 |
| 无特殊后缀 | 无（跨平台） | 所有平台共享代码 |

### 5.3 CGO 依赖

`go-hid` 需要 cgo，构建时必须设置：

```bash
CGO_ENABLED=1 go build ...
```

系统需安装：
- `gcc`（编译 C 桥接代码）
- `libhidapi-dev`（hidapi 开发库，Linux hidraw 后端）

### 5.4 权限要求

| 操作 | 所需权限 | 获取方式 |
|------|---------|---------|
| 访问 HID 设备 (`/dev/hidraw*`) | `plugdev` 组 或 udev 规则 | 安装 udev 规则文件 |
| BLE 扫描 | `bluetooth` 组 或 polkit 策略 | `sudo usermod -aG bluetooth $USER` |
| 读取 `/sys/class/hwmon/` | 通常所有用户可读 | 无需操作 |
| 执行 `nvidia-smi` | 通常所有用户可执行 | 无需操作 |
| 系统托盘 | 无需特殊权限 | 无需操作 |
| 全局热键 (X11) | 无需特殊权限 | 无需操作 |

### 5.5 Wayland 兼容性

| 功能 | X11 | Wayland |
|------|-----|---------|
| Wails 窗口 | ✅ | ✅ 通过 WebKit2GTK |
| 全局热键 | ✅ XGrabKey | ❌ Wayland 协议不支持 |
| 系统托盘 | ✅ XEmbed/SNI | ✅ StatusNotifierItem |
| 窗口管理 | ✅ | ⚠️ 部分限制 |

**建议**：首次移植以 X11/XWayland 为目标。Wayland 的全局热键问题可在未来通过 Portal API 补充。

### 5.6 不应复制的文件清单

| THRM 路径 | 原因 |
|-----------|------|
| `bridge/TempBridge/Program.cs` | C# 温度桥接，Linux 用 Go 原生替代 |
| `build_bridge.bat` | Windows 批处理脚本 |
| `build.bat` | Windows 批处理脚本 |
| `build/` 目录 | Windows 构建产物 |
| `lib/LibreHardwareMonitorLib.dll` | Windows DLL |
| `BS2PRO-Controller.sln` | Visual Studio 解决方案文件 |
| 所有 `*_windows.go` 文件 | Windows 专有实现，不应出现在 Linux 二进制中 |
| `cmd/core/fatal_log_windows.go` | Windows 专有崩溃日志（需新建 `fatal_log_linux.go`） |
| `cmd/core/instance_windows.go` | Windows 单实例锁 |
| `internal/autostart/autostart.go` | Windows 注册表/计划任务 |
| `internal/coreapp/platform_windows.go` | PawnIO 驱动 + Windows 注册表 |
| `internal/hotkey/manager.go` | `//go:build windows` |
| `internal/temperature/cpu_windows.go` | Windows WMI |
| `internal/temperature/exec_windows.go` | Windows `HideWindow` |
| `internal/ipc/transport_windows.go` | Windows 命名管道 |
| `internal/tray/shell_windows.go` | Windows `FindWindow` API |
| `internal/bridge/process_windows.go` | Windows `OpenProcess` |
| `internal/bridge/spawn_windows.go` | Windows `HideWindow` |
| `internal/bridge/supported_windows.go` | Windows 下始终返回 true |
| `internal/guiapp/process_windows.go` | Windows 进程创建标志 |
| `internal/guiapp/window_backdrop_windows.go` | Windows Mica 效果检测 |
| `internal/plugins/fnqpowermode/plugin_windows.go` | Lenovo WMI 插件 |
| `internal/plugins/fnqpowermode/support_windows.go` | Lenovo WMI 检测 |
| `internal/plugins/fnqpowermode/powershell_windows.go` | PowerShell 进程创建 |
| `internal/device/flydigi_hid_winapi_windows.go` | Windows 原生 HID API |
| `internal/device/flydigi_hid_windows.go` | Windows HID 抽象层 |

---

## 六、建议执行顺序和时间估算

```
第 1 步：Phase 0 —— 环境准备                                    (0.5 天)
第 2 步：Phase 1 —— 复制所有平台无关代码，验证编译通过               (1-2 天)
  里程碑：项目骨架编译通过
第 3 步：Phase 2 —— HID/BLE 适配 + udev 规则                     (2-3 天)
  里程碑：可连接设备，读取设备状态，手动控制风扇转速
第 4 步：Phase 3 —— 温度监控替代                                  (1-2 天)
  里程碑：可读取 CPU/GPU 温度，完整的温度-风扇曲线控制循环工作
第 5 步：Phase 4 —— 桌面集成（IPC/托盘/自启动/热键/电源通知）       (3-5 天)
  里程碑：完整桌面应用体验（图标、快捷键、自启动）
第 6 步：Phase 5 —— 组装、构建、打包、调试                         (2-4 天)
  里程碑：可发布版本
```

**总计预估**：~~2-3 周全职工作~~。

**最早可验证功能时间点**：Phase 2 完成后（约 3-4 天）即可插入真实设备，测试基本的设备连接和风扇控制功能。

---

## 附录：关键依赖文档列表

| 文档 | 位置 |
|------|------|
| THRM 协议参考 | `references/THRM/docs/bs2pro-ota-ble-commands.md` |
| HID 抓包数据 | `references/THRM/scripts/hid_data.md` |
| BLE 抓包数据 | `references/THRM/scripts/ble_data.md` |
| BS1 笔记 | `references/THRM/scripts/bs1.md` |
| Wails Linux 支持 | https://wails.io/docs/guides/linux-distro-support/ |
| go-hid 文档 | https://pkg.go.dev/github.com/sstallion/go-hid |
| tinygo bluetooth | https://pkg.go.dev/tinygo.org/x/bluetooth |
| golang.design/x/hotkey | https://pkg.go.dev/golang.design/x/hotkey |
| Linux hwmon 规范 | https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface |
| XDG Autostart 规范 | https://specifications.freedesktop.org/autostart-spec/autostart-spec-latest.html |

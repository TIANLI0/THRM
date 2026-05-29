# THRM

<p align="center">
  <img src="docs/assets/thrm-poster-light.png" alt="THRM 海报" width="960">
</p>

> 面向飞智 BS1 / BS2 / BS2 PRO 的第三方散热控制工具

> 只想正常使用？请直接下载 Releases 页面里的 Windows 安装包。你不需要安装 Go、Node.js、Bun、Wails 或 .NET SDK。

## 普通用户先看

### 下载与安装

1. 前往 [Releases](https://github.com/TIANLI0/BS2PRO-Controller/releases/latest) 下载最新 Windows 安装包。
2. 完成安装后启动 THRM。
3. 程序会自动拉起后台组件，无需手动单独启动核心服务或温度桥接。
4. 按设备类型连接：
   - BS1：使用 BLE / 蓝牙连接。
   - BS2 / BS2 PRO：使用 HID 方式连接，按系统正常接入后在应用内连接即可。
5. 首次进入状态页后，先确认温度、风扇转速和控制模式都已正常显示。

### 你不需要做什么

- 不需要克隆仓库。
- 不需要安装 Go、Node.js、Bun、Wails CLI。
- 不需要自己构建 `THRM.exe`、`THRM Core.exe` 或 `THRM TempBridge.exe`。

### 核心功能

- 支持飞智 BS1 / BS2 / BS2 PRO。
- 实时读取 CPU / GPU 温度，并显示当前风扇状态。
- 支持智能变频、固定转速、手动挡位和风扇曲线控制。
- 提供温度与风扇历史记录面板。
- 支持系统托盘、开机自启、快捷键切换等桌面端能力。
- 保持开源，可持续迭代。

### 配置与日志

- 配置文件优先保存在 `%USERPROFILE%\\.thrm\\config.json`。
- 如果默认用户目录不可写，会回退到 `<安装目录>\\config\\config.json`。
- 运行日志写入 `<安装目录>\\logs\\app_YYYY-MM-DD.log` 和 `<安装目录>\\logs\\debug_YYYY-MM-DD.log`。
- 旧版 `.bs2pro-controller` 配置会在首次读取时自动迁移。

### 常见问题

#### 设备无法连接

1. 确认设备型号和连接方式匹配：BS1 走蓝牙，BS2 / BS2 PRO 走 HID。
2. 断开设备后重新连接，再回到 THRM 中点击连接。
3. 如果是 BS2 / BS2 PRO，请确认 Windows 已正常识别设备。

#### 温度没有显示

1. 重新启动 THRM，让温度桥接组件重新初始化。
2. 检查安装目录中的 `bridge` 文件是否完整。
3. 如果你同时运行了其它硬件监控工具，先暂时关闭后再试。

#### 程序关闭后仍在后台

- THRM 支持最小化到托盘运行；如需完全退出，请从托盘菜单执行退出操作。

<details>
<summary>开发者 / 构建说明</summary>

### 技术栈

- Go 1.25+
- Wails v2
- Next.js 16
- TypeScript
- Tailwind CSS 4
- C# 温度桥接程序

### 开发环境要求

- Go 1.25+
- Node.js 18+
- Bun
- Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- .NET SDK 8.0+
- go-winres：`go install github.com/tc-hib/go-winres@latest`
- NSIS 3.x（可选，用于生成安装程序）

### 本地开发

```bash
go mod tidy
cd frontend
bun install
cd ..
wails dev
```

### 构建

```bash
build_bridge.bat
build.bat
```

构建完成后主要输出位于 `build/bin/`：

- `THRM.exe`
- `THRM Core.exe`
- `bridge/THRM TempBridge.exe`

如果本机安装了 NSIS，构建脚本还会一并生成 Windows 安装程序。

### GitHub Actions 自动构建

- Pull Request 会自动执行 Windows 构建，并在对应的 Actions 运行中上传构建产物。
- 推送到 `main` 或 `dev` 的普通提交也会自动构建，并保留一份短期 Actions 产物，便于快速回归验证。
- 推送 `v*` 或 `V*` 标签时，会自动构建并创建 GitHub Release。
- Release 默认附带安装包 `THRM-amd64-installer.exe` 和便携包 `THRM-windows-portable.zip`。

### 项目结构

```text
cmd/core/                 Go 核心服务
internal/                 设备、配置、日志、温度、IPC 等内部模块
bridge/TempBridge/        C# 温度桥接程序
frontend/                 Wails 前端界面
scripts/                  资源与辅助脚本
build/windows/installer/  Windows 安装脚本
```

### 贡献

欢迎提交 Issue 和 Pull Request。

1. Fork 本项目。
2. 创建分支。
3. 提交修改。
4. 发起 Pull Request。

</details>

## 作者

- TIANLI0 - [GitHub](https://github.com/TIANLI0)
- Email: wutianli@tianli0.top

## 致谢

- [Wails](https://wails.io/)
- [Next.js](https://nextjs.org/)
- 飞智 BS1 / BS2 / BS2 PRO 设备与相关社区反馈

## 免责声明

本项目为第三方开源项目，与飞智官方无关。使用本软件产生的任何问题由用户自行承担。

## 开源许可

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。

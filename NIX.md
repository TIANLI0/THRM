# THRM 的 Nix 打包

本仓库在保留上游源码的同时增加了 Nix Flake 打包。

- 上游版本：**v3.9.0**（release tag）
- 构建目标：`x86_64-linux`（`aarch64-linux` 已在 flake 中声明，但未实机验证）
- 构建方式：Wails v3（Go + cgo）+ Next.js 16 静态导出，与上游 Linux `production` 配置一致（GTK 4 + WebKitGTK 6.0）

flake 通过 `fetchFromGitHub` 固定构建上游 release tag，而不是直接构建本仓库的工作树；因此本仓库里的 Go/前端源码只是上游快照，打包逻辑集中在 `nix/thrm.nix`。

> 固定打包上游稳定 release tag（当前 v3.9.0），不跟随 flake 所在分支（包括 dev）的工作树；打包脚本只更新稳定 vX.Y.Z 版本。
>
> 当前 PR 目标分支为 `dev`；下面的 flake 地址带 `?ref=dev`，合入 `main` 后去掉 `?ref=dev`。

## 快速使用

```bash
# 直接运行 GUI（需要图形会话）
nix run "github:TIANLI0/THRM?ref=dev"

# 构建，产物在 ./result
nix build "github:TIANLI0/THRM?ref=dev"
./result/bin/thrm
```

也可以克隆后构建：

```bash
git clone https://github.com/TIANLI0/THRM
cd THRM
nix build .#thrm
```

NixOS 上临时体验（不写系统配置）：

```bash
nix shell "github:TIANLI0/THRM?ref=dev" -c thrm
```

## 打包产物

```text
$out/bin/thrm               GUI 主程序（内嵌前端静态资源）
$out/bin/thrm-core          后台核心服务（由 GUI 自动拉起）
$out/bin/thrm-rapl-helper   可选：读取 Intel RAPL 功耗
$out/lib/udev/rules.d/99-flydigi-fan.rules
$out/share/applications/thrm.desktop
$out/share/icons/hicolor/1024x1024/apps/thrm.png
```

三个二进制放在同一目录是有意为之：`thrm` 以自身位置定位 `thrm-core`，
`thrm-core` 也按自身位置（或 `THRM_RAPL_HELPER` 环境变量）定位 RAPL helper。

两个入口都通过 `wrapProgram` 追加 `LD_LIBRARY_PATH=/run/opengl-driver/lib`。
NixOS 的 NVIDIA 用户态库（包括 NVML 的 `libnvidia-ml.so.1`）不在默认 loader
搜索路径中；没有这一步，`thrm-core` 无法 `dlopen` NVML，界面上的独立显卡温度
和功耗会显示为 0。

运行时闭包约 1.2 GiB（主要来自 GTK 4 与 WebKitGTK 6.0）。

## 构建原理

前端（`frontend/dist`）不在仓库里，因此 flake 分两步构建：

1. 用 bun 冻结 `frontend/node_modules`（固定输出哈希的 FOD）；
2. 在无网络沙箱中执行 `next build` 静态导出；
3. 交给 `buildGoModule` 通过 `//go:embed all:frontend/dist` 嵌入。

仓库里已提交 `frontend/bindings`，所以构建**不需要** wails3 CLI。

## NixOS 集成（推荐：flake module）

把本 flake 加为 input，并导入 `nixosModules.default`：

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    thrm.url = "github:TIANLI0/THRM?ref=dev";
  };

  outputs = { nixpkgs, thrm, ... }: {
    nixosConfigurations.host = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        thrm.nixosModules.default
        ./configuration.nix
      ];
    };
  };
}
```

`configuration.nix` 只需要：

```nix
{
  programs.thrm.enable = true;

  # 让普通用户读写飞智 HID 设备（GUI 与核心服务运行时必需）
  users.users.<用户名>.extraGroups = [ "input" "bluetooth" ];
}
```

`programs.thrm.enable = true` 会安装包、udev 规则，并创建带
`cap_dac_read_search+ep` 的 `/run/wrappers/bin/thrm-rapl-helper`，同时把
`THRM_RAPL_HELPER` 指向它。需要固定包版本或改用其他包时，用
`programs.thrm.package` 覆盖。

如果不想用 module，等价的手动写法：

```nix
{ pkgs, thrm, ... }:
let
  thrmPkg = thrm.packages.${pkgs.stdenv.hostPlatform.system}.default;
in
{
  environment.systemPackages = [ thrmPkg ];
  services.udev.packages = [ thrmPkg ];
  users.users.<用户名>.extraGroups = [ "input" "bluetooth" ];

  security.wrappers.thrm-rapl-helper = {
    source = "${thrmPkg}/bin/thrm-rapl-helper";
    capabilities = "cap_dac_read_search+ep";
  };
  environment.sessionVariables.THRM_RAPL_HELPER =
    "/run/wrappers/bin/thrm-rapl-helper";
}
```

改完后需要你自己执行 `sudo nixos-rebuild switch`。udev 规则装上后需要重新插拔设备，或执行：

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=usb --subsystem-match=hidraw
```

## CPU 功耗（Intel RAPL）

`/sys/class/powercap/intel-rapl:0/energy_uj` 通常是 `0400 root`。Nix store 只读，
无法对 store 里的 `thrm-rapl-helper` 执行 `setcap`；必须通过 capability wrapper
运行。上面的 module 已自动创建。验证：

```bash
getcap /run/wrappers/bin/thrm-rapl-helper
# 期望：cap_dac_read_search=ep

/run/wrappers/bin/thrm-rapl-helper
# 期望：约 1 秒后输出一个功率值，例如 12.34 W
```

如果 helper 能输出功率、GUI 仍显示 0，请**完全退出 THRM 后重新启动**：core 只在
第一次读取失败时尝试启动 helper，失败后不会重试，直到 core 重启。也可以确认新
进程确实拿到了环境变量：

```bash
tr '\0' '\n' < /proc/$(pgrep -f thrm-core | head -n1)/environ | grep THRM_RAPL_HELPER
```

没有 `/sys/class/powercap/intel-rapl:0` 的机器（部分 AMD、虚拟机、旧内核）不会
显示 CPU 功耗，这是上游 helper 只支持 Intel RAPL 固定路径的限制。

## 开发

```bash
nix develop      # go_1_27、bun、nodejs、wails3、pkg-config、curl/git 与 GTK/WebKit 开发库
nix fmt          # nixfmt
nix flake check  # 评估所有输出并构建 checks.default
```

要修改上游代码，请另建一份上游检出（本仓库的 flake 固定构建上游 tag，不读取工作树里的源码）：

```bash
git clone https://github.com/TIANLI0/THRM.git /path/to/THRM
cd /path/to/THRM
./build.sh       # wails3 由 devShell 提供，产物在 build/
```

## 升级上游版本

版本号在 `nix/thrm.nix` 顶部（`version`，对应上游 tag `v${version}`），三个哈希也在那里。升级流程：

1. 把 `version` 改成新 tag（去掉 `v`）。
2. 三处哈希先换成 `lib.fakeHash`。
3. `nix build .#thrm`，按报错中的 derivation 名称对应填回：
   - `*-source` → `srcHash`
   - `*-thrm-frontend-deps-*` → `frontendDepsHash`
   - `*-go-modules` → `vendorHash`
4. 通过后再跑一次 `nix flake check`。

以上步骤已由 `./scripts/update.sh --update` 自动完成（见下一节）。手动流程只在需要钉住某个非最新版本时使用。

注意：不同 tag 的 `go.mod` 要求可能不同（v3.9.0 要求 Go 1.27.3，正好是 nixpkgs `go_1_27` 的版本）。若新 tag 需要 nixpkgs 尚未提供的 Go 版本，`GOTOOLCHAIN=local` 会导致构建失败，需要先更新 nixpkgs 输入。

## 自动跟进上游版本

仓库带有 `scripts/update.sh` 和 GitHub Actions 工作流 `.github/workflows/update.yml`：每天检查上游稳定版 release，自动更新 `nix/thrm.nix` 的版本号与三个哈希，完整构建并通过 `nix flake check` 后，提交、打 tag 并发布本仓库的 GitHub Release（附 `nix run` 安装命令）。

只跟踪上游稳定 tag（`vX.Y.Z`），`nightly-*` 预发布会被忽略：检测优先使用 GitHub 的 `/releases/latest` 跳转（不消耗 API 配额），失败时回退到 GitHub API，再回退到 `git ls-remote --tags`。

本地运行：

```bash
./scripts/update.sh                    # 只检查；有新版时退出码 10
./scripts/update.sh --check --json     # 机器可读结果到 stdout
./scripts/update.sh --update           # 更新版本、解析哈希、构建并跑 flake check
./scripts/update.sh --update --force   # 对当前版本重新解析哈希
```

脚本只改 `nix/thrm.nix`，失败时自动恢复原文件；它不会 commit 或 push。`THRM_LATEST_VERSION=3.10.0` 可离线指定版本，主要用于测试脚本本身。

启用 CI 自动发布：

1. 首次启用时，打开仓库的 **Actions** 页面，点击启用工作流。
2. 确认 **Settings → Actions → General → Workflow permissions** 允许 **Read and write permissions**（工作流需要 push 提交和创建 Release）。
3. 在 Actions 页面手动运行一次 `Update THRM packaging`（`workflow_dispatch`），确认构建与发布链路正常；之后它会在每天 03:17 UTC 自动检查。
4. 可选：配置 Cachix 缓存。在仓库 secrets 中添加 `CACHIX_CACHE`（缓存名）和 `CACHIX_AUTH_TOKEN`；配置后 CI 会把构建结果推送到缓存，用户可用 `cachix use <cache>` 直接拉取二进制。未配置时相关步骤自动跳过。

工作流提交时带有 `[skip ci]`，避免触发上游的 `build-and-release` 构建；发布 tag 使用 `nix-v${version}`（如 `nix-v3.9.0`），与上游 `v${version}` 区分，可直接安装：

```bash
nix run github:TIANLI0/THRM?ref=nix-v3.9.0
```

> 发布 tag 使用 `nix-v${version}`，与上游 `v${version}` 区分。

注意事项：

- GitHub 定时任务可能延迟数十分钟；仓库连续 60 天无活动时定时工作流会被自动禁用，需要手动重新启用。
- 上游若要求比 nixpkgs 更新的 Go 版本，构建会失败且不会发布；此时先执行 `nix flake update` 再重跑。
- 工作流每次只提交 `nix/thrm.nix`，不会改动上游源码。

## 已知限制

- flake 固定打包上游 v3.9.0；上游后续提交不会自动进入构建，需等 `scripts/update.sh` 更新版本与哈希。
- 只支持 Linux；Wails v3 的 Linux 后端要求 GTK 4 + WebKitGTK 6.0（上游可选用 `EXTRA_TAGS=gtk3` 走 GTK3 栈，本包未采用）。
- 上游单元测试需要真实桌面会话/设备树，打包时 `doCheck = false`。
- GUI 需要图形会话（Wayland/X11）；无头环境只能验证二进制启动与依赖链接。
- NVIDIA 温度/功耗通过 NVML 读取。NixOS 上 `libnvidia-ml.so.1` 位于
  `/run/opengl-driver/lib`，包内已用 wrapper 为 `thrm` 和 `thrm-core` 追加该
  路径；如果绕开包装脚本直接运行二进制，或把二进制复制到别处，需要自行设置
  `LD_LIBRARY_PATH=/run/opengl-driver/lib`，否则独立显卡温度/功耗为 0。AMD/Intel
  核显走 sysfs，不受此影响。
- GUI 内嵌的 WebKitGTK 需要可用的非特权 user namespace 才能启动其进程沙箱（bubblewrap）。如果系统或容器禁用了 user namespace，程序会以 `bwrap: setting up uid map: Permission denied` 退出；此时可以设置 `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` 仅禁用该沙箱（安全性由外层容器负责）。正常的 NixOS 桌面会话默认可用。
- `aarch64-linux` 未在实机验证。

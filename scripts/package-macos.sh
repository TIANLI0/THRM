#!/bin/bash
# 把 THRM 打包成可分发的 macOS 产物：
#   build/dist/THRM.app          GUI
#   build/dist/thrm-core         核心服务
#   build/dist/com.tianli0.thrm.core.plist   LaunchAgent 模板
#   build/dist/THRM-macOS-<ver>.zip          上面三样的压缩包
#
# 用法：
#   ./scripts/package-macos.sh                # 版本号默认 dev
#   VERSION=3.9.0 ./scripts/package-macos.sh  # 指定版本
#   CODESIGN_ID="你的证书名" ./scripts/package-macos.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-dev}"
OUT="$ROOT/build/dist"
APP_NAME="THRM"

echo "=== 打包 THRM macOS $VERSION ==="

# 1. 前端
echo "--- 构建前端 ---"
cd "$ROOT/frontend"
npm run build > /dev/null
cd "$ROOT"

# 2. 编译
echo "--- 编译核心服务 ---"
mkdir -p "$OUT"
LDFLAGS="-s -w -X github.com/TIANLI0/THRM/internal/version.BuildVersion=$VERSION"
# 目标最低 macOS：Go 1.27 官方支持 13+，但 CGO 默认用当前 SDK 编译 C 部分，
# 会把 minos 顶到 26.0。必须同时给 CGO_CFLAGS 和 CGO_LDFLAGS 设下限。
export CGO_CFLAGS="-mmacosx-version-min=${MACOS_MIN:-13.0}"
export CGO_LDFLAGS="-mmacosx-version-min=${MACOS_MIN:-13.0}"
echo "    最低 macOS: ${MACOS_MIN:-13.0}"

CGO_ENABLED=1 go build -trimpath -ldflags="$LDFLAGS" -o "$OUT/thrm-core" ./cmd/core

echo "--- 编译 GUI ---"
CGO_ENABLED=1 go build -trimpath -ldflags="$LDFLAGS" -o "$OUT/thrm-gui" .

# 3. 组 .app
echo "--- 组装 THRM.app ---"
APP="$OUT/$APP_NAME.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$OUT/thrm-gui" "$APP/Contents/MacOS/THRM"
chmod +x "$APP/Contents/MacOS/THRM"

if [ -f "$ROOT/build/appicon.png" ]; then
  cp "$ROOT/build/appicon.png" "$APP/Contents/Resources/appicon.png"
fi

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>THRM</string>
	<key>CFBundleIdentifier</key><string>com.tianli0.thrm</string>
	<key>CFBundleName</key><string>THRM</string>
	<key>CFBundleDisplayName</key><string>THRM</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>${VERSION}.0</string>
	<key>CFBundleVersion</key><string>${VERSION}.0</string>
	<key>LSMinimumSystemVersion</key><string>26.0</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
</dict>
</plist>
PLIST

# 4. 签名
if [ -n "${CODESIGN_ID:-}" ]; then
  echo "--- 签名（${CODESIGN_ID}）---"
  codesign --force --sign "$CODESIGN_ID" "$APP"
  codesign --verify --verbose=1 "$APP"
else
  echo "--- 跳过签名（未设置 CODESIGN_ID）---"
fi

# 5. LaunchAgent（路径占位，安装时替换）
echo "--- 生成 LaunchAgent ---"
sed -e "s|__THRM_CORE_PATH__|$HOME/Applications/THRM/thrm-core|g" \
    -e "s|__THRM_LOG_PATH__|$HOME/Library/Logs/thrm-core.log|g" \
    "$ROOT/launchagent/com.tianli0.thrm.core.plist" \
    > "$OUT/com.tianli0.thrm.core.plist"

# 6. 清理中间产物 + 打包
rm -f "$OUT/thrm-gui"
echo "--- 生成 zip ---"
cd "$OUT"
rm -f "THRM-macOS-$VERSION.zip"
zip -qr "THRM-macOS-$VERSION.zip" "$APP_NAME.app" thrm-core com.tianli0.thrm.core.plist

echo
echo "=== 完成 ==="
ls -lh "$OUT" | sed 's/^/  /'
echo
echo "  产物目录: $OUT"

#!/bin/bash
# THRM Linux installer for a source checkout (run ./build.sh first).
# 便携包里的安装脚本是 scripts/install-portable.sh，两者的权限处理保持一致。
set -e

INSTALL_DIR="${1:-$HOME/.local/bin}"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
UDEV_RULES_DIR="/etc/udev/rules.d"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== THRM — Install ==="

# 1. Install binaries
echo "--- Installing binaries to $INSTALL_DIR ---"
mkdir -p "$INSTALL_DIR"
install -Dm755 "$PROJECT_ROOT/build/thrm" "$INSTALL_DIR/thrm"
install -Dm755 "$PROJECT_ROOT/build/thrm-core" "$INSTALL_DIR/thrm-core"

# 2. Install application icon
ICON_SRC="$PROJECT_ROOT/build/appicon.png"
if [ -f "$ICON_SRC" ]; then
    echo "--- Installing application icon ---"
    mkdir -p "$ICON_DIR"
    install -Dm644 "$ICON_SRC" "$ICON_DIR/thrm.png"
else
    echo "WARNING: $ICON_SRC not found, skipping icon"
fi

# 3. Create .desktop entry
# StartupWMClass 让任务栏把窗口归到这个图标下。
echo "--- Creating desktop entry ---"
mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/thrm.desktop" << EOF
[Desktop Entry]
Type=Application
Name=THRM
Comment=Flydigi BS Series Fan Controller
Exec="$INSTALL_DIR/thrm"
Icon=thrm
Terminal=false
Categories=Utility;
StartupWMClass=thrm
EOF

# 4. Install udev rules — 缺了它普通用户读写 /dev/hidraw* 会被拒绝，只能 sudo 运行。
UDEV_RULES_FILE="$PROJECT_ROOT/scripts/99-flydigi-fan.rules"
if [ -f "$UDEV_RULES_FILE" ]; then
    echo "--- Installing udev rules (requires sudo) ---"
    sudo install -Dm644 "$UDEV_RULES_FILE" "$UDEV_RULES_DIR/99-flydigi-fan.rules"
    sudo udevadm control --reload-rules
    sudo udevadm trigger --subsystem-match=usb --subsystem-match=hidraw
    echo "udev rules installed"
else
    echo "WARNING: $UDEV_RULES_FILE not found, skipping udev rules"
fi

# 5. Group membership
# 蓝牙连接的散热器走 uhid，热插时机常常早于会话 ACL 生效；规则里的 GROUP="input"
# 兜底需要用户本人在 input 组，否则依旧是 permission denied。
if ! id -nG "$USER" | tr ' ' '\n' | grep -qx input; then
    echo ""
    echo "--- Adding $USER to the 'input' group (requires sudo) ---"
    echo "    需要它才能在蓝牙 HID / 非本地会话下访问散热器。"
    if sudo usermod -aG input "$USER"; then
        echo "    已加入 input 组，需要重新登录（或重启）后生效。"
    else
        echo "    WARNING: 加入 input 组失败，可手动执行: sudo usermod -aG input $USER"
    fi
fi

# BS1 通过 BLE GATT 通信，由 BlueZ 处理，与 hidraw 权限无关。
if ! id -nG "$USER" | tr ' ' '\n' | grep -qx bluetooth; then
    echo ""
    echo "NOTE: 如需使用 BS1（蓝牙 BLE 连接），建议加入 bluetooth 组："
    echo "  sudo usermod -aG bluetooth $USER"
fi

# 6. Check PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "WARNING: $INSTALL_DIR is not in your PATH."
    echo "Add the following to your ~/.bashrc or ~/.profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo ""
echo "=== Installation complete ==="
echo "Run 'thrm' from your terminal or application launcher."

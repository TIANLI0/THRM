#!/bin/bash
set -e

INSTALL_DIR="${1:-$HOME/.local/bin}"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
UDEV_RULES_DIR="/etc/udev/rules.d"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== FanControlLinux Install ==="

# 1. Install binaries
echo "--- Installing binaries to $INSTALL_DIR ---"
mkdir -p "$INSTALL_DIR"
install -Dm755 "$PROJECT_ROOT/build/thrm" "$INSTALL_DIR/thrm"
install -Dm755 "$PROJECT_ROOT/build/thrm-core" "$INSTALL_DIR/thrm-core"

# 2. Install application icon
ICON_SRC="$PROJECT_ROOT/frontend/public/brand/appicon.png"
if [ -f "$ICON_SRC" ]; then
    echo "--- Installing application icon ---"
    mkdir -p "$ICON_DIR"
    install -Dm644 "$ICON_SRC" "$ICON_DIR/thrm.png"
else
    echo "WARNING: $ICON_SRC not found, skipping icon"
fi

# 3. Create .desktop entry
echo "--- Creating desktop entry ---"
mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/thrm.desktop" << EOF
[Desktop Entry]
Type=Application
Name=THRM Fan Control
Comment=Flydigi BS Series Fan Controller
Exec=$INSTALL_DIR/thrm
Icon=thrm
Terminal=false
Categories=Utility;
EOF

# 4. Install udev rules (USB-only, skip for BLE)
UDEV_RULES_FILE="$PROJECT_ROOT/scripts/99-flydigi-fan.rules"
if [ -f "$UDEV_RULES_FILE" ]; then
    echo "--- Installing udev rules (may require sudo) ---"
    sudo cp "$UDEV_RULES_FILE" "$UDEV_RULES_DIR/"
    sudo udevadm control --reload-rules
    sudo udevadm trigger
    echo "udev rules installed"
else
    echo "WARNING: $UDEV_RULES_FILE not found, skipping udev rules"
fi

# 5. Check PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "WARNING: $INSTALL_DIR is not in your PATH."
    echo "Add the following to your ~/.bashrc or ~/.profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo ""
echo "=== Installation complete ==="
echo "Run 'thrm' from your terminal or application launcher."

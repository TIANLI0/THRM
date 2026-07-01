#!/bin/bash
# Installer bundled inside the THRM Linux portable tarball.
# Unlike scripts/install.sh (which expects a source checkout), this script
# installs from the flat layout shipped in the tarball (binaries sit next to it).
set -e

HERE="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${1:-$HOME/.local/bin}"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
UDEV_RULES_DIR="/etc/udev/rules.d"

echo "=== THRM Fan Control — Install ==="

# 1. Install binaries
echo "--- Installing binaries to $INSTALL_DIR ---"
install -Dm755 "$HERE/thrm" "$INSTALL_DIR/thrm"
install -Dm755 "$HERE/thrm-core" "$INSTALL_DIR/thrm-core"

# 2. Install application icon
if [ -f "$HERE/appicon.png" ]; then
    echo "--- Installing application icon ---"
    install -Dm644 "$HERE/appicon.png" "$ICON_DIR/thrm.png"
else
    echo "WARNING: appicon.png not found, skipping icon"
fi

# 3. Create .desktop entry
echo "--- Creating desktop entry ---"
mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/thrm.desktop" << EOF
[Desktop Entry]
Type=Application
Name=THRM Fan Control
Comment=Flydigi BS Series Fan Controller
Exec="$INSTALL_DIR/thrm"
Icon=thrm
Terminal=false
Categories=Utility;
EOF

# 4. Install udev rules (USB-only, skip for BLE)
if [ -f "$HERE/99-flydigi-fan.rules" ]; then
    echo "--- Installing udev rules (may require sudo) ---"
    sudo cp "$HERE/99-flydigi-fan.rules" "$UDEV_RULES_DIR/"
    sudo udevadm control --reload-rules
    sudo udevadm trigger
    echo "udev rules installed"
else
    echo "WARNING: 99-flydigi-fan.rules not found, skipping udev rules"
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

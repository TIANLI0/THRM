#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

OS_NAME="$(uname -s)"
echo "=== THRM $OS_NAME Build ==="

# Resolve build version: THRM_BUILD_VERSION env override -> build/config.yml info.version -> "dev".
VERSION="${THRM_BUILD_VERSION:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(sed -nE 's/^[[:space:]]+version:[[:space:]]*"?([^"#[:space:]]+)"?.*/\1/p' "$PROJECT_ROOT/build/config.yml" | head -n1)
fi
if [ -z "$VERSION" ]; then
    echo "WARNING: could not resolve version from build/config.yml, using 'dev'"
    VERSION="dev"
fi
echo "Building version: $VERSION"

LDFLAGS="-s -w -X github.com/TIANLI0/THRM/internal/version.BuildVersion=$VERSION"

mkdir -p "$BUILD_DIR"

if [ "$OS_NAME" = "Darwin" ]; then
    # macOS: core + GUI are packaged together into build/bin/THRM.app (+ .dmg).
    BIN_DIR="$BUILD_DIR/bin"
    mkdir -p "$BIN_DIR"
    # Info.plist / Finder only accept a purely numeric x.y.z version.
    PRODUCT_VERSION=$(printf '%s' "$VERSION" | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')
    PRODUCT_VERSION="${PRODUCT_VERSION:-0.0.0}"
    export MACOSX_DEPLOYMENT_TARGET=12.0
    export CGO_CFLAGS="-mmacosx-version-min=12.0"
    export CGO_LDFLAGS="-mmacosx-version-min=12.0"

    echo "--- Building core service ---"
    # Embed an Info.plist so CoreBluetooth finds the Bluetooth usage string
    # even when thrm-core runs outside THRM.app.
    # The plist path is relative so a project path containing spaces cannot split -extldflags.
    cd "$PROJECT_ROOT"
    CGO_ENABLED=1 go build \
        -trimpath \
        -ldflags="$LDFLAGS -linkmode=external -extldflags=-Wl,-sectcreate,__TEXT,__info_plist,build/darwin/core-Info.plist" \
        -o "$BIN_DIR/thrm-core" \
        ./cmd/core/

    echo "--- Building GUI and app bundle ---"
    wails3 task package:darwin VERSION="$VERSION" PRODUCT_VERSION="$PRODUCT_VERSION" BIN_DIR=build/bin

    echo "=== Build complete ==="
    ls -lh "$BIN_DIR"/*.dmg
    du -sh "$BIN_DIR/THRM.app"
    exit 0
fi

# 1. Build core service (requires CGO for go-hid)
echo "--- Building core service ---"
CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/thrm-core" \
    ./cmd/core/

# 2. Build RAPL helper (optional, only needs file capability at runtime)
echo "--- Building RAPL helper ---"
CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/thrm-rapl-helper" \
    ./cmd/rapl-helper/

# 3. Build GUI (requires CGO for Wails/WebKit2GTK)
echo "--- Building GUI ---"
cd "$PROJECT_ROOT"
wails3 task build VERSION="$VERSION" BIN_DIR=build

echo "=== Build complete ==="
echo "Binaries:"
ls -lh "$BUILD_DIR/thrm" "$BUILD_DIR/thrm-core" "$BUILD_DIR/thrm-rapl-helper"

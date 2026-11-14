#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

echo "=== THRM Linux Build ==="

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

#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

echo "=== FanControlLinux Build ==="

# Resolve build version: THRM_BUILD_VERSION env override -> wails.json productVersion -> "dev".
VERSION="${THRM_BUILD_VERSION:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(grep -oE '"productVersion"[[:space:]]*:[[:space:]]*"[^"]+"' "$PROJECT_ROOT/wails.json" | sed -E 's/.*"([^"]+)"$/\1/')
fi
if [ -z "$VERSION" ]; then
    echo "WARNING: could not resolve version from wails.json, using 'dev'"
    VERSION="dev"
fi
echo "Building version: $VERSION"

LDFLAGS="-s -w -X github.com/TIANLI0/THRM/internal/version.BuildVersion=$VERSION"

mkdir -p "$BUILD_DIR"

# 1. Build frontend (skip if dist already exists)
echo "--- Building frontend ---"
if [ ! -d "$PROJECT_ROOT/frontend/dist" ]; then
    cd "$PROJECT_ROOT/frontend"
    bun install
    bun run build
    cd "$PROJECT_ROOT"
else
    echo "frontend/dist already exists, skipping"
fi

# 2. Build core service (requires CGO for go-hid)
echo "--- Building core service ---"
CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/thrm-core" \
    ./cmd/core/

# 3. Build GUI (requires CGO for Wails/WebKit2GTK)
echo "--- Building GUI ---"
CGO_ENABLED=1 go build \
    -trimpath \
    -tags "production,webkit2_41" \
    -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/thrm" \
    .

echo "=== Build complete ==="
echo "Binaries:"
ls -lh "$BUILD_DIR/thrm" "$BUILD_DIR/thrm-core"

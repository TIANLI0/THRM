#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"

echo "=== FanControlLinux Build ==="

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
    -ldflags="-s -w" \
    -o "$BUILD_DIR/thrm-core" \
    ./cmd/core/

# 3. Build GUI (requires CGO for Wails/WebKit2GTK)
echo "--- Building GUI ---"
CGO_ENABLED=1 go build \
    -tags "production,webkit2_41" \
    -ldflags="-s -w" \
    -o "$BUILD_DIR/thrm" \
    .

echo "=== Build complete ==="
echo "Binaries:"
ls -lh "$BUILD_DIR/thrm" "$BUILD_DIR/thrm-core"

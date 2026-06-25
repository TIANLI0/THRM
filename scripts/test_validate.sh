#!/bin/bash
# test_validate.sh — FanControlLinux Steps 1-3 Structure & Dependency Validation
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

PASS=0
FAIL=0

check() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "  PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test Suite 0: Build & Structure ==="

echo "--- TS0.1: Directory structure ---"
for dir in cmd/core internal/appmeta internal/autostart internal/bridge internal/config \
    internal/coreapp internal/curveprofiles internal/device internal/deviceproto \
    internal/deviceprofileexec internal/guiapp internal/hotkey internal/ipc \
    internal/logger internal/notifier internal/plugins/fnqpowermode internal/powernotify \
    internal/smartcontrol internal/temperature internal/theme internal/tray \
    internal/types internal/version scripts build; do
    check "Directory exists: $dir" test -d "$dir"
done

echo "--- TS0.2: Build tag correctness ---"
check "meta_linux.go has //go:build linux" grep -q "//go:build linux" internal/appmeta/meta_linux.go
check "autostart_linux.go has //go:build linux" grep -q "//go:build linux" internal/autostart/autostart_linux.go
check "manager_linux.go has //go:build linux" grep -q "//go:build linux" internal/hotkey/manager_linux.go
check "powernotify_linux.go has //go:build linux" grep -q "//go:build linux" internal/powernotify/powernotify_linux.go
check "platform_linux.go has //go:build linux" grep -q "//go:build linux" internal/coreapp/platform_linux.go
check "fatal_log_linux.go has //go:build linux" grep -q "//go:build linux" cmd/core/fatal_log_linux.go

echo "--- TS0.3: No empty Go files ---"
while IFS= read -r -d '' f; do
    check "Non-empty: $f" test -s "$f"
done < <(find internal cmd -name "*.go" -print0 2>/dev/null)

echo ""
echo "=== Test Suite 6: Scripts & Config ==="

echo "--- TS6.1: udev rules file exists ---"
check "udev rules file exists" test -f scripts/99-flydigi-fan.rules
check "udev rules file non-empty" test -s scripts/99-flydigi-fan.rules

echo "--- TS6.2: udev rules VID/PID coverage ---"
check "VID 0x37D7 present" grep -q "37d7\|37D7" scripts/99-flydigi-fan.rules
check "PID 1001 (BS2)" grep -q "1001" scripts/99-flydigi-fan.rules
check "PID 1002 (BS2PRO)" grep -q "1002" scripts/99-flydigi-fan.rules
check "PID 1003 (BS3)" grep -q "1003" scripts/99-flydigi-fan.rules
check "PID 1004 (BS3PRO)" grep -q "1004" scripts/99-flydigi-fan.rules

echo "--- TS6.3: udev rules permissions ---"
check "TAG+=uaccess present" grep -q 'TAG+="uaccess"' scripts/99-flydigi-fan.rules

echo "--- TS6.4: udevadm verify ---"
if command -v udevadm &>/dev/null; then
    if udevadm verify scripts/99-flydigi-fan.rules 2>&1; then
        echo "  PASS: udevadm verify passed"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: udevadm verify failed"
        FAIL=$((FAIL + 1))
    fi
else
    echo "  SKIP: udevadm not found"
fi

echo "--- TS6.5: LICENSE exists ---"
check "LICENSE file exists" test -f LICENSE
check "LICENSE non-empty" test -s LICENSE

echo ""
echo "=== Test Suite 7: go.mod Dependency Audit ==="

echo "--- TS7.1: No Windows-only direct dependencies ---"
for dep in go-winio go-ole wmi winrt go-winloader webview2; do
    check "go.mod does not contain $dep as direct dep" bash -c "! grep -v '// indirect' go.mod | grep -q \"$dep\""
done

echo "--- TS7.2: Currently imported deps present ---"
for dep in hotkey dbus sys; do
    check "go.mod contains $dep" grep -q "$dep" go.mod
done
echo "  NOTE: go-hid, bluetooth, beeep, systray, zap, lumberjack, gopsutil, wails"
echo "        will be added when source code is migrated in Steps 4-11."

echo "--- TS7.3: go.sum integrity ---"
check "go mod verify" go mod verify

echo "--- TS7.4: go mod tidy clean ---"
cp go.mod go.mod.bak
cp go.sum go.sum.bak 2>/dev/null || true
check "go mod tidy succeeds" go mod tidy
check "go mod tidy produces no changes" diff go.mod go.mod.bak
mv go.mod.bak go.mod
mv go.sum.bak go.sum 2>/dev/null || true

echo "--- TS7.5: Go version ---"
check "Go version 1.26+ in go.mod" grep -q "go 1.26" go.mod

echo "--- TS7.6: No go.sum hash mismatches ---"
check "go.sum has entries" test -s go.sum

echo ""
echo "========================================"
echo "Results: $PASS passed, $FAIL failed"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0

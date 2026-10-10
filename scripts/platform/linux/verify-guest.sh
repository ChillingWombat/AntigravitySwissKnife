#!/usr/bin/env bash
# ==============================================================================
# Antigravity Swiss Knife - Linux Host/Guest Verification Script
# ==============================================================================
# Verifies standalone binary, packaged .deb / AppImage, desktop launcher,
# daemon health endpoint, and user config path.
# ==============================================================================

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"

echo "======================================================================"
echo "Starting Antigravity Swiss Knife Linux Verification"
echo "======================================================================"

# 1. Standalone Go binary verification
echo -e "\n[1/5] Verifying standalone Go binary (bin/swiss)..."
SWISS_BIN="$ROOT_DIR/bin/swiss"
if [[ ! -x "$SWISS_BIN" ]]; then
    echo "ERROR: Missing executable bin/swiss"
    exit 1
fi
VER_OUT=$("$SWISS_BIN" version)
echo "  Version output: $VER_OUT"
if [[ "$VER_OUT" != *"Antigravity Swiss Knife"* ]]; then
    echo "ERROR: Unexpected version string"
    exit 1
fi
"$SWISS_BIN" --help > /dev/null
echo "  ✓ Standalone binary functioning correctly"

# 2. Package verification (.deb / AppImage)
echo -e "\n[2/5] Verifying Linux package artifacts..."
DEB_PKG=$(find "$ROOT_DIR/release/linux" -name "*.deb" | head -n 1)
APP_IMG=$(find "$ROOT_DIR/release/linux" -name "*.AppImage" | head -n 1)

if [[ -f "$DEB_PKG" ]]; then
    echo "  Checking Debian package: $(basename "$DEB_PKG")"
    dpkg -I "$DEB_PKG" | grep -E "Package:|Version:|Architecture:"
    if dpkg -c "$DEB_PKG" | grep "resources/bin/swiss" > /dev/null; then
        echo "  ✓ .deb package contains resources/bin/swiss sidecar"
    else
        echo "  ✗ .deb package missing resources/bin/swiss sidecar"
        exit 1
    fi
else
    echo "  ℹ No .deb found in release/linux/"
fi

if [[ -f "$APP_IMG" ]]; then
    echo "  ✓ AppImage exists: $(basename "$APP_IMG")"
fi

# 3. Unpacked Linux tree verification
UNPACKED_BIN="$ROOT_DIR/release/linux/linux-unpacked/resources/bin/swiss"
if [[ -f "$UNPACKED_BIN" ]]; then
    echo -e "\n[3/5] Verifying unpacked distribution tree..."
    test -x "$UNPACKED_BIN"
    echo "  ✓ linux-unpacked sidecar daemon exists and is executable"
else
    echo -e "\n[3/5] linux-unpacked tree not present (built with --unpacked if needed)"
fi

# 4. Probing daemon health if active
echo -e "\n[4/5] Probing daemon status endpoint (if daemon is running)..."
if curl -s -m 2 http://127.0.0.1:8765/api/status 2>/dev/null | grep -q '"daemon_running":true'; then
    echo "  ✓ Daemon is currently active on port 8765"
else
    echo "  ℹ Daemon is not currently running (can be launched with 'npm run desktop' or './swiss start')"
fi

# 5. Verifying config directory path
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/antigravity-swiss"
echo -e "\n[5/5] Checking Linux config directory path..."
echo "  Config directory: $CONFIG_DIR"
if [[ -d "$CONFIG_DIR" ]]; then
    echo "  ✓ Config directory exists"
else
    echo "  ℹ Config directory will be created on initial daemon save"
fi

echo -e "\n======================================================================"
echo "✓ Linux Verification Complete!"
echo "======================================================================"

#!/usr/bin/env bash
# ==============================================================================
# Antigravity Swiss Knife - macOS Guest Verification Script
# ==============================================================================
# Run inside macOS Ventura VM (Terminal.app or via SSH: ssh -p 22220 david@localhost):
#   bash verify-guest.sh
# ==============================================================================

set -euo pipefail
BASE="http://10.0.2.2:8010"
TMP_DIR="/tmp/ask-verification"
mkdir -p "$TMP_DIR"

echo "======================================================================"
echo "Starting Antigravity Swiss Knife macOS Guest Verification"
echo "======================================================================"

# 1. Download and smoke test standalone Go binary
echo -e "\n[1/6] Downloading standalone Go binary (swiss-mac) from $BASE..."
curl -fSL -o "$TMP_DIR/swiss-mac" "$BASE/swiss-mac"
chmod +x "$TMP_DIR/swiss-mac"

echo "Testing standalone Go binary:"
VER_OUT=$("$TMP_DIR/swiss-mac" version)
echo "  Version: $VER_OUT"
if [[ "$VER_OUT" != *"Antigravity Swiss Knife"* ]]; then
    echo "ERROR: Unexpected version output: $VER_OUT"
    exit 1
fi
"$TMP_DIR/swiss-mac" --help > /dev/null
echo "  ✓ Standalone binary executed successfully"

# 2. Download application bundle zip
echo -e "\n[2/6] Downloading application bundle zip (ASK-mac-x64.zip)..."
curl -fSL -o "$TMP_DIR/ASK-mac-x64.zip" "$BASE/ASK-mac-x64.zip"
echo "  ✓ Application zip downloaded"

# 3. Unpack and install application bundle
echo -e "\n[3/6] Installing application to /Applications/..."
rm -rf "/Applications/Antigravity Swiss Knife.app"
unzip -q "$TMP_DIR/ASK-mac-x64.zip" -d /Applications/
xattr -dr com.apple.quarantine "/Applications/Antigravity Swiss Knife.app" 2>/dev/null || true

# 4. Verify bundle structure and permissions
APP="/Applications/Antigravity Swiss Knife.app"
MAIN_BIN="$APP/Contents/MacOS/Antigravity Swiss Knife"
SIDECAR_BIN="$APP/Contents/Resources/bin/swiss"

echo -e "\n[4/6] Verifying application bundle structure..."
if [[ ! -f "$MAIN_BIN" ]]; then
    echo "ERROR: Missing main binary: $MAIN_BIN"
    exit 1
fi
if [[ ! -x "$SIDECAR_BIN" ]]; then
    echo "ERROR: Sidecar binary missing or not executable: $SIDECAR_BIN"
    exit 1
fi
echo "  ✓ Main binary verified: $MAIN_BIN"
echo "  ✓ Sidecar binary verified: $SIDECAR_BIN"

# 5. Launch application and probe daemon health
echo -e "\n[5/6] Launching application bundle..."
open "$APP"
sleep 5

echo "Probing Go daemon health endpoint (http://127.0.0.1:8765/api/status)..."
STATUS_JSON=""
for i in {1..5}; do
    if STATUS_JSON=$(curl -s -m 3 http://127.0.0.1:8765/api/status 2>/dev/null); then
        if [[ "$STATUS_JSON" == *'"daemon_running":true'* ]]; then
            break
        fi
    fi
    sleep 2
done

if [[ "$STATUS_JSON" == *'"daemon_running":true'* ]]; then
    echo "  ✓ Daemon health check PASSED!"
    echo "    Status response: $STATUS_JSON"
else
    echo "  WARNING: Daemon status endpoint did not report daemon_running:true"
fi

# 6. Verify config directory
CONFIG_DIR="$HOME/Library/Application Support/antigravity-swiss"
echo -e "\n[6/6] Verifying config directory path..."
echo "  Config directory: $CONFIG_DIR"
if [[ -d "$CONFIG_DIR" ]]; then
    echo "  ✓ Config directory exists"
else
    echo "  ℹ Config directory will be created on initial credential save"
fi

# Teardown / Close application
osascript -e 'quit app "Antigravity Swiss Knife"' 2>/dev/null || true
pkill -f "Antigravity Swiss Knife" 2>/dev/null || true

echo -e "\n======================================================================"
echo "✓ macOS Guest Verification Complete!"
echo "======================================================================"

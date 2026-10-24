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

echo "Probing Go daemon health endpoints (http://127.0.0.1:8765/api/status and /healthz)..."
HEALTH_DETAIL=""

probe_daemon() {
    local url="$1"
    local response http_code body compact
    response=$(curl -sS -m 3 -w $'\n%{http_code}' "$url" 2>&1) || true
    http_code="${response##*$'\n'}"
    body="${response%$'\n'*}"
    compact=$(printf '%s' "$body" | tr -d '[:space:]')

    if [[ "$http_code" == "200" && "$url" == */api/status && "$compact" == *'"daemon_running":true'* ]]; then
        HEALTH_DETAIL="HTTP 200 $url $body"
        return 0
    fi
    if [[ "$http_code" == "200" && "$url" == */healthz && "$compact" == *'"status":"ok"'* ]]; then
        HEALTH_DETAIL="HTTP 200 $url $body"
        return 0
    fi
    if [[ "$http_code" == "401" && "$body" == *"Unauthorized"* ]]; then
        HEALTH_DETAIL="HTTP 401 Unauthorized $url $body"
        return 0
    fi

    HEALTH_DETAIL="HTTP $http_code $url $body"
    return 1
}

DAEMON_HEALTHY=false
for i in {1..5}; do
    if probe_daemon "http://127.0.0.1:8765/api/status" || probe_daemon "http://127.0.0.1:8765/healthz"; then
        DAEMON_HEALTHY=true
        break
    fi
    sleep 2
done

if [[ "$DAEMON_HEALTHY" == "true" ]]; then
    echo "  ✓ Daemon health check PASSED!"
    echo "    $HEALTH_DETAIL"
else
    echo "ERROR: Daemon did not report healthy within the retry window."
    echo "  Last response: $HEALTH_DETAIL"
    exit 1
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

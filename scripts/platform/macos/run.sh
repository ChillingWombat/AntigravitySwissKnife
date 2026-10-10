#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../release/macos" && pwd)"
ARCH="$(uname -m)"

if [ "${ARCH}" = "arm64" ] && [ -d "${DIR}/mac-arm64/Antigravity Swiss Knife.app" ]; then
  APP="${DIR}/mac-arm64/Antigravity Swiss Knife.app"
elif [ -d "${DIR}/mac/Antigravity Swiss Knife.app" ]; then
  APP="${DIR}/mac/Antigravity Swiss Knife.app"
elif [ -d "${DIR}/Antigravity Swiss Knife.app" ]; then
  APP="${DIR}/Antigravity Swiss Knife.app"
elif [ -d "${DIR}/app/Antigravity Swiss Knife.app" ]; then
  APP="${DIR}/app/Antigravity Swiss Knife.app"
else
  APP="${DIR}/mac-arm64/Antigravity Swiss Knife.app"
fi

echo "[Production Release] Launching Antigravity Swiss Knife macOS app (${ARCH}): ${APP}"
open "${APP}" --args "$@"

#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../release/linux" && pwd)"

if [ -f "${DIR}/Antigravity-Swiss-Knife-0.2.0-x86_64.AppImage" ]; then
  exec "${DIR}/Antigravity-Swiss-Knife-0.2.0-x86_64.AppImage" --no-sandbox "$@"
elif [ -f "${DIR}/linux-unpacked/antigravity-swiss-knife" ]; then
  exec "${DIR}/linux-unpacked/antigravity-swiss-knife" --no-sandbox "$@"
elif [ -f "${DIR}/app/antigravity-swiss-knife" ]; then
  exec "${DIR}/app/antigravity-swiss-knife" --no-sandbox "$@"
else
  echo "Error: No packaged Linux application found in ${DIR}"
  exit 1
fi

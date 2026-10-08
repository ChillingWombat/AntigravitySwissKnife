#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OS="$(uname -s)"

echo "=== Antigravity Swiss Knife Production Launcher ==="

case "${OS}" in
  Linux*)
    if [ -f "${SCRIPT_DIR}/linux/run.sh" ]; then
      exec bash "${SCRIPT_DIR}/linux/run.sh" "$@"
    elif [ -f "${SCRIPT_DIR}/linux/app/antigravity-swiss-knife" ]; then
      exec "${SCRIPT_DIR}/linux/app/antigravity-swiss-knife" "$@"
    elif [ -f "${SCRIPT_DIR}/linux/linux-unpacked/antigravity-swiss-knife" ]; then
      exec "${SCRIPT_DIR}/linux/linux-unpacked/antigravity-swiss-knife" "$@"
    else
      echo "Error: Linux release build not found in ${SCRIPT_DIR}/linux/"
      echo "To build Linux release: npm run release:linux"
      exit 1
    fi
    ;;
  Darwin*)
    if [ -f "${SCRIPT_DIR}/macos/run.sh" ]; then
      exec bash "${SCRIPT_DIR}/macos/run.sh" "$@"
    elif [ -d "${SCRIPT_DIR}/macos/mac-arm64/Antigravity Swiss Knife.app" ]; then
      open "${SCRIPT_DIR}/macos/mac-arm64/Antigravity Swiss Knife.app" --args "$@"
    elif [ -d "${SCRIPT_DIR}/macos/app/Antigravity Swiss Knife.app" ]; then
      open "${SCRIPT_DIR}/macos/app/Antigravity Swiss Knife.app" --args "$@"
    elif [ -d "${SCRIPT_DIR}/macos/mac/Antigravity Swiss Knife.app" ]; then
      open "${SCRIPT_DIR}/macos/mac/Antigravity Swiss Knife.app" --args "$@"
    else
      echo "Error: macOS release build not found in ${SCRIPT_DIR}/macos/"
      echo "To build macOS release: npm run release:mac"
      exit 1
    fi
    ;;
  MINGW*|MSYS*|CYGWIN*)
    if [ -f "${SCRIPT_DIR}/windows/run.bat" ]; then
      cmd.exe /c "${SCRIPT_DIR}/windows/run.bat" "$@"
    elif [ -f "${SCRIPT_DIR}/windows/app/Antigravity Swiss Knife.exe" ]; then
      "${SCRIPT_DIR}/windows/app/Antigravity Swiss Knife.exe" "$@"
    elif [ -f "${SCRIPT_DIR}/windows/win-unpacked/Antigravity Swiss Knife.exe" ]; then
      "${SCRIPT_DIR}/windows/win-unpacked/Antigravity Swiss Knife.exe" "$@"
    else
      echo "Error: Windows release build not found in ${SCRIPT_DIR}/windows/"
      echo "To build Windows release: npm run release:win"
      exit 1
    fi
    ;;
  *)
    echo "Unsupported operating system: ${OS}"
    exit 1
    ;;
esac

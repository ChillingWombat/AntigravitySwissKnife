#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WIKI_DIR="$REPO_DIR/docs/wiki"
TMP_WIKI="/tmp/antigravity-wiki-push"

echo "==> Preparing wiki documents from docs/wiki/..."
rm -rf "$TMP_WIKI"
mkdir -p "$TMP_WIKI"
cp -r "$WIKI_DIR"/* "$TMP_WIKI"/

cd "$TMP_WIKI"
git init
git config user.name "ChillingWombat"
git config user.email "david@example.com"
git add .
git commit -m "docs(wiki): publish complete Antigravity Swiss Knife documentation"
git branch -M master

TOKEN=$(gh auth token)
WIKI_URL="https://x-access-token:${TOKEN}@github.com/ChillingWombat/AntigravitySwissKnife.wiki.git"

echo "==> Pushing to $WIKI_URL..."
git push --force "$WIKI_URL" master
echo "==> GitHub Wiki published successfully!"

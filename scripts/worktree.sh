#!/usr/bin/env bash
# ==============================================================================
# Multi-Agent Git Worktree Management Utility
# ==============================================================================
# Solves concurrent branch collisions when multiple agents or developers
# collaborate on the repository simultaneously.
#
# Usage:
#   ./scripts/worktree.sh list
#   ./scripts/worktree.sh create <branch-name> [base-ref]
#   ./scripts/worktree.sh remove <branch-name>
#   ./scripts/worktree.sh sync
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT_COMMON_DIR="$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || git rev-parse --git-dir 2>/dev/null || true)"
if [ -n "${GIT_COMMON_DIR}" ] && [ -d "${GIT_COMMON_DIR}" ]; then
  ROOT_DIR="$(cd "${GIT_COMMON_DIR}/.." && pwd)"
else
  ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
fi
WORKTREES_DIR="${ROOT_DIR}/.worktrees"

cmd="${1:-list}"

case "${cmd}" in
  list)
    echo "=== Active Git Worktrees ==="
    git -C "${ROOT_DIR}" worktree list
    ;;

  create)
    branch="${2}"
    if [ -z "${branch}" ]; then
      echo "Error: Branch name required. Example: ./scripts/worktree.sh create feature/my-feature"
      exit 1
    fi
    base="${3:-origin/main}"
    mkdir -p "${WORKTREES_DIR}"

    # Sanitize branch name for folder path (replace slashes with hyphens)
    target_dir="${WORKTREES_DIR}/${branch//\//-}"

    if [ -d "${target_dir}" ]; then
      echo "Worktree directory already exists at: ${target_dir}"
      exit 0
    fi

    echo "Creating worktree for branch '${branch}' based on '${base}'..."
    if git -C "${ROOT_DIR}" show-ref --verify --quiet "refs/heads/${branch}"; then
      git -C "${ROOT_DIR}" worktree add "${target_dir}" "${branch}"
    elif git -C "${ROOT_DIR}" show-ref --verify --quiet "refs/remotes/origin/${branch}"; then
      git -C "${ROOT_DIR}" worktree add --track -b "${branch}" "${target_dir}" "origin/${branch}"
    else
      git -C "${ROOT_DIR}" worktree add -b "${branch}" "${target_dir}" "${base}"
    fi

    echo "Setting up dependency links..."
    if [ -d "${ROOT_DIR}/node_modules" ] && [ ! -e "${target_dir}/node_modules" ]; then
      ln -s "${ROOT_DIR}/node_modules" "${target_dir}/node_modules" 2>/dev/null || true
    fi
    if [ -d "${ROOT_DIR}/frontend/node_modules" ] && [ ! -e "${target_dir}/frontend/node_modules" ]; then
      ln -s "${ROOT_DIR}/frontend/node_modules" "${target_dir}/frontend/node_modules" 2>/dev/null || true
    fi

    echo ""
    echo "✓ Worktree created successfully!"
    echo "  Location: ${target_dir}"
    echo "  Branch:   ${branch}"
    echo ""
    echo "To work in this isolated workspace:"
    echo "  cd \"${target_dir}\""
    ;;

  remove)
    branch="${2}"
    if [ -z "${branch}" ]; then
      echo "Error: Branch name or path required to remove."
      exit 1
    fi
    target_dir="${WORKTREES_DIR}/${branch//\//-}"
    if [ ! -d "${target_dir}" ] && [ -d "${WORKTREES_DIR}/${branch}" ]; then
      target_dir="${WORKTREES_DIR}/${branch}"
    elif [ ! -d "${target_dir}" ] && [ -d "${branch}" ]; then
      target_dir="$(cd "${branch}" && pwd)"
    fi

    if [ -d "${target_dir}" ]; then
      echo "Removing worktree at: ${target_dir}"
      git -C "${ROOT_DIR}" worktree remove --force "${target_dir}" 2>/dev/null || rm -rf "${target_dir}"
      git -C "${ROOT_DIR}" worktree prune
      echo "✓ Worktree removed."
    else
      echo "Worktree not found at: ${target_dir}"
    fi
    ;;

  sync)
    echo "Pruning stale worktree references..."
    git -C "${ROOT_DIR}" worktree prune
    git -C "${ROOT_DIR}" worktree list
    ;;

  *)
    echo "Usage: ./scripts/worktree.sh {list|create <branch> [base]|remove <branch>|sync}"
    exit 1
    ;;
esac

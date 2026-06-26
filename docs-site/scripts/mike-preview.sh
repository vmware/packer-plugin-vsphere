#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Build and serve a local multi-version preview with mike (no push to remote).

# Usage:
#   mike-preview.sh [--fresh] [--deploy-only] [--serve-only] [VERSION ...]
#
# Deploys tagged releases (newest gets the "latest" alias) plus the current
# working tree as "development", then runs mike serve.
#
# Environment:
#   MIKE_PREVIEW_BRANCH        Local git branch (default: docs-preview)
#   MIKE_PREVIEW_PORT          Serve port (default: 8001)
#   MIKE_PREVIEW_VERSIONS      Space-separated release tags (default: last 3 git tags)
#   MIKE_PREVIEW_CURRENT       Version id for working tree (default: development)
#   MIKE_PREVIEW_CURRENT_TITLE Title shown for the working-tree version
#   MIKE_PREVIEW_SKIP_DEVELOPMENT  Skip deploying the working tree (default: false)
#   MIKE_COMMIT_MESSAGE            Optional git commit message for mike commits
#   MIKE_COMMIT_VERSION            Expands {version} in MIKE_COMMIT_MESSAGE

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

# shellcheck source=lib/paths.sh
source "${LIB_DIR}/paths.sh"
# shellcheck source=lib/mike-env.sh
source "${LIB_DIR}/mike-env.sh"
# shellcheck source=lib/sync-web-docs.sh
source "${LIB_DIR}/sync-web-docs.sh"

BRANCH="${MIKE_PREVIEW_BRANCH:-docs-preview}"
PORT="${MIKE_PREVIEW_PORT:-8001}"
CURRENT="${MIKE_PREVIEW_CURRENT:-development}"
CURRENT_TITLE="${MIKE_PREVIEW_CURRENT_TITLE:-}"
SKIP_DEVELOPMENT="${MIKE_PREVIEW_SKIP_DEVELOPMENT:-false}"

FRESH=false
DEPLOY_ONLY=false
SERVE_ONLY=false
declare -a VERSIONS=()

usage() {
  cat <<EOF
usage: $0 [--fresh] [--deploy-only] [--serve-only] [VERSION ...]

  --fresh        Reset the local preview branch before deploying
  --deploy-only  Deploy versions without starting the server
  --serve-only   Serve an existing preview branch without redeploying
  --no-development  Deploy tagged releases only (omit working-tree preview)
  --help         Show this help

Deploys tagged releases to a local mike branch, then serves them. The newest
tagged version receives the latest alias. Unless --no-development is set,
the current working tree is also deployed as MIKE_PREVIEW_CURRENT (default:
development) with a title derived from the current git branch name.

Tagged VERSION arguments override MIKE_PREVIEW_VERSIONS (e.g. 2.1.0 2.2.0).

Environment:
  MIKE_PREVIEW_BRANCH        Local git branch (default: docs-preview)
  MIKE_PREVIEW_PORT          Serve port (default: 8001)
  MIKE_PREVIEW_VERSIONS      Space-separated release tags (default: last 3 git tags)
  MIKE_PREVIEW_CURRENT       Version id for working tree (default: development)
  MIKE_PREVIEW_CURRENT_TITLE Title for the working-tree version (default: branch name)
  MIKE_PREVIEW_SKIP_DEVELOPMENT  Skip working-tree deploy (default: false)
  MIKE_COMMIT_MESSAGE          Optional git commit message (supports {version}, {branch})

Examples:
  $0
  $0 --fresh 2.0.0 2.1.0 2.2.0
  $0 --no-development --fresh 2.0.0 2.1.0 2.2.0
  $0 --serve-only
EOF
  exit 1
}

default_versions() {
  git -C "$REPO_ROOT" tag -l 'v*' --sort=v:refname | sed 's/^v//' | tail -3
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fresh) FRESH=true; shift ;;
    --deploy-only) DEPLOY_ONLY=true; shift ;;
    --serve-only) SERVE_ONLY=true; shift ;;
    --no-development) SKIP_DEVELOPMENT=true; shift ;;
    -h | --help) usage ;;
    -*)
      echo "error: unknown option: $1" >&2
      usage
      ;;
    *)
      VERSIONS+=("${1#v}")
      shift
      ;;
  esac
done

if ((${#VERSIONS[@]} == 0)); then
  if [[ -n "${MIKE_PREVIEW_VERSIONS:-}" ]]; then
    # shellcheck disable=SC2206
    VERSIONS=($MIKE_PREVIEW_VERSIONS)
  else
    while IFS= read -r version; do
      [[ -n "$version" ]] && VERSIONS+=("$version")
    done < <(default_versions)
  fi
fi

resolve_mike
resolve_zensical
export PATH="${DOCS_SITE_DIR}/.venv/bin:${PATH}"
export MIKE_BRANCH="$BRANCH"

restore_web_docs() {
  if git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$REPO_ROOT" checkout HEAD -- .web-docs 2>/dev/null || true
  fi
}

latest_version() {
  local version
  for version in "${VERSIONS[@]}"; do
    version="${version#v}"
    if git -C "$REPO_ROOT" rev-parse "v${version}" >/dev/null 2>&1; then
      printf '%s\n' "$version"
    fi
  done | sort -V | tail -1
}

development_title() {
  if [[ -n "$CURRENT_TITLE" ]]; then
    printf '%s' "$CURRENT_TITLE"
    return
  fi
  local branch
  branch="$(git -C "$REPO_ROOT" branch --show-current 2>/dev/null || true)"
  if [[ -n "$branch" ]]; then
    printf '%s' "$branch"
  else
    printf 'branch'
  fi
}

ensure_preview_branch_ready() {
  if ! git -C "$REPO_ROOT" rev-parse --verify "$BRANCH" >/dev/null 2>&1; then
    echo "error: preview branch '${BRANCH}' does not exist." >&2
    echo "Deploy first, e.g.:" >&2
    echo "  make docs-serve-mike VERSIONS=\"2.0.0 2.1.0 2.1.1 2.1.2 2.2.0\"" >&2
    if [[ "$BRANCH" != "docs-preview" ]]; then
      echo "Or use the default local preview branch:" >&2
      echo "  unset MIKE_PREVIEW_BRANCH" >&2
      echo "  make docs-serve-mike-only" >&2
    fi
    exit 1
  fi

  if ! git -C "$REPO_ROOT" cat-file -e "${BRANCH}:versions.json" 2>/dev/null; then
    echo "error: branch '${BRANCH}' has no mike versions (missing versions.json)." >&2
    echo "Deploy to this branch before serving, e.g.:" >&2
    echo "  MIKE_PREVIEW_BRANCH=${BRANCH} make docs-serve-mike VERSIONS=\"2.0.0 2.1.0 2.1.1 2.1.2 2.2.0\"" >&2
    exit 1
  fi

  echo "Serving from local branch '${BRANCH}'."
  mike_cmd list
  echo ""
}

deploy_tagged_version() {
  local version="$1"
  local with_latest="$2"
  local tag="v${version}"

  if ! git -C "$REPO_ROOT" rev-parse "$tag" >/dev/null 2>&1; then
    echo "warning: skipping ${version} (tag ${tag} not found)" >&2
    return 0
  fi

  echo "Deploying ${version} from ${tag}..."
  sync_web_docs_from_tag "$tag"
  rm -rf "${DOCS_SITE_DIR}/site"
  INCLUDE_EXTRA=true "${SCRIPT_DIR}/prepare-docs.sh"
  MIKE_COMMIT_VERSION="$version"
  if [[ "$with_latest" == "true" ]]; then
    mike_cmd deploy -t "v${version}" --update-aliases "$version" latest
  else
    mike_cmd deploy -t "v${version}" "$version"
  fi
}

deploy_tagged_versions() {
  local latest version
  local -a sorted_versions=()
  latest="$(latest_version)"
  if [[ -z "$latest" ]]; then
    echo "warning: no tagged versions to deploy" >&2
    return 0
  fi

  while IFS= read -r version; do
    [[ -n "$version" ]] && sorted_versions+=("$version")
  done < <(printf '%s\n' "${VERSIONS[@]}" | sed 's/^v//' | sort -V)

  for version in "${sorted_versions[@]}"; do
    if [[ "$version" == "$latest" ]]; then
      deploy_tagged_version "$version" true
    else
      deploy_tagged_version "$version" false
    fi
  done

  mike_cmd set-default latest
}

deploy_development() {
  echo "Deploying current working tree as ${CURRENT}..."
  restore_web_docs
  rm -rf "${DOCS_SITE_DIR}/site"
  INCLUDE_EXTRA=true "${SCRIPT_DIR}/prepare-docs.sh"
  MIKE_COMMIT_VERSION="$CURRENT"
  mike_cmd delete next 2>/dev/null || true
  mike_cmd deploy -t "$(development_title)" "$CURRENT"
}

if [[ "$SERVE_ONLY" != "true" ]]; then
  if [[ "$FRESH" == "true" ]]; then
    echo "Resetting local preview branch ${BRANCH}..."
    git -C "$REPO_ROOT" branch -D "$BRANCH" 2>/dev/null || true
  fi

  trap restore_web_docs EXIT

  deploy_tagged_versions
  if [[ "$SKIP_DEVELOPMENT" == "true" ]]; then
    mike_cmd delete "$CURRENT" 2>/dev/null || true
  else
    deploy_development
  fi

  trap - EXIT

  echo ""
  mike_cmd list
  echo ""
fi

if [[ "$DEPLOY_ONLY" == "true" ]]; then
  echo "Preview branch ${BRANCH} is ready. Run: make docs-serve-mike-only"
  exit 0
fi

if [[ "$SERVE_ONLY" == "true" ]]; then
  ensure_preview_branch_ready
fi

echo "Serving versioned docs at http://localhost:${PORT}/"
echo "Press Ctrl+C to stop."
mike_cmd serve -a "localhost:${PORT}"

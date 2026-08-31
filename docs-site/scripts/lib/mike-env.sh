#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Shared mike and Zensical binary resolution for versioned docs deploy/preview.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"

resolve_mike() {
  MIKE="${DOCS_SITE_DIR}/.venv/bin/mike"
  if [[ ! -x "$MIKE" ]]; then
    MIKE="$(command -v mike || true)"
  fi
  if [[ -z "${MIKE:-}" || ! -x "$MIKE" ]]; then
    echo "error: mike not found; run: make docs-deps" >&2
    exit 1
  fi
  # mike invokes zensical by name; keep the venv bin first on PATH.
  local bindir
  bindir="$(cd "$(dirname "$MIKE")" && pwd)"
  export PATH="${bindir}:${PATH}"
}

resolve_zensical() {
  ZENSICAL="${DOCS_SITE_DIR}/.venv/bin/zensical"
  if [[ ! -x "$ZENSICAL" ]]; then
    ZENSICAL="$(command -v zensical || true)"
  fi
  if [[ -z "${ZENSICAL:-}" || ! -x "$ZENSICAL" ]]; then
    echo "error: zensical not found; run: make docs-deps" >&2
    exit 1
  fi
}

mike_config_file() {
  if [[ -f "${DOCS_SITE_DIR}/zensical.build.toml" ]]; then
    printf '%s\n' "zensical.build.toml"
  else
    printf '%s\n' "zensical.toml"
  fi
}

mike_commit_args() {
  MIKE_CMD_COMMIT_ARGS=()

  if [[ -z "${MIKE_COMMIT_MESSAGE:-}" ]]; then
    return 0
  fi

  local msg="$MIKE_COMMIT_MESSAGE"
  if [[ -n "${MIKE_COMMIT_VERSION:-}" ]]; then
    msg="${msg//\{version\}/$MIKE_COMMIT_VERSION}"
  fi
  msg="${msg//\{branch\}/${MIKE_BRANCH:-gh-pages}}"
  MIKE_CMD_COMMIT_ARGS=(-m "$msg")
}

mike_cmd() {
  local subcommand="$1"
  shift
  local branch="${MIKE_BRANCH:-gh-pages}"
  local config
  config="$(mike_config_file)"
  MIKE_CMD_COMMIT_ARGS=()
  case "$subcommand" in
    deploy | set-default | delete | rename | retitle)
      mike_commit_args
      ;;
  esac
  (
    cd "$DOCS_SITE_DIR"
    PATH="${DOCS_SITE_DIR}/.venv/bin:${PATH}"
    export PATH
    if ((${#MIKE_CMD_COMMIT_ARGS[@]})); then
      "$MIKE" "$subcommand" -b "$branch" -F "$config" "${MIKE_CMD_COMMIT_ARGS[@]}" "$@"
    else
      "$MIKE" "$subcommand" -b "$branch" -F "$config" "$@"
    fi
  )
}

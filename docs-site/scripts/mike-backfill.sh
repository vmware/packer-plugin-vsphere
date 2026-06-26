#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Backfill multiple documentation versions to GitHub Pages with mike.

# Usage:
#   mike-backfill.sh [VERSION ...]
#
# Publishes documentation for one or more release tags to gh-pages. Each
# version is deployed via mike-deploy.sh. The highest version receives the
# latest alias.
#
# When no VERSION arguments are given, prompts for a space-separated list
# and asks for confirmation before pushing.
#
# Environment:
#   MIKE_BACKFILL_VERSIONS  Space-separated versions (skips the version prompt)
#   MIKE_BRANCH             Remote git branch for mike (default: gh-pages)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

# shellcheck source=lib/paths.sh
source "${LIB_DIR}/paths.sh"

usage() {
  cat <<EOF
usage: $0 [VERSION ...]

  Publish documentation for one or more release tags to the gh-pages branch.

  When no VERSION arguments are given, prompts for a space-separated list
  (e.g. 2.0.0 2.1.0 2.2.0). The highest version receives the latest alias.

  --help    Show this help

Environment:
  MIKE_BACKFILL_VERSIONS  Space-separated versions; skips the version prompt
  MIKE_BRANCH             Remote branch for mike (default: gh-pages)

Examples:
  make docs-backfill
  $0 2.0.0 2.1.0 2.2.0
  MIKE_BACKFILL_VERSIONS="2.0.0 2.2.0" $0
EOF
  exit 1
}

declare -a VERSIONS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
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

if ((${#VERSIONS[@]} == 0)) && [[ -n "${MIKE_BACKFILL_VERSIONS:-}" ]]; then
  # shellcheck disable=SC2206
  VERSIONS=($MIKE_BACKFILL_VERSIONS)
fi

if ((${#VERSIONS[@]} == 0)); then
  echo "Recent release tags:"
  git -C "$REPO_ROOT" tag -l 'v*' --sort=v:refname | tail -5 | sed 's/^/  /' || true
  echo ""
  printf 'Versions to publish (space-separated, highest last for latest alias): '
  read -r line
  # shellcheck disable=SC2206
  VERSIONS=($line)
fi

if ((${#VERSIONS[@]} == 0)); then
  echo "error: no versions provided" >&2
  exit 1
fi

declare -a sorted_versions=()
while IFS= read -r version; do
  [[ -n "$version" ]] && sorted_versions+=("${version#v}")
done < <(printf '%s\n' "${VERSIONS[@]}" | sort -V)

latest="${sorted_versions[${#sorted_versions[@]} - 1]}"

echo ""
echo "Will deploy to gh-pages:"
for version in "${sorted_versions[@]}"; do
  if [[ "$version" == "$latest" ]]; then
    echo "  - ${version} (with latest alias)"
  else
    echo "  - ${version}"
  fi
done
echo ""
printf 'Continue? [y/N] '
read -r confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  echo "Aborted."
  exit 0
fi

restore_web_docs() {
  if git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$REPO_ROOT" checkout HEAD -- .web-docs 2>/dev/null || true
  fi
}

trap restore_web_docs EXIT

for version in "${sorted_versions[@]}"; do
  tag="v${version}"
  if ! git -C "$REPO_ROOT" rev-parse "$tag" >/dev/null 2>&1; then
    echo "error: tag ${tag} not found" >&2
    exit 1
  fi

  echo ""
  echo "Deploying ${version} from ${tag}..."
  if [[ "$version" == "$latest" ]]; then
    "${SCRIPT_DIR}/mike-deploy.sh" "$version" --update-latest
  else
    "${SCRIPT_DIR}/mike-deploy.sh" "$version"
  fi
done

trap - EXIT
restore_web_docs

echo ""
echo "Deployed documentation for the following versions: ${sorted_versions[*]}"

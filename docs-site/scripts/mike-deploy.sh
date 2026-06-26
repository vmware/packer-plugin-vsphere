#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Deploy a documentation version to GitHub Pages with mike.

# Usage:
#   mike-deploy.sh <version> [--update-latest]
#
# Stages documentation from the matching git tag (v<version>), builds with
# Zensical, and pushes the result to the gh-pages branch.
#
# Environment:
#   MIKE_BRANCH          Remote git branch for mike (default: gh-pages)
#   MIKE_COMMIT_MESSAGE  Optional git commit message for mike deploy/set-default
#   MIKE_COMMIT_VERSION  Expands {version} in MIKE_COMMIT_MESSAGE
#   WEB_DOCS_DIR         Source component READMEs (default: <repo>/.web-docs)
#   INCLUDE_EXTRA   Always true for this script (community shell, home injections)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

# shellcheck source=lib/paths.sh
source "${LIB_DIR}/paths.sh"
# shellcheck source=lib/mike-env.sh
source "${LIB_DIR}/mike-env.sh"
# shellcheck source=lib/sync-web-docs.sh
source "${LIB_DIR}/sync-web-docs.sh"

usage() {
  cat <<EOF
usage: $0 <version> [--update-latest]

  <version>         Release version without the v prefix (e.g. 2.2.0)
  --update-latest   Also set the latest alias and default version

Stages .web-docs from tag v<version>, applies the current docs-site shell
from extra/, builds, and pushes to gh-pages with mike.

Environment:
  MIKE_BRANCH          Remote branch for mike (default: gh-pages)
  MIKE_COMMIT_MESSAGE  Optional git commit message (supports {version}, {branch})
  MIKE_COMMIT_VERSION  Set automatically to the deployed version

Examples:
  $0 2.2.0
  $0 2.2.0 --update-latest
EOF
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help) usage ;;
    --update-latest)
      UPDATE_LATEST=true
      shift
      ;;
    -*)
      echo "error: unknown option: $1" >&2
      usage
      ;;
    *)
      if [[ -n "${VERSION:-}" ]]; then
        echo "error: unexpected argument: $1" >&2
        usage
      fi
      VERSION="${1#v}"
      shift
      ;;
  esac
done

UPDATE_LATEST="${UPDATE_LATEST:-false}"
[[ -n "${VERSION:-}" ]] || usage

TAG="v${VERSION}"
WEB_DOCS_SYNCED=false

restore_web_docs() {
  if [[ "$WEB_DOCS_SYNCED" == "true" ]] && git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$REPO_ROOT" checkout HEAD -- .web-docs 2>/dev/null || true
  fi
}

if git -C "$REPO_ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Staging documentation for ${VERSION} from ${TAG}..."
  sync_web_docs_from_tag "$TAG"
  WEB_DOCS_SYNCED=true
else
  echo "warning: ${TAG} not found; using current .web-docs" >&2
fi

trap restore_web_docs EXIT

rm -rf "${DOCS_SITE_DIR}/site"
INCLUDE_EXTRA=true "${SCRIPT_DIR}/prepare-docs.sh"

trap - EXIT
restore_web_docs

resolve_mike
export MIKE_BRANCH="${MIKE_BRANCH:-gh-pages}"
MIKE_COMMIT_VERSION="$VERSION"

if [[ "$UPDATE_LATEST" == "true" ]]; then
  mike_cmd deploy --push --update-aliases "$VERSION" latest
  mike_cmd set-default --push latest
else
  mike_cmd deploy --push "$VERSION"
fi

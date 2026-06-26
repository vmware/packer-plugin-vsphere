#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Replace .web-docs with the tree from a release tag.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"

sync_web_docs_from_tag() {
  local tag="$1"

  if ! git -C "$REPO_ROOT" rev-parse "$tag" >/dev/null 2>&1; then
    echo "error: tag ${tag} not found" >&2
    return 1
  fi

  rm -rf "${WEB_DOCS_DIR}"
  git -C "$REPO_ROOT" checkout "$tag" -- .web-docs
}

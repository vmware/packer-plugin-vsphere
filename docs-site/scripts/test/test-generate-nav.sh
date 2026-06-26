#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for Zensical navigation generation in staged markdown.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/../lib"
# shellcheck source=assertions.sh
source "${SCRIPT_DIR}/assertions.sh"
# shellcheck source=../lib/generate-nav.sh
source "${LIB_DIR}/generate-nav.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

WEB_DOCS_DIR="${tmpdir}/.web-docs"
mkdir -p "${WEB_DOCS_DIR}/components/builder/vsphere-iso"
printf '%s\n' '# Builder' >"${WEB_DOCS_DIR}/components/builder/vsphere-iso/README.md"

INCLUDE_EXTRA=true generate_nav >"${tmpdir}/with-extra.toml"
INCLUDE_EXTRA=false generate_nav >"${tmpdir}/without-extra.toml"

with_extra="$(cat "${tmpdir}/with-extra.toml")"
without_extra="$(cat "${tmpdir}/without-extra.toml")"

assert_contains "$with_extra" '"builders/index.md"' \
  "includes builders index when INCLUDE_EXTRA=true"
assert_not_contains "$without_extra" '"builders/index.md"' \
  "omits builders index when INCLUDE_EXTRA=false"
assert_not_contains "$without_extra" "Community" \
  "omits community nav when INCLUDE_EXTRA=false"

echo "All generate-nav tests passed."

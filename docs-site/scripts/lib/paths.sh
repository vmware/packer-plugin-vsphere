#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Shared paths and defaults for docs-site scripts.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${LIB_DIR}/.." && pwd)"
DOCS_SITE_DIR="$(cd "${SCRIPTS_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DOCS_SITE_DIR}/.." && pwd)"

WEB_DOCS_DIR="${WEB_DOCS_DIR:-${REPO_ROOT}/.web-docs}"
STAGING_DIR="${DOCS_SITE_DIR}/.build/docs"
BUILD_CONFIG="${DOCS_SITE_DIR}/zensical.build.toml"
INCLUDE_EXTRA="${INCLUDE_EXTRA:-true}"
HOME_TITLE="Packer Plugin for VMware vSphere"

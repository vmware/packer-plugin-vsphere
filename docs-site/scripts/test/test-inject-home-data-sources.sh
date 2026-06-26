#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for data sources section injection in staged markdown.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/../lib"
# shellcheck source=assertions.sh
source "${SCRIPT_DIR}/assertions.sh"
# shellcheck source=../lib/inject-home-data-sources.sh
source "${LIB_DIR}/inject-home-data-sources.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

WEB_DOCS_DIR="${tmpdir}/.web-docs"
STAGING_DIR="${tmpdir}/staging"
mkdir -p "${WEB_DOCS_DIR}/components/data-source/virtualmachine"
mkdir -p "$STAGING_DIR"

cat >"${WEB_DOCS_DIR}/components/data-source/virtualmachine/README.md" <<'EOF'
Type: `vsphere-virtualmachine`

This data source retrieves information about existing virtual machines from vSphere.

## Configuration Reference
EOF

cat >"${STAGING_DIR}/index.md" <<'EOF'
### Components

The plugin includes builders and post-processors for creating virtual machine images.

#### Builders

- [vsphere-iso](builders/vsphere-iso/) - Builds from ISO.

#### Post-Processors

- [vsphere](../post-processors/vsphere/) - Uploads artifacts.

### Requirements

Plugin requirements go here.
EOF

inject_home_data_sources "${STAGING_DIR}/index.md"
content="$(cat "${STAGING_DIR}/index.md")"

assert_contains "$content" "builders, post-processors, and data sources" \
  "updates components intro when data sources exist"
assert_contains "$content" "#### Data Sources" \
  "adds Data Sources heading"
assert_contains "$content" "[virtualmachine](data-sources/virtualmachine/)" \
  "lists data source with site-relative link"
assert_contains "$content" "retrieves information about existing virtual machines" \
  "includes data source summary"
assert_contains "$content" $'#### Post-Processors\n\n- [vsphere]' \
  "preserves post-processors section"
assert_contains "$content" $'#### Data Sources\n\n- [virtualmachine]' \
  "places data sources after post-processors"

WEB_DOCS_DIR="${tmpdir}/.web-docs-empty"
mkdir -p "${WEB_DOCS_DIR}/components/builder/vsphere-iso"
cat >"${WEB_DOCS_DIR}/components/builder/vsphere-iso/README.md" <<'EOF'
Builder docs.
EOF

cat >"${STAGING_DIR}/index-no-ds.md" <<'EOF'
### Components

The plugin includes builders and post-processors for creating virtual machine images.

#### Post-Processors

- [vsphere](../post-processors/vsphere/) - Uploads artifacts.

### Requirements
EOF

inject_home_data_sources "${STAGING_DIR}/index-no-ds.md"
content="$(cat "${STAGING_DIR}/index-no-ds.md")"

assert_not_contains "$content" "#### Data Sources" \
  "skips injection when no data sources exist"
assert_contains "$content" "builders and post-processors" \
  "leaves intro unchanged when no data sources exist"

echo "All inject-home-data-sources tests passed."

#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for internal link and anchor fixes in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/fix-internal-links.sh
source "${LIB_DIR}/fix-internal-links.sh"

test_same_page_link() {
  local input='[vApp](builders/vsphere-iso.md#vapp-options-configuration)'
  local output
  output="$(fix_internal_links "$input" "builders/vsphere-iso.md")"
  assert_contains "$output" '[vApp](#vapp-options-configuration)' "same-page .md link"
  assert_not_contains "$output" 'builders/vsphere-iso.md' "no absolute same-page path"
}

test_sibling_post_processor_link() {
  local input='[vSphere](post-processors/vsphere.md)'
  local output
  output="$(fix_internal_links "$input" "post-processors/vsphere-template.md")"
  assert_contains "$output" '[vSphere](vsphere/)' "sibling post-processor link"
}

test_content_library_anchor() {
  local input='[content library import configuration](#content-library-import-configuration)'
  local output
  output="$(fix_internal_links "$input" "builders/vsphere-iso.md")"
  assert_contains "$output" '#content-library-configuration' "content library anchor"
}

test_location_field_anchor() {
  local input='Defaults to [`cluster`](#cluster).'
  local output
  output="$(fix_internal_links "$input" "builders/vsphere-iso.md")"
  assert_contains "$output" '#location-configuration' "location field anchor"
}

test_customization_anchor() {
  local input='Refer to the [Linux options](#linux-options) section.'
  local output
  output="$(fix_internal_links "$input" "builders/vsphere-clone.md")"
  assert_contains "$output" '#linux-customization-settings' "linux options anchor"
}

test_ovf_anchor() {
  local input='use an OVF template instead by setting the [ovf](#ovf) option'
  local output
  output="$(fix_internal_links "$input" "builders/vsphere-iso.md")"
  assert_contains "$output" '[ovf](#content-library-configuration)' "ovf field anchor"
}

test_home_page_component_link() {
  local input='[vsphere-iso](builders/vsphere-iso.md)'
  local output
  output="$(fix_internal_links "$input" "index.md")"
  assert_contains "$output" '[vsphere-iso](builders/vsphere-iso/)' "home page component link"
  assert_not_contains "$output" '../builders/' "home page link does not escape site root"
}

main() {
  test_same_page_link
  test_sibling_post_processor_link
  test_content_library_anchor
  test_location_field_anchor
  test_customization_anchor
  test_ovf_anchor
  test_home_page_component_link
  echo "All fix-internal-links tests passed."
}

main "$@"

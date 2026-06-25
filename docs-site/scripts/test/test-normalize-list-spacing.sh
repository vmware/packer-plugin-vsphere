#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for collapsing and preserving blank lines between list item in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/strip-codegen-comments.sh
source "${LIB_DIR}/strip-codegen-comments.sh"
# shellcheck source=../lib/normalize-list-spacing.sh
source "${LIB_DIR}/normalize-list-spacing.sh"

test_strip_then_normalize_list_spacing() {
  local input
  input="$(cat <<'EOF'
- `cd_label` (string) - CD Label

<!-- End of code generated from the comments of the CDConfig struct -->

<!-- Code generated from the comments of the CDRomConfig struct; DO NOT EDIT MANUALLY -->

- `cdrom_type` (string) - Defaults to `ide`.
EOF
)"
  local output
  output="$(strip_codegen_comments "$input")"
  output="$(normalize_list_spacing "$output")"
  assert_not_contains "$output" "Code generated from" "codegen comments removed"
  assert_contains "$output" $'- `cd_label` (string) - CD Label\n- `cdrom_type`' "list items adjacent"
}

test_preserve_blank_after_code_block() {
  local input
  input="$(cat <<'EOF'
- `cd_files` (list) - Long item with code:

  ```hcl
  x = 1
  ```

- `cd_label` (string) - CD Label
EOF
)"
  local output
  output="$(normalize_list_spacing "$input")"
  assert_contains "$output" $'```\n\n- `cd_label`' "blank preserved after code block"
}

test_collapse_blank_between_simple_items() {
  local input
  input="$(cat <<'EOF'
- `cd_label` (string) - CD Label

- `cdrom_type` (string) - Defaults to `ide`.
EOF
)"
  local output
  output="$(normalize_list_spacing "$input")"
  assert_contains "$output" $'CD Label\n- `cdrom_type`' "blank removed between simple items"
  assert_not_contains "$output" $'CD Label\n\n- `cdrom_type`' "no double newline between items"
}

test_collapse_blank_after_nested_sublist() {
  local input
  input="$(cat <<'EOF'
    * `uuid` - UUID.

  For example, adding the following outputs the
  MAC addresses for each Ethernet device:
EOF
)"
  local output
  output="$(normalize_list_spacing "$input")"
  assert_contains "$output" $'UUID.\n  For example' "continuation follows nested sublist"
  assert_not_contains "$output" $'UUID.\n\n  For example' "no blank after nested sublist"
}

test_fix_sibling_list_breaks_after_tabs() {
  local input=$'    === "JSON"\n\n        ```json\n        {}\n        ```\n- `network_interface` (NetworkInterfaces) - Next field.'
  local output
  output="$(normalize_list_spacing "$input")"
  assert_contains "$output" $'        ```\n\n- `network_interface`' "blank line before sibling list item"
}

main() {
  test_strip_then_normalize_list_spacing
  test_preserve_blank_after_code_block
  test_collapse_blank_between_simple_items
  test_collapse_blank_after_nested_sublist
  test_fix_sibling_list_breaks_after_tabs
  echo "All normalize-list-spacing tests passed."
}

main "$@"

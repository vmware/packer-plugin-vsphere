#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for Zensical admonition conversion in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/convert-admonitions.sh
source "${LIB_DIR}/convert-admonitions.sh"
# shellcheck source=../lib/strip-codegen-comments.sh
source "${LIB_DIR}/strip-codegen-comments.sh"

test_block_note() {
  local input=$'-> **Note:** First line.\nSecond line.\n\nNext paragraph.'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" "!!! note" "block note header"
  assert_contains "$output" "    First line." "block note body"
  assert_not_contains "$output" "-> **Note:**" "legacy syntax removed"
}

test_list_nested_note() {
  local input=$'- insecure_connection (bool)\n  Defaults to false.\n  \n  -> **Note:** Helpful detail.\n  More detail.\n\n- next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "list nested admonition indented"
  assert_contains "$output" $'        Helpful detail.' "indented admonition body"
  assert_contains "$output" "More detail." "admonition continuation"
  assert_not_contains "$output" "<div class=\"admonition" "no html admonition in list"
}

test_disk_size_inline_note() {
  local input=$'- `disk_size` (int64) - Cannot be used with `linked_clone`.\n  -> **Note:** Only the primary disk size.\n  Additional disks are not supported.\n\n- next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "disk_size indented admonition"
  assert_contains "$output" "Only the primary disk size." "disk_size note body"
}

test_tilde_note() {
  local input=$'  ~> **Note:**  The full path must be provided.\n  For example, rp-packer.\n\n- next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "tilde note indented admonition"
  assert_contains "$output" "The full path must be provided." "tilde note body"
}

test_tilde_important_in_list() {
  local input=$'  ~> **Important:** When using tag blocks the category must exist.'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! warning' "important in list"
  assert_contains "$output" "When using tag blocks" "important body"
}

test_tilde_important_top_level() {
  local input=$'~> **Important:** When using tag blocks the category must exist.'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" "!!! warning" "important maps to warning at top level"
}

test_tilde_notes_with_list() {
  local input=$'~> **Notes:**\n  - Option A\n  - Option B\n\n### Next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" "!!! note" "top-level notes block"
  assert_contains "$output" "    - Option A" "sub-list in admonition body"
}

test_note_after_code_in_list() {
  local input=$'- item\n  \n  ```hcl\n  foo = 1\n  ```\n  \n  ~> **Note:** Configuration keys conflict.\n  are ignored.\n\n- next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "note after code block in list"
  assert_contains "$output" "Configuration keys conflict." "note body preserved"
}

test_bold_note_without_arrow() {
  local input=$'  **NOTE**: Guests using Windows with scp issues.\n  Use SFTP instead.\n\n- next'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "bold NOTE indented admonition"
  assert_contains "$output" "Guests using Windows" "bold NOTE body"
}

test_tip_syntax() {
  local input=$'  --> **Tip:** Use `none` to disable the manifest.'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! tip' "tip indented admonition"
}

test_skip_inline_code_example() {
  local input=$'  `-> **Note:** Refer to the docs`'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" '`-> **Note:**' "inline code example preserved"
}

test_skip_hcl_version_operator() {
  local input=$'      version = "~> 1"'
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" '"~> 1"' "hcl version operator preserved"
  assert_not_contains "$output" "!!!" "no admonition in hcl"
}

test_firmware_note_after_continuation() {
  local input
  input="$(cat <<'EOF'
- `firmware` (string) - The firmware for the virtual machine.

  The available options for this setting are: 'bios', 'efi', and
  'efi-secure'.

  -> **Note:** Use `efi-secure` for UEFI Secure Boot.

- next
EOF
)"
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'    !!! note' "firmware note converted"
  assert_contains "$output" "Use \`efi-secure\` for UEFI Secure Boot." "firmware note body"
  assert_not_contains "$output" $'machine.\n  \n  The available' "blank line removed after bullet"
  assert_not_contains "$output" "-> **Note:**" "legacy note syntax removed"
}

test_vapp_note_before_continuation() {
  local input
  input="$(cat <<'EOF'
- `properties` (map[string]string) - Supply configuration parameters.
  from an imported OVF or OVA file.

  -> **Note:** The only supported usage path for vApp properties.
  These generally come from an existing template.

  You cannot set values for vApp properties on scratch-built VMs.

- next
EOF
)"
  local output
  output="$(convert_admonitions "$input")"
  assert_contains "$output" $'        These generally come from an existing template.\n  You cannot set values' "continuation stays in list after note"
  assert_not_contains "$output" $'template.\n\n  You cannot' "no blank after list admonition"
}

test_http_ip_notes_after_list_item() {
  local input
  input="$(cat <<'EOF'
- `http_ip` (string) - The IP address to use for the HTTP server to serve the `http_directory`.

<!-- End of code generated from the comments of the BootConfig struct -->

~> **Notes:**
  - The options `http_bind_address` and `http_interface` are mutually exclusive.
  - Both `http_bind_address` and `http_interface` have higher priority than `http_ip`.

### Floppy Configuration
EOF
)"
  local output
  output="$(convert_admonitions "$(strip_codegen_comments "$input")")"
  assert_contains "$output" $'- `http_ip` (string) - The IP address' "http_ip list item preserved"
  assert_contains "$output" $'    !!! note' "notes nested under list item"
  assert_contains "$output" $'        - The options `http_bind_address`' "note body indented"
  assert_not_contains "$output" $'http_directory`.\n!!! note' "no top-level admonition header"
  assert_contains "$output" $'directory`.\n\n    !!! note' "blank before nested note after bullet"
}

main() {
  test_block_note
  test_list_nested_note
  test_disk_size_inline_note
  test_tilde_note
  test_tilde_important_in_list
  test_tilde_important_top_level
  test_tilde_notes_with_list
  test_note_after_code_in_list
  test_bold_note_without_arrow
  test_tip_syntax
  test_skip_inline_code_example
  test_skip_hcl_version_operator
  test_firmware_note_after_continuation
  test_vapp_note_before_continuation
  test_http_ip_notes_after_list_item
  echo "All admonition conversion tests passed."
}

main "$@"

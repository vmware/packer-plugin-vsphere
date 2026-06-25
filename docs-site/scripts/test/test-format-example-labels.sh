#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for HCL/JSON example label formatting in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/format-example-labels.sh
source "${LIB_DIR}/format-example-labels.sh"

test_top_level_bold() {
  local input=$'Intro text.\n\nHCL Example:\n\n```hcl\nfoo = 1\n```'
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" "**HCL Example:**" "top-level label bold"
  assert_not_contains "$output" $'  **HCL Example:**' "top-level label not indented"
}

test_plural_labels() {
  local input=$'HCL Examples:\n\nJSON Examples\n'
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" "**HCL Examples:**" "plural hcl label"
  assert_contains "$output" "**JSON Examples:**" "plural json label without colon normalized"
}

test_nested_after_bullet() {
  local input=$'- `tag` (string) - Filter tags.\n  \n  HCL Example:\n  \n  ```hcl\n  tag {}\n  ```'
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'    **HCL Example:**' "nested label indented under bullet"
  assert_contains "$output" $'    ```hcl' "nested code fence indented under bullet"
}

test_nested_after_bullet_continuation() {
  local input=$'  You cannot set values.\n  \n  JSON Example:\n  \n  ```json\n  {}\n  ```'
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'    **JSON Example:**' "nested after continuation text"
}

test_top_level_example_after_list_item() {
  local input
  input="$(cat <<'EOF'
- Examples are available in the [examples](https://github.com/example) directory.

HCL Example:

```hcl
source "vsphere-supervisor" "example" {}
```

JSON Example:

```json
{}
```
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'directory.\n\n**HCL Example:**' "example label not nested under list item"
  assert_not_contains "$output" $'    **HCL Example:**' "example label not indented under list"
}

test_skip_variant_labels() {
  local input=$'HCL Example with image import:\nJSON Example with image import:'
  local output
  output="$(format_example_labels "$input")"
  assert_not_contains "$output" "**HCL Example with image import:**" "variant label unchanged"
}

test_normalize_aliases() {
  local input
  input="$(cat <<'EOF'
In JSON:

Usage example (JSON):

  Usage example (HCL):

In HCL:

In HCL2:
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" "**JSON Example:**" "In JSON alias"
  assert_contains "$output" "**HCL Example:**" "In HCL alias"
  assert_not_contains "$output" "In HCL2:" "In HCL2 alias replaced"
  assert_not_contains "$output" "In JSON:" "legacy In JSON removed"
  assert_not_contains "$output" "Usage example (JSON):" "legacy usage json removed"
  assert_not_contains "$output" "Usage example (HCL):" "legacy usage hcl removed"
}

test_aliases_in_same_list_item() {
  local input
  input="$(cat <<'EOF'
- `cd_files` ([]string) - Place files on a CD.

  Usage example (JSON):

  ```json
  "cd_label": "cidata"
  ```

  Usage example (HCL):

  ```hcl
  cd_label = "cidata"
  ```

- next
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" "    **JSON Example:**" "json alias in list"
  assert_contains "$output" "    **HCL Example:**" "hcl alias in list"
  assert_not_contains "$output" "Usage example" "all usage aliases replaced"
}

test_skip_code_fence() {
  local input=$'```text\nHCL Example:\n```'
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" 'HCL Example:' "label inside fence unchanged"
  assert_not_contains "$output" "**HCL Example:**" "label inside fence not bolded"
}

test_label_after_continuation_text() {
  local input
  input="$(cat <<'EOF'
- `cd_files` ([]string) - Place files on a CD.
  File globbing is allowed.

  Usage example (JSON):

  ```json
  "cd_label": "cidata"
  ```
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'globbing is allowed.\n\n    **JSON Example:**' "blank line before nested label"
  assert_not_contains "$output" $'allowed.\n    **JSON Example:**' "label not inline with text"
}

test_vapp_properties_examples() {
  local input
  input="$(cat <<'EOF'
- `properties` (map[string]string) - Supply configuration parameters.

  Omitting the `vapp` block entirely disables vApp support. Including at least one property enables vApp
  options for scratch-built virtual machines and creates any listed keys that do not already exist.

  HCL Example:

  ```hcl
  vapp { properties = {} }
  ```

  JSON Example:

  ```json
  "vapp": {}
  ```

- next
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" "configuration parameters." "bullet continuation preserved"
  assert_contains "$output" "Omitting the \`vapp\` block" "vapp note stays in list item"
  assert_contains "$output" $'exist.\n\n    **HCL Example:**' "hcl example on its own line"
  assert_contains "$output" "    **JSON Example:**" "json example nested in list"
}

test_vapp_note_before_examples() {
  local input
  input="$(cat <<'EOF'
- `properties` (map[string]string) - Supply configuration parameters.
  from an imported OVF or OVA file.

    !!! note
        The only supported usage path for vApp properties.
        These generally come from an existing template.

  You cannot set values for vApp properties on scratch-built VMs.
  that lack a vApp configuration, or on property keys that do not exist.

  HCL Example:

  ```hcl
  vapp { properties = {} }
  ```

  JSON Example:

  ```json
  "vapp": {}
  ```

- next
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'        These generally come from an existing template.\n  You cannot set values' "continuation stays in list after note"
  assert_contains "$output" $'exist.\n\n    **HCL Example:**' "blank before nested hcl example"
  assert_contains "$output" "    **JSON Example:**" "json example nested in list"
}

test_preserve_blank_before_nested_admonition_after_continuation() {
  local input
  input="$(cat <<'EOF'
- `firmware` (string) - The firmware for the virtual machine.
  The available options are: `bios` and `efi`.
    !!! note
        Use `efi-secure` for UEFI Secure Boot.
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'`efi`.\n\n    !!! note' "blank before note after continuation"
}

test_blank_before_nested_admonition_after_bullet() {
  local input
  input="$(cat <<'EOF'
- `http_ip` (string) - The IP address to use for the HTTP server.
    !!! note
        - Option A
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'HTTP server.\n\n    !!! note' "blank before note after bullet"
}

test_examples_after_block_admonition() {
  local input
  input="$(cat <<'EOF'
Both formats can be used together.

!!! warning

    When using `tag` blocks with `category` and `name`, the tag `category` must already exist
    in vSphere and be associable with virtual machines. The plugin will create tags within existing
    categories if they do not exist and the account context used to run the build has the appropriate
    privileges.

HCL Example:

```hcl
tag {}
```

JSON Example:

```json
{}
```
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'privileges.\n\n**HCL Example:**\n\n```hcl' "examples after admonition stay top-level"
  assert_not_contains "$output" $'    **HCL Example:**' "examples not nested under admonition body"
}

test_nested_fence_preserves_inner_indentation() {
  local input
  input="$(cat <<'EOF'
- `properties` (map[string]string) - Supply configuration parameters.

  HCL Example:

  ```hcl
  vapp {
    properties = {
      hostname = var.hostname
    }
  }
  ```

  JSON Example:

  ```json
  "vapp": {
    "properties": {
      "hostname": "example"
    }
  }
  ```
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  assert_contains "$output" $'    vapp {\n      properties = {' "hcl inner indent preserved in list"
  assert_contains "$output" $'        hostname = var.hostname' "hcl deep indent preserved in list"
  assert_contains "$output" $'    "vapp": {\n      "properties": {' "json inner indent preserved in list"
}

main() {
  test_top_level_bold
  test_plural_labels
  test_nested_after_bullet
  test_nested_after_bullet_continuation
  test_top_level_example_after_list_item
  test_skip_variant_labels
  test_normalize_aliases
  test_aliases_in_same_list_item
  test_label_after_continuation_text
  test_vapp_properties_examples
  test_vapp_note_before_examples
  test_preserve_blank_before_nested_admonition_after_continuation
  test_blank_before_nested_admonition_after_bullet
  test_examples_after_block_admonition
  test_nested_fence_preserves_inner_indentation
  test_skip_code_fence
  echo "All example label formatting tests passed."
}

main "$@"

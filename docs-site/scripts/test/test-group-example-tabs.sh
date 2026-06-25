#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for grouping HCL/JSON examples into content tabs in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/group-example-tabs.sh
source "${LIB_DIR}/group-example-tabs.sh"
# shellcheck source=../lib/format-example-labels.sh
source "${LIB_DIR}/format-example-labels.sh"

test_top_level_pair_hcl_first() {
  local input=$'**HCL Example:**\n\n```hcl\nfoo = 1\n```\n\n**JSON Example:**\n\n```json\n{}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" '=== "HCL"' "hcl tab present"
  assert_contains "$output" '=== "JSON"' "json tab present"
  assert_not_contains "$output" "**HCL Example:**" "label removed"
  assert_contains "$output" $'=== "HCL"\n\n    ```hcl' "hcl fence indented under hcl tab"
}

test_top_level_pair_json_first() {
  local input=$'**JSON Example:**\n\n```json\n{}\n```\n\n**HCL Example:**\n\n```hcl\nfoo = 1\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'=== "HCL"\n\n    ```hcl' "hcl tab first even when json label first"
  assert_contains "$output" $'=== "JSON"\n\n    ```json' "json tab second"
}

test_nested_list_pair() {
  local input=$'    **HCL Example:**\n    ```hcl\n    foo = 1\n    ```\n\n    **JSON Example:**\n    ```json\n    {}\n    ```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'    === "HCL"' "nested hcl tab indented"
  assert_contains "$output" $'    === "JSON"' "nested json tab indented"
  assert_contains "$output" $'        ```hcl' "nested hcl fence indented under tab"
  assert_contains "$output" $'        ```json' "nested json fence indented under tab"
}

test_nested_solo_hcl_keeps_fence_indent() {
  local input=$'    **HCL Example:**\n    ```hcl\n    floppy_content = {\n      "meta-data" = "x"\n    }\n    ```\n\n- next'
  local output
  output="$(group_example_tabs "$input")"
  assert_not_contains "$output" "**HCL Example:**" "solo label removed"
  assert_contains "$output" $'    ```hcl\n    floppy_content' "fence stays at list indent"
  assert_not_contains "$output" $'        ```hcl' "fence not double-indented"
}

test_single_hcl_unchanged() {
  local input=$'**HCL Example:**\n\n```hcl\nfoo = 1\n```\n\nMore text.'
  local output
  output="$(group_example_tabs "$input")"
  assert_not_contains "$output" "**HCL Example:**" "single label removed"
  assert_contains "$output" '```hcl' "single fence kept"
  assert_not_contains "$output" '=== "HCL"' "no tabs for single example"
}

test_skip_inside_fence() {
  local input=$'```text\n**HCL Example:**\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" "**HCL Example:**" "label inside fence unchanged"
  assert_not_contains "$output" '=== "HCL"' "no tabs inside fence"
}

test_plural_labels() {
  local input=$'**HCL Examples:**\n\n```hcl\nfoo = 1\n```\n\n**JSON Examples:**\n\n```json\n{}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" '=== "HCL"' "plural labels grouped"
  assert_contains "$output" '=== "JSON"' "plural labels grouped"
}

test_nested_preserves_inner_indentation() {
  local input=$'    **HCL Example:**\n    ```hcl\n    vapp {\n      properties = {\n        hostname = var.hostname\n      }\n    }\n    ```\n\n    **JSON Example:**\n    ```json\n    "vapp": {\n      "properties": {\n        "hostname": "example"\n      }\n    }\n    ```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'      properties = {' "hcl nested block indent preserved"
  assert_contains "$output" $'        hostname = var.hostname' "hcl deep indent preserved"
  assert_contains "$output" $'      "properties": {' "json nested block indent preserved"
}

test_top_level_nested_hcl_indent() {
  local input=$'**HCL Example:**\n\n```hcl\nsource "vsphere-iso" "example" {\n  customize {\n    linux_options {\n      host_name = "foo"\n    }\n  }\n}\n```\n\n**JSON Example:**\n\n```json\n{}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'      host_name = "foo"' "nested hcl indent preserved in top-level tab"
  assert_contains "$output" $'    linux_options {' "hcl block indent preserved in top-level tab"
}

test_preserves_inner_indentation() {
  local input=$'**HCL Example:**\n\n```hcl\nfoo {\n  bar = 1\n}\n```\n\n**JSON Example:**\n\n```json\n{\n  "baz": 2\n}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'  bar = 1' "hcl inner indent preserved"
  assert_contains "$output" $'  "baz": 2' "json inner indent preserved"
}

test_multi_fence_section_separate_blocks() {
  local input=$'**HCL Examples:**\n\n```hcl\ncustomize {\n  windows_sysprep_text = file("a.xml")\n}\n```\n\n```hcl\ncustomize {\n  windows_sysprep_text = templatefile("a.xml", {})\n}\n```\n\n**JSON Examples:**\n\n```json\n{"customize": {}}\n```\n\n```json\n{"customize": {"var1": "example"}}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'    customize {\n      windows_sysprep_text = file("a.xml")' "first hcl block"
  assert_contains "$output" $'    }\n    ```\n\n    ```hcl' "separate hcl fences in tab"
  assert_contains "$output" $'templatefile("a.xml", {})' "second hcl block"
  assert_not_contains "$output" $'          ```' "no over-indented fence markers in content"
}

test_iso_style_section_grouping() {
  local input=$'**JSON Example:**\n\n```json\n{}\n```\n\n```json\n{"a":1}\n```\n\n**HCL Example:**\n\n```hcl\nfoo = 1\n```\n\n```hcl\nbar = 2\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" '=== "HCL"' "hcl tab in section group"
  assert_contains "$output" '=== "JSON"' "json tab in section group"
  assert_not_contains "$output" "**JSON Example:**" "json label removed"
  assert_not_contains "$output" "**HCL Example:**" "hcl label removed"
  assert_contains "$output" $'bar = 2' "second hcl block preserved"
}

test_mismatched_indent_pair() {
  local input=$'    **HCL Example:**\n    ```hcl\nfoo = 1\n```\n\n**JSON Example:**\n```json\n{}\n```'
  local output
  output="$(group_example_tabs "$input")"
  assert_contains "$output" $'    === "HCL"' "nested hcl tab"
  assert_contains "$output" $'    === "JSON"' "json tab paired across indents"
}

test_supervisor_examples_with_image_import_variant() {
  local input
  input="$(cat <<'EOF'
- Examples are available in the [examples](https://github.com/example) directory.

HCL Example:

```hcl
source "vsphere-supervisor" "example" {
  image_name = "ubuntu"
}
```

HCL Example with image import:

```hcl
source "vsphere-supervisor" "example" {
  import_source_url = "https://example.com/example.ovf"
}
```

JSON Example:

```json
{
  "builders": [{ "type": "vsphere-supervisor", "image_name": "ubuntu" }]
}
```

JSON Example with image import:

```json
{
  "builders": [{ "type": "vsphere-supervisor", "import_source_url": "https://example.com/example.ovf" }]
}
```
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  output="$(group_example_tabs "$output")"
  assert_contains "$output" $'=== "HCL"\n\n    ```hcl\n    source "vsphere-supervisor"' "basic hcl tab"
  assert_contains "$output" $'=== "JSON"\n\n    ```json' "basic json tab"
  assert_contains "$output" "**Example with image import:**" "variant heading before second tab group"
  assert_not_contains "$output" "HCL Example with image import:" "variant hcl label consumed"
}

test_cd_files_prose_not_swallowed() {
  local input
  input="$(cat <<'EOF'
- `cd_files` ([]string) - Place files on a CD.

  **JSON Example:**
  ```json
  "cd_files": ["a"]
  ```

  **HCL Example:**
  ```hcl
  cd_files = ["a"]
  ```

  The above will create a CD with two files.

  Since globbing is also supported,

  ```hcl
  cd_files = ["./dir/*"]
  ```

- next
EOF
)"
  local output
  output="$(format_example_labels "$input")"
  output="$(group_example_tabs "$output")"
  assert_contains "$output" "The above will create a CD with two files." "prose after examples preserved"
  assert_contains "$output" 'cd_files = ["./dir/*"]' "globbing example preserved"
}

main() {
  test_top_level_pair_hcl_first
  test_top_level_pair_json_first
  test_nested_list_pair
  test_top_level_nested_hcl_indent
  test_preserves_inner_indentation
  test_nested_preserves_inner_indentation
  test_nested_solo_hcl_keeps_fence_indent
  test_single_hcl_unchanged
  test_skip_inside_fence
  test_plural_labels
  test_multi_fence_section_separate_blocks
  test_iso_style_section_grouping
  test_mismatched_indent_pair
  test_supervisor_examples_with_image_import_variant
  test_cd_files_prose_not_swallowed
  echo "All group-example-tabs tests passed."
}

main "$@"

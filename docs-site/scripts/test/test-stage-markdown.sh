#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Integration tests for the staged markdown transform pipeline.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/stage-markdown.sh
source "${LIB_DIR}/stage-markdown.sh"

test_export_options_examples() {
  local input
  input="$(cat <<'EOF'
- `options` ([]string) - Advanced image export options. Available options include:
  * `mac` - MAC address is exported for each Ethernet device.
  * `uuid` - UUID is exported for the virtual machine.
  * `nodevicesubtypes` - Resource subtypes for CD/DVD drives, floppy
    drives, and SCSI controllers are not exported.

  For example, adding the following export configuration option outputs the
  MAC addresses for each Ethernet device in the OVF descriptor:

  HCL Example:

  ```hcl
  export { options = ["mac"] }
  ```

  JSON: Example:

  ```json
  "export": { "options": ["mac"] }
  ```

- `output_format` (string) - The output format.
EOF
)"
  local output
  output="$(transform_markdown "$input")"
  assert_contains "$output" "    * \`mac\`" "nested option list"
  assert_contains "$output" $'exported.\n  For example' "example text in list item"
  assert_contains "$output" '=== "HCL"' "hcl example tab"
  assert_contains "$output" '=== "JSON"' "json example tab"
  assert_not_contains "$output" "JSON: Example:" "legacy json label removed"
}

main() {
  test_export_options_examples
  echo "All stage-markdown integration tests passed."
}

main "$@"

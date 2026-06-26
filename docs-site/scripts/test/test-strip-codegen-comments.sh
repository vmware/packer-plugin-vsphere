#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for removing generated-code HTML comments from staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/strip-codegen-comments.sh
source "${LIB_DIR}/strip-codegen-comments.sh"

test_strip_codegen_comments() {
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
  assert_not_contains "$output" "Code generated from" "codegen comments removed"
  assert_contains "$output" '`cd_label`' "first list item preserved"
  assert_contains "$output" '`cdrom_type`' "second list item preserved"
}

main() {
  test_strip_codegen_comments
  echo "All strip-codegen-comments tests passed."
}

main "$@"

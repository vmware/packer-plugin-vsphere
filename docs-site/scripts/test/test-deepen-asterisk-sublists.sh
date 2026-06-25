#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for indenting asterisk sublists in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/deepen-asterisk-sublists.sh
source "${LIB_DIR}/deepen-asterisk-sublists.sh"

test_deepen_asterisk_sublists() {
  local input
  input="$(cat <<'EOF'
- `options` ([]string) - Available options include:
  * `mac` - MAC address.
  * `uuid` - UUID.
EOF
)"
  local output
  output="$(deepen_asterisk_sublists "$input")"
  assert_contains "$output" "    * \`mac\`" "asterisk sublists indented"
  assert_not_contains "$output" $'\n  * `mac`' "no two-space asterisk sublists"
}

main() {
  test_deepen_asterisk_sublists
  echo "All deepen-asterisk-sublists tests passed."
}

main "$@"

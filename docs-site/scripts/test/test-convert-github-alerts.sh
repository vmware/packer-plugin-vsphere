#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for GitHub alert to Zensical admonition conversion in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/convert-github-alerts.sh
source "${LIB_DIR}/convert-github-alerts.sh"

test_loose_tip_body() {
  local input=$'> [!TIP]
If you need help or have questions about using the plugin, please refer to the
[documentation](https://example.com) or open a discussion.

## Issues'
  local output
  output="$(convert_github_alerts "$input")"
  assert_contains "$output" "!!! tip" "tip admonition header"
  assert_contains "$output" "If you need help" "tip body preserved"
  assert_not_contains "$output" "[!TIP]" "github alert marker removed"
}

test_quoted_warning_body() {
  local input=$'> [!WARNING]
> Issues that do not follow the guidelines may be closed.

## Pull Requests'
  local output
  output="$(convert_github_alerts "$input")"
  assert_contains "$output" "!!! warning" "warning admonition header"
  assert_contains "$output" "Issues that do not follow" "warning body preserved"
}

test_important_and_tip_sequence() {
  local input=$'> [!IMPORTANT]
> - Ensure that you are using a recent version of the plugin.

> [!TIP]
> - Learn about formatting code on GitHub.

## Next'
  local output
  output="$(convert_github_alerts "$input")"
  assert_contains "$output" "!!! warning" "important maps to warning"
  assert_contains "$output" "!!! tip" "second alert converted"
  assert_contains "$output" "Learn about formatting" "tip list preserved"
}

main() {
  test_loose_tip_body
  test_quoted_warning_body
  test_important_and_tip_sequence
  echo "All GitHub alert conversion tests passed."
}

main "$@"

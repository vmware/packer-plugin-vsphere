#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Run all script unit tests for staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

run_test() {
  local script="$1"
  echo "==> ${script##*/}"
  bash "$script"
}

run_test "${TEST_DIR}/test-rewrite-integration-links.sh"
run_test "${TEST_DIR}/test-fix-internal-links.sh"
run_test "${TEST_DIR}/test-convert-admonitions.sh"
run_test "${TEST_DIR}/test-convert-github-alerts.sh"
run_test "${TEST_DIR}/test-format-example-labels.sh"
run_test "${TEST_DIR}/test-group-example-tabs.sh"
run_test "${TEST_DIR}/test-strip-codegen-comments.sh"
run_test "${TEST_DIR}/test-deepen-asterisk-sublists.sh"
run_test "${TEST_DIR}/test-repair-code-fences.sh"
run_test "${TEST_DIR}/test-normalize-list-spacing.sh"
run_test "${TEST_DIR}/test-inject-home-data-sources.sh"
run_test "${TEST_DIR}/test-generate-nav.sh"
run_test "${TEST_DIR}/test-stage-markdown.sh"

echo "All docs-site script tests passed."

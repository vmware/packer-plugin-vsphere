#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for closing unclosed fenced code blocks in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/repair-code-fences.sh
source "${LIB_DIR}/repair-code-fences.sh"

test_repair_unclosed_fences() {
  local input=$'  ```json\n  {\n    "customize": {\n      "windows_sysprep_text": "example"\n    }\n  }\n\n- `network_interface` (NetworkInterfaces) - Next field.'
  local output
  output="$(repair_unclosed_fences "$input")"
  assert_contains "$output" $'```\n\n- `network_interface`' "closing fence inserted before list item"
}

test_preserve_yaml_playbook_fence() {
  local input
  input="$(cat <<'EOF'
```yaml
---
# cleanup-playbook.yml
- name: Clean up source virtual machine
  hosts: default
  tasks:
    - name: Truncate machine id
      file:
        state: absent
```
EOF
)"
  local output
  output="$(repair_unclosed_fences "$input")"
  assert_contains "$output" $'```yaml\n---\n# cleanup-playbook.yml\n- name: Clean up' "yaml playbook stays inside fence"
  assert_not_contains "$output" $'---\n```\n\n#' "fence not closed at yaml document marker"
}

main() {
  test_repair_unclosed_fences
  test_preserve_yaml_playbook_fence
  echo "All repair-code-fences tests passed."
}

main "$@"

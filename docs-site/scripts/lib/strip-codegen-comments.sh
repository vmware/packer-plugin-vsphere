#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Remove generated-code HTML comments from staged markdown.

set -euo pipefail

strip_codegen_comments() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    s/^[ \t]*<!-- (?:End of )?[Cc]ode generated from.*?-->[ \t]*\n//mg;
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  strip_codegen_comments "$(cat "$1")"
fi

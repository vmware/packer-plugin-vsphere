#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Apply the markdown transform pipeline to staged documentation content.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"
# shellcheck source=strip-codegen-comments.sh
source "${LIB_DIR}/strip-codegen-comments.sh"
# shellcheck source=deepen-asterisk-sublists.sh
source "${LIB_DIR}/deepen-asterisk-sublists.sh"
# shellcheck source=convert-admonitions.sh
source "${LIB_DIR}/convert-admonitions.sh"
# shellcheck source=format-example-labels.sh
source "${LIB_DIR}/format-example-labels.sh"
# shellcheck source=repair-code-fences.sh
source "${LIB_DIR}/repair-code-fences.sh"
# shellcheck source=group-example-tabs.sh
source "${LIB_DIR}/group-example-tabs.sh"
# shellcheck source=normalize-list-spacing.sh
source "${LIB_DIR}/normalize-list-spacing.sh"
# shellcheck source=rewrite-integration-links.sh
source "${LIB_DIR}/rewrite-integration-links.sh"
# shellcheck source=fix-internal-links.sh
source "${LIB_DIR}/fix-internal-links.sh"

transform_markdown() {
  local content="$1"
  local dest_rel="${2:-}"
  content="$(strip_codegen_comments "$content")"
  content="$(deepen_asterisk_sublists "$content")"
  content="$(convert_admonitions "$content")"
  content="$(format_example_labels "$content")"
  content="$(repair_unclosed_fences "$content")"
  content="$(group_example_tabs "$content")"
  content="$(normalize_list_spacing "$content")"
  content="$(rewrite_integration_links "$content")"
  content="$(fix_internal_links "$content" "$dest_rel")"
  printf '%s' "$content"
}

stage_file() {
  local src="$1"
  local dest="$2"
  local title="${3:-}"
  mkdir -p "$(dirname "$dest")"
  local content
  content="$(transform_markdown "$(cat "$src")" "${dest#${STAGING_DIR}/}")"
  if [[ -n "$title" ]]; then
    {
      printf '%s\n' "---"
      printf 'title: %s\n' "$title"
      if [[ "$title" == "$HOME_TITLE" ]]; then
        printf 'icon: lucide/home\n'
      fi
      printf '%s\n' "---"
      printf '\n'
      printf '# %s\n\n' "$title"
      printf '%s' "$content"
    } >"$dest"
  else
    printf '%s' "$content" >"$dest"
  fi
}

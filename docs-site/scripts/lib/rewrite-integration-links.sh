#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Rewrites HashiCorp integration paths in staged documentation to relative site links.

set -euo pipefail

rewrite_integration_links() {
  local content="$1"
  printf '%s' "$content" | perl -pe '
    my %dirs = (
      "builder" => "builders",
      "post-processor" => "post-processors",
      "data-source" => "data-sources",
    );
    s{/packer/integrations/(?:vmware|hashicorp)/(?:vsphere|vmware)/latest/components/(builder|post-processor|data-source)/([^#)\s"]+)(#[^)\s"]*)?}{
      my ($type, $slug, $anchor) = ($1, $2, $3 // "");
      "$dirs{$type}/$slug.md$anchor"
    }ge;
    s{/packer/docs/}{https://developer.hashicorp.com/packer/docs/}g;
    s{/packer/integrations/}{https://developer.hashicorp.com/packer/integrations/}g;
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  rewrite_integration_links "$(cat "$1")"
fi

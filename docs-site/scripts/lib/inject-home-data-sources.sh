#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Inject a Data Sources section on the home page when components exist.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"
# shellcheck source=generate-nav.sh
source "${LIB_DIR}/generate-nav.sh"

component_home_summary() {
  local readme="$1"
  [[ -f "$readme" ]] || return 0

  perl -0777 -ne '
    my @lines = split /\n/, $_;
    my @summary;
    my $started = 0;
    for my $line (@lines) {
      next if $line =~ /^Type:/;
      next if $line =~ /^Artifact /;
      next if $line =~ /^## /;
      next if $line =~ /^\s*$/ && !$started;
      last if $line =~ /^\s*$/ && $started;
      if ($line =~ /\S/) {
        $started = 1;
        push @summary, $line;
      }
    }
    print join " ", @summary;
  ' "$readme"
}

inject_home_data_sources() {
  local index_file="$1"
  [[ -f "$index_file" ]] || return 0
  grep -q '^#### Data Sources$' "$index_file" && return 0

  local -a section_lines=()
  local type slug title readme summary
  while IFS=$'\t' read -r type slug title; do
    [[ "$type" == "data-sources" ]] || continue
    readme="${WEB_DOCS_DIR}/components/data-source/${slug}/README.md"
    summary="$(component_home_summary "$readme")"
    section_lines+=("- [${slug}](data-sources/${slug}/) -")
    if [[ -n "$summary" ]]; then
      section_lines+=("  ${summary}")
    fi
    section_lines+=("")
  done < <(discover_components | sort -t$'\t' -k3,3)

  ((${#section_lines[@]} == 0)) && return 0

  local data_sources_section
  data_sources_section="$(printf '%s\n' "#### Data Sources" "" "${section_lines[@]}")"
  export DATA_SOURCES_SECTION="$data_sources_section"

  perl -0777 -i -pe '
    s/builders and post-processors/builders, post-processors, and data sources/g;
    if (s/(#### Post-Processors\n\n.*?)(\n### )/$1\n\n$ENV{DATA_SOURCES_SECTION}$2/s) {
      next;
    }
    if (s/(#### Post-Processors\n\n.*)/$1\n\n$ENV{DATA_SOURCES_SECTION}/s) {
      next;
    }
  ' "$index_file"
}

#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Generate Zensical navigation from discovered component documentation.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"

component_display_name() {
  case "$1" in
    vsphere-iso) echo "vSphere ISO" ;;
    vsphere-clone) echo "vSphere Clone" ;;
    vsphere-supervisor) echo "vSphere Supervisor" ;;
    vsphere) echo "vSphere" ;;
    vsphere-template) echo "vSphere Template" ;;
    virtualmachine) echo "Virtual Machine" ;;
    *) echo "$1" ;;
  esac
}

discover_components() {
  local dir type slug
  for dir in builder post-processor data-source; do
    case "$dir" in
      builder) type="builders" ;;
      post-processor) type="post-processors" ;;
      data-source) type="data-sources" ;;
    esac
    if [[ -d "${WEB_DOCS_DIR}/components/${dir}" ]]; then
      for slug_dir in "${WEB_DOCS_DIR}/components/${dir}"/*; do
        [[ -d "$slug_dir" ]] || continue
        slug="$(basename "$slug_dir")"
        if [[ -f "${slug_dir}/README.md" ]]; then
          printf '%s\t%s\t%s\n' "$type" "$slug" "$(component_display_name "$slug")"
        fi
      done
    fi
  done
}

generate_nav() {
  local -a nav_lines=()
  nav_lines+=('nav = [')
  nav_lines+=('  { Home = "index.md" },')

  local builders=()
  local post_processors=()
  local data_sources=()

  while IFS=$'\t' read -r type slug title; do
    case "$type" in
      builders) builders+=("    { \"${title}\" = \"builders/${slug}.md\" },") ;;
      post-processors) post_processors+=("    { \"${title}\" = \"post-processors/${slug}.md\" },") ;;
      data-sources) data_sources+=("    { \"${title}\" = \"data-sources/${slug}.md\" },") ;;
    esac
  done < <(discover_components | sort -t$'\t' -k1,1 -k3,3)

  if ((${#builders[@]} > 0)); then
    nav_lines+=('  { Builders = [')
    if [[ "$INCLUDE_EXTRA" == "true" ]]; then
      nav_lines+=('    "builders/index.md",')
    fi
    nav_lines+=("${builders[@]}")
    nav_lines+=('  ]},')
  fi

  if ((${#post_processors[@]} > 0)); then
    nav_lines+=('  { "Post-Processors" = [')
    if [[ "$INCLUDE_EXTRA" == "true" ]]; then
      nav_lines+=('    "post-processors/index.md",')
    fi
    nav_lines+=("${post_processors[@]}")
    nav_lines+=('  ]},')
  fi

  if ((${#data_sources[@]} > 0)); then
    nav_lines+=('  { "Data Sources" = [')
    if [[ "$INCLUDE_EXTRA" == "true" ]]; then
      nav_lines+=('    "data-sources/index.md",')
    fi
    nav_lines+=("${data_sources[@]}")
    nav_lines+=('  ]},')
  fi

  if [[ "$INCLUDE_EXTRA" == "true" ]] && [[ -d "${DOCS_SITE_DIR}/extra/community" ]]; then
    nav_lines+=('  { Community = [')
    nav_lines+=('    "community/index.md",')
    nav_lines+=('    { Support = "community/support.md" },')
    nav_lines+=('    { Contributing = "community/contributing.md" },')
    nav_lines+=('    { "Code of Conduct" = "community/code-of-conduct.md" },')
    nav_lines+=('    { Releases = "https://github.com/vmware/packer-plugin-vsphere/releases" },')
    nav_lines+=('    { Discussions = "https://github.com/vmware/packer-plugin-vsphere/discussions" },')
    nav_lines+=('    { "Search Issues" = "https://github.com/vmware/packer-plugin-vsphere/issues" },')
    nav_lines+=('    { "Open an Issue" = "https://github.com/vmware/packer-plugin-vsphere/issues/new/choose" },')
    nav_lines+=('    { License = "community/license.md" },')
    nav_lines+=('  ]},')
  fi

  nav_lines+=(']')
  printf '%s\n' "${nav_lines[@]}"
}

#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Stage .web-docs and extra content for Zensical, rewrite links, and emit nav config.

# Usage:
#   prepare-docs.sh [--help]
#
# Writes staged markdown to docs-site/.build/docs and generates
# docs-site/zensical.build.toml for Zensical.
#
# Environment:
#   INCLUDE_EXTRA   Include community pages, home injections, section indexes (default: true)
#   WEB_DOCS_DIR    Source component READMEs (default: <repo>/.web-docs)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

# shellcheck source=lib/paths.sh
source "${LIB_DIR}/paths.sh"
# shellcheck source=lib/stage-markdown.sh
source "${LIB_DIR}/stage-markdown.sh"
# shellcheck source=lib/generate-nav.sh
source "${LIB_DIR}/generate-nav.sh"
# shellcheck source=lib/inject-home-sections.sh
source "${LIB_DIR}/inject-home-sections.sh"
# shellcheck source=lib/inject-home-data-sources.sh
source "${LIB_DIR}/inject-home-data-sources.sh"

usage() {
  cat <<EOF
usage: $0 [--help]

  Stage .web-docs and docs-site/extra content for Zensical.

  Output:
    docs-site/.build/docs/     Staged markdown and assets
    docs-site/zensical.build.toml   Generated Zensical config with nav

  Transforms component READMEs (strip codegen comments, fix lists and links),
  injects home-page snippets, copies community pages, and builds navigation
  from discovered components.

Environment:
  INCLUDE_EXTRA   true|false — site shell from extra/ (default: true)
  WEB_DOCS_DIR    Path to component README sources (default: <repo>/.web-docs)

Examples:
  make docs-prepare
  INCLUDE_EXTRA=true $0
  WEB_DOCS_DIR=/path/to/.web-docs $0
EOF
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help) usage ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      ;;
  esac
done

main() {
  if [[ ! -d "$WEB_DOCS_DIR" ]]; then
    echo "error: web docs directory not found: ${WEB_DOCS_DIR}" >&2
    exit 1
  fi

  has_component_docs() {
    local component_dir="$1"
    local slug_dir
    [[ -d "${WEB_DOCS_DIR}/components/${component_dir}" ]] || return 1
    for slug_dir in "${WEB_DOCS_DIR}/components/${component_dir}"/*; do
      [[ -d "$slug_dir" ]] || continue
      if [[ -f "${slug_dir}/README.md" ]]; then
        return 0
      fi
    done
    return 1
  }

  rm -rf "${DOCS_SITE_DIR}/.build"
  mkdir -p "${STAGING_DIR}/assets"

  if [[ -f "${DOCS_SITE_DIR}/assets/header-logo.png" ]]; then
    cp "${DOCS_SITE_DIR}/assets/header-logo.png" "${STAGING_DIR}/assets/header-logo.png"
  fi

  if [[ -f "${DOCS_SITE_DIR}/stylesheets/extra.css" ]]; then
    mkdir -p "${STAGING_DIR}/stylesheets"
    cp "${DOCS_SITE_DIR}/stylesheets/extra.css" "${STAGING_DIR}/stylesheets/extra.css"
  fi

  if [[ -d "${DOCS_SITE_DIR}/javascripts" ]]; then
    mkdir -p "${STAGING_DIR}/javascripts"
    cp "${DOCS_SITE_DIR}/javascripts/"*.js "${STAGING_DIR}/javascripts/" 2>/dev/null || true
  fi

  if [[ -f "${REPO_ROOT}/LICENSE" ]]; then
    cp "${REPO_ROOT}/LICENSE" "${STAGING_DIR}/LICENSE"
  fi

  if [[ -f "${WEB_DOCS_DIR}/README.md" ]]; then
    stage_file "${WEB_DOCS_DIR}/README.md" "${STAGING_DIR}/index.md" "${HOME_TITLE}"
    if [[ "$INCLUDE_EXTRA" == "true" ]]; then
      inject_intro_upper "${STAGING_DIR}/index.md"
      inject_home_data_sources "${STAGING_DIR}/index.md"
      inject_intro_lower "${STAGING_DIR}/index.md"
    fi
  fi

  local dir type slug_dir slug
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
          stage_file "${slug_dir}/README.md" "${STAGING_DIR}/${type}/${slug}.md" "$(component_display_name "$slug")"
        fi
      done
    fi
  done

  if [[ "$INCLUDE_EXTRA" == "true" ]]; then
    for section in builders post-processors; do
      local index_page="${DOCS_SITE_DIR}/extra/${section}/index.md"
      if [[ -f "$index_page" ]]; then
        mkdir -p "${STAGING_DIR}/${section}"
        cp "$index_page" "${STAGING_DIR}/${section}/index.md"
      fi
    done
    if has_component_docs data-source; then
      local data_sources_index="${DOCS_SITE_DIR}/extra/data-sources/index.md"
      if [[ -f "$data_sources_index" ]]; then
        mkdir -p "${STAGING_DIR}/data-sources"
        cp "$data_sources_index" "${STAGING_DIR}/data-sources/index.md"
      fi
    fi
  fi

  if [[ "$INCLUDE_EXTRA" == "true" ]] && [[ -d "${DOCS_SITE_DIR}/extra/community" ]]; then
    mkdir -p "${STAGING_DIR}/community"
    for page in "${DOCS_SITE_DIR}/extra/community"/*.md; do
      [[ -f "$page" ]] || continue
      cp "$page" "${STAGING_DIR}/community/$(basename "$page")"
    done
  fi

  mkdir -p "$(dirname "$BUILD_CONFIG")"
  local nav_file="${DOCS_SITE_DIR}/.build/nav.toml"
  generate_nav >"$nav_file"
  awk -v nav_file="$nav_file" '
    /^\[project\.theme\]/ {
      while ((getline line < nav_file) > 0) {
        print line
      }
      close(nav_file)
      print ""
    }
    { print }
  ' "${DOCS_SITE_DIR}/zensical.toml" >"$BUILD_CONFIG"
  rm -f "$nav_file"

  echo "Documentation staged in ${STAGING_DIR}."
  echo "Navigation configuration generated in ${BUILD_CONFIG}."
}

main "$@"

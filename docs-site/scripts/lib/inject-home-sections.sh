#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Inject home page sections from docs-site/extra snippets.

set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=paths.sh
source "${LIB_DIR}/paths.sh"

inject_intro_upper() {
  local index_file="$1"
  local intro_file="${DOCS_SITE_DIR}/extra/intro-upper.md"
  [[ -f "$intro_file" ]] || return 0

  INTRO_FILE="$intro_file" perl -0777 -i -pe '
    open my $fh, "<", $ENV{INTRO_FILE} or die $!;
    my $intro = do { local $/; <$fh> };
    close $fh;
    chomp $intro;
    s/(\n)(### Installation)/\n$intro\n\n$2/s;
  ' "$index_file"
}

inject_intro_lower() {
  local index_file="$1"
  local lower_file="${DOCS_SITE_DIR}/extra/intro-lower.md"
  [[ -f "$lower_file" ]] || return 0

  LOWER_FILE="$lower_file" perl -0777 -i -pe '
    open my $fh, "<", $ENV{LOWER_FILE} or die $!;
    my $lower = do { local $/; <$fh> };
    close $fh;
    chomp $lower;
    $_ .= "\n\n" . $lower . "\n";
  ' "$index_file"
}

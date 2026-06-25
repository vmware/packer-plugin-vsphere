#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Indent asterisk sublists so Zensical renders nested lists correctly.

set -euo pipefail

deepen_asterisk_sublists() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    my @lines = split /\n/, $_, -1;
    my @out;
    my $in_fence = 0;

    for my $line (@lines) {
      if ($line =~ /^```/) {
        $in_fence = !$in_fence;
        push @out, $line;
        next;
      }
      if (!$in_fence && $line =~ /^  (\* )/) {
        $line =~ s/^  (\* )/    $1/;
      }
      push @out, $line;
    }

    $_ = join "\n", @out;
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  deepen_asterisk_sublists "$(cat "$1")"
fi

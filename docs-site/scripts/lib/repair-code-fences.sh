#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Close unclosed fenced code blocks before sibling list items or headings.

set -euo pipefail

repair_unclosed_fences() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    sub repair_document {
      my ($text) = @_;
      my @lines = split /\n/, $text, -1;
      my @out;
      my $in_fence = 0;
      my $close_indent = "";

      for my $line (@lines) {
        if ($line =~ /^(\s*)```/) {
          if ($in_fence) {
            $in_fence = 0;
            push @out, $line;
          } else {
            $in_fence = 1;
            $close_indent = $1;
            push @out, $line;
          }
          next;
        }

        if ($in_fence && ($line =~ /^- `/ || $line =~ /^#{2,6}\s/)) {
          push @out, $close_indent . "```";
          push @out, "";
          $in_fence = 0;
        }

        push @out, $line;
      }

      push @out, $close_indent . "```" if $in_fence;
      return join("\n", @out);
    }

    $_ = repair_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  repair_unclosed_fences "$(cat "$1")"
fi

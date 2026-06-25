#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Collapse or preserve blank lines between list items in staged markdown.

set -euo pipefail

normalize_list_spacing() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    sub collapse_list_gaps {
      my (@lines) = @_;
      my @out;

      for (my $i = 0; $i < @lines; $i++) {
        my $line = $lines[$i];

        if ($line =~ /^\s*$/ && @out && $i + 1 < @lines) {
          my $prev = $out[-1];
          my $next = $lines[$i + 1];

          if ($prev =~ /^- \S/) {
            my $j = $i + 1;
            while ($j < @lines && $lines[$j] =~ /^\s*$/) {
              $j++;
            }
            if ($j < @lines && $lines[$j] =~ /^- \S/) {
              $i = $j - 1;
              next;
            }
          }

          if ($prev =~ /^    \S/ && $next =~ /^  \S/ && $next !~ /^    /) {
            next;
          }
        }

        push @out, $line;
      }

      return @out;
    }

    sub fix_sibling_list_breaks {
      my (@lines) = @_;
      my @out;

      for my $line (@lines) {
        if ($line =~ /^- \S/ && @out) {
          my $j = $#out;
          while ($j >= 0 && $out[$j] =~ /^\s*$/) {
            $j--;
          }
          if ($j >= 0 && $out[$j] =~ /^    /) {
            push @out, "" unless @out && $out[-1] =~ /^\s*$/;
          }
        }
        push @out, $line;
      }

      return @out;
    }

    sub normalize_document {
      my ($text) = @_;
      my @lines = collapse_list_gaps(split /\n/, $text, -1);
      @lines = fix_sibling_list_breaks(@lines);
      return join("\n", @lines);
    }

    $_ = normalize_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  normalize_list_spacing "$(cat "$1")"
fi

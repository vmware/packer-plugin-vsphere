#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Convert GitHub-style alert blockquotes to Zensical admonitions.

set -euo pipefail

convert_github_alerts() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    sub admonition_type {
      my ($label) = @_;
      $label = uc $label;
      return "danger" if $label eq "CAUTION";
      return "warning" if $label =~ /^(IMPORTANT|WARNING)$/;
      return "tip" if $label eq "TIP";
      return "note";
    }

    sub is_github_alert_line {
      my ($line) = @_;
      return $line =~ /^>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*)$/i;
    }

    sub parse_github_alert_line {
      my ($line) = @_;
      if ($line =~ /^>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*)$/i) {
        return (uc $1, $2);
      }
      return;
    }

    sub convert_document {
      my ($text) = @_;
      my @lines = split /\n/, $text, -1;
      my @out;
      my $in_fence = 0;

      for (my $i = 0; $i < @lines; $i++) {
        my $line = $lines[$i];

        if ($line =~ /^```/) {
          $in_fence = !$in_fence;
          push @out, $line;
          next;
        }
        if ($in_fence) {
          push @out, $line;
          next;
        }

        if (!is_github_alert_line($line)) {
          push @out, $line;
          next;
        }

        my ($label, $first) = parse_github_alert_line($line);
        my $type = admonition_type($label);
        my @body;
        if (defined $first && $first =~ /\S/) {
          push @body, $first;
        }

        while ($i + 1 < @lines) {
          my $next = $lines[$i + 1];
          last if $next =~ /^#{1,6}\s/;
          last if is_github_alert_line($next);
          last if $next =~ /^\s*$/;

          if ($next =~ /^>\s?(.*)$/) {
            my ($quoted) = ($1);
            $i++;
            push @body, $quoted if defined $quoted && $quoted =~ /\S/;
            next;
          }

          if ($next !~ /^>/) {
            $i++;
            push @body, $next if $next =~ /\S/;
            next;
          }

          last;
        }

        push @out, "!!! ${type}";
        push @out, "";
        for my $part (@body) {
          push @out, "    ${part}";
        }
      }

      return join "\n", @out;
    }

    $_ = convert_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  convert_github_alerts "$(cat "$1")"
fi

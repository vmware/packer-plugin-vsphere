#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Convert Material-style admonition markers to Zensical admonitions or blockquotes.

set -euo pipefail

convert_admonitions() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    sub admonition_type {
      my ($label) = @_;
      $label = lc $label;
      $label =~ s/:$//;
      $label =~ s/\s+$//;
      return "warning" if $label =~ /^(important|warning)$/;
      return "tip" if $label eq "tip";
      return "danger" if $label eq "danger";
      return "info" if $label eq "info";
      return "note";
    }

    sub display_label {
      my ($label) = @_;
      $label =~ s/:$//;
      $label =~ s/\s+$//;
      return "Important" if lc($label) eq "important";
      return "Warning" if lc($label) eq "warning";
      return "Tip" if lc($label) eq "tip";
      return "Note" if lc($label) =~ /^notes?$/;
      return "Note" if $label eq "NOTE";
      return $label;
    }

    sub normalize_admonition_line {
      my ($line) = @_;
      if ($line =~ /^(\s*)`((?:~>|-+>)\s*\*\*.+)$/ && $line !~ /`\s*$/) {
        return $1 . $2;
      }
      return $line;
    }

    sub is_admonition_line {
      my ($line) = @_;
      $line = normalize_admonition_line($line);
      return 0 if $line =~ /^\s*`-+>\s*\*\*/;
      return 0 if $line =~ /^\s*~>\s*\d/;
      return 1 if $line =~ /^(\s*)(?:~>|-+>)\s*\*\*([^*]+?)\*\*:?\s*(.*)$/s;
      return 1 if $line =~ /^(\s*)\*\*(NOTE|Note|Notes|Important|Tip|Warning|Danger|Info)\*\*:?\s*(.*)$/s;
      return 0;
    }

    sub parse_admonition_line {
      my ($line) = @_;
      $line = normalize_admonition_line($line);
      if ($line =~ /^(\s*)(?:~>|-+>)\s*\*\*([^*]+?)\*\*:?\s*(.*)$/s) {
        return ($1, $2, $3);
      }
      if ($line =~ /^(\s*)\*\*(NOTE|Note|Notes|Important|Tip|Warning|Danger|Info)\*\*:?\s*(.*)$/s) {
        return ($1, $2, $3);
      }
      return;
    }

    sub recent_list_item_context {
      my ($out) = @_;
      for (my $i = $#$out; $i >= 0; $i--) {
        my $line = $out->[$i];
        next if $line =~ /^\s*$/;
        next if $line =~ /^[ \t]*<!--/;
        next if $line =~ /^  \S/;
        return 1 if $line =~ /^- \S/;
        return 0;
      }
      return 0;
    }

    sub is_continuation_line {
      my ($next, $indent, $in_list) = @_;
      return 0 if $next =~ /^\s*$/;
      return 0 if is_admonition_line($next);
      return 0 if $next =~ /^\s*#{1,6}\s/;
      return 0 if $next =~ /^\s*<!--/;
      return 0 if $next =~ /^\s*```/;

      if ($in_list) {
        return 1 if $next =~ /^\s{2,}\S/;
        return 1 if $next =~ /^\s*[-*+]\s/;
        return 0;
      }

      return 0 if $next =~ /^[-*+]\s/;
      return 1;
    }

    sub emit_block_admonition {
      my ($type, @body) = @_;
      my @out;
      push @out, "!!! ${type}";
      push @out, "";
      for my $part (@body) {
        push @out, "    ${part}";
      }
      return @out;
    }

    sub emit_list_admonition {
      my ($indent, $type, @body) = @_;
      my $base = length($indent) >= 4 ? $indent : "    ";
      my @out;
      push @out, "${base}!!! ${type}";
      for my $part (@body) {
        push @out, "${base}    ${part}";
      }
      return @out;
    }

    sub collapse_list_gaps {
      my (@lines) = @_;
      my @out;

      for (my $i = 0; $i < @lines; $i++) {
        my $line = $lines[$i];

        if ($line =~ /^\s*$/ && @out && $i + 1 < @lines) {
          my $prev = $out[-1];
          my $next = $lines[$i + 1];
          if ($prev =~ /^- / && $next =~ /^  \S/) {
            next;
          }
          if ($prev =~ /^\s*$/ && $next =~ /^    !!!/) {
            next;
          }
          if ($prev =~ /^  \S/ && $next =~ /^    !!!/) {
            push @out, "";
            while ($i + 1 < @lines && $lines[$i + 1] =~ /^\s*$/) {
              $i++;
            }
            next;
          }
          if ($prev =~ /^ {8,}\S/ && $next =~ /^  \S/ && $next !~ /^    /) {
            next;
          }
        }

        if ($line =~ /^    !!!/ && @out && $out[-1] =~ /^(- |  \S)/) {
          push @out, "" unless $out[-1] =~ /^\s*$/;
        }

        push @out, $line;
      }

      return @out;
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

        if (!is_admonition_line($line)) {
          push @out, $line;
          next;
        }

        my ($indent, $label, $first) = parse_admonition_line($line);
        my $type = admonition_type($label);
        my $in_list = length($indent) > 0;

        my @body;
        if (length $first) {
          $first =~ s/^\s+//;
          push @body, $first;
        }

        while ($i + 1 < @lines && is_continuation_line($lines[$i + 1], $indent, $in_list)) {
          $i++;
          my $body_line = $lines[$i];
          $body_line =~ s/^\s+//;
          push @body, $body_line;
        }

        if ($in_list || (!$in_list && recent_list_item_context(\@out))) {
          while (@out && $out[-1] =~ /^\s*$/) {
            pop @out;
          }
          if (@out && $out[-1] =~ /^- \S/) {
            push @out, "";
          }
          push @out, emit_list_admonition($indent, $type, @body);
        } else {
          push @out, emit_block_admonition($type, @body);
        }
      }

      return join("\n", collapse_list_gaps(@out));
    }

    $_ = convert_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  convert_admonitions "$(cat "$1")"
fi

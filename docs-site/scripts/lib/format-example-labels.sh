#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Bold and nest HCL/JSON example labels in staged documentation.

set -euo pipefail

format_example_labels() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    my @labels = (
      "HCL Examples",
      "JSON Examples",
      "HCL Example",
      "JSON Example",
    );

    my %example_aliases = (
      "Usage example (JSON)" => "JSON Example",
      "Usage example (HCL)"  => "HCL Example",
      "In JSON"              => "JSON Example",
      "JSON: Example"        => "JSON Example",
      "In HCL2"              => "HCL Example",
      "In HCL"               => "HCL Example",
    );

    sub normalize_example_alias_line {
      my ($line) = @_;
      for my $alias (sort { length($b) <=> length($a) } keys %example_aliases) {
        if ($line =~ /^(\s*)\Q$alias\E:?\s*$/) {
          return $1 . $example_aliases{$alias} . ":";
        }
      }
      return $line;
    }

    sub example_label_on_line {
      my ($line) = @_;
      return is_example_label_line(normalize_example_alias_line($line));
    }

    sub is_example_label_line {
      my ($line) = @_;
      for my $label (@labels) {
        return $label if $line =~ /^\s*\*{0,2}\Q$label\E:?\*{0,2}\s*$/;
      }
      return;
    }

    sub bold_example_label {
      my ($label) = @_;
      return "**${label}:**";
    }

    sub in_admonition_body_context {
      my ($lines, $idx) = @_;
      for (my $j = $idx - 1; $j >= 0; $j--) {
        my $prev = $lines->[$j];
        next if $prev =~ /^\s*$/;
        return 1 if $prev =~ /^\s*!!! /;
        return 0 if $prev =~ /^[^\s]/;
        return 0 if $prev =~ /^\s*#{1,6}\s/;
        return 0 if $prev =~ /^\s*[-*+]\s/;
        return 0 if $prev =~ /^<!--/;
        next if $prev =~ /^\s+\S/;
        return 0;
      }
      return 0;
    }

    sub in_list_context {
      my ($lines, $idx) = @_;
      # Column-0 example labels are always top-level, even after a list item.
      return 0 unless $lines->[$idx] =~ /^\s+\S/;

      for (my $j = $idx - 1; $j >= 0; $j--) {
        my $prev = $lines->[$j];
        next if $prev =~ /^\s*$/;
        return 1 if $prev =~ /^\s*[-*+]\s/;
        return 1 if $prev =~ /^\s{2,}\S/;
        return 0 if $prev =~ /^\s*#{1,6}\s/;
        return 0 if $prev =~ /^```/;
        return 0;
      }
      return 0;
    }

    sub should_stop_list_example_block {
      my ($line) = @_;
      return 1 if $line =~ /^#{1,6}\s/;
      return 1 if $line =~ /^- /;
      return 1 if $line =~ /^<!--/;
      return 0;
    }

    sub indent_line_for_list {
      my ($line) = @_;
      return $line if $line =~ /^\s*$/;
      if ($line =~ /^(\s*)(.*)$/) {
        my ($spaces, $rest) = ($1, $2);
        return $line if length($spaces) >= 4;
        return "    ${rest}";
      }
      return "    ${line}";
    }

    sub leading_spaces {
      my ($line) = @_;
      return 0 unless $line =~ /^(\s*)/;
      return length($1);
    }

    sub reindent_block_content {
      my ($content_ref, $base_indent) = @_;
      my @content = @$content_ref;
      return () unless @content;

      my $min_indent;
      for my $line (@content) {
        next if $line =~ /^\s*$/;
        my $spaces = leading_spaces($line);
        $min_indent = $spaces if !defined $min_indent || $spaces < $min_indent;
      }
      $min_indent = 0 unless defined $min_indent;

      my $prefix = " " x $base_indent;
      return map {
        if (/^\s*$/) {
          "";
        } else {
          my $body = $_;
          if ($min_indent > 0) {
            if ($body =~ s/^\s{$min_indent}//) {
              # stripped common base indent
            } else {
              $body =~ s/^\s+//;
            }
          }
          $prefix . $body;
        }
      } @content;
    }

    sub append_indented_example_block {
      my ($lines, $start, $out) = @_;
      my $i = $start;
      while ($i < @$lines && $lines->[$i] =~ /^\s*$/) {
        $i++;
      }
      while ($i < @$lines) {
        my $line = $lines->[$i];
        last if should_stop_list_example_block($line);
        last if example_label_on_line($line);

        if ($line =~ /^\s*```/) {
          push @$out, indent_line_for_list($line);
          $i++;
          my @content;
          while ($i < @$lines && $lines->[$i] !~ /^\s*```\s*$/) {
            push @content, $lines->[$i];
            $i++;
          }
          push @$out, reindent_block_content(\@content, 4);
          if ($i < @$lines) {
            push @$out, indent_line_for_list($lines->[$i]);
            $i++;
          }
          last;
        }

        push @$out, indent_line_for_list($line);
        $i++;
      }
      return $i - 1;
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
          if ($prev =~ /^  \S/ && $next =~ /^  \S/) {
            next;
          }
          if ($prev =~ /^\s*$/ && ($next =~ /^    !!!/ || example_label_on_line($next))) {
            next;
          }
          if ($prev =~ /^  \S/ && ($next =~ /^    !!!/ || example_label_on_line($next))) {
            push @out, "";
            while ($i + 1 < @lines && $lines[$i + 1] =~ /^\s*$/) {
              $i++;
            }
            next;
          }
          if ($prev =~ /^ {8,}\S/ && $next =~ /^  \S/ && $next !~ /^    /) {
            next;
          }
          if ($prev =~ /^    \S/ && $next =~ /^  \S/ && $next !~ /^    /) {
            next;
          }
        }

        if (($line =~ /^    !!!/ || example_label_on_line($line) || $line =~ /^\s{4}\*\*.*Example:\*\*/)
            && @out && $out[-1] =~ /^(- |  \S)/) {
          push @out, "" unless $out[-1] =~ /^\s*$/;
        }

        push @out, $line;
      }

      return @out;
    }

    sub format_document {
      my ($text) = @_;
      my @lines = collapse_list_gaps(split /\n/, $text, -1);
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

        $line = normalize_example_alias_line($line);

        my $label = is_example_label_line($line);
        if ($label) {
          my $formatted = bold_example_label($label);
          if (in_list_context(\@lines, $i)) {
            while (@out && $out[-1] =~ /^\s*$/) {
              pop @out;
            }
            if (@out && $out[-1] =~ /\S/ && $out[-1] !~ /^```/) {
              push @out, "";
            }
            push @out, "    ${formatted}";
            $i = append_indented_example_block(\@lines, $i + 1, \@out);
          } else {
            push @out, $formatted;
          }
          next;
        }

        push @out, $line;
      }

      return join("\n", collapse_list_gaps(@out));
    }

    $_ = format_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  format_example_labels "$(cat "$1")"
fi

#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Group adjacent HCL/JSON example blocks into Zensical content tabs.

set -euo pipefail

group_example_tabs() {
  local content="$1"
  printf '%s' "$content" | perl -0777 -pe '
    use strict;
    use warnings;

    sub example_label {
      my ($line) = @_;
      return unless defined $line && length $line;
      return unless $line =~ /^(\s*)\*{0,2}(HCL|JSON) Examples?(?: with ([^:*]+))?\:\*{0,2}\s*$/;
      return ($1, lc $2, $3);
    }

    sub example_suffixes_match {
      my ($left, $right) = @_;
      $left = "" unless defined $left;
      $right = "" unless defined $right;
      $left =~ s/^\s+|\s+$//g;
      $right =~ s/^\s+|\s+$//g;
      return $left eq $right;
    }

    sub emit_variant_heading {
      my ($out, $indent, $suffix) = @_;
      return unless defined $suffix && $suffix ne "";
      $suffix =~ s/^\s+|\s+$//g;
      push @$out, $indent . "**Example with ${suffix}:**";
      push @$out, "";
    }

    sub skip_label_and_fences {
      my ($lines, $idx) = @_;
      my (undef, $kind) = example_label($lines->[$$idx]);
      return unless defined $kind;
      $$idx++;
      skip_blank_lines($lines, $idx);
      read_consecutive_fences($lines, $idx, $kind);
    }

    sub find_opposite_label_index {
      my ($lines, $start, $indent, $suffix, $want_kind) = @_;
      for (my $j = $start; $j < @$lines; $j++) {
        last if $lines->[$j] =~ /^##\s/;
        my ($ind, $kind, $suf) = example_label($lines->[$j]);
        next unless defined $kind;
        if ($kind eq $want_kind && indents_compatible($indent, $ind) && example_suffixes_match($suffix, $suf)) {
          return $j;
        }
        skip_label_and_fences($lines, \$j, $kind);
        $j--;
      }
      return;
    }

    sub tab_title {
      my ($kind) = @_;
      return $kind eq "hcl" ? "HCL" : "JSON";
    }

    sub indents_compatible {
      my ($a, $b) = @_;
      return 1 if $a eq $b;
      return 1 if ($a eq "" && $b eq "    ") || ($a eq "    " && $b eq "");
      return 0;
    }

    sub tab_indent_for_pair {
      my ($a, $b) = @_;
      return length($a) >= length($b) ? $a : $b;
    }

    sub skip_blank_lines {
      my ($lines, $idx) = @_;
      while ($$idx < @$lines && $lines->[$$idx] =~ /^\s*$/) {
        $$idx++;
      }
    }

    sub read_fence_block {
      my ($lines, $idx) = @_;
      return unless $$idx < @$lines && $lines->[$$idx] =~ /^\s*```/;

      my @block;
      push @block, $lines->[$$idx];
      $$idx++;
      while ($$idx < @$lines && $lines->[$$idx] !~ /^\s*```\s*$/) {
        push @block, $lines->[$$idx];
        $$idx++;
      }
      if ($$idx < @$lines) {
        push @block, $lines->[$$idx];
        $$idx++;
      }
      return \@block;
    }

    sub fence_lang {
      my ($fence_ref) = @_;
      return unless @$fence_ref && $fence_ref->[0] =~ /^\s*```(\w+)/;
      return lc $1;
    }

    sub read_consecutive_fences {
      my ($lines, $idx, $lang) = @_;
      my @blocks;
      while ($$idx < @$lines) {
        skip_blank_lines($lines, $idx);
        last unless $$idx < @$lines;
        my $block = read_fence_block($lines, $idx);
        last unless $block && fence_lang($block) eq $lang;
        push @blocks, $block;
      }
      return @blocks;
    }

    sub as_fence_list {
      my ($arg) = @_;
      return @$arg if ref($arg) eq "ARRAY" && @$arg && ref($arg->[0]) eq "ARRAY";
      return ($arg);
    }

    sub indent_tab_fences {
      my ($fences_arg, $content_indent) = @_;
      my @result;
      my @blocks = as_fence_list($fences_arg);
      for my $i (0 .. $#blocks) {
        push @result, indent_fence_block($blocks[$i], $content_indent);
        push @result, "" if $i < $#blocks;
      }
      return @result;
    }

    sub leading_spaces {
      my ($line) = @_;
      return 0 unless $line =~ /^(\s*)/;
      return length($1);
    }

    sub indent_fence_block {
      my ($fence_ref, $content_indent) = @_;
      my @lines = @$fence_ref;
      return () unless @lines;

      my $open = shift @lines;
      my $close = pop @lines;
      my @content = @lines;

      my $min_indent;
      for my $line (@content) {
        next if $line =~ /^\s*$/;
        my $spaces = leading_spaces($line);
        $min_indent = $spaces if !defined $min_indent || $spaces < $min_indent;
      }
      $min_indent = 0 unless defined $min_indent;

      my $prefix = " " x $content_indent;
      my @result;

      $open =~ s/^\s+//;
      push @result, $prefix . $open;

      for my $line (@content) {
        if ($line =~ /^\s*$/) {
          push @result, "";
          next;
        }

        my $body = $line;
        if ($min_indent > 0) {
          if ($body =~ s/^\s{$min_indent}//) {
            # stripped common base indent
          } else {
            $body =~ s/^\s+//;
          }
        }
        push @result, $prefix . $body;
      }

      $close =~ s/^\s+//;
      push @result, $prefix . $close;

      return @result;
    }

    sub emit_tab_group {
      my ($out, $indent, $first_kind, $first_fences, $second_kind, $second_fences) = @_;
      my $content_indent = length($indent) + 4;

      push @$out, $indent . "=== \"" . tab_title($first_kind) . "\"";
      push @$out, "";
      push @$out, indent_tab_fences($first_fences, $content_indent);
      push @$out, $indent . "=== \"" . tab_title($second_kind) . "\"";
      push @$out, "";
      push @$out, indent_tab_fences($second_fences, $content_indent);
    }

    sub emit_single_fence {
      my ($out, $indent, $fence) = @_;
      push @$out, @$fence;
    }

    sub try_group_example_section {
      my ($lines, $idx, $out) = @_;
      my ($indent1, $kind1, $suffix1) = example_label($lines->[$$idx]);
      return 0 unless defined $kind1;

      my $scan = $$idx + 1;
      my @first_blocks = read_consecutive_fences($lines, \$scan, $kind1);
      return 0 unless @first_blocks;

      my $between = $scan;
      skip_blank_lines($lines, \$between);
      my ($indent2, $kind2, $suffix2) = example_label($lines->[$between]);
      return 0 unless defined $kind2 && $kind2 ne $kind1 && indents_compatible($indent1, $indent2);
      return 0 unless example_suffixes_match($suffix1, $suffix2);

      $scan = $between + 1;
      my @second_blocks = read_consecutive_fences($lines, \$scan, $kind2);
      return 0 unless @second_blocks;

      return 0 if @first_blocks == 1 && @second_blocks == 1;

      my $indent = tab_indent_for_pair($indent1, $indent2);
      emit_variant_heading($out, $indent, $suffix1);
      if ($kind1 eq "hcl") {
        emit_tab_group($out, $indent, "hcl", \@first_blocks, "json", \@second_blocks);
      } else {
        emit_tab_group($out, $indent, "hcl", \@second_blocks, "json", \@first_blocks);
      }
      $$idx = $scan - 1;
      return 1;
    }

    sub group_document {
      my ($text) = @_;
      my @lines = split /\n/, $text, -1;
      my @out;
      my $in_fence = 0;
      my %consumed_labels;

      for (my $i = 0; $i < @lines; $i++) {
        my $line = $lines[$i];

        if ($line =~ /^\s*```/) {
          $in_fence = !$in_fence;
          push @out, $line;
          next;
        }
        if ($in_fence) {
          push @out, $line;
          next;
        }

        if ($consumed_labels{$i}) {
          skip_label_and_fences(\@lines, \$i);
          next;
        }

        if (try_group_example_section(\@lines, \$i, \@out)) {
          next;
        }

        my ($indent, $kind, $suffix) = example_label($line);
        unless (defined $kind) {
          push @out, $line;
          next;
        }
        $i++;
        skip_blank_lines(\@lines, \$i);
        my $after_first = $i;
        my @first_blocks = read_consecutive_fences(\@lines, \$i, $kind);
        unless (@first_blocks) {
          push @out, $line;
          $i = $after_first - 1;
          next;
        }

        my $opposite = $kind eq "hcl" ? "json" : "hcl";
        my $peek = $i;
        skip_blank_lines(\@lines, \$peek);
        my ($indent2, $kind2, $suffix2) = ($peek < @lines && defined $lines[$peek]) ? example_label($lines[$peek]) : ();
        my $match_idx;
        if (
          defined $kind2
          && $kind2 eq $opposite
          && indents_compatible($indent, $indent2)
          && example_suffixes_match($suffix, $suffix2)
        ) {
          $match_idx = $peek;
        } else {
          $match_idx = find_opposite_label_index(\@lines, $after_first, $indent, $suffix, $opposite);
        }

        if (defined $match_idx) {
          my $json_idx = $match_idx;
          my $scan = $json_idx + 1;
          skip_blank_lines(\@lines, \$scan);
          my @second_blocks = read_consecutive_fences(\@lines, \$scan, $opposite);
          if (@second_blocks) {
            my $pair_indent = tab_indent_for_pair($indent, (example_label($lines[$json_idx]))[0]);
            my ($hcl_blocks, $json_blocks);
            if ($kind eq "hcl") {
              $hcl_blocks = \@first_blocks;
              $json_blocks = \@second_blocks;
            } else {
              $hcl_blocks = \@second_blocks;
              $json_blocks = \@first_blocks;
            }
            emit_variant_heading(\@out, $pair_indent, $suffix);
            emit_tab_group(\@out, $pair_indent, "hcl", $hcl_blocks, "json", $json_blocks);
            $consumed_labels{$json_idx} = 1;
            if (defined $peek && $match_idx == $peek) {
              $i = $scan - 1;
            } else {
              $i--;
            }
            next;
          }
          $i = $after_first - 1;
        }

        emit_variant_heading(\@out, $indent, $suffix);
        if (@first_blocks == 1) {
          emit_single_fence(\@out, $indent, $first_blocks[0]);
        } else {
          push @out, indent_tab_fences(\@first_blocks, length($indent));
        }
        $i--;
        next;
      }

      return join("\n", @out);
    }

    $_ = group_document($_);
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 <file>" >&2
    exit 1
  fi
  group_example_tabs "$(cat "$1")"
fi

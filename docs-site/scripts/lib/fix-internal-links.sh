#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Fix internal documentation links and anchors after integration URL rewriting.

set -euo pipefail

fix_internal_links() {
  local content="$1"
  local dest_rel="${2:-}"
  export DEST_REL="$dest_rel"
  printf '%s' "$content" | perl -pe '
    BEGIN {
      %anchor_fix = (
        "content-library-import-configuration" => "content-library-destination-configuration",
        "content-library-configuration" => "content-library-destination-configuration",
        "customization-example" => "linux-customization-settings",
        "linux-options" => "linux-customization-settings",
        "windows-options" => "windows-customization-settings",
        "network_interface" => "network-interface-settings",
        "host_name" => "linux-customization-settings",
        "domain" => "linux-customization-settings",
        "pci-passthrough-configuration" => "hardware-configuration",
        "ssh_password" => "ssh",
        "ssh_private_key_file" => "ssh",
        "ssh_agent_auth" => "ssh",
        "ssh_keypair_name" => "ssh",
        "vm_name" => "location-configuration",
        "cluster" => "location-configuration",
        "host" => "location-configuration",
        "resource_pool" => "location-configuration",
        "overwrite" => "content-library-destination-configuration",
        "ovf" => "content-library-destination-configuration",
        "storage" => "storage-configuration",
        "disk_size" => "clone-configuration",
      );
    }

    my $dest = $ENV{DEST_REL} // "";

    sub fix_anchor {
      my ($anchor) = @_;
      $anchor =~ s/^#//;
      my $lower = lc $anchor;
      if (exists $anchor_fix{$lower}) {
        return "#" . $anchor_fix{$lower};
      }
      return "#" . $anchor;
    }

    sub fix_path_link {
      my ($link, $dest) = @_;
      return $link if $link =~ m{^(https?:|mailto:)};

      my ($path, $anchor) = split(/#/, $link, 2);
      $anchor = defined $anchor ? "#" . $anchor : "";

      if ($path =~ m{^(builders|post-processors|data-sources)/([^/]+)\.md$}) {
        my ($section, $slug) = ($1, $2);
        my $target = "$section/$slug.md";

        if ($dest eq $target) {
          return fix_anchor($anchor) if length $anchor;
          return $link;
        }

        my ($dest_section) = $dest =~ m{^(builders|post-processors|data-sources)/};
        if (defined $dest_section && $dest_section eq $section) {
          return "$slug/$anchor";
        }

        if ($dest eq "index.md") {
          return "$section/$slug/$anchor";
        }

        return "../$section/$slug/$anchor";
      }

      if (length $anchor) {
        my $fixed = fix_anchor($anchor);
        return ($path // "") . $fixed;
      }

      return $link;
    }

    s{
      (\]\()
      ([^)]+)
      (\))
    }{
      my ($open, $link, $close) = ($1, $2, $3);
      if ($link =~ /^#/) {
        $open . fix_anchor($link) . $close;
      } else {
        $open . fix_path_link($link, $dest) . $close;
      }
    }gxe;
  '
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "usage: $0 <file> [dest_rel_path]" >&2
    exit 1
  fi
  export DEST_REL="${2:-}"
  fix_internal_links "$(cat "$1")" "${2:-}"
fi

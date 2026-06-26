#!/usr/bin/env bash
# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# Tests for integration link rewriting in staged markdown.

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "${TEST_DIR}/.." && pwd)"
LIB_DIR="${SCRIPTS_DIR}/lib"

# shellcheck source=assertions.sh
source "${TEST_DIR}/assertions.sh"
# shellcheck source=../lib/rewrite-integration-links.sh
source "${LIB_DIR}/rewrite-integration-links.sh"

test_vmware_vsphere_builder_link() {
  local input='[vsphere-iso](/packer/integrations/vmware/vsphere/latest/components/builder/vsphere-iso)'
  local output
  output="$(rewrite_integration_links "$input")"
  assert_contains "$output" 'builders/vsphere-iso.md' "vmware vsphere builder link"
  assert_not_contains "$output" '/packer/integrations/vmware' "no raw integration path"
}

test_hashicorp_vsphere_post_processor_link() {
  local input='[vSphere](/packer/integrations/hashicorp/vsphere/latest/components/post-processor/vsphere)'
  local output
  output="$(rewrite_integration_links "$input")"
  assert_contains "$output" 'post-processors/vsphere.md' "hashicorp vsphere post-processor link"
}

test_hashicorp_vmware_builder_link_with_anchor() {
  local input='[vApp](/packer/integrations/hashicorp/vmware/latest/components/builder/vsphere-clone#vapp-options-configuration)'
  local output
  output="$(rewrite_integration_links "$input")"
  assert_contains "$output" 'builders/vsphere-clone.md#vapp-options-configuration' "builder link with anchor"
}

test_cross_plugin_link_to_hashicorp() {
  local input='[`ssh_interface`](/packer/integrations/hashicorp/amazon/latest/components/builder/ebs#ssh_interface)'
  local output
  output="$(rewrite_integration_links "$input")"
  assert_contains "$output" 'https://developer.hashicorp.com/packer/integrations/hashicorp/amazon/latest/components/builder/ebs#ssh_interface' "cross-plugin external link"
}

test_packer_docs_link() {
  local input='[packer init](/packer/docs/commands/init)'
  local output
  output="$(rewrite_integration_links "$input")"
  assert_contains "$output" 'https://developer.hashicorp.com/packer/docs/commands/init' "packer docs external link"
}

main() {
  test_vmware_vsphere_builder_link
  test_hashicorp_vsphere_post_processor_link
  test_hashicorp_vmware_builder_link_with_anchor
  test_cross_plugin_link_to_hashicorp
  test_packer_docs_link
  echo "All rewrite-integration-links tests passed."
}

main "$@"

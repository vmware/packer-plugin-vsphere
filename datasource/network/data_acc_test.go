// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package network

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceNetwork(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
		"name":    acc.Network,
		"cluster": acc.Cluster,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.Network {
		t.Fatalf("unexpected network name: expected %q, got %q", acc.Network, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty network ID")
	}
	switch got := output.GetAttr("type").AsString(); got {
	case "Network", "DistributedVirtualPortgroup", "OpaqueNetwork":
	default:
		t.Fatalf("unexpected network type %q", got)
	}
	acceptance.AssertTagsShape(t, output)
}

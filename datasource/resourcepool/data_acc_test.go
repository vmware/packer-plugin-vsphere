// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package resourcepool

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceResourcePool(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
		"name":    acc.ResourcePool,
		"cluster": acc.Cluster,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.ResourcePool {
		t.Fatalf("unexpected resource pool name: expected %q, got %q", acc.ResourcePool, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty resource pool ID")
	}
	if got := output.GetAttr("path").AsString(); got == "" {
		t.Fatal("expected a non-empty builder-ready resource pool path")
	}
}

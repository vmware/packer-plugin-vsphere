// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package computecluster

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceComputeCluster(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
		"name": acc.Cluster,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.Cluster {
		t.Fatalf("unexpected cluster name: expected %q, got %q", acc.Cluster, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty cluster ID")
	}
	if got := output.GetAttr("resource_pool").AsString(); got == "" {
		t.Fatal("expected a non-empty root resource pool path")
	}
}

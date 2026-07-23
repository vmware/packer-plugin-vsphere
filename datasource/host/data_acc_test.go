// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package host

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceHost(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]interface{}{
		"name":    acc.Host,
		"cluster": acc.Cluster,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.Host {
		t.Fatalf("unexpected host name: expected %q, got %q", acc.Host, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty host ID")
	}
	if got := output.GetAttr("cluster").AsString(); got != acc.Cluster {
		t.Fatalf("unexpected host cluster: expected %q, got %q", acc.Cluster, got)
	}
	summary := output.GetAttr("summary")
	capacity, _ := summary.GetAttr("memory_capacity").AsBigFloat().Int64()
	free, _ := summary.GetAttr("memory_free").AsBigFloat().Int64()
	if capacity <= 0 {
		t.Fatalf("expected positive host memory capacity, got %d", capacity)
	}
	if free < 0 || free > capacity {
		t.Fatalf("expected host free memory between 0 and %d, got %d", capacity, free)
	}
}

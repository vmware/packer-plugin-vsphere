// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package datastore

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceDatastore(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]interface{}{
		"name": acc.Datastore,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.Datastore {
		t.Fatalf("unexpected datastore name: expected %q, got %q", acc.Datastore, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty datastore ID")
	}
	summary := output.GetAttr("summary")
	capacity, _ := summary.GetAttr("capacity").AsBigFloat().Int64()
	free, _ := summary.GetAttr("free").AsBigFloat().Int64()
	if capacity <= 0 {
		t.Fatalf("expected positive datastore capacity, got %d", capacity)
	}
	if free < 0 || free > capacity {
		t.Fatalf("expected datastore free space between 0 and %d, got %d", capacity, free)
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibrary

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceContentLibrary(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]interface{}{
		"name": acc.ContentLibrary,
	}))

	if got := output.GetAttr("name").AsString(); got != acc.ContentLibrary {
		t.Fatalf("unexpected content library name: expected %q, got %q", acc.ContentLibrary, got)
	}
	if got := output.GetAttr("id").AsString(); got == "" {
		t.Fatal("expected a non-empty content library ID")
	}
}

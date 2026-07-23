// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package virtualmachine

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceVirtualMachine(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]interface{}{
		"name":     acc.Template,
		"template": true,
	}))

	if got := output.GetAttr("vm_name").AsString(); got != acc.Template {
		t.Fatalf("unexpected virtual machine name: expected %q, got %q", acc.Template, got)
	}
}

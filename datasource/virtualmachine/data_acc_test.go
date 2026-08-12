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
	output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
		"name":     acc.Template,
		"template": true,
	}))

	if got := output.GetAttr("vm_name").AsString(); got != acc.Template {
		t.Fatalf("unexpected virtual machine name: expected %q, got %q", acc.Template, got)
	}
	acceptance.AssertTagsShape(t, output)

	t.Run("tags output", func(t *testing.T) {
		d, err := acceptance.TestConn()
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		vm, err := d.FindVM(acc.Template)
		if err != nil {
			t.Fatalf("find template %q: %v", acc.Template, err)
		}

		acceptance.AttachTagsTemporarily(t, vm.Reference(), acc.TagCategory, acc.TagA, acc.TagB)

		output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
			"name":     acc.Template,
			"template": true,
		}))
		acceptance.AssertTagsShape(t, output)
		acceptance.AssertContainsTag(t, output, acc.TagCategory, acc.TagA)
		acceptance.AssertContainsTag(t, output, acc.TagCategory, acc.TagB)

		filtered := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
			"name":     acc.Template,
			"template": true,
			"tag": []map[string]any{
				{"category": acc.TagCategory, "name": acc.TagA},
			},
		}))
		acceptance.AssertContainsTag(t, filtered, acc.TagCategory, acc.TagA)
		acceptance.AssertContainsTag(t, filtered, acc.TagCategory, acc.TagB)
	})
}

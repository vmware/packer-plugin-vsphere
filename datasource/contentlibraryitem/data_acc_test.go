// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibraryitem

import (
	"fmt"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestAccDatasourceContentLibraryItem(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()

	tests := []struct {
		name     string
		itemName string
		itemType string
	}{
		{name: "VMTX", itemName: acc.ContentLibraryVMTX, itemType: "vm-template"},
		{name: "OVF", itemName: acc.ContentLibraryOVF, itemType: "ovf"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
				"content_library": acc.ContentLibrary,
				"name":            test.itemName,
				"type":            test.itemType,
			}))

			if got := output.GetAttr("name").AsString(); got != test.itemName {
				t.Fatalf("unexpected item name: expected %q, got %q", test.itemName, got)
			}
			if got := output.GetAttr("id").AsString(); got == "" {
				t.Fatal("expected a non-empty content library item ID")
			}
			if got := output.GetAttr("library").AsString(); got != acc.ContentLibrary {
				t.Fatalf("unexpected library: expected %q, got %q", acc.ContentLibrary, got)
			}
			if got := output.GetAttr("type").AsString(); got != test.itemType {
				t.Fatalf("unexpected item type: expected %q, got %q", test.itemType, got)
			}
			expectedPath := fmt.Sprintf("%s/%s", acc.ContentLibrary, test.itemName)
			if got := output.GetAttr("path").AsString(); got != expectedPath {
				t.Fatalf("unexpected item path: expected %q, got %q", expectedPath, got)
			}
			acceptance.AssertTagsShape(t, output)
		})
	}

	t.Run("tags output", func(t *testing.T) {
		d, err := acceptance.TestConn()
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		item, err := d.ResolveContentLibraryItem(acc.ContentLibrary, acc.ContentLibraryOVF)
		if err != nil {
			t.Fatalf("resolve content library item %q/%q: %v", acc.ContentLibrary, acc.ContentLibraryOVF, err)
		}

		ref := types.ManagedObjectReference{
			Type:  contentLibraryItemMoRefType,
			Value: item.ID,
		}
		acceptance.AttachTagsTemporarily(t, ref, acc.TagCategory, acc.TagA, acc.TagB)

		output := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
			"content_library": acc.ContentLibrary,
			"name":            acc.ContentLibraryOVF,
			"type":            "ovf",
		}))
		acceptance.AssertTagsShape(t, output)
		acceptance.AssertContainsTag(t, output, acc.TagCategory, acc.TagA)
		acceptance.AssertContainsTag(t, output, acc.TagCategory, acc.TagB)

		filtered := acceptance.ExecuteDatasource(t, &Datasource{}, acceptance.DatasourceConfig(acc, map[string]any{
			"content_library": acc.ContentLibrary,
			"name":            acc.ContentLibraryOVF,
			"type":            "ovf",
			"tag": []map[string]any{
				{"category": acc.TagCategory, "name": acc.TagA},
			},
		}))
		acceptance.AssertContainsTag(t, filtered, acc.TagCategory, acc.TagA)
		acceptance.AssertContainsTag(t, filtered, acc.TagCategory, acc.TagB)
	})
}

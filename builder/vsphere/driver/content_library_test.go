// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi/vapi/library"
)

func TestLibraryFilePath(t *testing.T) {
	tc := []struct {
		filePath        string
		libraryName     string
		libraryItemName string
		fileName        string
		valid           bool
	}{
		{
			filePath:        "lib/item/file",
			libraryName:     "lib",
			libraryItemName: "item",
			fileName:        "file",
			valid:           true,
		},
		{
			filePath:        "/lib/item/file",
			libraryName:     "lib",
			libraryItemName: "item",
			fileName:        "file",
			valid:           true,
		},
		{
			filePath: "/lib/item/filedir/file",
			valid:    false,
		},
		{
			filePath: "/lib/item",
			valid:    false,
		},
		{
			filePath: "/lib",
			valid:    false,
		},
	}

	for _, c := range tc {
		libraryFilePath := &LibraryFilePath{path: c.filePath}
		if err := libraryFilePath.Validate(); err != nil {
			if c.valid {
				t.Fatalf("unexpected result: expected '%s' to be valid", c.filePath)
			}
			continue
		}
		libraryName := libraryFilePath.GetLibraryName()
		if libraryName != c.libraryName {
			t.Fatalf("unexpected result: expected '%s', but returned '%s'", c.libraryName, libraryName)
		}
		libraryItemName := libraryFilePath.GetLibraryItemName()
		if libraryItemName != c.libraryItemName {
			t.Fatalf("unexpected result: expected '%s', but returned '%s'", c.libraryItemName, libraryItemName)
		}
		fileName := libraryFilePath.GetFileName()
		if fileName != c.fileName {
			t.Fatalf("unexpected result: expected '%s', but returned '%s'", c.fileName, fileName)
		}
	}
}

func TestDeployContentLibraryItem_UnsupportedType(t *testing.T) {
	d := &VCenterDriver{}
	config := &ContentLibraryDeployConfig{
		Name: "test-vm",
		Item: &library.Item{
			Name: "unsupported-item",
			Type: "iso",
		},
	}

	_, err := d.DeployContentLibraryItem(context.Background(), config, &packersdk.BasicUi{})
	if err == nil {
		t.Fatal("expected error for unsupported item type")
	}
	if !strings.Contains(err.Error(), "unsupported content library item type 'iso'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeployContentLibraryItem_NilConfig(t *testing.T) {
	d := &VCenterDriver{}
	_, err := d.DeployContentLibraryItem(context.Background(), nil, &packersdk.BasicUi{})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestDeployContentLibraryItem_MissingName(t *testing.T) {
	d := &VCenterDriver{}
	config := &ContentLibraryDeployConfig{
		Item: &library.Item{
			Type: library.ItemTypeVMTX,
		},
	}
	_, err := d.DeployContentLibraryItem(context.Background(), config, &packersdk.BasicUi{})
	if err == nil {
		t.Fatal("expected error for missing VM name")
	}
}

func TestDeployContentLibraryItem_NilItem(t *testing.T) {
	d := &VCenterDriver{}
	config := &ContentLibraryDeployConfig{
		Name: "test-vm",
	}
	_, err := d.DeployContentLibraryItem(context.Background(), config, &packersdk.BasicUi{})
	if err == nil {
		t.Fatal("expected error for nil content library item")
	}
	if !strings.Contains(err.Error(), "content library item is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWrapContentLibraryError(t *testing.T) {
	d := &VCenterDriver{}

	tests := []struct {
		name           string
		err            error
		expectContains string
	}{
		{
			name:           "Permission error is wrapped",
			err:            fmt.Errorf("403 Forbidden: not authorized"),
			expectContains: "insufficient permissions to access content library 'Example Library' or item 'example-item'",
		},
		{
			name:           "Unauthorized error is wrapped",
			err:            fmt.Errorf("unauthorized access"),
			expectContains: "insufficient permissions to access content library 'Example Library' or item 'example-item'",
		},
		{
			name:           "Non-permission error is unchanged",
			err:            fmt.Errorf("content library item not found"),
			expectContains: "content library item not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.wrapContentLibraryError(tt.err, "Example Library", "example-item")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.expectContains) {
				t.Fatalf("expected error containing %q, got %q", tt.expectContains, err.Error())
			}
		})
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibraryitem

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
	"github.com/zclconf/go-cty/cty"
)

func TestDatasource_Execute(t *testing.T) {
	model := simulator.VPX()

	vcSim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyContentLibraryConfiguration([]vcsim.SimulatedContentLibraryConfig{
		{Name: "lib01", DatastoreIndex: 0},
		{Name: "lib02", DatastoreIndex: 0},
	}); err != nil {
		t.Fatalf("error creating simulator content libraries: %s", err)
	}

	if err := vcSim.ApplyContentLibraryItemConfiguration([]vcsim.SimulatedContentLibraryItemConfig{
		{Library: "lib01", Name: "linux-debian-13", Type: "ovf"},
		{
			Library: "lib01",
			Name:    "linux-debian-13.5.0-amd64",
			Type:    "iso",
			Files:   []string{"debian-13.5.0-amd64-netinst.iso"},
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Library: "lib01",
			Name:    "linux-debian-13.6.0-amd64",
			Type:    "iso",
			Files:   []string{"debian-13.6.0-amd64-netinst.iso"},
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "kind", Name: "iso"},
			},
		},
		{Library: "lib02", Name: "linux-debian-13", Type: "iso", Files: []string{"debian-13.6.0-amd64-netinst.iso"}},
	}); err != nil {
		t.Fatalf("error creating simulator content library items: %s", err)
	}

	simulatorPassword, _ := vcSim.Server.URL.User.Password()
	connectConfig := common.ConnectConfig{
		VCenterServer:      vcSim.Server.URL.Host,
		Username:           vcSim.Server.URL.User.Username(),
		Password:           simulatorPassword,
		InsecureConnection: true,
		Datacenter:         vcSim.Datacenter.Name(),
	}

	tests := []struct {
		name          string
		expectFailure bool
		expectName    string
		expectType    string
		expectPath    string
		expectTags    []Tag
		config        Config
	}{
		{
			name:       "exact name ovf",
			expectName: "linux-debian-13",
			expectType: "ovf",
			expectPath: "lib01/linux-debian-13",
			expectTags: []Tag{},
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13",
			},
		},
		{
			name:       "iso with resolved file path",
			expectName: "linux-debian-13.5.0-amd64",
			expectType: "iso",
			expectPath: "lib01/linux-debian-13.5.0-amd64/debian-13.5.0-amd64-netinst.iso",
			expectTags: []Tag{
				{Category: "env", Name: "Packer"},
			},
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13.5.0-amd64",
				Type:           "iso",
			},
		},
		{
			name:       "regex unique match",
			expectName: "linux-debian-13",
			expectType: "ovf",
			expectTags: []Tag{},
			config: Config{
				ContentLibrary: "lib01",
				NameRegex:      "^linux-debian-13$",
			},
		},
		{
			name:       "type filter narrows to unique",
			expectName: "linux-debian-13",
			expectTags: []Tag{},
			config: Config{
				ContentLibrary: "lib01",
				Type:           "ovf",
			},
		},
		{
			name:       "glob multiple with latest",
			expectName: "linux-debian-13.6.0-amd64",
			expectType: "iso",
			expectPath: "lib01/linux-debian-13.6.0-amd64/debian-13.6.0-amd64-netinst.iso",
			expectTags: []Tag{
				{Category: "env", Name: "Packer"},
				{Category: "kind", Name: "iso"},
			},
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13.*-amd64",
				Latest:         true,
			},
		},
		{
			name:          "glob multiple without latest",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13.*-amd64",
			},
		},
		{
			name:          "no item matches name",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				Name:           "does-not-exist",
			},
		},
		{
			name:          "type mismatch",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13",
				Type:           "iso",
			},
		},
		{
			name:          "invalid type",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				Type:           "bogus",
			},
		},
		{
			name:          "invalid name_regex",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				NameRegex:      "(",
			},
		},
		{
			name:          "missing content_library",
			expectFailure: true,
			config: Config{
				Name: "linux-debian-13",
			},
		},
		{
			name:          "content library not found",
			expectFailure: true,
			config: Config{
				ContentLibrary: "does-not-exist",
			},
		},
		{
			name:       "item scoped to its library",
			expectName: "linux-debian-13",
			expectType: "iso",
			expectPath: "lib02/linux-debian-13/debian-13.6.0-amd64-netinst.iso",
			expectTags: []Tag{},
			config: Config{
				ContentLibrary: "lib02",
			},
		},
		{
			name:       "tag filter unique match",
			expectName: "linux-debian-13.6.0-amd64",
			expectType: "iso",
			expectTags: []Tag{
				{Category: "env", Name: "Packer"},
				{Category: "kind", Name: "iso"},
			},
			config: Config{
				ContentLibrary: "lib01",
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
					{Category: "kind", Name: "iso"},
				},
			},
		},
		{
			name:          "tag filter multiple matches",
			expectFailure: true,
			config: Config{
				ContentLibrary: "lib01",
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
				},
			},
		},
		{
			name:       "name and tag combined",
			expectName: "linux-debian-13.5.0-amd64",
			expectType: "iso",
			expectTags: []Tag{
				{Category: "env", Name: "Packer"},
			},
			config: Config{
				ContentLibrary: "lib01",
				Name:           "linux-debian-13.5.0-amd64",
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := Datasource{
				config: tt.config,
			}
			ds.config.ConnectConfig = connectConfig

			if err := ds.Configure(); err != nil {
				if !tt.expectFailure {
					t.Fatalf("unexpected Configure error: %s", err)
				}
				return
			}

			output, err := ds.Execute()
			if tt.expectFailure {
				if err == nil {
					t.Fatalf("expected failure, got success: %#v", output)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected Execute error: %s", err)
			}

			if got := output.GetAttr("name").AsString(); got != tt.expectName {
				t.Fatalf("expected name %q, got %q", tt.expectName, got)
			}
			if output.GetAttr("id").AsString() == "" {
				t.Fatalf("expected non-empty id")
			}
			if got := output.GetAttr("library").AsString(); got != tt.config.ContentLibrary {
				t.Fatalf("expected library %q, got %q", tt.config.ContentLibrary, got)
			}
			if tt.expectType != "" {
				if got := output.GetAttr("type").AsString(); got != tt.expectType {
					t.Fatalf("expected type %q, got %q", tt.expectType, got)
				}
			}
			if tt.expectPath != "" {
				if got := output.GetAttr("path").AsString(); got != tt.expectPath {
					t.Fatalf("expected path %q, got %q", tt.expectPath, got)
				}
			}
			assertTags(t, output.GetAttr("tags"), tt.expectTags)
		})
	}
}

func assertTags(t *testing.T, tagsAttr cty.Value, want []Tag) {
	t.Helper()
	if tagsAttr.IsNull() {
		if len(want) == 0 {
			return
		}
		t.Fatalf("expected %d tags, got null", len(want))
	}
	gotSlice := tagsAttr.AsValueSlice()
	if len(gotSlice) != len(want) {
		t.Fatalf("expected %d tags, got %d", len(want), len(gotSlice))
	}
	got := make(map[string]string, len(gotSlice))
	for _, tagVal := range gotSlice {
		name := tagVal.GetAttr("name").AsString()
		category := tagVal.GetAttr("category").AsString()
		got[name] = category
	}
	for _, tag := range want {
		category, ok := got[tag.Name]
		if !ok {
			t.Fatalf("missing expected tag %q", tag.Name)
		}
		if category != tag.Category {
			t.Fatalf("tag %q: expected category %q, got %q", tag.Name, tag.Category, category)
		}
	}
}

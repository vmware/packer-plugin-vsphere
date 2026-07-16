// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibrary

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vsphere"
)

func TestDatasource_Execute(t *testing.T) {
	model := simulator.VPX()

	vcSim, err := vsphere.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyContentLibraryConfiguration([]vsphere.SimulatedContentLibraryConfig{
		{
			Name:           "lib01",
			DatastoreIndex: 0,
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Name:           "lib02",
			DatastoreIndex: 0,
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "kind", Name: "iso"},
			},
		},
		{
			Name:           "lib-vendor",
			DatastoreIndex: 0,
		},
	}); err != nil {
		t.Fatalf("error creating simulator content libraries: %s", err)
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
		config        Config
	}{
		{
			name:          "exact name",
			expectFailure: false,
			expectName:    "lib01",
			config: Config{
				Name: "lib01",
			},
		},
		{
			name:          "glob unique match",
			expectFailure: false,
			expectName:    "lib-vendor",
			config: Config{
				Name: "lib-vendor",
			},
		},
		{
			name:          "glob multiple matches",
			expectFailure: true,
			config: Config{
				Name: "lib0*",
			},
		},
		{
			name:          "no content library matches name",
			expectFailure: true,
			config: Config{
				Name: "does-not-exist",
			},
		},
		{
			name:          "invalid name_regex",
			expectFailure: true,
			config: Config{
				NameRegex: "(",
			},
		},
		{
			name:          "regex unique match",
			expectFailure: false,
			expectName:    "lib02",
			config: Config{
				NameRegex: "^lib02$",
			},
		},
		{
			name:          "tag filter unique match",
			expectFailure: false,
			expectName:    "lib02",
			config: Config{
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
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
				},
			},
		},
		{
			name:          "name and tag combined",
			expectFailure: false,
			expectName:    "lib01",
			config: Config{
				Name: "lib01",
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

			nameVal := output.GetAttr("name")
			if nameVal.AsString() != tt.expectName {
				t.Fatalf("expected name %q, got %q", tt.expectName, nameVal.AsString())
			}
			idVal := output.GetAttr("id")
			if idVal.AsString() == "" {
				t.Fatalf("expected non-empty id")
			}
		})
	}
}

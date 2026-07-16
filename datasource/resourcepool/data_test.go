// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package resourcepool

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vsphere"
)

func TestDatasource_Execute(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 2
	model.ClusterHost = 1

	vcSim, err := vsphere.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyComputeClusterConfiguration([]vsphere.SimulatedComputeClusterConfig{
		{Name: "w01-cl01"},
		{Name: "w01-cl02"},
	}); err != nil {
		t.Fatalf("error customizing simulator compute clusters: %s", err)
	}

	if err := vcSim.ApplyResourcePoolConfiguration([]vsphere.SimulatedResourcePoolConfig{
		{
			Path:         "rp-parent",
			ClusterIndex: 0,
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Path:         "rp-parent/rp-child",
			ClusterIndex: 0,
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
		{
			Path:         "rp-parent",
			ClusterIndex: 1,
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
	}); err != nil {
		t.Fatalf("error creating simulator resource pools: %s", err)
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
		expectPath    string
		config        Config
	}{
		{
			name:          "exact name with cluster",
			expectFailure: false,
			expectName:    "rp-child",
			expectPath:    "rp-parent/rp-child",
			config: Config{
				Name:    "rp-child",
				Cluster: "w01-cl01",
			},
		},
		{
			name:          "no resource pool matches name",
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
			name:          "multiple name matches without cluster",
			expectFailure: true,
			config: Config{
				Name: "rp-parent",
			},
		},
		{
			name:          "name with cluster unique",
			expectFailure: false,
			expectName:    "rp-parent",
			expectPath:    "rp-parent",
			config: Config{
				Name:    "rp-parent",
				Cluster: "w01-cl02",
			},
		},
		{
			name:          "regex match single under cluster",
			expectFailure: false,
			expectName:    "rp-child",
			expectPath:    "rp-parent/rp-child",
			config: Config{
				NameRegex: "^rp-child$",
				Cluster:   "w01-cl01",
			},
		},
		{
			name:          "tag filter unique match",
			expectFailure: false,
			expectName:    "rp-child",
			expectPath:    "rp-parent/rp-child",
			config: Config{
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
					{Category: "tier", Name: "gold"},
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
			name:          "cluster not found",
			expectFailure: true,
			config: Config{
				Cluster: "unexpected_cluster",
			},
		},
		{
			name:          "host not found",
			expectFailure: true,
			config: Config{
				Host: "unexpected_host",
			},
		},
		{
			name:          "host filter with unique name",
			expectFailure: false,
			expectName:    "rp-child",
			expectPath:    "rp-parent/rp-child",
			config: Config{
				Name: "rp-child",
				Host: "DC0_C0_H0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.ConnectConfig = connectConfig

			ds := Datasource{config: tc.config}
			if err := ds.Configure(); err != nil {
				t.Fatalf("Failed to configure datasource: %s", err)
			}

			result, err := ds.Execute()
			if err != nil && !tc.expectFailure {
				t.Fatalf("unexpected failure: %s", err)
			}
			if err == nil && tc.expectFailure {
				t.Fatalf("expected failure, but execution succeeded")
			}
			if err != nil {
				return
			}

			gotName := result.GetAttr("name").AsString()
			if gotName != tc.expectName {
				t.Errorf("expected name %q, got %q", tc.expectName, gotName)
			}
			gotID := result.GetAttr("id").AsString()
			if gotID == "" {
				t.Errorf("expected non-empty id")
			}
			gotPath := result.GetAttr("path").AsString()
			if gotPath != tc.expectPath {
				t.Errorf("expected path %q, got %q", tc.expectPath, gotPath)
			}
		})
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package network

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

func TestDatasource_Execute(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = 1

	vcSim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyComputeClusterConfiguration([]vcsim.SimulatedComputeClusterConfig{
		{Name: "w01-cl01"},
	}); err != nil {
		t.Fatalf("error customizing simulator compute clusters: %s", err)
	}

	if err := vcSim.ApplyHostConfiguration([]vcsim.SimulatedHostConfig{
		{Name: "esx-01"},
	}); err != nil {
		t.Fatalf("error customizing simulator hosts: %s", err)
	}

	// Supported NetworkList order in VPX: standard Network, DV uplink PG, DVPG.
	if err := vcSim.ApplyNetworkConfiguration([]vcsim.SimulatedNetworkConfig{
		{
			Name: "pg-standard",
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Name: "pg-uplink",
		},
		{
			Name: "pg-distributed",
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
	}); err != nil {
		t.Fatalf("error customizing simulator networks: %s", err)
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
		config        Config
	}{
		{
			name:          "exact name",
			expectFailure: false,
			expectName:    "pg-distributed",
			expectType:    "DistributedVirtualPortgroup",
			config: Config{
				Name: "pg-distributed",
			},
		},
		{
			name:          "no network matches name",
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
			name:          "invalid type",
			expectFailure: true,
			config: Config{
				Type: "vlan",
			},
		},
		{
			name:          "multiple name matches without type",
			expectFailure: true,
			config: Config{
				Name: "pg-*",
			},
		},
		{
			name:          "friendly type standard",
			expectFailure: false,
			expectName:    "pg-standard",
			expectType:    "Network",
			config: Config{
				Type: "standard-port-group",
			},
		},
		{
			name:          "api alias type distributed",
			expectFailure: false,
			expectName:    "pg-distributed",
			expectType:    "DistributedVirtualPortgroup",
			config: Config{
				Name: "pg-distributed",
				Type: "DistributedVirtualPortgroup",
			},
		},
		{
			name:          "regex match single",
			expectFailure: false,
			expectName:    "pg-standard",
			expectType:    "Network",
			config: Config{
				NameRegex: "^pg-standard$",
			},
		},
		{
			name:          "tag filter unique match",
			expectFailure: false,
			expectName:    "pg-distributed",
			expectType:    "DistributedVirtualPortgroup",
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
			name:          "cluster scopes available networks",
			expectFailure: false,
			expectName:    "pg-standard",
			expectType:    "Network",
			config: Config{
				Name:    "pg-standard",
				Cluster: "w01-cl01",
			},
		},
		{
			name:          "host scopes available networks",
			expectFailure: false,
			expectName:    "pg-distributed",
			expectType:    "DistributedVirtualPortgroup",
			config: Config{
				Name: "pg-distributed",
				Host: "esx-01",
			},
		},
		{
			name:          "cluster not found",
			expectFailure: true,
			config: Config{
				Cluster: "missing-cluster",
			},
		},
		{
			name:          "host not found",
			expectFailure: true,
			config: Config{
				Host: "missing-host",
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
			typeVal := output.GetAttr("type")
			if typeVal.AsString() != tt.expectType {
				t.Fatalf("expected type %q, got %q", tt.expectType, typeVal.AsString())
			}
			idVal := output.GetAttr("id")
			if idVal.AsString() == "" {
				t.Fatalf("expected non-empty id")
			}
		})
	}
}

func TestNormalizeNetworkType(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "standard-port-group", want: "Network"},
		{in: "Network", want: "Network"},
		{in: "distributed-port-group", want: "DistributedVirtualPortgroup"},
		{in: "nsx-segment", want: "OpaqueNetwork"},
		{in: "OpaqueNetwork", want: "OpaqueNetwork"},
		{in: "vlan", wantErr: true},
	}
	for _, tt := range tests {
		got, err := normalizeNetworkType(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error: %s", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q: expected %q, got %q", tt.in, tt.want, got)
		}
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package host

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/units"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vsphere"
)

func int64Ptr(v int64) *int64 { return &v }
func int32Ptr(v int32) *int32 { return &v }

func TestDatasource_Execute(t *testing.T) {
	hostsToPrepare := []vsphere.SimulatedHostConfig{
		{
			Name:           "w01-cl01-esx01",
			MemoryCapacity: int64Ptr(int64(64 * units.GB)),
			MemoryUsageMB:  int32Ptr(int32(32 * 1024)), // 32 GiB used → 32 GiB free
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Name:           "w01-cl01-esx02",
			MemoryCapacity: int64Ptr(int64(128 * units.GB)),
			MemoryUsageMB:  int32Ptr(int32(16 * 1024)), // 16 GiB used → 112 GiB free
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
		{
			Name:           "w01-cl01-esx03",
			MemoryCapacity: int64Ptr(int64(64 * units.GB)),
			MemoryUsageMB:  int32Ptr(int32(60 * 1024)), // 60 GiB used → 4 GiB free
			Tags: []vsphere.Tag{
				{Category: "tier", Name: "bronze"},
			},
		},
	}

	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = len(hostsToPrepare)

	vcSim, err := vsphere.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyHostConfiguration(hostsToPrepare); err != nil {
		t.Fatalf("error customizing simulator hosts: %s", err)
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
		name                 string
		expectFailure        bool
		expectName           string
		expectCluster        string
		expectMemoryFree     int64
		expectMemoryCapacity int64
		config               Config
	}{
		{
			name:                 "exact name match",
			expectFailure:        false,
			expectName:           "w01-cl01-esx01",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(32 * units.GB),
			expectMemoryCapacity: int64(64 * units.GB),
			config: Config{
				Name: "w01-cl01-esx01",
			},
		},
		{
			name:          "no host matches name",
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
			name:                 "regex match single",
			expectFailure:        false,
			expectName:           "w01-cl01-esx03",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(4 * units.GB),
			expectMemoryCapacity: int64(64 * units.GB),
			config: Config{
				NameRegex: "^w01-cl01-esx03$",
			},
		},
		{
			name:          "multiple regex matches without most_free_memory",
			expectFailure: true,
			config: Config{
				NameRegex: "^w01-cl01-esx[0-9]+$",
			},
		},
		{
			name:                 "most_free_memory among regex matches",
			expectFailure:        false,
			expectName:           "w01-cl01-esx02",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(112 * units.GB),
			expectMemoryCapacity: int64(128 * units.GB),
			config: Config{
				NameRegex:      "^w01-cl01-esx[0-9]+$",
				MostFreeMemory: true,
			},
		},
		{
			name:                 "tag filter unique match",
			expectFailure:        false,
			expectName:           "w01-cl01-esx02",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(112 * units.GB),
			expectMemoryCapacity: int64(128 * units.GB),
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
			name:                 "tag filter with most_free_memory",
			expectFailure:        false,
			expectName:           "w01-cl01-esx02",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(112 * units.GB),
			expectMemoryCapacity: int64(128 * units.GB),
			config: Config{
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
				},
				MostFreeMemory: true,
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
			name:                 "cluster filter with most_free_memory",
			expectFailure:        false,
			expectName:           "w01-cl01-esx02",
			expectCluster:        "DC0_C0",
			expectMemoryFree:     int64(112 * units.GB),
			expectMemoryCapacity: int64(128 * units.GB),
			config: Config{
				Cluster:        "DC0_C0",
				MostFreeMemory: true,
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
			gotCluster := result.GetAttr("cluster").AsString()
			if gotCluster != tc.expectCluster {
				t.Errorf("expected cluster %q, got %q", tc.expectCluster, gotCluster)
			}

			summary := result.GetAttr("summary")
			if summary.IsNull() {
				t.Fatalf("expected summary object")
			}
			gotFree := summary.GetAttr("memory_free").AsBigFloat()
			free, _ := gotFree.Int64()
			if free != tc.expectMemoryFree {
				t.Errorf("expected memory_free %d, got %d", tc.expectMemoryFree, free)
			}
			gotCapacity := summary.GetAttr("memory_capacity").AsBigFloat()
			capacity, _ := gotCapacity.Int64()
			if capacity != tc.expectMemoryCapacity {
				t.Errorf("expected memory_capacity %d, got %d", tc.expectMemoryCapacity, capacity)
			}
		})
	}
}

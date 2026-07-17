// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package datastorecluster

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/units"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

func int64Ptr(v int64) *int64 { return &v }

func TestDatasource_Execute(t *testing.T) {
	datastoresToPrepare := []vcsim.SimulatedDatastoreConfig{
		{
			Name:      "w01-cl01-ds01",
			Capacity:  int64Ptr(int64(200 * units.GB)),
			FreeSpace: int64Ptr(int64(50 * units.GB)),
		},
		{
			Name:      "w01-cl01-ds02",
			Capacity:  int64Ptr(int64(200 * units.GB)),
			FreeSpace: int64Ptr(int64(100 * units.GB)),
		},
		{
			Name:      "w01-cl01-ds03",
			Capacity:  int64Ptr(int64(500 * units.GB)),
			FreeSpace: int64Ptr(int64(400 * units.GB)),
		},
		{
			Name:      "w01-cl01-ds04",
			Capacity:  int64Ptr(int64(500 * units.GB)),
			FreeSpace: int64Ptr(int64(300 * units.GB)),
		},
	}

	clustersToPrepare := []vcsim.SimulatedDatastoreClusterConfig{
		{
			Name:          "w01-cl01-dsc01",
			MemberIndexes: []int{0, 1},
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Name:          "w01-cl01-dsc02",
			MemberIndexes: []int{2, 3},
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
	}

	model := simulator.VPX()
	model.Datastore = len(datastoresToPrepare)
	model.Pod = len(clustersToPrepare)

	vcSim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyDatastoreConfiguration(datastoresToPrepare); err != nil {
		t.Fatalf("error customizing simulator datastores: %s", err)
	}
	if err := vcSim.ApplyDatastoreClusterConfiguration(clustersToPrepare); err != nil {
		t.Fatalf("error customizing simulator datastore clusters: %s", err)
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
		name           string
		expectFailure  bool
		expectName     string
		expectFree     int64
		expectCapacity int64
		expectMembers  []string
		config         Config
	}{
		{
			name:           "exact name match",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc01",
			expectFree:     int64(150 * units.GB),
			expectCapacity: int64(400 * units.GB),
			expectMembers:  []string{"w01-cl01-ds01", "w01-cl01-ds02"},
			config: Config{
				Name: "w01-cl01-dsc01",
			},
		},
		{
			name:          "no datastore cluster matches name",
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
			name:          "multiple regex matches without most_free_space",
			expectFailure: true,
			config: Config{
				NameRegex: "^w01-cl01-dsc[0-9]+$",
			},
		},
		{
			name:           "most_free_space among regex matches",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc02",
			expectFree:     int64(700 * units.GB),
			expectCapacity: int64(1000 * units.GB),
			expectMembers:  []string{"w01-cl01-ds03", "w01-cl01-ds04"},
			config: Config{
				NameRegex:     "^w01-cl01-dsc[0-9]+$",
				MostFreeSpace: true,
			},
		},
		{
			name:           "tag filter unique match",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc02",
			expectFree:     int64(700 * units.GB),
			expectCapacity: int64(1000 * units.GB),
			expectMembers:  []string{"w01-cl01-ds03", "w01-cl01-ds04"},
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
			name:           "tag filter with most_free_space",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc02",
			expectFree:     int64(700 * units.GB),
			expectCapacity: int64(1000 * units.GB),
			expectMembers:  []string{"w01-cl01-ds03", "w01-cl01-ds04"},
			config: Config{
				Tags: []Tag{
					{Category: "env", Name: "Packer"},
				},
				MostFreeSpace: true,
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
			name:           "host filter with most_free_space",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc02",
			expectFree:     int64(700 * units.GB),
			expectCapacity: int64(1000 * units.GB),
			expectMembers:  []string{"w01-cl01-ds03", "w01-cl01-ds04"},
			config: Config{
				Host:          "DC0_H0",
				MostFreeSpace: true,
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
			name:           "cluster filter with most_free_space",
			expectFailure:  false,
			expectName:     "w01-cl01-dsc02",
			expectFree:     int64(700 * units.GB),
			expectCapacity: int64(1000 * units.GB),
			expectMembers:  []string{"w01-cl01-ds03", "w01-cl01-ds04"},
			config: Config{
				Cluster:       "DC0_C0",
				MostFreeSpace: true,
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

			gotMembers := result.GetAttr("datastores").AsValueSlice()
			if len(gotMembers) != len(tc.expectMembers) {
				t.Fatalf("expected %d members, got %d", len(tc.expectMembers), len(gotMembers))
			}
			memberSet := make(map[string]struct{}, len(gotMembers))
			for _, m := range gotMembers {
				memberSet[m.AsString()] = struct{}{}
			}
			for _, expect := range tc.expectMembers {
				if _, ok := memberSet[expect]; !ok {
					t.Errorf("expected member %q in %v", expect, memberSet)
				}
			}

			summary := result.GetAttr("summary")
			if summary.IsNull() {
				t.Fatalf("expected summary object")
			}
			gotFree := summary.GetAttr("free").AsBigFloat()
			free, _ := gotFree.Int64()
			if free != tc.expectFree {
				t.Errorf("expected free %d, got %d", tc.expectFree, free)
			}
			gotCapacity := summary.GetAttr("capacity").AsBigFloat()
			capacity, _ := gotCapacity.Int64()
			if capacity != tc.expectCapacity {
				t.Errorf("expected capacity %d, got %d", tc.expectCapacity, capacity)
			}
		})
	}
}

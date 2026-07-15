// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package computecluster

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vsphere"
)

func TestDatasource_Execute(t *testing.T) {
	clustersToPrepare := []vsphere.SimulatedComputeClusterConfig{
		{
			Name: "w01-cl01",
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
			},
		},
		{
			Name: "w01-cl02",
			Tags: []vsphere.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
	}

	model := simulator.VPX()
	model.Cluster = len(clustersToPrepare)
	model.ClusterHost = 1
	model.Host = 0

	vcSim, err := vsphere.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyComputeClusterConfiguration(clustersToPrepare); err != nil {
		t.Fatalf("error customizing simulator compute clusters: %s", err)
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
			name:          "exact name match",
			expectFailure: false,
			expectName:    "w01-cl01",
			config: Config{
				Name: "w01-cl01",
			},
		},
		{
			name:          "no compute cluster matches name",
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
			name:          "regex match single",
			expectFailure: false,
			expectName:    "w01-cl02",
			config: Config{
				NameRegex: "^w01-cl02$",
			},
		},
		{
			name:          "multiple regex matches",
			expectFailure: true,
			config: Config{
				NameRegex: "^w01-cl[0-9]+$",
			},
		},
		{
			name:          "tag filter unique match",
			expectFailure: false,
			expectName:    "w01-cl02",
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
			gotPool := result.GetAttr("resource_pool").AsString()
			if gotPool == "" {
				t.Errorf("expected non-empty resource_pool")
			}
			if !strings.HasSuffix(gotPool, "/Resources") && gotPool != "Resources" {
				t.Errorf("expected root resource pool path ending in /Resources or name Resources, got %q", gotPool)
			}
		})
	}
}

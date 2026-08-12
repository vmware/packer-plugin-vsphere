// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

func TestListAttachedTags(t *testing.T) {
	hostsToPrepare := []vcsim.SimulatedHostConfig{
		{
			Name: "w01-cl01-esx01",
			Tags: []vcsim.Tag{
				{Category: "env", Name: "Packer"},
				{Category: "tier", Name: "gold"},
			},
		},
		{
			Name: "w01-cl01-esx02",
		},
	}

	model := simulator.VPX()
	model.Host = 0
	model.Cluster = 1
	model.ClusterHost = len(hostsToPrepare)

	vcSim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	if err := vcSim.ApplyHostConfiguration(hostsToPrepare); err != nil {
		t.Fatalf("error customizing simulator hosts: %s", err)
	}

	simulatorPassword, _ := vcSim.Server.URL.User.Password()
	dr, err := driver.NewDriver(&driver.ConnectConfig{
		VCenterServer:      vcSim.Server.URL.Host,
		Username:           vcSim.Server.URL.User.Username(),
		Password:           simulatorPassword,
		InsecureConnection: true,
		Datacenter:         vcSim.Datacenter.Name(),
	})
	if err != nil {
		t.Fatalf("error creating driver: %s", err)
	}
	vcDriver := dr.(*driver.VCenterDriver)

	hosts, err := vcDriver.Finder.HostSystemList(vcDriver.Ctx, "*")
	if err != nil {
		t.Fatalf("error listing hosts: %s", err)
	}
	if len(hosts) < 2 {
		t.Fatalf("expected at least 2 hosts, got %d", len(hosts))
	}

	t.Run("returns attached tags", func(t *testing.T) {
		got, err := ListAttachedTags(vcDriver, hosts[0].Reference())
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		want := map[string]string{
			"Packer": "env",
			"gold":   "tier",
		}
		if len(got) != len(want) {
			t.Fatalf("expected %d tags, got %d: %#v", len(want), len(got), got)
		}
		for _, tag := range got {
			category, ok := want[tag.Name]
			if !ok {
				t.Fatalf("unexpected tag %q", tag.Name)
			}
			if tag.Category != category {
				t.Fatalf("tag %q: expected category %q, got %q", tag.Name, category, tag.Category)
			}
		}
	})

	t.Run("empty when no tags", func(t *testing.T) {
		got, err := ListAttachedTags(vcDriver, hosts[1].Reference())
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty tags, got %#v", got)
		}
	})

	t.Run("map from tags", func(t *testing.T) {
		in := []Tag{{Name: "a", Category: "c"}}
		out := MapFromTags(in, func(name, category string) types.KeyValue {
			return types.KeyValue{Key: category, Value: name}
		})
		if len(out) != 1 || out[0].Key != "c" || out[0].Value != "a" {
			t.Fatalf("unexpected map result: %#v", out)
		}
	})
}

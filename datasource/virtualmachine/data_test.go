// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package virtualmachine

import (
	"testing"
	"time"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
	"github.com/zclconf/go-cty/cty"
)

func TestDatasource_Execute(t *testing.T) {
	machinesToPrepare := []vcsim.SimulatedVmConfig{
		{
			Name: "first-vm",
			Tags: []vcsim.Tag{
				{
					Category: "operating-system-class",
					Name:     "Linux",
				},
			},
		}, {
			Name: "second-vm",
			Tags: []vcsim.Tag{
				{
					Category: "operating-system-class",
					Name:     "Linux",
				},
				{
					Category: "security-team",
					Name:     "red",
				},
				{
					Category: "security-team",
					Name:     "blue",
				},
			},
			Template: true,
		}, {
			Name: "machine-three",
			Tags: []vcsim.Tag{
				{
					Category: "operating-system-class",
					Name:     "Linux",
				},
				{
					Category: "security-team",
					Name:     "blue",
				},
			},
			CreationTime: time.Now().AddDate(0, 0, 1),
		},
	}

	model := simulator.VPX()
	model.Datacenter = 2
	model.Machine = 8

	vcSim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("error creating vCenter simulator: %s", err)
	}
	defer vcSim.Stop()

	err = vcSim.ApplyVmConfiguration(machinesToPrepare)
	if err != nil {
		t.Fatalf("error customizing simulator: %s", err)
	}

	simulatorPassword, _ := vcSim.Server.URL.User.Password()
	connectConfig := common.ConnectConfig{
		VCenterServer:      vcSim.Server.URL.Host,
		Username:           vcSim.Server.URL.User.Username(),
		Password:           simulatorPassword,
		InsecureConnection: true,
		Datacenter:         vcSim.Datacenter.Name(),
	}

	dsTestConfigs := []struct {
		name          string
		expectFailure bool
		expectVmName  string
		expectTags    []Tag
		config        Config
	}{
		{
			name:          "first-vm was found by name, no error",
			expectFailure: false,
			expectVmName:  "first-vm",
			expectTags: []Tag{
				{Category: "operating-system-class", Name: "Linux"},
			},
			config: Config{
				Name: "first-vm",
			},
		},
		{
			name:          "no machines match the filter, error",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				Name: "firstest-vm",
			},
		},
		{
			name:          "second-vm was found by the regex, no error",
			expectFailure: false,
			expectVmName:  "second-vm",
			expectTags: []Tag{
				{Category: "operating-system-class", Name: "Linux"},
				{Category: "security-team", Name: "red"},
				{Category: "security-team", Name: "blue"},
			},
			config: Config{
				NameRegex: "^seco.*m$",
			},
		},
		{
			name:          "multiple machines match the regex, but latest not used, error",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				NameRegex: ".*-vm",
			},
		},
		{
			name:          "multiple guests match the regex and latest used, no error",
			expectFailure: false,
			expectVmName:  "machine-three",
			expectTags: []Tag{
				{Category: "operating-system-class", Name: "Linux"},
				{Category: "security-team", Name: "blue"},
			},
			config: Config{
				NameRegex: "^[^_]+$",
				Latest:    true,
			},
		},
		{
			name:          "found machine that is a template, no error",
			expectFailure: false,
			expectVmName:  "second-vm",
			expectTags: []Tag{
				{Category: "operating-system-class", Name: "Linux"},
				{Category: "security-team", Name: "red"},
				{Category: "security-team", Name: "blue"},
			},
			config: Config{
				Template: true,
			},
		},
		{
			name:          "found multiple machines at the host, error",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				Host: "DC0_H0",
			},
		},
		{
			name:          "cluster host not found, error",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				Host: "unexpected_host",
			},
		},
		{
			name:          "invalid name_regex",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				NameRegex: "(",
			},
		},
		{
			name:          "found machine with defined set of tags, no error",
			expectFailure: false,
			expectVmName:  "second-vm",
			expectTags: []Tag{
				{Category: "operating-system-class", Name: "Linux"},
				{Category: "security-team", Name: "red"},
				{Category: "security-team", Name: "blue"},
			},
			config: Config{
				Tags: []Tag{
					{
						Category: "security-team",
						Name:     "blue",
					},
					{
						Category: "security-team",
						Name:     "red",
					},
				},
			},
		},
		{
			name:          "found multiple machines with defined set of tags, error",
			expectFailure: true,
			expectVmName:  "",
			config: Config{
				Tags: []Tag{
					{
						Category: "operating-system-class",
						Name:     "Linux",
					},
				},
			},
		},
	}

	for _, testConfig := range dsTestConfigs {
		t.Run(testConfig.name, func(t *testing.T) {
			testConfig.config.ConnectConfig = connectConfig

			ds := Datasource{
				config: testConfig.config,
			}
			err := ds.Configure()
			if err != nil {
				t.Fatalf("Failed to configure datasource: %s", err)
			}

			result, err := ds.Execute()
			if err != nil && !testConfig.expectFailure {
				t.Fatalf("unexpected failure: %s", err)
			}
			if err == nil && testConfig.expectFailure {
				t.Errorf("expected failure, but execution succeeded")
			}
			if err == nil {
				vmName := result.GetAttr("vm_name").AsString()
				if vmName != testConfig.expectVmName {
					t.Errorf("expected vm name `%s`, but got `%s`", testConfig.expectVmName, vmName)
				}
				assertTags(t, result.GetAttr("tags"), testConfig.expectTags)
			}
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

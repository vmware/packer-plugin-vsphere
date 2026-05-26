// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestNewVAppConfigSpec(t *testing.T) {
	spec := newVAppConfigSpec(map[string]string{
		"hostname":    "host1",
		"public-keys": "ssh-rsa AAA",
	})

	if spec == nil {
		t.Fatal("expected non-nil VmConfigSpec")
	}
	if len(spec.Property) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(spec.Property))
	}
	if len(spec.OvfEnvironmentTransport) != 2 {
		t.Fatalf("expected guestInfo transports, got %v", spec.OvfEnvironmentTransport)
	}

	ids := make(map[string]types.VAppPropertySpec)
	for _, p := range spec.Property {
		if p.Operation != types.ArrayUpdateOperationAdd {
			t.Fatalf("expected add operation for %s, got %s", p.Info.Id, p.Operation)
		}
		ids[p.Info.Id] = p
	}

	if ids["public-keys"].Info.Label != "SSH Public Keys" {
		t.Fatalf("unexpected label for public-keys: %s", ids["public-keys"].Info.Label)
	}
	if ids["hostname"].Info.Value != "host1" {
		t.Fatalf("unexpected hostname value: %s", ids["hostname"].Info.Value)
	}
}

func TestBuildVAppConfigSpecNoVAppWithoutEnable(t *testing.T) {
	_, err := buildVAppConfigSpecFromInfo(nil, false, map[string]string{"hostname": "host1"})
	if err == nil {
		t.Fatal("expected error when vApp is missing and enable is false")
	}
}

func TestBuildVAppConfigSpecEditsExistingProperty(t *testing.T) {
	userConfigurable := true
	vApp := &types.VmConfigInfo{
		Property: []types.VAppPropertyInfo{
			{
				Key:              7,
				Id:               "hostname",
				UserConfigurable: &userConfigurable,
			},
		},
	}

	spec, err := buildVAppConfigSpecFromInfo(vApp, false, map[string]string{"hostname": "new-host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil || len(spec.Property) != 1 {
		t.Fatalf("expected one property spec, got %#v", spec)
	}
	if spec.Property[0].Operation != types.ArrayUpdateOperationEdit {
		t.Fatalf("expected edit operation, got %s", spec.Property[0].Operation)
	}
	if spec.Property[0].Info.Value != "new-host" {
		t.Fatalf("unexpected value: %s", spec.Property[0].Info.Value)
	}
}

func TestBuildVAppConfigSpecAddsMissingPropertyWhenEnabled(t *testing.T) {
	userConfigurable := true
	vApp := &types.VmConfigInfo{
		Property: []types.VAppPropertyInfo{
			{
				Key:              1,
				Id:               "hostname",
				UserConfigurable: &userConfigurable,
			},
		},
	}

	spec, err := buildVAppConfigSpecFromInfo(vApp, true, map[string]string{
		"hostname":    "host1",
		"public-keys": "ssh-rsa AAA",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Property) != 2 {
		t.Fatalf("expected edit and add operations, got %d specs", len(spec.Property))
	}
}

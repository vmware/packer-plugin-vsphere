// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	pbmsim "github.com/vmware/govmomi/pbm/simulator"
	"github.com/vmware/govmomi/vim25/types"
)

func TestVCenterDriver_FindCompatibleDatastore(t *testing.T) {
	sim := mustVPXSimulator(t)
	d := newSimulatorDriver(sim)

	ds, err := d.FindCompatibleDatastore(pbmsim.DefaultEncryptionProfileID, "DC0_H0", "")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ds == nil || ds.Name() == "" {
		t.Fatal("expected a compatible datastore name")
	}
}

func TestVCenterDriver_FindCompatibleDatastore_WithCluster(t *testing.T) {
	sim := mustVPXSimulator(t)
	d := newSimulatorDriver(sim)

	ds, err := d.FindCompatibleDatastore(pbmsim.DefaultEncryptionProfileID, "", "DC0_C0")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ds == nil || ds.Name() == "" {
		t.Fatal("expected a compatible datastore name")
	}
}

func TestVCenterDriver_FindCompatibleDatastore_EmptyPolicyID(t *testing.T) {
	sim := mustVPXSimulator(t)
	d := newSimulatorDriver(sim)

	_, err := d.FindCompatibleDatastore("", "DC0_H0", "")
	if err == nil {
		t.Fatal("expected error for empty policy ID")
	}
}

func TestVCenterDriver_FindStoragePolicyID(t *testing.T) {
	sim := mustVPXSimulator(t)
	d := newSimulatorDriver(sim)

	id, err := d.FindStoragePolicyID("VM Encryption Policy")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if id != pbmsim.DefaultEncryptionProfileID {
		t.Fatalf("unexpected policy ID: got %q, want %q", id, pbmsim.DefaultEncryptionProfileID)
	}
}

func TestDiskDatastoreName_FromFileName(t *testing.T) {
	disk := &types.VirtualDisk{}
	disk.Backing = &types.VirtualDiskFlatVer2BackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName: "[local-ssd01-esx01] vm/disk.vmdk",
		},
	}
	got, err := diskDatastoreName(&VCenterDriver{}, disk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "local-ssd01-esx01" {
		t.Fatalf("unexpected datastore name: got %q", got)
	}
}

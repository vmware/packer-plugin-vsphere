// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/govmomi/vapi/vcenter"
)

func TestBuildVmtxLibraryItemDeploySpec(t *testing.T) {
	config := &ContentLibraryDeployConfig{
		Name:       "example-vm",
		Annotation: "Packer built",
	}
	target := &contentLibraryDeployTarget{
		Target: vcenter.Target{
			ResourcePoolID: "resgroup-1",
			HostID:         "host-1",
			FolderID:       "group-v1",
		},
		datastoreID: "datastore-1",
	}

	deploy := buildVmtxLibraryItemDeploySpec(config, target)

	if deploy.Name != "example-vm" {
		t.Fatalf("unexpected name: %q", deploy.Name)
	}
	if deploy.Description != "Packer built" {
		t.Fatalf("unexpected description: %q", deploy.Description)
	}
	if deploy.DiskStorage == nil || deploy.DiskStorage.Datastore != "datastore-1" {
		t.Fatalf("unexpected disk storage: %#v", deploy.DiskStorage)
	}
	if deploy.DiskStorage.StoragePolicy == nil || deploy.DiskStorage.StoragePolicy.Type != "USE_SOURCE_POLICY" {
		t.Fatalf("unexpected disk storage policy: %#v", deploy.DiskStorage.StoragePolicy)
	}
	if deploy.VMHomeStorage == nil || deploy.VMHomeStorage.Datastore != "datastore-1" {
		t.Fatalf("unexpected VM home storage: %#v", deploy.VMHomeStorage)
	}
	if deploy.VMHomeStorage.StoragePolicy == nil || deploy.VMHomeStorage.StoragePolicy.Type != "USE_SOURCE_POLICY" {
		t.Fatalf("unexpected VM home storage policy: %#v", deploy.VMHomeStorage.StoragePolicy)
	}
	if deploy.Placement == nil {
		t.Fatal("expected placement")
	}
	if deploy.Placement.ResourcePool != "resgroup-1" {
		t.Fatalf("unexpected resource pool: %q", deploy.Placement.ResourcePool)
	}
	if deploy.Placement.Host != "host-1" {
		t.Fatalf("unexpected host: %q", deploy.Placement.Host)
	}
	if deploy.Placement.Folder != "group-v1" {
		t.Fatalf("unexpected folder: %q", deploy.Placement.Folder)
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestAddStorageDevices(t *testing.T) {
	config := &StorageConfig{
		DiskControllerType: []string{"pvscsi"},
		Storage: []Disk{
			{
				DiskSize:            3072,
				DiskThinProvisioned: true,
				ControllerIndex:     0,
			},
			{
				DiskSize:            20480,
				DiskThinProvisioned: true,
				ControllerIndex:     0,
			},
		},
	}

	noExistingDevices := object.VirtualDeviceList{}
	storageConfigSpec, err := config.AddStorageDevices(noExistingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(storageConfigSpec) != 3 {
		t.Fatalf("unexpected result: expected '3', but returned '%d'", len(storageConfigSpec))
	}

	existingDevices := object.VirtualDeviceList{}
	device, err := existingDevices.CreateNVMEController()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, device)

	storageConfigSpec, err = config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(storageConfigSpec) != 3 {
		t.Fatalf("unexpected result: expected '3', but returned '%d'", len(storageConfigSpec))
	}
}

func TestAddStorageDevicesExplicitUnit(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}
	controller, err := existingDevices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, controller)

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: existingDevices.NewKey(),
		},
		CapacityInKB: 1024,
	}
	controllerObj, err := existingDevices.FindDiskController(existingDevices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	assignDiskAtUnit(existingDevices, disk, controllerObj, 0, true)
	existingDevices = append(existingDevices, disk)

	config := &StorageConfig{
		Storage: []Disk{
			{
				DiskSize:       4096,
				ControllerUnit: "scsi0:1",
			},
		},
	}

	storageConfigSpec, err := config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(storageConfigSpec) != 1 {
		t.Fatalf("unexpected result: expected '1', but returned '%d'", len(storageConfigSpec))
	}

	added := storageConfigSpec[0].GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
	if added.UnitNumber == nil || *added.UnitNumber != 1 {
		t.Fatalf("unexpected unit number: expected 1, got %#v", added.UnitNumber)
	}
}

func TestAddStorageDevicesExplicitUnitNewController(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}
	controller, err := existingDevices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, controller)

	config := &StorageConfig{
		DiskControllerType: []string{"pvscsi"},
		Storage: []Disk{
			{
				DiskSize:       4096,
				ControllerUnit: "scsi1:0",
			},
		},
	}

	storageConfigSpec, err := config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(storageConfigSpec) != 2 {
		t.Fatalf("unexpected result: expected '2', but returned '%d'", len(storageConfigSpec))
	}

	var added *types.VirtualDisk
	for _, spec := range storageConfigSpec {
		disk, ok := spec.GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		if disk.UnitNumber != nil && *disk.UnitNumber == 0 {
			added = disk
			break
		}
	}
	if added == nil {
		t.Fatal("expected disk at scsi1:0")
	}
}

func TestAddStorageDevicesMixedLegacyAndExplicit(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}
	controller, err := existingDevices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, controller)

	config := &StorageConfig{
		DiskControllerType: []string{"pvscsi", "pvscsi"},
		Storage: []Disk{
			{
				DiskSize:        2048,
				ControllerIndex: 0,
			},
			{
				DiskSize:       4096,
				ControllerUnit: "scsi2:0",
			},
		},
	}

	storageConfigSpec, err := config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(storageConfigSpec) != 4 {
		t.Fatalf("unexpected result: expected '4', but returned '%d'", len(storageConfigSpec))
	}

	devices := append(object.VirtualDeviceList{}, existingDevices...)
	for _, spec := range storageConfigSpec {
		devices = append(devices, spec.GetVirtualDeviceConfigSpec().Device)
	}

	if FindControllerByBus(devices, ControllerKindSCSI, 2) == nil {
		t.Fatal("expected scsi2 controller from on-demand creation")
	}
}

func TestAddStorageDevicesMixedControllerKinds(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}
	// Templates always have at least one existing SCSI controller (scsi0);
	// without it, a brand-new SCSI controller would be assigned bus 0, not
	// bus 1, so "scsi1:0" below would not match.
	controller, err := existingDevices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, controller)

	config := &StorageConfig{
		DiskControllerType: []string{"pvscsi", "sata", "nvme"},
		Storage: []Disk{
			{DiskSize: 2048, ControllerUnit: "scsi1:0"},
			{DiskSize: 2048, ControllerUnit: "sata0:0"},
			{DiskSize: 2048, ControllerUnit: "nvme0:0"},
		},
	}

	storageConfigSpec, err := config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices := append(object.VirtualDeviceList{}, existingDevices...)
	for _, spec := range storageConfigSpec {
		devices = append(devices, spec.GetVirtualDeviceConfigSpec().Device)
	}

	if len(storageConfigSpec) != 6 {
		t.Fatalf("unexpected result: expected '6' (3 controllers + 3 disks), but returned '%d'", len(storageConfigSpec))
	}
	if FindControllerByBus(devices, ControllerKindSCSI, 1) == nil {
		t.Fatal("expected scsi1 controller from on-demand creation")
	}
	if FindControllerByBus(devices, ControllerKindSATA, 0) == nil {
		t.Fatal("expected sata0 controller from on-demand creation")
	}
	if FindControllerByBus(devices, ControllerKindNVMe, 0) == nil {
		t.Fatal("expected nvme0 controller from on-demand creation")
	}
}

func TestAddStorageDevicesMismatchedControllerKindOrder(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}

	// scsi0 is referenced first but disk_controller_type[0] is "nvme": the
	// type-index allocator is purely order-based across ALL controller
	// kinds, not per-kind, so this must fail rather than silently create an
	// NVMe controller for what the caller addressed as scsi0.
	config := &StorageConfig{
		DiskControllerType: []string{"nvme", "pvscsi"},
		Storage: []Disk{
			{DiskSize: 2048, ControllerUnit: "scsi0:0"},
			{DiskSize: 2048, ControllerUnit: "nvme0:0"},
		},
	}

	_, err := config.AddStorageDevices(existingDevices)
	if err == nil || !strings.Contains(err.Error(), "created controller bus does not match") {
		t.Fatalf("expected a controller bus mismatch error, got %v", err)
	}
}

func TestAddStorageDevicesTypePoolExhausted(t *testing.T) {
	existingDevices := object.VirtualDeviceList{}

	config := &StorageConfig{
		Storage: []Disk{
			{
				DiskSize:       4096,
				ControllerUnit: "scsi0:0",
			},
		},
	}

	_, err := config.AddStorageDevices(existingDevices)
	if err == nil || !strings.Contains(err.Error(), "no disk_controller_type entries remain") {
		t.Fatalf("expected type pool exhausted error, got %v", err)
	}
}

// TestAddStorageDevices_WithStoragePolicy verifies that a disk with a
// StoragePolicyID gets a VirtualMachineDefinedProfileSpec in its config spec,
// while a disk without a policy gets no profile entry.
func TestAddStorageDevices_WithStoragePolicy(t *testing.T) {
	const policyUUID = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"

	config := &StorageConfig{
		DiskControllerType: []string{"pvscsi"},
		Storage: []Disk{
			{DiskSize: 10240, DiskThinProvisioned: true, ControllerIndex: 0, StoragePolicyID: policyUUID},
			{DiskSize: 20480, DiskThinProvisioned: true, ControllerIndex: 0},
		},
	}

	specs, err := config.AddStorageDevices(object.VirtualDeviceList{})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	// 1 controller + 2 disks
	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d", len(specs))
	}

	// specs[0] = controller (no profile expected)
	// specs[1] = first disk (policy set)
	// specs[2] = second disk (no policy)
	disk0spec := specs[1].(*types.VirtualDeviceConfigSpec)
	if len(disk0spec.Profile) != 1 {
		t.Fatalf("expected 1 profile on disk 0, got %d", len(disk0spec.Profile))
	}
	profileSpec, ok := disk0spec.Profile[0].(*types.VirtualMachineDefinedProfileSpec)
	if !ok {
		t.Fatal("expected VirtualMachineDefinedProfileSpec on disk 0")
	}
	if profileSpec.ProfileId != policyUUID {
		t.Fatalf("expected profile ID %q, got %q", policyUUID, profileSpec.ProfileId)
	}

	disk1spec := specs[2].(*types.VirtualDeviceConfigSpec)
	if len(disk1spec.Profile) != 0 {
		t.Fatalf("expected no profile on disk 1, got %d", len(disk1spec.Profile))
	}
}

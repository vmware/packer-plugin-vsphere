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

func TestBuildStoragePlacementConfigSpecMatchesAddStorageDevices(t *testing.T) {
	config := StorageConfig{
		DiskControllerType: []string{"pvscsi"},
		Storage: []Disk{
			{DiskSize: 4096, ControllerUnit: "scsi1:0"},
			{DiskSize: 8192, ControllerUnit: "scsi1:1"},
		},
	}

	existingDevices := object.VirtualDeviceList{}
	controller, err := existingDevices.CreateSCSIController(controllerTypePVSCSI)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	existingDevices = append(existingDevices, controller)

	placementSpecs, newDiskKeys, err := BuildStoragePlacementConfigSpec(StoragePlacementInput{
		StorageConfig:   config,
		ExistingDevices: existingDevices,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	directSpecs, err := config.AddStorageDevices(existingDevices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(placementSpecs) != len(directSpecs) {
		t.Fatalf("expected %d device specs, got %d", len(directSpecs), len(placementSpecs))
	}
	if len(newDiskKeys) != len(config.Storage) {
		t.Fatalf("expected %d new disk keys, got %d", len(config.Storage), len(newDiskKeys))
	}
}

func TestBuildStoragePlacementConfigSpecWithPrimaryResize(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController(controllerTypePVSCSI)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: devices.NewKey(),
		},
		CapacityInKB: 1024,
	}
	controllerObj, err := devices.FindDiskController(devices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	assignDiskAtUnit(devices, disk, controllerObj, 0, true)
	devices = append(devices, disk)

	specs, _, err := BuildStoragePlacementConfigSpec(StoragePlacementInput{
		ExistingDevices: StorageExistingDevices(devices),
		PrimaryDiskSize: 4096,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected one resize spec, got %d", len(specs))
	}

	editSpec := specs[0].GetVirtualDeviceConfigSpec()
	if editSpec.Operation != types.VirtualDeviceConfigSpecOperationEdit {
		t.Fatalf("expected edit operation, got %v", editSpec.Operation)
	}
}

func TestDiskDatastoreMappingsFromRecommendation(t *testing.T) {
	recommendation := types.ClusterRecommendation{
		Action: []types.BaseClusterAction{
			&types.StoragePlacementAction{
				RelocateSpec: types.VirtualMachineRelocateSpec{
					Disk: []types.VirtualMachineRelocateSpecDiskLocator{
						{DiskId: 2000, Datastore: types.ManagedObjectReference{Type: "Datastore", Value: "datastore-1"}},
						{DiskId: 2001, Datastore: types.ManagedObjectReference{Type: "Datastore", Value: "datastore-2"}},
					},
				},
			},
		},
	}

	mappings := diskDatastoreMappingsFromRecommendation(recommendation)
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].diskKey != 2000 || mappings[1].diskKey != 2001 {
		t.Fatalf("unexpected disk key mappings: %#v", mappings)
	}
}

func TestDestinationOnlyActions(t *testing.T) {
	recommendation := types.ClusterRecommendation{
		Action: []types.BaseClusterAction{
			&types.StoragePlacementAction{
				Destination: types.ManagedObjectReference{Type: "Datastore", Value: "datastore-1"},
			},
			&types.StoragePlacementAction{
				Destination: types.ManagedObjectReference{Type: "Datastore", Value: "datastore-2"},
				RelocateSpec: types.VirtualMachineRelocateSpec{
					Disk: []types.VirtualMachineRelocateSpecDiskLocator{
						{DiskId: 1, Datastore: types.ManagedObjectReference{Type: "Datastore", Value: "ignored"}},
					},
				},
			},
		},
	}

	destinations := destinationOnlyActions(recommendation)
	if len(destinations) != 1 {
		t.Fatalf("expected 1 destination-only action, got %d", len(destinations))
	}
	if destinations[0].Value != "datastore-1" {
		t.Fatalf("unexpected destination: %#v", destinations[0])
	}
}

func TestAddStorageDevicesForPlacementDoesNotMutateTemplateControllers(t *testing.T) {
	templateDevices := object.VirtualDeviceList{}
	controller, err := templateDevices.CreateSCSIController(controllerTypePVSCSI)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	templateDevices = append(templateDevices, controller)

	controllerObj, err := templateDevices.FindDiskController(templateDevices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	initialDeviceCount := len(controllerObj.GetVirtualController().Device)

	config := StorageConfig{
		Storage: []Disk{{DiskSize: 4096, ControllerUnit: "scsi0:1"}},
	}
	_, err = config.AddStorageDevicesForPlacement(StorageExistingDevices(templateDevices))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(controllerObj.GetVirtualController().Device) != initialDeviceCount {
		t.Fatal("expected template controller device list to remain unchanged during placement")
	}
}

func TestValidateAggregateStorageCapacity(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController(controllerTypeLSILogic)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	controllerObj, err := devices.FindDiskController(devices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	for i := 0; i < maxLSISCSIDisksPerVM; i++ {
		disk := &types.VirtualDisk{
			VirtualDevice: types.VirtualDevice{
				Key:           devices.NewKey(),
				ControllerKey: controllerObj.GetVirtualController().Key,
			},
			CapacityInKB: 1024,
		}
		devices = append(devices, disk)
	}

	counts := countVirtualDisksByKind(devices)
	if counts[ControllerKindSCSI] != maxLSISCSIDisksPerVM {
		t.Fatalf("expected %d SCSI disks in fixture, got %d", maxLSISCSIDisksPerVM, counts[ControllerKindSCSI])
	}

	err = validateAggregateStorageCapacity(devices)
	if err != nil {
		t.Fatalf("expected 60 SCSI disks to be valid, got %v", err)
	}

	extra := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key:           devices.NewKey(),
			ControllerKey: controllerObj.GetVirtualController().Key,
		},
		CapacityInKB: 1024,
	}
	devices = append(devices, extra)
	err = validateAggregateStorageCapacity(devices)
	if err == nil || !strings.Contains(err.Error(), "supports maximum 60 SCSI disks") {
		t.Fatalf("expected aggregate SCSI capacity error after extra disk, got %v", err)
	}
}

func TestNewVirtualDiskKeysFromConfigSpec(t *testing.T) {
	specs := []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    &types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2000}},
		},
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    &types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2001}},
		},
	}

	keys := newVirtualDiskKeysFromConfigSpec(specs)
	if len(keys) != 2 || keys[0] != 2000 || keys[1] != 2001 {
		t.Fatalf("unexpected disk keys: %#v", keys)
	}
}

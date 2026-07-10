// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// StoragePlacementInput describes VM storage changes submitted to Storage DRS.
type StoragePlacementInput struct {
	StorageConfig   StorageConfig
	ExistingDevices object.VirtualDeviceList
	PrimaryDiskSize int64
}

// StorageExistingDevices returns template disks and controllers for placement.
func StorageExistingDevices(devices object.VirtualDeviceList) object.VirtualDeviceList {
	existing := object.VirtualDeviceList{}
	existing = append(existing, devices.SelectByType((*types.VirtualDisk)(nil))...)
	existing = append(existing, devices.SelectByType((*types.VirtualController)(nil))...)
	return existing
}

// BuildStoragePlacementConfigSpec builds device changes for Storage DRS using the
// same storage attachment logic as clone and create.
func BuildStoragePlacementConfigSpec(input StoragePlacementInput) ([]types.BaseVirtualDeviceConfigSpec, []int32, error) {
	var deviceChanges []types.BaseVirtualDeviceConfigSpec

	if input.PrimaryDiskSize > 0 && len(input.ExistingDevices) > 0 {
		resizeSpecs, err := buildPrimaryDiskResizeSpec(input.ExistingDevices, input.PrimaryDiskSize)
		if err != nil {
			return nil, nil, fmt.Errorf("error building primary disk resize spec: %s", err)
		}
		deviceChanges = append(deviceChanges, resizeSpecs...)
	}

	storageSpecs, err := input.StorageConfig.AddStorageDevicesForPlacement(input.ExistingDevices)
	if err != nil {
		return nil, nil, fmt.Errorf("error building storage device spec: %s", err)
	}
	deviceChanges = append(deviceChanges, storageSpecs...)

	return deviceChanges, newVirtualDiskKeysFromConfigSpec(storageSpecs), nil
}

func buildPrimaryDiskResizeSpec(devices object.VirtualDeviceList, diskSize int64) ([]types.BaseVirtualDeviceConfigSpec, error) {
	disk, err := findDisk(devices)
	if err != nil {
		return nil, err
	}

	resized := *disk
	resized.CapacityInKB = diskSize * 1024
	resized.CapacityInBytes = resized.CapacityInKB * 1024

	return []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:    &resized,
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		},
	}, nil
}

func newVirtualDiskKeysFromConfigSpec(specs []types.BaseVirtualDeviceConfigSpec) []int32 {
	var keys []int32
	for _, spec := range specs {
		configSpec := spec.GetVirtualDeviceConfigSpec()
		if configSpec.Operation != types.VirtualDeviceConfigSpecOperationAdd {
			continue
		}
		disk, ok := configSpec.Device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		keys = append(keys, disk.Key)
	}
	return keys
}

type diskDatastoreMapping struct {
	diskKey int32
	ref     types.ManagedObjectReference
}

func diskDatastoreMappingsFromRecommendation(recommendation types.ClusterRecommendation) []diskDatastoreMapping {
	var mappings []diskDatastoreMapping
	for _, action := range recommendation.Action {
		relocateAction, ok := action.(*types.StoragePlacementAction)
		if !ok {
			continue
		}
		for _, locator := range relocateAction.RelocateSpec.Disk {
			mappings = append(mappings, diskDatastoreMapping{
				diskKey: locator.DiskId,
				ref:     locator.Datastore,
			})
		}
	}
	return mappings
}

func destinationOnlyActions(recommendation types.ClusterRecommendation) []types.ManagedObjectReference {
	var destinations []types.ManagedObjectReference
	for _, action := range recommendation.Action {
		relocateAction, ok := action.(*types.StoragePlacementAction)
		if !ok || relocateAction.Destination.Type == "" {
			continue
		}
		if len(relocateAction.RelocateSpec.Disk) > 0 {
			continue
		}
		destinations = append(destinations, relocateAction.Destination)
	}
	return destinations
}

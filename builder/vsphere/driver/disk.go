// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"errors"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type Disk struct {
	DiskSize            int64
	DiskEagerlyScrub    bool
	DiskThinProvisioned bool
	ControllerIndex     int
	ControllerUnit      string
}

type StorageConfig struct {
	DiskControllerType []string
	Storage            []Disk
	DatastoreRefs      []*types.ManagedObjectReference
}

// AddStorageDevices adds virtual storage devices to an existing device list.
// Disks with ControllerUnit attach at explicit addresses; disks without use
// disk_controller_index against newly created controllers from DiskControllerType.
func (c *StorageConfig) AddStorageDevices(existingDevices object.VirtualDeviceList) ([]types.BaseVirtualDeviceConfigSpec, error) {
	return c.addStorageDevices(existingDevices, storageAttachOptions{linkControllerDevices: true, validateAggregate: true})
}

// AddStorageDevicesForPlacement builds device config specs without mutating shared
// template controller state. Used for Storage DRS placement requests only.
func (c *StorageConfig) AddStorageDevicesForPlacement(existingDevices object.VirtualDeviceList) ([]types.BaseVirtualDeviceConfigSpec, error) {
	devices := append(object.VirtualDeviceList{}, existingDevices...)
	return c.addStorageDevices(devices, storageAttachOptions{linkControllerDevices: false, validateAggregate: false})
}

type storageAttachOptions struct {
	linkControllerDevices bool
	validateAggregate     bool
}

func (c *StorageConfig) addStorageDevices(existingDevices object.VirtualDeviceList, opts storageAttachOptions) ([]types.BaseVirtualDeviceConfigSpec, error) {
	if len(c.Storage) == 0 {
		return nil, nil
	}

	newDevices := object.VirtualDeviceList{}
	typeIndex := 0

	var legacyDisks []int
	var explicitDisks []int
	for i, disk := range c.Storage {
		if disk.ControllerUnit != "" {
			explicitDisks = append(explicitDisks, i)
		} else {
			legacyDisks = append(legacyDisks, i)
		}
	}

	var legacyControllers []types.BaseVirtualController
	if len(legacyDisks) > 0 {
		for _, controllerType := range c.DiskControllerType {
			device, err := createController(existingDevices, controllerType)
			if err != nil {
				return nil, err
			}
			kind := controllerKindForType(controllerType)
			if err := validateControllerCount(existingDevices, kind, 1); err != nil {
				return nil, err
			}

			existingDevices = append(existingDevices, device)
			newDevices = append(newDevices, device)

			controller, err := existingDevices.FindDiskController(existingDevices.Name(device))
			if err != nil {
				return nil, err
			}
			legacyControllers = append(legacyControllers, controller)
		}

		for _, i := range legacyDisks {
			disk := c.buildDisk(existingDevices, c.Storage[i], i)
			if c.Storage[i].ControllerIndex >= len(legacyControllers) {
				return nil, fmt.Errorf("storage[%d].'disk_controller_index' references an unknown disk controller", i)
			}
			controller := legacyControllers[c.Storage[i].ControllerIndex]
			if err := validateControllerDiskCapacity(existingDevices, controller, 1); err != nil {
				return nil, fmt.Errorf("storage[%d]: %s", i, err)
			}
			existingDevices.AssignController(disk, controller)
			existingDevices = append(existingDevices, disk)
			newDevices = append(newDevices, disk)
		}

		typeIndex = len(c.DiskControllerType)
	}

	pendingUnits := map[string]struct{}{}
	for _, i := range explicitDisks {
		raw := c.Storage[i].ControllerUnit
		if err := ValidateControllerUnitRuntime(existingDevices, raw, pendingUnits); err != nil {
			return nil, fmt.Errorf("storage[%d]: %s", i, err)
		}

		addr, err := ParseControllerUnit(raw)
		if err != nil {
			return nil, fmt.Errorf("storage[%d]: %s", i, err)
		}

		controller := FindControllerByBus(existingDevices, addr.Kind, addr.Bus)
		if controller == nil {
			if typeIndex >= len(c.DiskControllerType) {
				available := ListControllerAddresses(existingDevices)
				if len(available) == 0 {
					return nil, fmt.Errorf("storage[%d]: %s", i, typePoolExhaustedError(raw))
				}
				return nil, fmt.Errorf("storage[%d]: %s", i, controllerNotFoundError(raw, available))
			}

			controllerType := c.DiskControllerType[typeIndex]
			typeIndex++

			device, err := createController(existingDevices, controllerType)
			if err != nil {
				return nil, fmt.Errorf("storage[%d]: %s", i, err)
			}
			kind := controllerKindForType(controllerType)
			if err := validateControllerCount(existingDevices, kind, 1); err != nil {
				return nil, fmt.Errorf("storage[%d]: %s", i, err)
			}

			existingDevices = append(existingDevices, device)
			newDevices = append(newDevices, device)

			controller, err = existingDevices.FindDiskController(existingDevices.Name(device))
			if err != nil {
				return nil, fmt.Errorf("storage[%d]: %s", i, err)
			}

			k, bus, ok := controllerBusNumber(controller)
			if !ok || k != addr.Kind || bus != addr.Bus {
				return nil, fmt.Errorf("storage[%d]: created controller bus does not match %q", i, raw)
			}
		}

		if err := validateControllerDiskCapacity(existingDevices, controller, 1); err != nil {
			return nil, fmt.Errorf("storage[%d]: %s", i, err)
		}

		disk := c.buildDisk(existingDevices, c.Storage[i], i)
		assignDiskAtUnit(existingDevices, disk, controller, int32(addr.Unit), opts.linkControllerDevices)
		existingDevices = append(existingDevices, disk)
		newDevices = append(newDevices, disk)
		pendingUnits[raw] = struct{}{}
	}

	if opts.validateAggregate {
		if err := validateAggregateStorageCapacity(existingDevices); err != nil {
			return nil, err
		}
	}

	return newDevices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
}

func (c *StorageConfig) buildDisk(devices object.VirtualDeviceList, dc Disk, index int) *types.VirtualDisk {
	backing := &types.VirtualDiskFlatVer2BackingInfo{
		DiskMode:        string(types.VirtualDiskModePersistent),
		ThinProvisioned: types.NewBool(dc.DiskThinProvisioned),
		EagerlyScrub:    types.NewBool(dc.DiskEagerlyScrub),
	}

	if index < len(c.DatastoreRefs) && c.DatastoreRefs[index] != nil {
		backing.Datastore = c.DatastoreRefs[index]
	}

	return &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key:     devices.NewKey(),
			Backing: backing,
		},
		CapacityInKB: dc.DiskSize * 1024,
	}
}

// findDisk scans a list of virtual devices and retrieves a single virtual disk.
// Returns an error if no disk or multiple disks are found.
// TODO: Add support for multiple disks.
func findDisk(devices object.VirtualDeviceList) (*types.VirtualDisk, error) {
	var disks []*types.VirtualDisk
	for _, device := range devices {
		switch d := device.(type) {
		case *types.VirtualDisk:
			disks = append(disks, d)
		}
	}

	switch len(disks) {
	case 0:
		return nil, errors.New("error finding virtual disk")
	case 1:
		return disks[0], nil
	}
	return nil, errors.New("more than one virtual disk found, only a single disk is allowed")
}

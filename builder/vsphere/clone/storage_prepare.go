// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"fmt"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// busKey identifies a controller bus (e.g. scsi0) referenced by an explicit
// disk_controller_unit address, independent of the unit portion.
type busKey struct {
	kind driver.ControllerKind
	bus  int
}

func (c *CloneConfig) prepareStorage() []error {
	var errs []error

	if len(c.StorageConfig.Storage) == 0 {
		return errs
	}

	if err := driver.ValidateDiskControllerTypes(c.StorageConfig.DiskControllerType); err != nil {
		errs = append(errs, err)
	}

	hasLegacy := false
	for _, storage := range c.StorageConfig.Storage {
		if storage.DiskControllerUnit == "" {
			hasLegacy = true
			break
		}
	}

	// nextTypeIndex tracks the next unconsumed disk_controller_type entry that
	// would create a new controller. Legacy storage blocks always create one
	// controller per disk_controller_type entry, so explicit-unit blocks start
	// consuming entries after that fixed offset. Multiple explicit-unit blocks
	// addressing the same bus share a single controller and therefore consume
	// only one disk_controller_type entry the first time that bus is seen,
	// mirroring the allocation order used by AddStorageDevices at clone time.
	//
	// This is still a best-effort estimate: whether a bus already exists on
	// the source (the `template`, or the VM deployed from a
	// `content_library_source` VM template) and thus needs no new controller
	// at all cannot be known until its devices are read at clone/deploy time,
	// so controller existence itself remains a runtime-only check (see
	// ValidateControllerUnitRuntime).
	nextTypeIndex := 0
	if hasLegacy {
		nextTypeIndex = len(c.StorageConfig.DiskControllerType)
	}
	busTypeIndex := map[busKey]int{}

	allExplicit := true
	seenUnits := map[string]int{}

	for i, storage := range c.StorageConfig.Storage {
		if storage.DiskControllerUnit != "" && storage.DiskControllerIndex != 0 {
			errs = append(errs, fmt.Errorf("storage[%d]: 'disk_controller_unit' and 'disk_controller_index' are mutually exclusive", i))
		}

		if storage.DiskControllerUnit != "" {
			typeIndex := 0
			if addr, err := driver.ParseControllerUnit(storage.DiskControllerUnit); err == nil {
				key := busKey{kind: addr.Kind, bus: addr.Bus}
				if idx, ok := busTypeIndex[key]; ok {
					typeIndex = idx
				} else {
					typeIndex = nextTypeIndex
					busTypeIndex[key] = typeIndex
					nextTypeIndex++
				}
			}

			if err := driver.ValidateControllerUnitStaticForConfig(
				storage.DiskControllerUnit,
				c.StorageConfig.DiskControllerType,
				typeIndex,
			); err != nil {
				errs = append(errs, fmt.Errorf("storage[%d]: %s", i, err))
			}
			if prior, ok := seenUnits[storage.DiskControllerUnit]; ok {
				errs = append(errs, fmt.Errorf("storage[%d]: unit %q is already assigned at storage[%d]. Each unit can only be used once", i, storage.DiskControllerUnit, prior))
			} else {
				seenUnits[storage.DiskControllerUnit] = i
			}
			continue
		}

		allExplicit = false
	}

	if !allExplicit && len(c.StorageConfig.DiskControllerType) == 0 {
		errs = append(errs, fmt.Errorf("'disk_controller_type' is required when storage blocks use 'disk_controller_index'"))
	}

	return errs
}

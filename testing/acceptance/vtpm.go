// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// CheckNoVTPM asserts the virtual machine has no virtual TPM devices.
func CheckNoVTPM(vm driver.VirtualMachine) error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}
	tpms := devices.SelectByType((*types.VirtualTPM)(nil))
	if len(tpms) != 0 {
		return fmt.Errorf("expected remove_vtpm to leave zero vTPM devices, got %d", len(tpms))
	}
	return nil
}

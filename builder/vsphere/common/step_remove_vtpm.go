// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type RemoveVTPMConfig

package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type RemoveVTPMConfig struct {
	// Remove the virtual trusted platform module (vTPM) device from the virtual
	// machine after shutdown. Defaults to `false`.
	//
	// -> **Note:** A virtual machine with a vTPM cannot be exported as OVF/OVA
	// (`export`) or imported to a content library as an OVF template
	// (`content_library_destination` with `ovf` set to `true`). Set this option
	// to `true` to remove the device after shutdown. A content library VM
	// template (`ovf` unset or `false`) can keep the vTPM.
	RemoveVTPM bool `mapstructure:"remove_vtpm"`
}

// Prepare returns warnings when a vTPM would block OVF/OVA export or a
// content library OVF template import.
func (c *RemoveVTPMConfig) Prepare(vtpmEnabled, exportOVF, contentLibraryOVF bool) []string {
	if !vtpmEnabled || c.RemoveVTPM {
		return nil
	}

	var ops []string
	if exportOVF {
		ops = append(ops, "OVF/OVA export is configured")
	}
	if contentLibraryOVF {
		ops = append(ops, "content library OVF template import is configured")
	}
	if len(ops) == 0 {
		return nil
	}

	return []string{fmt.Sprintf("vTPM is enabled and %s; this will fail unless 'remove_vtpm' is true", strings.Join(ops, " and "))}
}

type StepRemoveVTPM struct {
	Config *RemoveVTPMConfig
}

func (s *StepRemoveVTPM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	if !s.Config.RemoveVTPM {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(driver.VirtualMachine)

	ui.Say("Removing vTPM...")
	err := vm.RemoveVTPM()

	if err != nil {
		state.Put("error", fmt.Errorf("error removing vTPM: %v", err))
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepRemoveVTPM) Cleanup(state multistep.StateBag) {
	// no cleanup
}

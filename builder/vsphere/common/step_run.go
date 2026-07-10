// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type RunConfig

package common

import (
	"context"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type RunConfig struct {
	// The priority of boot devices.
	//
	// The available boot devices are: `floppy`, `cdrom`, `ethernet`, and
	// `disk`.
	//
	// -> **Note:** For `vsphere-iso`, if not set, the boot order is
	// temporarily set to `disk,cdrom` for the duration of the build and then
	// cleared upon build completion. For `vsphere-clone`, if not set, the
	// boot order is inherited from the source.
	BootOrder string `mapstructure:"boot_order"`
}

type StepRun struct {
	Config   *RunConfig
	SetOrder bool
}

func (s *StepRun) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(*driver.VirtualMachineDriver)

	if s.Config.BootOrder != "" {
		ui.Say("Setting boot order...")
		order := strings.Split(s.Config.BootOrder, ",")
		if err := vm.SetBootOrder(order); err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	} else {
		if s.SetOrder {
			ui.Say("Setting temporary boot order...")
			if err := vm.SetBootOrder([]string{"disk", "cdrom"}); err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
		}
	}

	ui.Say("Powering on virtual machine...")
	err := vm.PowerOn()
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepRun) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(*driver.VirtualMachineDriver)

	if s.Config.BootOrder == "" && s.SetOrder {
		ui.Say("Clearing boot order...")
		if err := vm.SetBootOrder([]string{"-"}); err != nil {
			state.Put("error", err)
			return
		}
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	ui.Say("Powering off virtual machine...")

	err := vm.PowerOff()
	if err != nil {
		ui.Errorf("%s", err)
	}
}

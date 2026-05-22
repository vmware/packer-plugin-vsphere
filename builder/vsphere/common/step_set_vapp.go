// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// StepSetVApp applies vApp property configuration to the virtual machine.
type StepSetVApp struct {
	Config *VAppConfig
}

func (s *StepSetVApp) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config == nil || !s.Config.Active() {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(driver.VirtualMachine)

	ui.Say("Configuring vApp properties...")
	if err := vm.SetVAppProperties(ctx, s.Config.Properties); err != nil {
		state.Put("error", fmt.Errorf("error configuring vApp properties: %s", err))
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepSetVApp) Cleanup(state multistep.StateBag) {}

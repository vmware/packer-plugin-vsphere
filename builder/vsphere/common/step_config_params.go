// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ConfigParamsConfig

package common

import (
	"context"
	"fmt"
	"log"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type ConfigParamsConfig struct {
	// A map of key-value pairs to send to the [`extraConfig`](https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.ConfigSpec.html#extraConfig).
	// in the vSphere API's `VirtualMachineConfigSpec`.
	//
	// HCL Example:
	//
	// ```hcl
	//   configuration_parameters = {
	//     "disk.EnableUUID" = "TRUE"
	//     "svga.autodetect" = "TRUE"
	//     "log.keepOld"     = "15"
	//   }
	// ```
	//
	// JSON Example:
	//
	// ```json
	//   "configuration_parameters": {
	//     "disk.EnableUUID": "TRUE",
	//     "svga.autodetect": "TRUE",
	//     "log.keepOld": "15"
	//   }
	// ```
	//
	// ~> **Note:** Configuration keys that would conflict with parameters that
	// are explicitly configurable through other fields in the `ConfigSpec` object
	// are silently ignored. Refer to the [`VirtualMachineConfigSpec`](https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.ConfigSpec.html)
	// in the vSphere API documentation.
	ConfigParams map[string]string `mapstructure:"configuration_parameters"`
	// Enable or disable time synchronization between the guest operating system and the
	// ESX host at startup and after VM operations that may introduce time drift (such
	// as resume from suspend, vMotion, or snapshot restore). If set to `true`, time
	// synchronization is explicitly enabled. If set to `false`, time synchronization is
	// explicitly disabled. If omitted, the builder does not modify the virtual
	// machine's time synchronization settings:
	//   - `vsphere-iso` builder uses the vSphere default for new virtual machines
	//      (`true`).
	//   - `vsphere-clone` builder inherits the setting from the source virtual machine.
	ToolsSyncTime *bool `mapstructure:"tools_sync_time"`
	// Enable or disable periodic time synchronization between the guest operating
	// system and the ESX host. Use this setting only if the guest operating system does
	// not have native time synchronization.
	//   - `vsphere-iso` builder uses the vSphere default for new virtual machines
	//      (`false`).
	//   - `vsphere-clone` builder inherits the setting from the source virtual machine.
	ToolsSyncTimePeriodically *bool `mapstructure:"tools_sync_time_periodically"`
	// Automatically check for and upgrade VMware Tools after a virtual machine
	// power cycle. Defaults to `false`.
	ToolsUpgradePolicy bool `mapstructure:"tools_upgrade_policy"`
}

type StepConfigParams struct {
	Config *ConfigParamsConfig
}

func (c *ConfigParamsConfig) Prepare() []error {
	var errs []error

	if c.ToolsSyncTimePeriodically != nil && *c.ToolsSyncTimePeriodically {
		if c.ToolsSyncTime == nil || !*c.ToolsSyncTime {
			errs = append(errs, fmt.Errorf("'tools_sync_time_periodically' requires 'tools_sync_time' to be set to 'true'"))
		}
	}

	return errs
}

func (s *StepConfigParams) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(*driver.VirtualMachineDriver)
	configParams := make(map[string]string)

	if s.Config.ConfigParams != nil {
		configParams = s.Config.ConfigParams
	}

	var info *types.ToolsConfigInfo

	if s.Config.ToolsSyncTime != nil || s.Config.ToolsSyncTimePeriodically != nil || s.Config.ToolsUpgradePolicy {
		info = &types.ToolsConfigInfo{}

		// Gate: Whether time synchronization is allowed.
		if s.Config.ToolsSyncTime != nil {
			info.SyncTimeWithHostAllowed = s.Config.ToolsSyncTime
		}

		// Optional: Whether periodic time synchronization is allowed.
		if s.Config.ToolsSyncTimePeriodically != nil {
			info.SyncTimeWithHost = s.Config.ToolsSyncTimePeriodically
		}

		if s.Config.ToolsUpgradePolicy {
			info.ToolsUpgradePolicy = "UpgradeAtPowerCycle"
		}
	}

	ui.Say("Adding configuration parameters...")

	// Iterate over the map and log each key-value pair.
	for key, value := range configParams {
		log.Printf("[INFO] Adding: %s = %v", key, value)
	}

	if err := vm.AddConfigParams(configParams, info); err != nil {
		state.Put("error", fmt.Errorf("error adding configuration parameters: %v", err))
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepConfigParams) Cleanup(state multistep.StateBag) {}

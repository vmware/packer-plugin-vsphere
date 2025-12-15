// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// StepApplyTags applies tags to a virtual machine or template.
type StepApplyTags struct {
	TagsConfig *TagsConfig
	Ctx        interpolate.Context
}

func (s *StepApplyTags) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	// Skip if no tags configured
	if s.TagsConfig == nil || (len(s.TagsConfig.Tags) == 0 && len(s.TagsConfig.Tag) == 0) {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(driver.VirtualMachine)
	d := state.Get("driver").(driver.Driver)

	ui.Say("Applying tags to virtual machine...")

	// Get REST client from driver
	restClient := d.GetRestClient()

	// Create tag manager with the interpolation context
	tagManager := NewTagManager(restClient, s.TagsConfig, s.Ctx)

	// Validate configuration
	if err := tagManager.ValidateConfig(); err != nil {
		state.Put("error", fmt.Errorf("tag configuration validation failed: %w", err))
		return multistep.ActionHalt
	}

	// Apply tags to VM
	vmRef := vm.Reference()
	if err := tagManager.ApplyTags(ctx, vmRef); err != nil {
		state.Put("error", fmt.Errorf("failed to apply tags: %w", err))
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepApplyTags) Cleanup(state multistep.StateBag) {
	// Tags are persistent metadata, no cleanup needed
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
)

// StepApplyTags applies tags to a template in the post-processor context.
type StepApplyTags struct {
	TagsConfig *common.TagsConfig
	Ctx        interpolate.Context
}

func (s *StepApplyTags) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	// Skip if no tags configured
	if s.TagsConfig == nil || (len(s.TagsConfig.Tags) == 0 && len(s.TagsConfig.Tag) == 0) {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*govmomi.Client)
	vm := state.Get("vm").(*object.VirtualMachine)

	ui.Say("Applying tags to template...")

	// Create REST client from govmomi client
	restClient := rest.NewClient(client.Client)
	credentials := client.URL().User
	if rawCredentials, ok := state.GetOk("rest_credentials"); ok {
		if c, ok := rawCredentials.(*url.Userinfo); ok {
			credentials = c
		}
	}

	if err := restClient.Login(ctx, credentials); err != nil {
		state.Put("error", fmt.Errorf("failed to login to REST API: %w", err))
		return multistep.ActionHalt
	}

	// Create tag manager with the interpolation context
	tagManager := common.NewTagManager(restClient, s.TagsConfig, s.Ctx)

	// Validate configuration
	if err := tagManager.ValidateConfig(); err != nil {
		state.Put("error", fmt.Errorf("tag configuration validation failed: %w", err))
		return multistep.ActionHalt
	}

	// Apply tags to VM/template
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

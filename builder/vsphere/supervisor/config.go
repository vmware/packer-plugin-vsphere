// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package supervisor

import (
	packercommon "github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
)

const (
	DefaultCommUsername = "packer"
)

type Config struct {
	packercommon.PackerConfig `mapstructure:",squash"`
	CommunicatorConfig        communicator.Config `mapstructure:",squash"`
	ValidatePublishConfig     `mapstructure:",squash"`
	ConnectSupervisorConfig   `mapstructure:",squash"`
	ImportImageConfig         `mapstructure:",squash"`
	CreateSourceConfig        `mapstructure:",squash"`
	WatchSourceConfig         `mapstructure:",squash"`
	PublishSourceConfig       `mapstructure:",squash"`
	common.TagsConfig         `mapstructure:",squash"`

	ctx interpolate.Context
}

func (c *Config) Prepare(raws ...any) ([]string, error) {
	err := config.Decode(c, &config.DecodeOpts{
		PluginType:         common.BuilderId,
		Interpolate:        true,
		InterpolateContext: &c.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"boot_command",
			},
		},
	}, raws...)

	if err != nil {
		return nil, err
	}

	// Set a default username as it's required for both SSH and WinRM communicators.
	// This must call before the CommunicatorConfig.Prepare to avoid an error.
	commType := c.CommunicatorConfig.Type
	if (commType == "" || commType == "ssh") && c.CommunicatorConfig.SSHUsername == "" {
		c.CommunicatorConfig.SSHUsername = DefaultCommUsername
	}
	if commType == "winrm" && c.CommunicatorConfig.WinRMUser == "" {
		c.CommunicatorConfig.WinRMUser = DefaultCommUsername
	}

	errs := new(packersdk.MultiError)
	errs = packersdk.MultiErrorAppend(errs, c.CommunicatorConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.ConnectSupervisorConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.ValidatePublishConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.ImportImageConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.CreateSourceConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.WatchSourceConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.PublishSourceConfig.Prepare()...)

	if len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}

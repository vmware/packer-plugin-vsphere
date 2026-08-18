// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,OvfSourceConfig,ContentLibrarySourceConfig

package clone

import (
	packerCommon "github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
)

type Config struct {
	packerCommon.PackerConfig `mapstructure:",squash"`
	commonsteps.HTTPConfig    `mapstructure:",squash"`
	commonsteps.CDConfig      `mapstructure:",squash"`

	common.ConnectConfig              `mapstructure:",squash"`
	CloneConfig                       `mapstructure:",squash"`
	common.LocationConfig             `mapstructure:",squash"`
	common.HardwareConfig             `mapstructure:",squash"`
	common.ConfigParamsConfig         `mapstructure:",squash"`
	common.FlagConfig                 `mapstructure:",squash"`
	common.CDRomConfig                `mapstructure:",squash"`
	common.RemoveCDRomConfig          `mapstructure:",squash"`
	common.ReattachCDRomConfig        `mapstructure:",squash"`
	common.RemoveNetworkAdapterConfig `mapstructure:",squash"`
	common.RemoveVTPMConfig           `mapstructure:",squash"`
	common.FloppyConfig               `mapstructure:",squash"`
	common.RunConfig                  `mapstructure:",squash"`
	common.BootConfig                 `mapstructure:",squash"`
	common.WaitIpConfig               `mapstructure:",squash"`
	Comm                              communicator.Config `mapstructure:",squash"`
	common.ShutdownConfig             `mapstructure:",squash"`
	common.TagsConfig                 `mapstructure:",squash"`

	// Create a snapshot of the virtual machine to use as a base for linked
	// clones. Defaults to `false`.
	CreateSnapshot bool `mapstructure:"create_snapshot"`
	// The name of the snapshot when `create_snapshot` is `true`.
	// Defaults to `Created By Packer`.
	SnapshotName string `mapstructure:"snapshot_name"`
	// Convert the cloned virtual machine to a template after the build is
	// complete. Defaults to `false`.
	// If set to `true`, the virtual machine can not be imported to a content
	// library.
	ConvertToTemplate bool `mapstructure:"convert_to_template"`
	// The configuration for exporting the virtual machine to an OVF.
	// The virtual machine is not exported if [export configuration](#export-configuration)
	// is not specified.
	Export *common.ExportConfig `mapstructure:"export"`
	// The configuration for importing a VM template or OVF template to a
	// content library. The template will not be imported if no
	// [content library destination configuration](#content-library-destination-configuration)
	// is specified. If set, `convert_to_template` must be set to `false`.
	ContentLibraryDestinationConfig *common.ContentLibraryDestinationConfig `mapstructure:"content_library_destination"`
	// The customization options for the virtual machine.
	// Refer to the [customization options](#customization) section for more
	// information.
	CustomizeConfig *CustomizeConfig `mapstructure:"customize"`

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

	warnings := make([]string, 0)
	errs := new(packersdk.MultiError)

	errs = packersdk.MultiErrorAppend(errs, c.ConnectConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.CloneConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.LocationConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.HardwareConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.ConfigParamsConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.FlagConfig.Prepare(&c.HardwareConfig)...)
	errs = packersdk.MultiErrorAppend(errs, c.HTTPConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CDRomConfig.Prepare(&c.ReattachCDRomConfig)...)
	errs = packersdk.MultiErrorAppend(errs, c.CDConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.BootConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.WaitIpConfig.Prepare()...)
	errs = packersdk.MultiErrorAppend(errs, c.Comm.Prepare(&c.ctx)...)

	disableWarnings, disableErrs := c.ValidateDisableIpWait(c.Comm.Type, c.Comm.Host())
	warnings = append(warnings, disableWarnings...)
	errs = packersdk.MultiErrorAppend(errs, disableErrs...)

	_, shutdownErrs := c.ShutdownConfig.Prepare(c.Comm)
	// shutdownWarnings, shutdownErrs := c.ShutdownConfig.Prepare(c.Comm)
	// warnings = append(warnings, shutdownWarnings...)
	errs = packersdk.MultiErrorAppend(errs, shutdownErrs...)

	if c.Export != nil {
		errs = packersdk.MultiErrorAppend(errs, c.Export.Prepare(&c.ctx, &c.LocationConfig, &c.PackerConfig)...)
	}
	if c.ContentLibraryDestinationConfig != nil {
		errs = packersdk.MultiErrorAppend(errs, c.ContentLibraryDestinationConfig.Prepare(&c.LocationConfig)...)
	}
	if c.CustomizeConfig != nil {
		customizeWarnings, customizeErrors := c.CustomizeConfig.Prepare()
		errs = packersdk.MultiErrorAppend(errs, customizeErrors...)
		warnings = append(warnings, customizeWarnings...)
	}

	exportOVF := c.Export != nil
	contentLibraryOVF := c.ContentLibraryDestinationConfig != nil && c.ContentLibraryDestinationConfig.Ovf
	warnings = append(warnings, c.RemoveVTPMConfig.Prepare(c.VTPMEnabled, exportOVF, contentLibraryOVF)...)

	if len(errs.Errors) > 0 {
		return nil, errs
	}

	if len(warnings) > 0 {
		return warnings, nil
	}

	return nil, nil
}

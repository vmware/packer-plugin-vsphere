// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package vsphere_template

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/govmomi"
	vsphere "github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	vspherepost "github.com/vmware/packer-plugin-vsphere/post-processor/vsphere"
)

const (
	BuilderIdESX               = "mitchellh.vmware-esx"
	ArtifactConfFormat         = "artifact.conf.format"
	ArtifactConfKeepRegistered = "artifact.conf.keep_registered"
	ArtifactConfSkipExport     = "artifact.conf.skip_export"
)

var builtins = map[string]string{
	vspherepost.BuilderId:            "vmware",
	BuilderIdESX:                     "vmware",
	vsphere.BuilderId:                "vsphere",
	"packer.post-processor.artifice": "artifice",
}

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	// The fully qualified domain name or IP address of the vSphere endpoint.
	Host string `mapstructure:"host" required:"true"`
	// The username to use to authenticate to the vSphere endpoint.
	Username string `mapstructure:"username" required:"true"`
	// The password to use to authenticate to the vSphere endpoint.
	Password string `mapstructure:"password" required:"true"`
	// Skip the verification of the server certificate. Defaults to `false`.
	Insecure bool `mapstructure:"insecure"`
	// The name of the datacenter to use.
	// Required when the vCenter instance has more than one datacenter.
	Datacenter string `mapstructure:"datacenter"`
	// The name of the template.
	// If not specified, the name of the virtual machine will be used.
	TemplateName string `mapstructure:"template_name"`
	// The name of the virtual machine folder path where the template will be created.
	Folder string `mapstructure:"folder"`
	// Create a snapshot before marking as a template. Defaults to `false`.
	SnapshotEnable bool `mapstructure:"snapshot_enable"`
	// The name of the snapshot. Required when `snapshot_enable` is `true`.
	SnapshotName string `mapstructure:"snapshot_name"`
	// A description for the snapshot. Required when `snapshot_enable` is `true`.
	SnapshotDescription string `mapstructure:"snapshot_description"`
	// Keep the virtual machine registered after marking as a template.
	ReregisterVM config.Trilean `mapstructure:"reregister_vm"`
	// Overwrite existing template. Defaults to `false`.
	Override bool `mapstructure:"override"`
	// Tags configuration for the template.
	vsphere.TagsConfig `mapstructure:",squash"`

	ctx interpolate.Context
}

type PostProcessor struct {
	config Config
	url    *url.URL
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec {
	return p.config.FlatMapstructure().HCL2Spec()
}

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         vsphere.BuilderId,
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)

	if err != nil {
		return err
	}

	errs := new(packersdk.MultiError)
	vc := map[string]*string{
		"host":     &p.config.Host,
		"username": &p.config.Username,
		"password": &p.config.Password,
	}

	for key, ptr := range vc {
		if *ptr == "" {
			errs = packersdk.MultiErrorAppend(
				errs, fmt.Errorf("error: %s must be set", key))
		}
	}

	sdk, err := url.Parse(fmt.Sprintf("https://%v/sdk", p.config.Host))
	if err != nil {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("error using endpoint: %s", err))
		return errs
	}

	sdk.User = url.UserPassword(p.config.Username, p.config.Password)
	p.url = sdk

	// Validate tags configuration
	if len(p.config.Tags) > 0 || len(p.config.Tag) > 0 {
		tagManager := vsphere.NewTagManager(nil, &p.config.TagsConfig, p.config.ctx)
		if err := tagManager.ValidateConfig(); err != nil {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("tag configuration validation failed: %s", err))
		}
	}

	if len(errs.Errors) > 0 {
		return errs
	}
	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, artifact packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	// Check if the artifact is supported by the post-processor.
	if _, ok := builtins[artifact.BuilderId()]; !ok {
		return nil, false, false, fmt.Errorf(
			"error: unsupported artifact type %s. supported types: vsphere-iso exported OVF or vSphere post-processor", artifact.BuilderId())
	}

	f := artifact.State(ArtifactConfFormat)
	k := artifact.State(ArtifactConfKeepRegistered)
	s := artifact.State(ArtifactConfSkipExport)

	// Validate artifact configuration for export
	if f != "" && k != "true" && s == "false" {
		return nil, false, false, fmt.Errorf("error: `keep_registered` must be set to `true` for export")
	}

	// If the virtual machine is still powered on and immediately marked as a template it will fail.
	// Pause for a few seconds to allow the virtual machine to prepare for the next step.

	ui.Say("Pausing momentarily to prepare for the next step...")
	time.Sleep(10 * time.Second)
	c, err := govmomi.NewClient(context.Background(), p.url, p.config.Insecure)
	if err != nil {
		return nil, false, false, fmt.Errorf("error connecting to vsphere endpoint: %s", err)
	}

	defer p.Logout(c)

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("client", c)
	state.Put("rest_credentials", url.UserPassword(p.config.Username, p.config.Password))

	steps := []multistep.Step{
		&stepChooseDatacenter{
			Datacenter: p.config.Datacenter,
		},
		&stepCreateFolder{
			Folder: p.config.Folder,
		},
		NewStepCreateSnapshot(artifact, p),
		NewStepMarkAsTemplate(artifact, p),
		&StepApplyTags{
			TagsConfig: &p.config.TagsConfig,
			Ctx:        p.config.ctx,
		},
	}
	runner := commonsteps.NewRunnerWithPauseFn(steps, p.config.PackerConfig, ui, state)
	runner.Run(ctx, state)
	if rawErr, ok := state.GetOk("error"); ok {
		return nil, false, false, rawErr.(error)
	}
	return artifact, true, true, nil
}

func (p *PostProcessor) Logout(c *govmomi.Client) {
	_ = c.Logout(context.Background())
}

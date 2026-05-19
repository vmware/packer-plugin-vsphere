// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor

import (
	"context"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
)

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	warnings, errs := b.config.Prepare(raws...)
	if errs != nil {
		return nil, warnings, errs
	}

	return nil, warnings, nil
}

func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	state := new(multistep.BasicStateBag)
	state.Put("debug", b.config.PackerDebug)
	state.Put("hook", hook)
	state.Put("ui", ui)
	logger := &PackerLogger{UI: ui}
	state.Put("logger", logger)

	var steps []multistep.Step
	if b.config.CommunicatorConfig.Type == "ssh" {
		// Generate SSH key pairs for connecting to the source VM.
		// Adding this first as it's required for creating a source VM with default bootstrap data.
		steps = append(steps, &communicator.StepSSHKeyGen{
			CommConf: &b.config.CommunicatorConfig,
		})
	}

	steps = append(steps,
		// Connect to the Supervisor cluster where the source VM created.
		&StepConnectSupervisor{
			Config: &b.config.ConnectSupervisorConfig,
		},
		// Validate if VM publish feature is enabled and the required config is valid.
		&StepValidatePublish{
			Config: &b.config.ValidatePublishConfig,
		},
	)

	// conditionally add steps to validate import spec and import images from source URL as VM image.
	if b.config.ImportSourceURL != "" {
		steps = append(steps,
			&StepImportImage{
				ImportImageConfig: &b.config.ImportImageConfig,
			},
		)
	}

	steps = append(steps,
		// Create a source VM and other related resources in Supervisor cluster.
		&StepCreateSource{
			Config:             &b.config.CreateSourceConfig,
			CommunicatorConfig: &b.config.CommunicatorConfig,
		},
		// Watch for the source VM to be powered on and accessible.
		&StepWatchSource{
			Config: &b.config.WatchSourceConfig,
		},
	)

	if b.config.CommunicatorConfig.Type != "none" {
		// Connect to the source VM via specified communicator.
		steps = append(steps, b.getCommunicatorStepConnect())
		// Run provisioners defined in the Packer template.
		steps = append(steps, new(commonsteps.StepProvision))
	}

	// Apply tags to the virtual machine.
	steps = append(steps, &common.StepApplyTags{
		TagsConfig: &b.config.TagsConfig,
		Ctx:        b.config.ctx,
	})

	// Publish the provisioned source VM to a vSphere content library (if specified).
	steps = append(steps, &StepPublishSource{
		Config: &b.config.PublishSourceConfig,
	})

	b.runner = commonsteps.NewRunnerWithPauseFn(steps, b.config.PackerConfig, ui, state)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	logger.Info("Build 'vsphere-supervisor' finished successfully.")
	return nil, nil
}

func (b *Builder) getCommunicatorStepConnect() *communicator.StepConnect {
	stepConnect := &communicator.StepConnect{
		Config: &b.config.CommunicatorConfig,
		Host:   common.CommHost(b.config.CommunicatorConfig.Host()),
	}

	if b.config.CommunicatorConfig.Type == "ssh" {
		stepConnect.SSHConfig = b.config.CommunicatorConfig.SSHConfigFunc()
		return stepConnect
	}

	// Communicator type is WinRM.
	stepConnect.WinRMConfig = func(multistep.StateBag) (*communicator.WinRMConfig, error) {
		return &communicator.WinRMConfig{
			Username: b.config.CommunicatorConfig.WinRMUser,
			Password: b.config.CommunicatorConfig.WinRMPassword,
		}, nil
	}
	return stepConnect
}

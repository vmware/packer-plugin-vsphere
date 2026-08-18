// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package iso

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type Builder struct {
	config Config
	runner multistep.Runner
}

// ConfigSpec returns an HCL2 object specification based on the Builder's configuration mapping.
func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

// Prepare processes the given raw inputs, validates the configuration, and returns warnings or errors if any occur.
func (b *Builder) Prepare(raws ...any) ([]string, []string, error) {
	warnings, errs := b.config.Prepare(raws...)
	if errs != nil {
		return nil, warnings, errs
	}

	return nil, warnings, nil
}

// Run executes the build process steps for the `Builder`, leveraging the provided context, UI, and lifecycle hook.
// It initializes state, configures steps sequentially, and manages interactions with the virtual machine driver.
// Returns a finalized artifact or an error if the build process fails.
func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	state := new(multistep.BasicStateBag)
	state.Put("debug", b.config.PackerDebug)
	state.Put("hook", hook)
	state.Put("ui", ui)

	var steps []multistep.Step

	steps = append(steps,
		&common.StepConnect{
			Config: &b.config.ConnectConfig,
		},
		&common.StepResolveDatastore{
			Datastore:        b.config.Datastore,
			DatastoreCluster: b.config.DatastoreCluster,
			StoragePolicy:    b.config.StorageConfig.FirstStoragePolicyName(),
			Host:             b.config.Host,
			Cluster:          b.config.Cluster,
			DiskCount:        len(b.config.StorageConfig.Storage),
		},
		&common.StepDownload{
			DownloadStep: &commonsteps.StepDownload{
				Checksum:    b.config.ISOChecksum,
				Description: "ISO",
				Extension:   b.config.TargetExtension,
				ResultKey:   "iso_path",
				TargetPath:  b.config.TargetPath,
				Url:         b.config.ISOUrls,
			},
			Url:                  b.config.ISOUrls,
			ResultKey:            "iso_path",
			Datastore:            b.config.Datastore,
			Host:                 b.config.Host,
			LocalCacheOverwrite:  b.config.LocalCacheOverwrite,
			RemoteCacheOverwrite: b.config.RemoteCacheOverwrite || b.config.LocalCacheOverwrite,
			RemoteCacheDatastore: b.config.RemoteCacheDatastore,
			RemoteCachePath:      b.config.RemoteCachePath,
		},
		&commonsteps.StepCreateCD{
			Files:   b.config.CDFiles,
			Content: b.config.CDContent,
			Label:   b.config.CDLabel,
		},
		&common.StepRemoteUpload{
			Datastore:                  b.config.Datastore,
			Host:                       b.config.Host,
			SetHostForDatastoreUploads: b.config.SetHostForDatastoreUploads,
			RemoteCacheCleanup:         b.config.RemoteCacheCleanup,
			RemoteCacheOverwrite:       b.config.RemoteCacheOverwrite,
			RemoteCacheDatastore:       b.config.RemoteCacheDatastore,
			RemoteCachePath:            b.config.RemoteCachePath,
		},
		&StepCreateVM{
			Config:   &b.config.CreateConfig,
			Location: &b.config.LocationConfig,
			Force:    b.config.PackerForce,
			Firmware: b.config.Firmware,
		},
		&common.StepConfigureHardware{
			Config: &b.config.HardwareConfig,
		},
		&common.StepAddFlag{
			FlagConfig: b.config.FlagConfig,
		},
		&common.StepAddCDRom{
			Config: &b.config.CDRomConfig,
		},
		&common.StepConfigParams{
			Config: &b.config.ConfigParamsConfig,
		},
		&commonsteps.StepCreateFloppy{
			Files:       b.config.FloppyFiles,
			Directories: b.config.FloppyDirectories,
			Content:     b.config.FloppyContent,
			Label:       b.config.FloppyLabel,
		},
		&common.StepAddFloppy{
			Config:                     &b.config.FloppyConfig,
			Datastore:                  b.config.Datastore,
			Host:                       b.config.Host,
			SetHostForDatastoreUploads: b.config.SetHostForDatastoreUploads,
		},
		&common.StepApplyTags{
			TagsConfig: &b.config.TagsConfig,
			Ctx:        b.config.ctx,
		},
		&common.StepSetVApp{
			Config: &b.config.VAppConfig,
		},
	)

	// Set the address for the HTTP server based on the configuration
	// provided by the user.
	if addrs := b.config.HTTPAddress; addrs != "" && addrs != common.DefaultHttpBindAddress {
		// Validate and use the specified HTTPAddress.
		err := common.ValidateHTTPAddress(addrs)
		if err != nil {
			ui.Errorf("error validating IP address for HTTP server: %s", err)
			return nil, err
		}
		state.Put("http_bind_address", addrs)
	} else if intf := b.config.HTTPInterface; intf != "" {
		// Use the specified HTTPInterface.
		state.Put("http_interface", intf)
	} else {
		// Use IP discovery if neither HTTPAddress nor HTTPInterface
		// is specified.
		steps = append(steps, &common.StepHTTPIPDiscover{
			HTTPIP:  b.config.HTTPIP,
			Network: b.config.GetIPNet(),
		})
	}

	steps = append(steps,
		commonsteps.HTTPServerFromHTTPConfig(&b.config.HTTPConfig),
	)

	if b.config.Comm.Type == "ssh" && b.config.VAppConfig.NeedsEphemeralSSHKey(b.config.Comm) {
		steps = append(steps,
			&common.StepSshKeyPair{
				Debug:        b.config.PackerDebug,
				DebugKeyPath: fmt.Sprintf("%s.pem", b.config.PackerBuildName),
				Comm:         &b.config.Comm,
			},
		)
	}

	steps = append(steps,
		&common.StepRun{
			Config:   &b.config.RunConfig,
			SetOrder: true,
		},
		&common.StepBootCommand{
			Config: &b.config.BootConfig,
			Ctx:    b.config.ctx,
			VMName: b.config.VMName,
		},
	)

	if b.config.Comm.Type != "none" {
		if !b.config.DisableIpWait {
			steps = append(steps, &common.StepWaitForIp{
				Config: &b.config.WaitIpConfig,
			})
		}
		steps = append(steps,
			&communicator.StepConnect{
				Config:    &b.config.Comm,
				Host:      common.CommHost(b.config.Comm.Host()),
				SSHConfig: b.config.Comm.SSHConfigFunc(),
			},
			&commonsteps.StepProvision{},
		)
	}

	steps = append(steps,
		&common.StepShutdown{
			Config: &b.config.ShutdownConfig,
		},
		&common.StepRemoveFloppy{
			Datastore: b.config.Datastore,
			Host:      b.config.Host,
		},
		&common.StepRemoveCDRom{
			Config: &b.config.RemoveCDRomConfig,
		},
		&common.StepReattachCDRom{
			Config:      &b.config.ReattachCDRomConfig,
			CDRomConfig: &b.config.CDRomConfig,
		},
		&common.StepRemoveNetworkAdapter{
			Config: &b.config.RemoveNetworkAdapterConfig,
		},
		&common.StepRemoveVTPM{
			Config: &b.config.RemoveVTPMConfig,
		},
		&common.StepCreateSnapshot{
			CreateSnapshot: b.config.CreateSnapshot,
			SnapshotName:   b.config.SnapshotName,
		},
		&common.StepConvertToTemplate{
			ConvertToTemplate: b.config.ConvertToTemplate,
		},
	)

	if b.config.ContentLibraryDestinationConfig != nil {
		steps = append(steps, &common.StepImportToContentLibrary{
			ContentLibConfig: b.config.ContentLibraryDestinationConfig,
		})
	}

	if b.config.Export != nil {
		steps = append(steps, &common.StepExport{
			Name:       b.config.Export.Name,
			Force:      b.config.Export.Force,
			ImageFiles: b.config.Export.ImageFiles,
			Manifest:   b.config.Export.Manifest,
			OutputDir:  b.config.Export.OutputDir.OutputDir,
			Options:    b.config.Export.Options,
			Format:     b.config.Export.Format,
		})
	}

	b.runner = commonsteps.NewRunnerWithPauseFn(steps, b.config.PackerConfig, ui, state)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if _, ok := state.GetOk("vm"); !ok {
		return nil, nil
	}

	vm := state.Get("vm").(*driver.VirtualMachineDriver)
	artifact := &common.Artifact{
		Name:                 b.config.VMName,
		Datacenter:           vm.Datacenter(),
		Location:             b.config.LocationConfig,
		ContentLibraryConfig: b.config.ContentLibraryDestinationConfig,
		VM:                   vm,
		StateData: map[string]any{
			"generated_data": state.Get("generated_data"),
			"metadata":       state.Get("metadata"),
			"SourceImageURL": state.Get("SourceImageURL"),
			"iso_path":       state.Get("iso_path"),
		},
	}

	if b.config.Export != nil {
		artifact.Outconfig = &b.config.Export.OutputDir
	}
	return artifact, nil
}

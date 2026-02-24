// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type FloppyConfig

package common

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type FloppyConfig struct {
	// Datastore path to a floppy image that will be mounted to the virtual
	// machine. Example: `[datastore] iso/foo.flp`.
	FloppyIMGPath string `mapstructure:"floppy_img_path"`
	// A list of local files to be mounted to the virtual machine's floppy
	// drive.
	FloppyFiles []string `mapstructure:"floppy_files"`
	// A list of directories to copy files from.
	FloppyDirectories []string `mapstructure:"floppy_dirs"`
	// Key/Values to add to the floppy disk. The keys represent the paths, and
	// the value contents. It can be used alongside `floppy_files` or
	// `floppy_dirs`, which is useful to add large files without loading them
	// into memory. If both specify a path, the contents in `floppy_content` will take
	// precedence.
	//
	// HCL Example:
	//
	// ```hcl
	// floppy_content = {
	//   "meta-data" = jsonencode(local.instance_data)
	//   "user-data" = templatefile("user-data", { packages = ["nginx"] })
	// }
	// ```
	FloppyContent map[string]string `mapstructure:"floppy_content"`
	// The label to use for the floppy disk that is attached when the virtual
	// machine is booted. This is most useful for cloud-init, Kickstart, or other
	// early initialization tools, which can benefit from labeled floppy disks.
	// By default, the floppy label will be 'packer'.
	FloppyLabel string `mapstructure:"floppy_label"`
}

type StepAddFloppy struct {
	Config                     *FloppyConfig
	Datastore                  string
	Host                       string
	SetHostForDatastoreUploads bool
}

func (s *StepAddFloppy) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(driver.VirtualMachine)
	d := state.Get("driver").(driver.Driver)

	if floppyPath, ok := state.GetOk("floppy_path"); ok {
		ui.Say("Uploading floppy image...")

		var ds driver.Datastore
		var err error

		// If a datastore was resolved (from datastore or datastore_cluster), use it.
		if resolvedDs, ok := state.GetOk("datastore"); ok {
			ds = resolvedDs.(driver.Datastore)
		} else {
			ds, err = d.FindDatastore(s.Datastore, s.Host)
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
		}
		vmDir, err := vm.GetDir()
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}

		// Create a new random number generator
		src := rand.NewSource(time.Now().UnixNano())
		r := rand.New(src)

		// Generate a unique ID for the floppy image using the packer-##########.flp.
		// This helps avoid conflicts with other floppy images that might be uploaded.
		// This naming pattern matches the one used by packer-sdk for generated ISOs.
		uniqueID := r.Int63n(9000000000) + 1000000000
		uploadPath := fmt.Sprintf("%v/packer-%d.flp", vmDir, uniqueID)
		if err := ds.UploadFile(floppyPath.(string), uploadPath, s.Host, s.SetHostForDatastoreUploads); err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		state.Put("uploaded_floppy_path", uploadPath)

		ui.Say("Adding generated floppy image...")
		floppyIMGPath := ds.ResolvePath(uploadPath)
		err = vm.AddFloppy(floppyIMGPath)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	if s.Config.FloppyIMGPath != "" {
		ui.Say("Adding floppy image...")
		err := vm.AddFloppy(s.Config.FloppyIMGPath)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	}

	return multistep.ActionContinue
}

func (s *StepAddFloppy) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)

	if UploadedFloppyPath, ok := state.GetOk("uploaded_floppy_path"); ok {
		ui.Say("Deleting floppy image...")

		var ds driver.Datastore
		var err error

		// If a datastore was resolved (from datastore or datastore_cluster), use it.
		if resolvedDs, ok := state.GetOk("datastore"); ok {
			ds = resolvedDs.(driver.Datastore)
		} else {
			ds, err = d.FindDatastore(s.Datastore, s.Host)
			if err != nil {
				state.Put("error", err)
				return
			}
		}

		err = ds.Delete(UploadedFloppyPath.(string))
		if err != nil {
			state.Put("error", err)
			return
		}

	}
}

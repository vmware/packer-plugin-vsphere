// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"context"
	"fmt"
	"path"
	"slices"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
)

type stepCreateFolder struct {
	Folder string
}

func (s *stepCreateFolder) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	cli := state.Get("client").(*govmomi.Client)
	dcPath := state.Get("dcPath").(string)

	ui.Say("Creating or checking destination folder...")

	base := path.Join(dcPath, "vm")
	fullPath := path.Join(base, s.Folder)
	si := object.NewSearchIndex(cli.Client)

	var folders []string
	var err error
	var ref object.Reference

	// Iterate over the path, saving non-existent folders.
	// Stop when an existing path is found; error if no existing path is found.
	for {
		ref, err = si.FindByInventoryPath(context.Background(), fullPath)
		if err != nil {
			state.Put("error", err)
			ui.Errorf("%s", err)
			return multistep.ActionHalt
		}

		if ref == nil {
			dir, folder := path.Split(fullPath)
			fullPath = path.Clean(dir)

			if fullPath == dcPath {
				err = fmt.Errorf("error finding base path %s", base)
				state.Put("error", err)
				ui.Errorf("%s", err)
				return multistep.ActionHalt
			}

			folders = append(folders, folder)
		} else {
			break
		}
	}

	if root, ok := ref.(*object.Folder); ok {
		for _, folder := range slices.Backward(folders) {
			ui.Sayf("Creating virtual machine folder %s...", folder)
			root, err = root.CreateFolder(context.Background(), folder)
			if err != nil {
				state.Put("error", err)
				ui.Errorf("%s", err)
				return multistep.ActionHalt
			}

			fullPath = path.Join(fullPath, folder)
		}
		root.SetInventoryPath(fullPath)
		state.Put("folder", root)
	} else {
		err = fmt.Errorf("error finding virtual machine folder at path %v", ref)
		state.Put("error", err)
		ui.Errorf("%s", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *stepCreateFolder) Cleanup(multistep.StateBag) {}

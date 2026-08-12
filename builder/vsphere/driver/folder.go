// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type Folder struct {
	driver *VCenterDriver
	folder *object.Folder
}

func (d *VCenterDriver) NewFolder(ref *types.ManagedObjectReference) *Folder {
	return &Folder{
		folder: object.NewFolder(d.Client.Client, *ref),
		driver: d,
	}
}

// FindFolder locates or creates a folder structure within a datastore based
// on the provided folder name. Returns a pointer to the Folder object or an
// error if the operation fails.
func (d *VCenterDriver) FindFolder(name string) (*Folder, error) {
	if name != "" {
		// If the folder does not exist, create it.
		parent := ""
		parentFolder, err := d.Finder.Folder(d.Ctx, path.Join(d.Datacenter.InventoryPath, "vm"))
		if err != nil {
			return nil, err
		}
		folders := strings.SplitSeq(name, "/")
		for folder := range folders {
			parent = path.Join(parent, folder)
			f, err := d.Finder.Folder(d.Ctx, path.Join(d.Datacenter.InventoryPath, "vm", parent))
			var notFoundError *find.NotFoundError
			if errors.As(err, &notFoundError) {
				f, err = parentFolder.CreateFolder(d.Ctx, folder)
			}
			if err != nil {
				return nil, err
			}
			parentFolder = f
		}
	}

	f, err := d.Finder.Folder(d.Ctx, path.Join(d.Datacenter.InventoryPath, "vm", name))
	if err != nil {
		return nil, err
	}

	return &Folder{
		folder: f,
		driver: d,
	}, nil
}

// Info retrieves properties of the folder object with optional filters specified
// as parameters. If no parameters are provided, all properties are returned.
func (f *Folder) Info(params ...string) (*mo.Folder, error) {
	var p []string
	if len(params) == 0 {
		p = []string{"*"}
	} else {
		p = params
	}
	var info mo.Folder
	err := f.folder.Properties(f.driver.Ctx, f.folder.Reference(), p, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// Path constructs and returns the full hierarchical path of the folder,
// starting from the datacenter level or an error.
func (f *Folder) Path() (string, error) {
	info, err := f.Info("name", "parent")
	if err != nil {
		return "", err
	}
	if info.Parent.Type == "Datacenter" {
		return "", nil
	}

	parent := f.driver.NewFolder(info.Parent)
	parentPath, err := parent.Path()
	if err != nil {
		return "", err
	}
	if parentPath == "" {
		return info.Name, nil
	}

	return fmt.Sprintf("%v/%v", parentPath, info.Name), nil
}

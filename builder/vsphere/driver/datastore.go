// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

type Datastore interface {
	Info(params ...string) (*mo.Datastore, error)
	FileExists(path string) bool
	DirExists(path string) bool
	Name() string
	ResolvePath(path string) string
	UploadFile(src, dst, host string, setHost bool) error
	Delete(path string) error
	MakeDirectory(path string) error
	Reference() types.ManagedObjectReference
}

type DatastoreDriver struct {
	ds     *object.Datastore
	driver *VCenterDriver
}

// NewDatastore creates a new Datastore object.
func (d *VCenterDriver) NewDatastore(ref *types.ManagedObjectReference) Datastore {
	return &DatastoreDriver{
		ds:     object.NewDatastore(d.Client.Client, *ref),
		driver: d,
	}
}

// FindDatastore locates a datastore by its name and an optional host.
// Returns a Datastore object or an error if the datastore is not found.
func (d *VCenterDriver) FindDatastore(name string, host string) (Datastore, error) {
	if name == "" {
		h, err := d.FindHost(host)
		if err != nil {
			return nil, fmt.Errorf("error finding host to return datastore: %s", err)
		}

		i, err := h.Info("datastore")
		if err != nil {
			return nil, fmt.Errorf("error returning datastore info from host: %s", err)
		}

		if len(i.Datastore) > 1 {
			return nil, fmt.Errorf("host has multiple datastores; specify the datastore name")
		}

		ds := d.NewDatastore(&i.Datastore[0])
		inf, err := ds.Info("name")
		if err != nil {
			return nil, fmt.Errorf("error returning datastore name: %s", err)
		}
		name = inf.Name
	}

	ds, err := d.Finder.Datastore(d.Ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error finding datastore with name %s: %s", name, err)
	}

	return &DatastoreDriver{
		ds:     ds,
		driver: d,
	}, nil
}

// GetDatastoreName retrieves the name of a datastore by its ID.
// Returns the name of the datastore or an error if the retrieval process
// fails.
func (d *VCenterDriver) GetDatastoreName(id string) (string, error) {
	obj := types.ManagedObjectReference{
		Type:  "Datastore",
		Value: id,
	}
	pc := property.DefaultCollector(d.VimClient)
	var me mo.ManagedEntity

	err := pc.RetrieveOne(d.Ctx, obj, []string{"name"}, &me)
	if err != nil {
		return id, err
	}
	return me.Name, nil
}

// Info retrieves properties of the datastore object with optional filters
// specified as parameters. If no parameters are provided, all properties are
// returned.
func (ds *DatastoreDriver) Info(params ...string) (*mo.Datastore, error) {
	var p []string
	if len(params) == 0 {
		p = []string{"*"}
	} else {
		p = params
	}
	var info mo.Datastore
	err := ds.ds.Properties(ds.driver.Ctx, ds.ds.Reference(), p, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (ds *DatastoreDriver) DirExists(filepath string) bool {
	_, err := ds.ds.Stat(ds.driver.Ctx, filepath)
	var notFoundError *object.DatastoreNoSuchDirectoryError
	return !errors.As(err, &notFoundError)
}

// FileExists checks if a file exists in a datastore.
func (ds *DatastoreDriver) FileExists(path string) bool {
	_, err := ds.ds.Stat(ds.driver.Ctx, path)
	return err == nil
}

// Name retrieves the name of a datastore.
func (ds *DatastoreDriver) Name() string {
	return ds.ds.Name()
}

// Reference retrieves the reference of a datastore.
func (ds *DatastoreDriver) Reference() types.ManagedObjectReference {
	return ds.ds.Reference()
}

// ResolvePath resolves a path in a datastore.
func (ds *DatastoreDriver) ResolvePath(path string) string {
	return ds.ds.Path(path)
}

// GetDatastoreFilePath retrieves the full path of a file in a specified datastore directory by its datastore ID and name.
func (d *VCenterDriver) GetDatastoreFilePath(datastoreID, dir, filename string) (string, error) {
	ref := types.ManagedObjectReference{Type: "Datastore", Value: datastoreID}
	ds := object.NewDatastore(d.VimClient, ref)

	b, err := ds.Browser(d.Ctx)
	if err != nil {
		return filename, err
	}
	ext := path.Ext(filename)
	pat := strings.Replace(filename, ext, "*"+ext, 1)
	spec := types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{pat},
	}

	task, err := b.SearchDatastore(d.Ctx, dir, &spec)
	if err != nil {
		return filename, err
	}

	info, err := task.WaitForResult(d.Ctx, nil)
	if err != nil {
		return filename, err
	}

	res, ok := info.Result.(types.HostDatastoreBrowserSearchResults)
	if !ok {
		return filename, fmt.Errorf("search(%s) result type=%T", pat, info.Result)
	}

	if len(res.File) != 1 {
		return filename, fmt.Errorf("search(%s) result files=%d", pat, len(res.File))
	}
	return res.File[0].GetFileInfo().Path, nil
}

// UploadFile uploads a file from the local source path to the destination path
// in the datastore, with optional host context.
func (ds *DatastoreDriver) UploadFile(src, dst, host string, setHost bool) error {
	p := soap.DefaultUpload
	ctx := ds.driver.Ctx

	if setHost && host != "" {
		h, err := ds.driver.FindHost(host)
		if err != nil {
			return err
		}
		ctx = ds.ds.HostContext(ctx, h.host)
	}

	return ds.ds.UploadFile(ctx, src, dst, &p)
}

// Delete deletes a file from a datastore by a path.
func (ds *DatastoreDriver) Delete(path string) error {
	dc, err := ds.driver.Finder.Datacenter(ds.driver.Ctx, ds.ds.DatacenterPath)
	if err != nil {
		return err
	}
	fm := ds.ds.NewFileManager(dc, false)
	return fm.Delete(ds.driver.Ctx, path)
}

// MakeDirectory creates a directory in a datastore by a path.
func (ds *DatastoreDriver) MakeDirectory(path string) error {
	dc, err := ds.driver.Finder.Datacenter(ds.driver.Ctx, ds.ds.DatacenterPath)
	if err != nil {
		return err
	}
	fm := ds.ds.NewFileManager(dc, false)
	return fm.FileManager.MakeDirectory(ds.driver.Ctx, path, dc, true)
}

// RemoveDatastorePrefix removes the datastore prefix from a path.
func RemoveDatastorePrefix(path string) string {
	res := object.DatastorePath{}
	if hadPrefix := res.FromString(path); hadPrefix {
		return res.Path
	}

	return path
}

type DatastoreIsoPath struct {
	path string
}

// Validate checks if the path matches the expected datastore ISO path format.
// Returns true if valid, otherwise false.
func (d *DatastoreIsoPath) Validate() bool {
	// Matches:
	// [datastore] /dir/subdir/file
	// [datastore] dir/subdir/file
	// [] /dir/subdir/file
	// [data-store] /dir/subdir/file
	// dir/subdir/file or dir/subdir/file
	matched, _ := regexp.MatchString(`^\s*(\[[^\[\]\/]*\])?\s*[^\[\]]+\s*$`, d.path)
	return matched
}

// GetFilePath removes the datastore name from the path and returns the trimmed
// file path portion of the datastore ISO path.
func (d *DatastoreIsoPath) GetFilePath() string {
	filePath := d.path
	parts := strings.Split(d.path, "]")
	if len(parts) > 1 {
		filePath = parts[1]
		filePath = strings.TrimSpace(filePath)
	}
	return filePath
}

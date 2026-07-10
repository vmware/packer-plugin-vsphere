// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
)

type Library struct {
	library *library.Library
}

// FindContentLibraryByName retrieves a content library by its name. Returns a
// Library object or an error if the library is not found.
func (d *VCenterDriver) FindContentLibraryByName(name string) (*Library, error) {
	lm := library.NewManager(d.RestClient.client)
	l, err := lm.GetLibraryByName(d.Ctx, name)
	if err != nil {
		return nil, err
	}
	return &Library{
		library: l,
	}, nil
}

// FindContentLibraryItem retrieves a content library item by its name within
// the specified library ID.  Returns the library item if found or an error if
// the item is not found or the retrieval process fails.
func (d *VCenterDriver) FindContentLibraryItem(libraryId string, name string) (*library.Item, error) {
	lm := library.NewManager(d.RestClient.client)
	items, err := lm.GetLibraryItems(d.Ctx, libraryId)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("content library item %s not found", name)
}

// FindContentLibraryItemUUID retrieves the UUID of a content library item
//
//	based on the given library ID and item name. Returns the UUID if found or
//	an error if the item is not found or the retrieval process fails.
func (d *VCenterDriver) FindContentLibraryItemUUID(libraryId string, name string) (string, error) {
	item, err := d.FindContentLibraryItem(libraryId, name)
	if err != nil {
		return "", err
	}
	return item.ID, nil
}

// FindContentLibraryFileDatastorePath checks if the provided ISO path belongs
// to a content library and retrieves its datastore path. Returns the datastore
// path if the ISO path is a content library path or an error if the path is
// not identified as a content library path or if the retrieval process fails.
func (d *VCenterDriver) FindContentLibraryFileDatastorePath(isoPath string) (string, error) {
	log.Printf("Check if ISO path is a Content Library path")
	err := d.RestClient.Login(d.Ctx)
	if err != nil {
		log.Printf("vCenter client not available. ISO path not identified as a Content Library path")
		return isoPath, err
	}

	libraryFilePath := &LibraryFilePath{path: isoPath}
	err = libraryFilePath.Validate()
	if err != nil {
		log.Printf("ISO path not identified as a Content Library path")
		return isoPath, err
	}
	libraryName := libraryFilePath.GetLibraryName()
	itemName := libraryFilePath.GetLibraryItemName()
	isoFile := libraryFilePath.GetFileName()

	lib, err := d.FindContentLibraryByName(libraryName)
	if err != nil {
		log.Printf("ISO path not identified as a Content Library path")
		return isoPath, err
	}
	log.Printf("ISO path identified as a Content Library path")
	log.Printf("Finding the equivalent datastore path for the Content Library ISO file path")
	libItem, err := d.FindContentLibraryItem(lib.library.ID, itemName)
	if err != nil {
		log.Printf("[WARN] Content library item %s not found: %s", itemName, err)
		return isoPath, err
	}
	datastoreName, err := d.GetDatastoreName(lib.library.Storage[0].DatastoreID)
	if err != nil {
		log.Printf("[WARN] Datastore not found for content library %s", libraryName)
		return isoPath, err
	}
	libItemDir := fmt.Sprintf("[%s] contentlib-%s/%s", datastoreName, lib.library.ID, libItem.ID)

	isoFilePath, err := d.GetDatastoreFilePath(lib.library.Storage[0].DatastoreID, libItemDir, isoFile)
	if err != nil {
		log.Printf("[WARN] Datastore path not found for %s", isoFile)
		return isoPath, err
	}

	_ = d.RestClient.Logout(d.Ctx)
	return path.Join(libItemDir, isoFilePath), nil
}

// UpdateContentLibraryItem updates the metadata of a content library item,
// such as its name and description. Returns an error if the update fails.
func (d *VCenterDriver) UpdateContentLibraryItem(item *library.Item, name string, description string) error {
	lm := library.NewManager(d.RestClient.client)
	item.Patch(&library.Item{
		ID:          item.ID,
		Name:        name,
		Description: &description,
	})
	return lm.UpdateLibraryItem(d.Ctx, item)
}

// DeleteContentLibraryItem deletes a content library item by its ID.
// Returns an error if the deletion fails.
func (d *VCenterDriver) DeleteContentLibraryItem(itemID string) error {
	lm := library.NewManager(d.RestClient.client)
	return lm.DeleteLibraryItem(d.Ctx, &library.Item{ID: itemID})
}

type LibraryFilePath struct {
	path string
}

// Validate checks the format of the LibraryFilePath and returns an error if
// the path is not in the expected format.
func (l *LibraryFilePath) Validate() error {
	l.path = strings.TrimLeft(l.path, "/")
	parts := strings.Split(l.path, "/")
	if len(parts) != 3 {
		return fmt.Errorf("content library file path must contain the names for the library, item, and file")
	}
	return nil
}

// GetLibraryName retrieves the library name from the content library file path.
func (l *LibraryFilePath) GetLibraryName() string {
	return strings.Split(l.path, "/")[0]
}

// GetLibraryItemName retrieves the library item name from the content library file path.
func (l *LibraryFilePath) GetLibraryItemName() string {
	return strings.Split(l.path, "/")[1]
}

// GetFileName retrieves the file name from the content library file path.
func (l *LibraryFilePath) GetFileName() string {
	return strings.Split(l.path, "/")[2]
}

// ContentLibraryDeployConfig contains configuration for deploying virtual
// machines from a vSphere content library item.
type ContentLibraryDeployConfig struct {
	Item *library.Item

	Name         string
	Folder       string
	Cluster      string
	Host         string
	ResourcePool string
	Datastore    string

	Network          string
	MacAddress       string
	Annotation       string
	VAppProperties   map[string]string
	DeploymentOption string

	PrimaryDiskSize int64
	StorageConfig   StorageConfig
}

// ResolveContentLibraryItem resolves a content library item by library and item name.
func (d *VCenterDriver) ResolveContentLibraryItem(libraryName, itemName string) (*library.Item, error) {
	if err := d.RestClient.Login(d.Ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with vCenter: %s", err)
	}

	lib, err := d.FindContentLibraryByName(libraryName)
	if err != nil {
		return nil, d.wrapContentLibraryError(
			fmt.Errorf("content library '%s' not found: %s", libraryName, err),
			libraryName, itemName,
		)
	}

	item, err := d.FindContentLibraryItem(lib.library.ID, itemName)
	if err != nil {
		return nil, d.wrapContentLibraryError(
			fmt.Errorf("content library item '%s' not found in library '%s': %s", itemName, libraryName, err),
			libraryName, itemName,
		)
	}

	log.Printf("[INFO] Resolved content library item '%s' (type=%s) in library '%s'", itemName, item.Type, libraryName)
	return item, nil
}

// DeployContentLibraryItem deploys a virtual machine from a content library item.
func (d *VCenterDriver) DeployContentLibraryItem(ctx context.Context, config *ContentLibraryDeployConfig, ui packersdk.Ui) (VirtualMachine, error) {
	if config == nil {
		return nil, fmt.Errorf("content library deployment configuration cannot be nil")
	}
	if config.Name == "" {
		return nil, fmt.Errorf("virtual machine name is required")
	}
	if config.Item == nil {
		return nil, fmt.Errorf("content library item is required")
	}

	item := config.Item
	switch item.Type {
	case library.ItemTypeOVF, library.ItemTypeVMTX:
	default:
		return nil, fmt.Errorf("unsupported content library item type '%s'; supported types are 'ovf' and 'vm-template'", item.Type)
	}

	target, err := d.buildContentLibraryDeployTarget(config)
	if err != nil {
		return nil, err
	}

	ui.Sayf("Deploying virtual machine from content library item '%s' (type=%s)...", item.Name, item.Type)

	switch item.Type {
	case library.ItemTypeOVF:
		return d.deployOvfLibraryItem(ctx, item, config, target)
	default:
		return d.deployVmtxLibraryItem(ctx, item, config, target)
	}
}

// filterContentLibraryOvf returns OVF deployment parameters for a library item.
func (d *VCenterDriver) filterContentLibraryOvf(ctx context.Context, itemID string, target vcenter.Target) (vcenter.FilterResponse, error) {
	if err := d.RestClient.Login(d.Ctx); err != nil {
		return vcenter.FilterResponse{}, fmt.Errorf("failed to authenticate with vCenter: %s", err)
	}

	m := vcenter.NewManager(d.RestClient.client)
	return m.FilterLibraryItem(ctx, itemID, vcenter.FilterRequest{Target: target})
}

type contentLibraryDeployTarget struct {
	vcenter.Target
	datastoreID string
}

func (d *VCenterDriver) buildContentLibraryDeployTarget(config *ContentLibraryDeployConfig) (*contentLibraryDeployTarget, error) {
	pool, err := d.FindResourcePool(config.Cluster, config.Host, config.ResourcePool)
	if err != nil {
		return nil, fmt.Errorf("error finding resource pool: %s", err)
	}

	folder, err := d.FindFolder(config.Folder)
	if err != nil {
		return nil, fmt.Errorf("error finding folder: %s", err)
	}

	target := &contentLibraryDeployTarget{
		Target: vcenter.Target{
			ResourcePoolID: pool.pool.Reference().Value,
			FolderID:       folder.folder.Reference().Value,
		},
	}

	if config.Cluster != "" && config.Host != "" {
		host, err := d.FindHost(config.Host)
		if err != nil {
			return nil, fmt.Errorf("error finding host: %s", err)
		}
		target.HostID = host.host.Reference().Value
	}

	if config.Datastore != "" {
		datastore, err := d.FindDatastore(config.Datastore, config.Host)
		if err != nil {
			return nil, fmt.Errorf("error finding datastore: %s", err)
		}
		target.datastoreID = datastore.Reference().Value
	}

	return target, nil
}

func (d *VCenterDriver) wrapContentLibraryError(err error, libraryName, itemName string) error {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "403") || strings.Contains(msg, "401") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "permission") ||
		strings.Contains(msg, "not authorized") {
		return fmt.Errorf("insufficient permissions to access content library '%s' or item '%s': %s", libraryName, itemName, err)
	}
	return err
}

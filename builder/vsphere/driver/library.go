// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/vmware/govmomi/vapi/library"
)

type Library struct {
	driver  *VCenterDriver
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
		driver:  d,
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

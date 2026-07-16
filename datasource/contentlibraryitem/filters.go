// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibraryitem

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// contentLibraryItemMoRefType is the managed object reference type used by the
// tagging service to associate tags with a content library item.
const contentLibraryItemMoRefType = "com.vmware.content.library.Item"

// findLibrary resolves exactly one content library by name.
func findLibrary(d *driver.VCenterDriver, lm *library.Manager, name string) (library.Library, error) {
	ids, err := lm.FindLibrary(d.Ctx, library.Find{Name: name})
	if err != nil {
		return library.Library{}, fmt.Errorf("failed to find content library %q: %w", name, err)
	}
	switch len(ids) {
	case 0:
		return library.Library{}, fmt.Errorf("no content library named %q was found", name)
	case 1:
		lib, err := lm.GetLibraryByID(d.Ctx, ids[0])
		if err != nil {
			return library.Library{}, fmt.Errorf("failed to retrieve content library %q: %w", name, err)
		}
		return *lib, nil
	default:
		return library.Library{}, fmt.Errorf("more than one content library named %q was found", name)
	}
}

// filterItems removes content library items that do not match the datasource
// filters.
func filterItems(itemList []library.Item, c Config, d *driver.VCenterDriver) ([]library.Item, error) {
	filterFuncs := make([]func(library.Item) (bool, error), 0)

	if c.Name != "" && c.Name != "*" {
		filterFuncs = append(filterFuncs, func(item library.Item) (bool, error) {
			ok, err := path.Match(c.Name, item.Name)
			if err != nil {
				return false, fmt.Errorf("invalid name glob: %w", err)
			}
			return ok, nil
		})
	}

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(item library.Item) (bool, error) {
			return re.MatchString(item.Name), nil
		})
	}

	if c.Type != "" {
		want := itemTypes[c.Type]
		filterFuncs = append(filterFuncs, func(item library.Item) (bool, error) {
			return strings.EqualFold(item.Type, want), nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(item library.Item) (bool, error) {
			return matcher.HasAll(types.ManagedObjectReference{
				Type:  contentLibraryItemMoRefType,
				Value: item.ID,
			})
		})
	}

	result := make([]library.Item, 0)
	for _, item := range itemList {
		ok := true
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(item)
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, item)
		}
	}

	return result, nil
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

// itemModTime returns a comparable score for selecting the latest item. The
// last modified time is preferred, falling back to the creation time.
func itemModTime(item library.Item) int64 {
	if item.LastModifiedTime != nil {
		return item.LastModifiedTime.UnixNano()
	}
	if item.CreationTime != nil {
		return item.CreationTime.UnixNano()
	}
	return 0
}

// resolveItemPath builds the content library path for an item. For ISO items
// the resolvable file is appended so the path can be consumed by the builder
// `iso_paths` option (`<library>/<item>/<file>`). Other item types return the
// `<library>/<item>` form.
func resolveItemPath(d *driver.VCenterDriver, lm *library.Manager, lib library.Library, item library.Item) (string, error) {
	base := path.Join(lib.Name, item.Name)

	if item.Type != library.ItemTypeISO {
		return base, nil
	}

	files, err := lm.ListLibraryItemFiles(d.Ctx, item.ID)
	if err != nil {
		return "", fmt.Errorf("failed to list files for content library item %q: %w", item.Name, err)
	}

	fileName := selectISOFile(files)
	if fileName == "" {
		return base, nil
	}

	return path.Join(lib.Name, item.Name, fileName), nil
}

// selectISOFile returns the name of the ISO file within an item, preferring a
// file with the `.iso` extension and falling back to the first file.
func selectISOFile(files []library.File) string {
	if len(files) == 0 {
		return ""
	}
	for _, f := range files {
		if strings.EqualFold(path.Ext(f.Name), ".iso") {
			return f.Name
		}
	}
	return files[0].Name
}

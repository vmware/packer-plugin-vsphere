// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package contentlibrary

import (
	"fmt"
	"regexp"

	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// contentLibraryMoRefType is the managed object reference type used by the
// tagging service to associate tags with a content library.
const contentLibraryMoRefType = "com.vmware.content.Library"

// filterLibraries removes content libraries that do not match the datasource
// filters.
func filterLibraries(libraryList []library.Library, c Config, d *driver.VCenterDriver) ([]library.Library, error) {
	filterFuncs := make([]func(library.Library) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(lib library.Library) (bool, error) {
			return re.MatchString(lib.Name), nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(lib library.Library) (bool, error) {
			return matcher.HasAll(types.ManagedObjectReference{
				Type:  contentLibraryMoRefType,
				Value: lib.ID,
			})
		})
	}

	result := make([]library.Library, 0)
	for _, lib := range libraryList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(lib)
			if err != nil {
				return nil, fmt.Errorf("failed to filter content library: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, lib)
		}
	}

	return result, nil
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

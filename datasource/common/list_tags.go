// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"fmt"

	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// ListAttachedTags returns all tags attached to ref as name/category pairs.
// An object with no tags yields an empty slice, not an error.
func ListAttachedTags(d *driver.VCenterDriver, ref types.ManagedObjectReference) ([]Tag, error) {
	if err := d.RestClient.Login(d.Ctx); err != nil {
		return nil, fmt.Errorf("failed to login to REST API: %w", err)
	}
	manager := tags.NewManager(d.RestClient.Client())

	attached, err := manager.GetAttachedTags(d.Ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to return tags for the object: %w", err)
	}

	out := make([]Tag, 0, len(attached))
	for _, realTag := range attached {
		category, err := manager.GetCategory(d.Ctx, realTag.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to return tag category for tag: %w", err)
		}
		out = append(out, Tag{
			Name:     realTag.Name,
			Category: category.Name,
		})
	}
	return out, nil
}

// MapFromTags converts common.Tag values into another package's tag type.
func MapFromTags[T any](in []Tag, new func(name, category string) T) []T {
	out := make([]T, len(in))
	for i, tag := range in {
		out[i] = new(tag.Name, tag.Category)
	}
	return out
}

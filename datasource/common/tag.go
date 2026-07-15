// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"errors"
	"fmt"

	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// Tag identifies a vSphere tag by name and category for datasource filters.
type Tag struct {
	Name     string
	Category string
}

// ValidateTags returns an error if any tag is missing name or category.
// Each invalid tag contributes the same message so callers can accumulate
// via MultiErrorAppend when wrapping.
func ValidateTags(tagList []Tag) error {
	var errs error
	for _, tag := range tagList {
		if tag.Name == "" || tag.Category == "" {
			errs = errors.Join(errs, errors.New("both name and category are required for tag"))
		}
	}
	return errs
}

// ObjectHasAllTags reports whether the object has all the required tags attached.
func ObjectHasAllTags(d *driver.VCenterDriver, ref types.ManagedObjectReference, required []Tag) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}

	err := d.RestClient.Login(d.Ctx)
	if err != nil {
		return false, fmt.Errorf("failed to login to REST API: %w", err)
	}

	tagMan := tags.NewManager(d.RestClient.Client())
	attached, err := tagMan.GetAttachedTags(d.Ctx, ref)
	if err != nil {
		return false, fmt.Errorf("failed return tags for the object: %w", err)
	}

	matchedTagsCount := 0
	for _, configTag := range required {
		configTagMatched := false
		for _, realTag := range attached {
			if configTag.Name != realTag.Name {
				continue
			}
			category, err := tagMan.GetCategory(d.Ctx, realTag.CategoryID)
			if err != nil {
				return false, fmt.Errorf("failed to return tag category for tag: %w", err)
			}
			if configTag.Category == category.Name {
				configTagMatched = true
				break
			}
		}
		if configTagMatched {
			matchedTagsCount++
		} else {
			break
		}
	}
	return matchedTagsCount == len(required), nil
}

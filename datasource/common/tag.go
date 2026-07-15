// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Tag

package common

import (
	"errors"
)

// Tag identifies a vSphere tag by name and category for datasource filters.
// Specify one or more `tag` blocks; every listed tag must be attached.
//
// HCL Example:
//
// ```hcl
//
//	tag {
//	  category = "environment"
//	  name     = "production"
//	}
//
// ```
type Tag struct {
	// Name of the tag that must be attached to the object.
	Name string `mapstructure:"name" required:"true"`
	// Name of the tag category that contains the tag.
	//
	// -> **Note:** Both `name` and `category` must be specified in the `tag`
	// filter.
	Category string `mapstructure:"category" required:"true"`
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

// MapTags converts HCL tag values from another package into common.Tag values.
func MapTags[T any](in []T, fields func(T) (name, category string)) []Tag {
	out := make([]Tag, len(in))
	for i, item := range in {
		name, category := fields(item)
		out[i] = Tag{Name: name, Category: category}
	}
	return out
}

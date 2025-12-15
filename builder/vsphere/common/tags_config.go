// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type TagConfig,TagsConfig

package common

// TagConfig represents a single tag configuration block.
type TagConfig struct {
	// The tag category name. Mutually exclusive with `id`.
	Category string `mapstructure:"category"`
	// The tag name within the category. Mutually exclusive with `id`.
	Name string `mapstructure:"name"`
	// The tag ID (URN). Mutually exclusive with `category` and `name`.
	ID string `mapstructure:"id"`
}

// TagsConfig holds all tag configurations.
type TagsConfig struct {
	// List of tag IDs to attach.
	Tags []string `mapstructure:"tags"`
	// List of tag configuration blocks.
	Tag []TagConfig `mapstructure:"tag"`
}

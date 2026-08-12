// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package contentlibraryitem

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/hcl2helper"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vim25/types"
	vsphere "github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
	"github.com/zclconf/go-cty/cty"
)

// itemTypes are the supported values for the `type` filter. They map directly
// to the content library API item types.
var itemTypes = map[string]string{
	library.ItemTypeISO:  library.ItemTypeISO,
	library.ItemTypeOVF:  library.ItemTypeOVF,
	library.ItemTypeVMTX: library.ItemTypeVMTX,
}

type Tag struct {
	Name     string `mapstructure:"name" required:"true"`
	Category string `mapstructure:"category" required:"true"`
}

type Config struct {
	common.PackerConfig   `mapstructure:",squash"`
	vsphere.ConnectConfig `mapstructure:",squash"`

	// Name of the content library to search for items. This filter is required.
	ContentLibrary string `mapstructure:"content_library" required:"true"`
	// Basic filter with glob support on the item name (e.g.
	// `linux-debian-13*-amd64` or `linux-debian-13`). Defaults to `*`.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support matched against the
	// item name (e.g. `^linux-debian-13\.[0-9]+\.[0-9]+-amd64$`). Default is
	// empty. The match is checked by substring. Use `^` and `$` to define a
	// full string. The expression must use
	// [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only items of the specified type. One of `iso`, `ovf`,
	// or `vm-template`. Default is empty and returns items of any type.
	Type string `mapstructure:"type"`
	// Filter to return only items that have all specified tags attached.
	Tags []Tag `mapstructure:"tag"`
	// This filter determines how to handle multiple items that were matched
	// with all previous filters. The item last modified time is used to find
	// the latest. By default, multiple matching items results in an error.
	Latest bool `mapstructure:"latest"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found content library item.
	Name string `mapstructure:"name"`
	// Unique identifier of the found content library item.
	ID string `mapstructure:"id"`
	// Name of the content library that contains the item.
	Library string `mapstructure:"library"`
	// Type of the found content library item. One of `iso`, `ovf`, or
	// `vm-template`.
	Type string `mapstructure:"type"`
	// Content library path of the found item.
	//
	// - For `iso` items, the path is returned in the `<library>/<item>/<file>`
	// form used by the builder `iso_paths` option.
	// - For `ovf` and `vm-template` items, the path is returned in the
	// `<library>/<item>` form.
	Path string `mapstructure:"path"`
	// Tags attached to the found content library item.
	Tags []Tag `mapstructure:"tags"`
}

func (d *Datasource) ConfigSpec() hcldec.ObjectSpec {
	return d.config.FlatMapstructure().HCL2Spec()
}

func (d *Datasource) Configure(raws ...any) error {
	err := config.Decode(&d.config, nil, raws...)
	if err != nil {
		return err
	}

	if d.config.Name == "" {
		d.config.Name = "*"
	}

	var errs error
	if d.config.ContentLibrary == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("'content_library' is required"))
	}
	if d.config.Type != "" {
		if _, ok := itemTypes[d.config.Type]; !ok {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("'type' must be one of 'iso', 'ovf', or 'vm-template', got %q", d.config.Type))
		}
	}
	if d.config.VCenterServer == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("'vcenter_server' is required"))
	}
	if d.config.Username == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("'username' is required"))
	}
	if d.config.Password == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("'password' is required"))
	}
	if err := dscommon.ValidateTags(toCommonTags(d.config.Tags)); err != nil {
		errs = packersdk.MultiErrorAppend(errs, err)
	}

	return errs
}

func (d *Datasource) OutputSpec() hcldec.ObjectSpec {
	return (&DatasourceOutput{}).FlatMapstructure().HCL2Spec()
}

func (d *Datasource) Execute() (cty.Value, error) {
	driverConfig := &driver.ConnectConfig{
		VCenterServer:      d.config.VCenterServer,
		Username:           d.config.Username,
		Password:           d.config.Password,
		InsecureConnection: d.config.InsecureConnection,
		Datacenter:         d.config.Datacenter,
	}

	dr, err := driver.NewDriver(driverConfig)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to initialize driver: %w", err)
	}

	vcDriver := dr.(*driver.VCenterDriver)
	lm := library.NewManager(vcDriver.GetRestClient())

	lib, err := findLibrary(vcDriver, lm, d.config.ContentLibrary)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	itemList, err := lm.GetLibraryItems(vcDriver.Ctx, lib.ID)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve content library items list: %w", err)
	}

	filtered, err := filterItems(itemList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter content library items: %w", err)
	}

	selected, err := dscommon.ResolveOne(
		filtered,
		d.config.Latest,
		func(items []library.Item) (library.Item, error) {
			return dscommon.SelectMax(items, func(item library.Item) (int64, error) {
				return itemModTime(item), nil
			})
		},
		"no content library item matches the filters",
		"more than one content library item matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	itemPath, err := resolveItemPath(vcDriver, lm, lib, selected)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	attached, err := dscommon.ListAttachedTags(vcDriver, types.ManagedObjectReference{
		Type:  contentLibraryItemMoRefType,
		Value: selected.ID,
	})
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	output := DatasourceOutput{
		Name:    selected.Name,
		ID:      selected.ID,
		Library: lib.Name,
		Type:    selected.Type,
		Path:    itemPath,
		Tags: dscommon.MapFromTags(attached, func(name, category string) Tag {
			return Tag{Name: name, Category: category}
		}),
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package contentlibrary

import (
	"errors"
	"fmt"
	"path"

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

type Tag struct {
	Name     string `mapstructure:"name" required:"true"`
	Category string `mapstructure:"category" required:"true"`
}

type Config struct {
	common.PackerConfig   `mapstructure:",squash"`
	vsphere.ConnectConfig `mapstructure:",squash"`

	// Basic filter with glob support on the content library name (e.g.
	// `lib*` or `lib01`). Defaults to `*`.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support matched against the
	// content library name (e.g. `^lib0[0-9]+$`). Default is empty. The
	// match is checked by substring. Use `^` and `$` to define a full string.
	// The expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only content libraries that have all specified tags
	// attached.
	Tags []Tag `mapstructure:"tag"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found content library.
	Name string `mapstructure:"name"`
	// Unique identifier of the found content library.
	ID string `mapstructure:"id"`
	// Tags attached to the found content library.
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

	libraryList, err := listContentLibraries(vcDriver, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve content libraries list: %w", err)
	}

	filtered, err := filterLibraries(libraryList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter content libraries: %w", err)
	}

	selected, err := dscommon.ExactlyOne(
		filtered,
		"no content library matches the filters",
		"more than one content library matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	attached, err := dscommon.ListAttachedTags(vcDriver, types.ManagedObjectReference{
		Type:  contentLibraryMoRefType,
		Value: selected.ID,
	})
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	output := DatasourceOutput{
		Name: selected.Name,
		ID:   selected.ID,
		Tags: dscommon.MapFromTags(attached, func(name, category string) Tag {
			return Tag{Name: name, Category: category}
		}),
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

func listContentLibraries(d *driver.VCenterDriver, name string) ([]library.Library, error) {
	lm := library.NewManager(d.GetRestClient())
	all, err := lm.GetLibraries(d.Ctx)
	if err != nil {
		return nil, err
	}

	if name == "*" {
		return all, nil
	}

	matched := make([]library.Library, 0)
	for _, lib := range all {
		ok, err := path.Match(name, lib.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid name glob: %w", err)
		}
		if ok {
			matched = append(matched, lib)
		}
	}
	return matched, nil
}

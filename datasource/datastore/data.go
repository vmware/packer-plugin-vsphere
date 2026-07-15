// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,Summary,DatasourceOutput

package datastore

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/hcl2helper"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	vsphere "github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
	"github.com/zclconf/go-cty/cty"
)

// Tag is the HCL `tag` filter block. Field docs live on datasource/common.Tag.
type Tag struct {
	Name     string `mapstructure:"name" required:"true"`
	Category string `mapstructure:"category" required:"true"`
}

type Config struct {
	common.PackerConfig   `mapstructure:",squash"`
	vsphere.ConnectConfig `mapstructure:",squash"`

	// Basic filter with glob support (e.g. `w01-cl01-vsan` or `*-vsan`).
	// Defaults to `*`. Using stricter globs does not reduce execution time
	// because the vSphere API returns the full inventory, but can improve
	// readability over regular expressions.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support
	// (e.g. `^w01-cl[0-9]+-vsan$`). Default is empty. The match is checked
	// by substring. Use `^` and `$` to define a full string. The expression
	// must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only datastores mounted on the specified ESX host.
	Host string `mapstructure:"host"`
	// Filter to return only datastores available to the specified compute
	// cluster.
	Cluster string `mapstructure:"cluster"`
	// Filter to return only datastores that have all specified tags attached.
	Tags []Tag `mapstructure:"tag"`
	// When more than one datastore matches the filters, select the datastore
	// with the most free space (`summary.free`). By default, multiple matches
	// result in an error.
	MostFreeSpace bool `mapstructure:"most_free_space"`
}

type Datasource struct {
	config Config
}

// Summary reports capacity fields from the datastore summary.
type Summary struct {
	// Total capacity of the datastore, in bytes.
	Capacity int64 `mapstructure:"capacity"`
	// Free space available on the datastore, in bytes.
	Free int64 `mapstructure:"free"`
}

type DatasourceOutput struct {
	// Name of the found datastore.
	Name string `mapstructure:"name"`
	// Managed object ID of the found datastore.
	ID string `mapstructure:"id"`
	// Capacity fields from the datastore summary.
	Summary Summary `mapstructure:"summary"`
}

func (d *Datasource) ConfigSpec() hcldec.ObjectSpec {
	return d.config.FlatMapstructure().HCL2Spec()
}

func (d *Datasource) Configure(raws ...interface{}) error {
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

	dsList, err := vcDriver.Finder.DatastoreList(vcDriver.Ctx, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve datastores list: %w", err)
	}

	filtered, err := filterDatastores(dsList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter datastores: %w", err)
	}

	selected, err := dscommon.ResolveOne(
		filtered,
		d.config.MostFreeSpace,
		func(items []*object.Datastore) (*object.Datastore, error) {
			ds, err := selectMostFreeSpace(vcDriver, items)
			if err != nil {
				return nil, fmt.Errorf("failed to select datastore with most free space: %w", err)
			}
			return ds, nil
		},
		"no datastore matches the filters",
		"more than one datastore matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	summary, err := datastoreSummary(vcDriver, selected)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	output := DatasourceOutput{
		Name: selected.Name(),
		ID:   selected.Reference().Value,
		Summary: Summary{
			Capacity: summary.Capacity,
			Free:     summary.FreeSpace,
		},
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

type datastoreSummaryInfo struct {
	Capacity  int64
	FreeSpace int64
}

func datastoreSummary(d *driver.VCenterDriver, ds *object.Datastore) (*datastoreSummaryInfo, error) {
	var obj mo.Datastore
	err := ds.Properties(d.Ctx, ds.Reference(), []string{"summary"}, &obj)
	if err != nil {
		return nil, fmt.Errorf("error retrieving summary for datastore %s: %w", ds.Name(), err)
	}
	return &datastoreSummaryInfo{
		Capacity:  obj.Summary.Capacity,
		FreeSpace: obj.Summary.FreeSpace,
	}, nil
}

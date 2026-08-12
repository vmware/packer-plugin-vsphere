// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package resourcepool

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/hcl2helper"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
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

	// Basic filter with glob support on the resource pool leaf name
	// (e.g. `rp-production*` or `rp-development`). Paths containing `/` are
	// passed to the inventory finder (e.g. `*/Resources/rp-production`).
	// Defaults to `*`.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support matched against the
	// pool name (e.g. `^rp-production[0-9]*$`). Default is empty. The match is
	// checked by substring. Use `^` and `$` to define a full string. The
	// expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only resource pools under the specified compute cluster.
	Cluster string `mapstructure:"cluster"`
	// Filter to return only resource pools available to the specified ESX host
	// (pools owned by the host's parent compute resource / cluster).
	Host string `mapstructure:"host"`
	// Filter to return only resource pools that have all specified tags
	// attached.
	Tags []Tag `mapstructure:"tag"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found resource pool.
	Name string `mapstructure:"name"`
	// Managed object ID of the found resource pool.
	ID string `mapstructure:"id"`
	// Builder-ready nested path under the compute cluster root `Resources` pool
	// (e.g. `rp-production` or `rp-parent/rp-child`). Empty for the
	// cluster/host root pool.
	Path string `mapstructure:"path"`
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

	poolList, err := listResourcePools(vcDriver, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve resource pools list: %w", err)
	}

	filtered, err := filterPools(poolList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter resource pools: %w", err)
	}

	selected, err := dscommon.ExactlyOne(
		filtered,
		"no resource pool matches the filters",
		"more than one resource pool matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	var selectedMo mo.ResourcePool
	err = property.DefaultCollector(vcDriver.Client.Client).RetrieveOne(
		vcDriver.Ctx, selected.Reference(), []string{"name"}, &selectedMo,
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("error retrieving resource pool name: %w", err)
	}

	ref := selected.Reference()
	builderPath, err := vcDriver.NewResourcePool(&ref).Path()
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("error resolving resource pool path: %w", err)
	}

	output := DatasourceOutput{
		Name: selectedMo.Name,
		ID:   selected.Reference().Value,
		Path: builderPath,
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

func listResourcePools(d *driver.VCenterDriver, name string) ([]*object.ResourcePool, error) {
	if strings.Contains(name, "/") {
		return d.Finder.ResourcePoolList(d.Ctx, name)
	}

	all, err := d.Finder.ResourcePoolList(d.Ctx, "*")
	if err != nil {
		return nil, err
	}
	if name == "*" {
		return all, nil
	}

	matched := make([]*object.ResourcePool, 0)
	for _, pool := range all {
		ok, err := path.Match(name, pool.Name())
		if err != nil {
			return nil, fmt.Errorf("invalid name glob: %w", err)
		}
		if ok {
			matched = append(matched, pool)
		}
	}
	return matched, nil
}

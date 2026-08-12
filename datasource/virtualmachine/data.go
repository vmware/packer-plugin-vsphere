// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package virtualmachine

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/hcl2helper"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/vmware/govmomi/object"
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

	// Basic filter with glob support (e.g. `ubuntu_basic*`). Defaults to `*`.
	// Using strict globs will not reduce execution time because vSphere API
	// returns the full inventory. But can be used for better readability over
	// regular expressions.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expressions support
	// (e.g. `ubuntu[-_]basic[0-9]*`). Default is empty. The match of the
	// regular expression is checked by substring. Use `^` and `$` to define a
	// full string. For example, the `^[^_]+$` filter will search names
	// without any underscores. The expression must use
	// [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only objects that are virtual machine templates.
	// Defaults to `false` and returns all virtual machines.
	Template bool `mapstructure:"template"`
	// Filter to search virtual machines only on the specified ESX host.
	Host string `mapstructure:"host"`
	// Filter to return only the virtual machines that have all specified tags
	// attached.
	Tags []Tag `mapstructure:"tag"`
	// This filter determines how to handle multiple virtual machines that were matched
	// with all previous filters. Virtual machine creation time is being used to find
	// the latest. By default, multiple matching machines results in an error.
	Latest bool `mapstructure:"latest"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found virtual machine.
	VmName string `mapstructure:"vm_name"`
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

	vmList, err := vcDriver.Finder.VirtualMachineList(vcDriver.Ctx, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve virtual machines list: %w", err)
	}

	filteredVms, err := filterVms(vmList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter virtual machines: %w", err)
	}

	selected, err := dscommon.ResolveOne(
		filteredVms,
		d.config.Latest,
		func(vms []*object.VirtualMachine) (*object.VirtualMachine, error) {
			vm, err := findLatestVM(vcDriver, vms)
			if err != nil {
				return nil, fmt.Errorf("failed to find the latest virtual machine: %w", err)
			}
			return vm, nil
		},
		"no virtual machine matches the filters",
		"more than one virtual machine matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	output := DatasourceOutput{
		VmName: selected.Name(),
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

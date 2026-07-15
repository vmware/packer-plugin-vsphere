// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,Summary,DatasourceOutput

package host

import (
	"errors"
	"fmt"

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

// Tag is the HCL `tag` filter block. Field docs live on datasource/common.Tag.
type Tag struct {
	Name     string `mapstructure:"name" required:"true"`
	Category string `mapstructure:"category" required:"true"`
}

type Config struct {
	common.PackerConfig   `mapstructure:",squash"`
	vsphere.ConnectConfig `mapstructure:",squash"`

	// Basic filter with glob support (e.g. `w01-cl01-esx01` or `*-esx*`).
	// Defaults to `*`. Using stricter globs does not reduce execution time
	// because the vSphere API returns the full inventory, but can improve
	// readability over regular expressions.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support
	// (e.g. `^w01-cl[0-9]+-esx[0-9]+$`). Default is empty. The match is checked
	// by substring. Use `^` and `$` to define a full string. The expression
	// must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only hosts that belong to the specified compute cluster.
	Cluster string `mapstructure:"cluster"`
	// Filter to return only hosts that have all specified tags attached.
	Tags []Tag `mapstructure:"tag"`
	// When more than one host matches the filters, select the host with the
	// most free memory (`summary.memory_free`). By default, multiple matches
	// result in an error.
	MostFreeMemory bool `mapstructure:"most_free_memory"`
}

type Datasource struct {
	config Config
}

// Summary reports memory capacity fields from the host summary.
type Summary struct {
	// Total memory capacity of the host, in bytes.
	MemoryCapacity int64 `mapstructure:"memory_capacity"`
	// Free memory available on the host, in bytes.
	MemoryFree int64 `mapstructure:"memory_free"`
}

type DatasourceOutput struct {
	// Name of the found host.
	Name string `mapstructure:"name"`
	// Managed object ID of the found host.
	ID string `mapstructure:"id"`
	// Name of the compute cluster that contains the host, if any.
	Cluster string `mapstructure:"cluster"`
	// Memory capacity fields from the host summary.
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

	hostList, err := vcDriver.Finder.HostSystemList(vcDriver.Ctx, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve hosts list: %w", err)
	}

	filtered, err := filterHosts(hostList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter hosts: %w", err)
	}

	selected, err := dscommon.ResolveOne(
		filtered,
		d.config.MostFreeMemory,
		func(items []*object.HostSystem) (*object.HostSystem, error) {
			host, err := selectMostFreeMemory(vcDriver, items)
			if err != nil {
				return nil, fmt.Errorf("failed to select host with most free memory: %w", err)
			}
			return host, nil
		},
		"no host matches the filters",
		"more than one host matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	clusterName, err := hostClusterName(vcDriver, selected)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	summary, err := hostMemorySummary(vcDriver, selected)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	output := DatasourceOutput{
		Name:    selected.Name(),
		ID:      selected.Reference().Value,
		Cluster: clusterName,
		Summary: Summary{
			MemoryCapacity: summary.MemoryCapacity,
			MemoryFree:     summary.MemoryFree,
		},
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

type hostMemorySummaryInfo struct {
	MemoryCapacity int64
	MemoryFree     int64
}

func hostMemorySummary(d *driver.VCenterDriver, host *object.HostSystem) (*hostMemorySummaryInfo, error) {
	var obj mo.HostSystem
	err := host.Properties(d.Ctx, host.Reference(), []string{"summary"}, &obj)
	if err != nil {
		return nil, fmt.Errorf("error retrieving summary for host %s: %w", host.Name(), err)
	}
	if obj.Summary.Hardware == nil {
		return nil, fmt.Errorf("host %s has no hardware summary", host.Name())
	}

	capacity := obj.Summary.Hardware.MemorySize
	used := int64(obj.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024
	free := capacity - used
	if free < 0 {
		free = 0
	}

	return &hostMemorySummaryInfo{
		MemoryCapacity: capacity,
		MemoryFree:     free,
	}, nil
}

func hostClusterName(d *driver.VCenterDriver, host *object.HostSystem) (string, error) {
	var obj mo.HostSystem
	err := host.Properties(d.Ctx, host.Reference(), []string{"parent"}, &obj)
	if err != nil {
		return "", fmt.Errorf("error retrieving parent for host %s: %w", host.Name(), err)
	}
	if obj.Parent == nil || obj.Parent.Type != "ClusterComputeResource" {
		return "", nil
	}

	var cluster mo.ClusterComputeResource
	err = property.DefaultCollector(d.Client.Client).RetrieveOne(d.Ctx, *obj.Parent, []string{"name"}, &cluster)
	if err != nil {
		return "", fmt.Errorf("error retrieving cluster name for host %s: %w", host.Name(), err)
	}
	return cluster.Name, nil
}

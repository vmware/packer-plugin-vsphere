// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package computecluster

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

	// Basic filter with glob support (e.g. `w01-cl01` or `*-cl*`).
	// Defaults to `*`. Using stricter globs does not reduce execution time
	// because the vSphere API returns the full inventory, but can improve
	// readability over regular expressions.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support
	// (e.g. `^w01-cl[0-9]+$`). Default is empty. The match is checked by
	// substring. Use `^` and `$` to define a full string. The expression must
	// use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only compute clusters that have all specified tags
	// attached.
	Tags []Tag `mapstructure:"tag"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found compute cluster.
	Name string `mapstructure:"name"`
	// Managed object ID of the found compute cluster.
	ID string `mapstructure:"id"`
	// Inventory path of the cluster root resource pool. Builders treat an empty
	// `resource_pool` as this root; an absolute path (starting with `/`) can be
	// passed through when an explicit pool is required.
	ResourcePool string `mapstructure:"resource_pool"`
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

	clusterList, err := vcDriver.Finder.ClusterComputeResourceList(vcDriver.Ctx, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve compute clusters list: %w", err)
	}

	filtered, err := filterClusters(clusterList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter compute clusters: %w", err)
	}

	selected, err := dscommon.ExactlyOne(
		filtered,
		"no compute cluster matches the filters",
		"more than one compute cluster matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	resourcePool, err := rootResourcePoolPath(vcDriver, selected)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	var selectedMo mo.ClusterComputeResource
	err = property.DefaultCollector(vcDriver.Client.Client).RetrieveOne(
		vcDriver.Ctx, selected.Reference(), []string{"name"}, &selectedMo,
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("error retrieving compute cluster name: %w", err)
	}

	output := DatasourceOutput{
		Name:         selectedMo.Name,
		ID:           selected.Reference().Value,
		ResourcePool: resourcePool,
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

func rootResourcePoolPath(d *driver.VCenterDriver, cluster *object.ClusterComputeResource) (string, error) {
	var clusterMo mo.ClusterComputeResource
	err := property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, cluster.Reference(), []string{"resourcePool"}, &clusterMo,
	)
	if err != nil {
		return "", fmt.Errorf("error retrieving resource pool for compute cluster %s: %w", cluster.Name(), err)
	}
	if clusterMo.ResourcePool == nil {
		return "", fmt.Errorf("compute cluster %s has no root resource pool", cluster.Name())
	}

	element, err := d.Finder.Element(d.Ctx, *clusterMo.ResourcePool)
	if err == nil && element != nil && element.Path != "" {
		return element.Path, nil
	}

	var poolMo mo.ResourcePool
	err = property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, *clusterMo.ResourcePool, []string{"name"}, &poolMo,
	)
	if err != nil {
		return "", fmt.Errorf("error retrieving root resource pool name: %w", err)
	}
	return poolMo.Name, nil
}

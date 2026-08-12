// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Tag,DatasourceOutput

package network

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

	// Basic filter with glob support on the network name (e.g. `pg-prod*` or
	// `VM Network`). Defaults to `*`.
	Name string `mapstructure:"name"`
	// Extended name filter with regular expression support matched against the
	// network name (e.g. `^pg-prod[0-9]*$`). Default is empty. The match is
	// checked by substring. Use `^` and `$` to define a full string. The
	// expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).
	NameRegex string `mapstructure:"name_regex"`
	// Filter to return only networks available to the specified compute
	// cluster.
	Cluster string `mapstructure:"cluster"`
	// Filter to return only networks available to the specified ESX host.
	Host string `mapstructure:"host"`
	// Optional filter by type. Use one of: `standard-port-group`,
	// `distributed-port-group`, or `nsx-segment`. Default is empty (any
	// supported type).
	//
	// -> **Note:** The corresponding API managed-object types are also
	// accepted: `standard-port-group` = `Network`, `distributed-port-group` =
	// `DistributedVirtualPortgroup`, and `nsx-segment` = `OpaqueNetwork`.
	//
	// -> **Note:** `nsx-segment` matches all opaque networks, not only NSX.
	Type string `mapstructure:"type"`
	// Filter to return only networks that have all specified tags attached.
	Tags []Tag `mapstructure:"tag"`
}

type Datasource struct {
	config Config
}

type DatasourceOutput struct {
	// Name of the found network.
	Name string `mapstructure:"name"`
	// Managed object ID of the found network.
	//
	// ~> **Note:** When using the data source result in a builder's `network` option,
	// use `data.vsphere-network.build.id` instead of `data.vsphere-network.build.name`
	// if the inventory contains more than one network with the same display name.
	ID string `mapstructure:"id"`
	// API managed-object type of the found network (`Network`,
	// `DistributedVirtualPortgroup`, or `OpaqueNetwork`).
	Type string `mapstructure:"type"`
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
	if d.config.Type != "" {
		if _, err := normalizeNetworkType(d.config.Type); err != nil {
			errs = packersdk.MultiErrorAppend(errs, err)
		}
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

	networkList, err := listNetworks(vcDriver, d.config.Name)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to retrieve networks list: %w", err)
	}

	filtered, err := filterNetworks(networkList, d.config, vcDriver)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), fmt.Errorf("failed to filter networks: %w", err)
	}

	selected, err := dscommon.ExactlyOne(
		filtered,
		"no network matches the filters",
		"more than one network matched the filters",
	)
	if err != nil {
		return cty.NullVal(cty.EmptyObject), err
	}

	ref := selected.Reference()
	output := DatasourceOutput{
		Name: networkLeafName(selected),
		ID:   ref.Value,
		Type: ref.Type,
	}

	return hcl2helper.HCL2ValueFromConfig(output, d.OutputSpec()), nil
}

func listNetworks(d *driver.VCenterDriver, name string) ([]object.NetworkReference, error) {
	all, err := d.Finder.NetworkList(d.Ctx, "*")
	if err != nil {
		return nil, err
	}

	supported := make([]object.NetworkReference, 0, len(all))
	for _, n := range all {
		if isSupportedNetworkType(n.Reference().Type) {
			supported = append(supported, n)
		}
	}

	if name == "*" {
		return supported, nil
	}

	matched := make([]object.NetworkReference, 0)
	for _, n := range supported {
		ok, err := path.Match(name, networkLeafName(n))
		if err != nil {
			return nil, fmt.Errorf("invalid name glob: %w", err)
		}
		if ok {
			matched = append(matched, n)
		}
	}
	return matched, nil
}

func networkLeafName(n object.NetworkReference) string {
	inventoryPath := n.GetInventoryPath()
	if inventoryPath != "" {
		return path.Base(inventoryPath)
	}
	return n.Reference().Value
}

func isSupportedNetworkType(apiType string) bool {
	switch apiType {
	case "Network", "DistributedVirtualPortgroup", "OpaqueNetwork":
		return true
	default:
		return false
	}
}

func normalizeNetworkType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "standard-port-group", "network":
		return "Network", nil
	case "distributed-port-group", "distributedvirtualportgroup":
		return "DistributedVirtualPortgroup", nil
	case "nsx-segment", "opaquenetwork":
		return "OpaqueNetwork", nil
	default:
		return "", fmt.Errorf("invalid type %q; expected standard-port-group, distributed-port-group, nsx-segment, or an API type alias", raw)
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package network

import (
	"fmt"
	"regexp"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// filterNetworks removes networks that do not match the datasource filters.
func filterNetworks(networkList []object.NetworkReference, c Config, d *driver.VCenterDriver) ([]object.NetworkReference, error) {
	filterFuncs := make([]func(object.NetworkReference) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(n object.NetworkReference) (bool, error) {
			return re.MatchString(networkLeafName(n)), nil
		})
	}

	if c.Type != "" {
		apiType, err := normalizeNetworkType(c.Type)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(n object.NetworkReference) (bool, error) {
			return n.Reference().Type == apiType, nil
		})
	}

	if c.Cluster != "" {
		refs, err := getClusterNetworkRefs(d, c.Cluster)
		if err != nil {
			return nil, err
		}
		allowed := refSet(refs)
		filterFuncs = append(filterFuncs, func(n object.NetworkReference) (bool, error) {
			_, ok := allowed[n.Reference().Value]
			return ok, nil
		})
	}

	if c.Host != "" {
		refs, err := getHostNetworkRefs(d, c.Host)
		if err != nil {
			return nil, err
		}
		allowed := refSet(refs)
		filterFuncs = append(filterFuncs, func(n object.NetworkReference) (bool, error) {
			_, ok := allowed[n.Reference().Value]
			return ok, nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(n object.NetworkReference) (bool, error) {
			return matcher.HasAll(n.Reference())
		})
	}

	result := make([]object.NetworkReference, 0)
	for _, n := range networkList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(n)
			if err != nil {
				return nil, fmt.Errorf("failed to filter network: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, n)
		}
	}

	return result, nil
}

func getClusterNetworkRefs(d *driver.VCenterDriver, clusterName string) ([]types.ManagedObjectReference, error) {
	obj, err := d.Finder.ClusterComputeResource(d.Ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined compute cluster: %w", err)
	}

	var cluster mo.ClusterComputeResource
	err = property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, obj.Reference(), []string{"network"}, &cluster,
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving networks of compute cluster: %w", err)
	}
	return cluster.Network, nil
}

func getHostNetworkRefs(d *driver.VCenterDriver, hostName string) ([]types.ManagedObjectReference, error) {
	obj, err := d.Finder.HostSystem(d.Ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined host system: %w", err)
	}

	var hostMo mo.HostSystem
	err = property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, obj.Reference(), []string{"network"}, &hostMo,
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving networks of host system: %w", err)
	}
	return hostMo.Network, nil
}

func refSet(refs []types.ManagedObjectReference) map[string]struct{} {
	out := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		out[ref.Value] = struct{}{}
	}
	return out
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

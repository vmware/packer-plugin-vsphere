// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package resourcepool

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// filterPools removes resource pools that do not match the datasource filters.
func filterPools(poolList []*object.ResourcePool, c Config, d *driver.VCenterDriver) ([]*object.ResourcePool, error) {
	filterFuncs := make([]func(*object.ResourcePool) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(pool *object.ResourcePool) (bool, error) {
			return re.MatchString(pool.Name()), nil
		})
	}

	if c.Cluster != "" {
		rootPath, err := clusterRootPoolPath(d, c.Cluster)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(pool *object.ResourcePool) (bool, error) {
			poolPath, err := inventoryPath(d, pool)
			if err != nil {
				return false, err
			}
			return underPath(poolPath, rootPath), nil
		})
	}

	if c.Host != "" {
		rootPath, err := hostRootPoolPath(d, c.Host)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(pool *object.ResourcePool) (bool, error) {
			poolPath, err := inventoryPath(d, pool)
			if err != nil {
				return false, err
			}
			return underPath(poolPath, rootPath), nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(pool *object.ResourcePool) (bool, error) {
			return matcher.HasAll(pool.Reference())
		})
	}

	result := make([]*object.ResourcePool, 0)
	for _, pool := range poolList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(pool)
			if err != nil {
				return nil, fmt.Errorf("failed to filter resource pool: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, pool)
		}
	}

	return result, nil
}

func clusterRootPoolPath(d *driver.VCenterDriver, clusterName string) (string, error) {
	cluster, err := d.Finder.ClusterComputeResource(d.Ctx, clusterName)
	if err != nil {
		return "", fmt.Errorf("error finding defined compute cluster: %w", err)
	}
	root, err := cluster.ResourcePool(d.Ctx)
	if err != nil {
		return "", fmt.Errorf("error retrieving root resource pool of compute cluster: %w", err)
	}
	return inventoryPath(d, root)
}

func hostRootPoolPath(d *driver.VCenterDriver, hostName string) (string, error) {
	host, err := d.Finder.HostSystem(d.Ctx, hostName)
	if err != nil {
		return "", fmt.Errorf("error finding defined host system: %w", err)
	}

	var hostMo mo.HostSystem
	err = property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, host.Reference(), []string{"parent"}, &hostMo,
	)
	if err != nil {
		return "", fmt.Errorf("error retrieving parent for host system: %w", err)
	}
	if hostMo.Parent == nil {
		return "", fmt.Errorf("host system %s has no parent compute resource", hostName)
	}

	rootRef, err := computeResourceRootPool(d, *hostMo.Parent)
	if err != nil {
		return "", err
	}
	root := object.NewResourcePool(d.Client.Client, rootRef)
	return inventoryPath(d, root)
}

func computeResourceRootPool(d *driver.VCenterDriver, parent types.ManagedObjectReference) (types.ManagedObjectReference, error) {
	switch parent.Type {
	case "ClusterComputeResource":
		var cluster mo.ClusterComputeResource
		err := property.DefaultCollector(d.Client.Client).RetrieveOne(
			d.Ctx, parent, []string{"resourcePool"}, &cluster,
		)
		if err != nil {
			return types.ManagedObjectReference{}, fmt.Errorf("error retrieving cluster resource pool: %w", err)
		}
		if cluster.ResourcePool == nil {
			return types.ManagedObjectReference{}, fmt.Errorf("compute cluster has no root resource pool")
		}
		return *cluster.ResourcePool, nil
	case "ComputeResource":
		var cr mo.ComputeResource
		err := property.DefaultCollector(d.Client.Client).RetrieveOne(
			d.Ctx, parent, []string{"resourcePool"}, &cr,
		)
		if err != nil {
			return types.ManagedObjectReference{}, fmt.Errorf("error retrieving compute resource pool: %w", err)
		}
		if cr.ResourcePool == nil {
			return types.ManagedObjectReference{}, fmt.Errorf("compute resource has no root resource pool")
		}
		return *cr.ResourcePool, nil
	default:
		return types.ManagedObjectReference{}, fmt.Errorf("unsupported host parent type %s", parent.Type)
	}
}

func inventoryPath(d *driver.VCenterDriver, pool *object.ResourcePool) (string, error) {
	if pool.InventoryPath != "" {
		return pool.InventoryPath, nil
	}
	element, err := d.Finder.Element(d.Ctx, pool.Reference())
	if err != nil {
		return "", fmt.Errorf("error resolving inventory path for resource pool: %w", err)
	}
	if element == nil || element.Path == "" {
		return "", fmt.Errorf("resource pool has empty inventory path")
	}
	return element.Path, nil
}

func underPath(poolPath, rootPath string) bool {
	if poolPath == rootPath {
		return true
	}
	prefix := rootPath
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return strings.HasPrefix(poolPath, prefix)
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

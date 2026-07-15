// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package host

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

// filterHosts removes hosts that do not match the datasource filters.
func filterHosts(hostList []*object.HostSystem, c Config, d *driver.VCenterDriver) ([]*object.HostSystem, error) {
	filterFuncs := make([]func(*object.HostSystem) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(host *object.HostSystem) (bool, error) {
			return re.MatchString(host.Name()), nil
		})
	}

	if c.Cluster != "" {
		clusterHosts, err := getClusterHostRefs(d, c.Cluster)
		if err != nil {
			return nil, err
		}
		allowed := refSet(clusterHosts)
		filterFuncs = append(filterFuncs, func(host *object.HostSystem) (bool, error) {
			_, ok := allowed[host.Reference().Value]
			return ok, nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(host *object.HostSystem) (bool, error) {
			return matcher.HasAll(host.Reference())
		})
	}

	result := make([]*object.HostSystem, 0)
	for _, host := range hostList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(host)
			if err != nil {
				return nil, fmt.Errorf("failed to filter host: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, host)
		}
	}

	return result, nil
}

func selectMostFreeMemory(d *driver.VCenterDriver, hostList []*object.HostSystem) (*object.HostSystem, error) {
	return dscommon.SelectMax(hostList, func(host *object.HostSystem) (int64, error) {
		summary, err := hostMemorySummary(d, host)
		if err != nil {
			return 0, err
		}
		return summary.MemoryFree, nil
	})
}

func getClusterHostRefs(d *driver.VCenterDriver, clusterName string) ([]types.ManagedObjectReference, error) {
	pc := property.DefaultCollector(d.Client.Client)
	obj, err := d.Finder.ClusterComputeResource(d.Ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined compute cluster: %w", err)
	}

	var cluster mo.ClusterComputeResource
	err = pc.RetrieveOne(d.Ctx, obj.Reference(), []string{"host"}, &cluster)
	if err != nil {
		return nil, fmt.Errorf("error retrieving hosts of compute cluster: %w", err)
	}
	return cluster.Host, nil
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

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package computecluster

import (
	"fmt"
	"regexp"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// filterClusters removes compute clusters that do not match the filters.
func filterClusters(clusterList []*object.ClusterComputeResource, c Config, d *driver.VCenterDriver) ([]*object.ClusterComputeResource, error) {
	filterFuncs := make([]func(*object.ClusterComputeResource) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(cluster *object.ClusterComputeResource) (bool, error) {
			name, err := clusterName(d, cluster)
			if err != nil {
				return false, err
			}
			return re.MatchString(name), nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(cluster *object.ClusterComputeResource) (bool, error) {
			return matcher.HasAll(cluster.Reference())
		})
	}

	result := make([]*object.ClusterComputeResource, 0)
	for _, cluster := range clusterList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(cluster)
			if err != nil {
				return nil, fmt.Errorf("failed to filter compute cluster: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, cluster)
		}
	}

	return result, nil
}

func clusterName(d *driver.VCenterDriver, cluster *object.ClusterComputeResource) (string, error) {
	var obj mo.ClusterComputeResource
	err := property.DefaultCollector(d.Client.Client).RetrieveOne(
		d.Ctx, cluster.Reference(), []string{"name"}, &obj,
	)
	if err != nil {
		return "", fmt.Errorf("error retrieving compute cluster name: %w", err)
	}
	return obj.Name, nil
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package datastorecluster

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

// filterClusters removes datastore clusters that do not match the filters.
func filterClusters(clusterList []*object.StoragePod, c Config, d *driver.VCenterDriver) ([]*object.StoragePod, error) {
	filterFuncs := make([]func(*object.StoragePod) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(pod *object.StoragePod) (bool, error) {
			return re.MatchString(pod.Name()), nil
		})
	}

	if c.Host != "" {
		hostDatastores, err := getHostDatastoreRefs(d, c.Host)
		if err != nil {
			return nil, err
		}
		allowed := refSet(hostDatastores)
		filterFuncs = append(filterFuncs, func(pod *object.StoragePod) (bool, error) {
			members, err := podMemberDatastoreRefs(d, pod)
			if err != nil {
				return false, err
			}
			return allMembersAllowed(members, allowed), nil
		})
	}

	if c.Cluster != "" {
		clusterDatastores, err := getClusterDatastoreRefs(d, c.Cluster)
		if err != nil {
			return nil, err
		}
		allowed := refSet(clusterDatastores)
		filterFuncs = append(filterFuncs, func(pod *object.StoragePod) (bool, error) {
			members, err := podMemberDatastoreRefs(d, pod)
			if err != nil {
				return false, err
			}
			return allMembersAllowed(members, allowed), nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(pod *object.StoragePod) (bool, error) {
			return matcher.HasAll(pod.Reference())
		})
	}

	result := make([]*object.StoragePod, 0)
	for _, pod := range clusterList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(pod)
			if err != nil {
				return nil, fmt.Errorf("failed to filter datastore cluster: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, pod)
		}
	}

	return result, nil
}

func selectMostFreeSpace(d *driver.VCenterDriver, clusters []*object.StoragePod) (*object.StoragePod, error) {
	return dscommon.SelectMax(clusters, func(cluster *object.StoragePod) (int64, error) {
		_, summary, err := clusterMembersAndSummary(d, cluster)
		if err != nil {
			return 0, err
		}
		return summary.FreeSpace, nil
	})
}

func getHostDatastoreRefs(d *driver.VCenterDriver, hostName string) ([]types.ManagedObjectReference, error) {
	pc := property.DefaultCollector(d.Client.Client)
	obj, err := d.Finder.HostSystem(d.Ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined host system: %w", err)
	}

	var host mo.HostSystem
	err = pc.RetrieveOne(d.Ctx, obj.Reference(), []string{"datastore"}, &host)
	if err != nil {
		return nil, fmt.Errorf("error retrieving datastores of host system: %w", err)
	}
	return host.Datastore, nil
}

func getClusterDatastoreRefs(d *driver.VCenterDriver, clusterName string) ([]types.ManagedObjectReference, error) {
	pc := property.DefaultCollector(d.Client.Client)
	obj, err := d.Finder.ClusterComputeResource(d.Ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined compute cluster: %w", err)
	}

	var cluster mo.ClusterComputeResource
	err = pc.RetrieveOne(d.Ctx, obj.Reference(), []string{"datastore"}, &cluster)
	if err != nil {
		return nil, fmt.Errorf("error retrieving datastores of compute cluster: %w", err)
	}
	return cluster.Datastore, nil
}

func podMemberDatastoreRefs(d *driver.VCenterDriver, pod *object.StoragePod) ([]types.ManagedObjectReference, error) {
	children, err := pod.Children(d.Ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing datastores in cluster %s: %w", pod.Name(), err)
	}

	members := make([]types.ManagedObjectReference, 0, len(children))
	for _, child := range children {
		ref := child.Reference()
		if ref.Type == "Datastore" {
			members = append(members, ref)
		}
	}
	return members, nil
}

func refSet(refs []types.ManagedObjectReference) map[string]struct{} {
	out := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		out[ref.Value] = struct{}{}
	}
	return out
}

func allMembersAllowed(members []types.ManagedObjectReference, allowed map[string]struct{}) bool {
	if len(members) == 0 {
		return false
	}
	for _, member := range members {
		if _, ok := allowed[member.Value]; !ok {
			return false
		}
	}
	return true
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	return dscommon.MapTags(tagList, func(tag Tag) (string, string) {
		return tag.Name, tag.Category
	})
}

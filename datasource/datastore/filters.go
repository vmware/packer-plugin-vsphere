// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package datastore

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

// filterDatastores removes datastores that do not match the datasource filters.
func filterDatastores(dsList []*object.Datastore, c Config, d *driver.VCenterDriver) ([]*object.Datastore, error) {
	filterFuncs := make([]func(*object.Datastore) (bool, error), 0)

	if c.NameRegex != "" {
		re, err := regexp.Compile(c.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		filterFuncs = append(filterFuncs, func(ds *object.Datastore) (bool, error) {
			return re.MatchString(ds.Name()), nil
		})
	}

	if c.Host != "" {
		hostDatastores, err := getHostDatastoreRefs(d, c.Host)
		if err != nil {
			return nil, err
		}
		allowed := refSet(hostDatastores)
		filterFuncs = append(filterFuncs, func(ds *object.Datastore) (bool, error) {
			_, ok := allowed[ds.Reference().Value]
			return ok, nil
		})
	}

	if c.Cluster != "" {
		clusterDatastores, err := getClusterDatastoreRefs(d, c.Cluster)
		if err != nil {
			return nil, err
		}
		allowed := refSet(clusterDatastores)
		filterFuncs = append(filterFuncs, func(ds *object.Datastore) (bool, error) {
			_, ok := allowed[ds.Reference().Value]
			return ok, nil
		})
	}

	if c.Tags != nil {
		matcher, err := dscommon.NewTagMatcher(d, toCommonTags(c.Tags))
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, func(ds *object.Datastore) (bool, error) {
			return matcher.HasAll(ds.Reference())
		})
	}

	result := make([]*object.Datastore, 0)
	for _, ds := range dsList {
		ok := len(filterFuncs) == 0
		for _, pass := range filterFuncs {
			var err error
			ok, err = pass(ds)
			if err != nil {
				return nil, fmt.Errorf("failed to filter datastore: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, ds)
		}
	}

	return result, nil
}

func selectMostFreeSpace(d *driver.VCenterDriver, dsList []*object.Datastore) (*object.Datastore, error) {
	return dscommon.SelectMax(dsList, func(ds *object.Datastore) (int64, error) {
		summary, err := datastoreSummary(d, ds)
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

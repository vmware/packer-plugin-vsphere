// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	// StorageDRSTimeout is the maximum time to wait for a Storage DRS recommendation.
	StorageDRSTimeout = 30 * time.Second

	// SelectionMethodDRS indicates the datastore was selected using Storage DRS.
	SelectionMethodDRS = "storage-drs"

	// SelectionMethodFallback indicates the datastore was selected using first-available fallback.
	SelectionMethodFallback = "first-available"
)

var _ DatastoreSelector = (*VCenterDriver)(nil)

// RequestStoragePlacement requests a placement recommendation from Storage DRS.
// It returns the placement result or an error if the request fails or times out.
func (d *VCenterDriver) RequestStoragePlacement(
	cluster types.ManagedObjectReference,
	vmSpec types.VirtualMachineConfigSpec,
	resourcePool *types.ManagedObjectReference,
) (*types.StoragePlacementResult, error) {
	ctx, cancel := context.WithTimeout(d.Ctx, StorageDRSTimeout)
	defer cancel()

	placementSpec := types.StoragePlacementSpec{
		Type:         string(types.StoragePlacementSpecPlacementTypeCreate),
		ConfigSpec:   &vmSpec,
		ResourcePool: resourcePool,
		PodSelectionSpec: types.StorageDrsPodSelectionSpec{
			StoragePod: &cluster,
		},
	}

	storageResourceManager := d.VimClient.ServiceContent.StorageResourceManager
	if storageResourceManager == nil {
		return nil, fmt.Errorf("storage resource manager not available")
	}

	req := types.RecommendDatastores{
		This:        *storageResourceManager,
		StorageSpec: placementSpec,
	}

	res, err := methods.RecommendDatastores(ctx, d.VimClient, &req)
	if err != nil {
		return nil, fmt.Errorf("error requesting storage placement: %s", err)
	}

	if len(res.Returnval.Recommendations) == 0 {
		return nil, fmt.Errorf("no storage placement recommendations returned")
	}

	return &res.Returnval, nil
}

// SelectDatastoresForDisks requests Storage DRS recommendations using the same
// storage device topology that clone and create apply at runtime.
func (d *VCenterDriver) SelectDatastoresForDisks(
	clusterName string,
	input StoragePlacementInput,
) ([]Datastore, string, error) {
	cluster, err := d.FindDatastoreCluster(clusterName)
	if err != nil {
		return nil, "", err
	}

	datastores, err := cluster.ListDatastores()
	if err != nil {
		return nil, "", err
	}

	if len(datastores) == 0 {
		return nil, "", fmt.Errorf("datastore cluster '%s' contains no available datastores", clusterName)
	}

	fallback := datastores[0]
	diskCount := len(input.StorageConfig.Storage)
	if diskCount == 0 {
		return nil, "", fmt.Errorf("no disks provided for storage placement")
	}

	deviceChanges, newDiskKeys, err := BuildStoragePlacementConfigSpec(input)
	if err != nil {
		return nil, "", err
	}

	vmSpec := types.VirtualMachineConfigSpec{
		Name:     fmt.Sprintf("packer-placement-request-%d", time.Now().UnixNano()),
		NumCPUs:  1,
		MemoryMB: 512,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", clusterName),
		},
		DeviceChange: deviceChanges,
	}

	resourcePoolRef := resourcePoolFromDatastores(d, datastores)

	placementResult, err := d.RequestStoragePlacement(cluster.Reference(), vmSpec, resourcePoolRef)
	if err == nil && placementResult != nil && len(placementResult.Recommendations) > 0 {
		recommendation := placementResult.Recommendations[0]
		if diskDatastores, ok := d.datastoresForNewDisks(recommendation, newDiskKeys, fallback); ok {
			return diskDatastores, SelectionMethodDRS, nil
		}
	}

	if err != nil {
		log.Printf("[WARN] Storage DRS failed for cluster '%s': %s. Using first-available fallback.", clusterName, err)
	}
	return duplicateDatastore(fallback, diskCount), SelectionMethodFallback, nil
}

func resourcePoolFromDatastores(d *VCenterDriver, datastores []Datastore) *types.ManagedObjectReference {
	if len(datastores) == 0 {
		return nil
	}

	dsInfo, err := datastores[0].Info("host")
	if err != nil || len(dsInfo.Host) == 0 {
		return nil
	}

	hostRef := dsInfo.Host[0].Key
	host := object.NewHostSystem(d.Client.Client, hostRef)
	hostInfo, err := host.ResourcePool(d.Ctx)
	if err != nil {
		return nil
	}

	ref := hostInfo.Reference()
	return &ref
}

func (d *VCenterDriver) resolveDatastoreRef(ref types.ManagedObjectReference) (Datastore, error) {
	datastoreObj := object.NewDatastore(d.Client.Client, ref)
	dsDriver := &DatastoreDriver{
		ds:     datastoreObj,
		driver: d,
	}
	info, err := dsDriver.Info("name")
	if err != nil {
		return dsDriver, nil
	}

	ds, err := d.Finder.Datastore(d.Ctx, info.Name)
	if err != nil {
		log.Printf("[WARN] Failed to find datastore '%s': %s. Using direct reference.", info.Name, err)
		return dsDriver, nil
	}

	return &DatastoreDriver{ds: ds, driver: d}, nil
}

func (d *VCenterDriver) datastoresForNewDisks(
	recommendation types.ClusterRecommendation,
	newDiskKeys []int32,
	fallback Datastore,
) ([]Datastore, bool) {
	if len(newDiskKeys) == 0 {
		return duplicateDatastore(fallback, 1), true
	}

	result := make([]Datastore, len(newDiskKeys))
	resolved := make([]bool, len(newDiskKeys))
	keyIndex := map[int32]int{}
	for i, key := range newDiskKeys {
		keyIndex[key] = i
	}

	for _, mapping := range diskDatastoreMappingsFromRecommendation(recommendation) {
		idx, ok := keyIndex[mapping.diskKey]
		if !ok {
			continue
		}
		ds, err := d.resolveDatastoreRef(mapping.ref)
		if err != nil {
			continue
		}
		result[idx] = ds
		resolved[idx] = true
	}

	unresolvedIdx := 0
	for _, destination := range destinationOnlyActions(recommendation) {
		ds, err := d.resolveDatastoreRef(destination)
		if err != nil {
			continue
		}

		if len(newDiskKeys) == 1 {
			result[0] = ds
			resolved[0] = true
			break
		}

		for unresolvedIdx < len(newDiskKeys) && resolved[unresolvedIdx] {
			unresolvedIdx++
		}
		if unresolvedIdx >= len(newDiskKeys) {
			break
		}
		result[unresolvedIdx] = ds
		resolved[unresolvedIdx] = true
		unresolvedIdx++
	}

	if !allResolved(resolved) {
		var first Datastore
		for i, ok := range resolved {
			if ok {
				first = result[i]
				break
			}
		}
		if first != nil {
			for i := range result {
				if !resolved[i] {
					result[i] = first
					resolved[i] = true
				}
			}
		}
	}

	if allResolved(resolved) {
		return result, true
	}

	return nil, false
}

func allResolved(resolved []bool) bool {
	for _, ok := range resolved {
		if !ok {
			return false
		}
	}
	return true
}

func duplicateDatastore(datastore Datastore, count int) []Datastore {
	result := make([]Datastore, count)
	for i := 0; i < count; i++ {
		result[i] = datastore
	}
	return result
}

// SelectDatastoreFromCluster selects a datastore from a cluster using Storage
// DRS. It attempts to get a Storage DRS recommendation and falls back to the
// first available datastore if Storage DRS fails or times out.
func (d *VCenterDriver) SelectDatastoreFromCluster(
	clusterName string,
) (Datastore, string, error) {
	cluster, err := d.FindDatastoreCluster(clusterName)
	if err != nil {
		return nil, "", err
	}

	datastores, err := cluster.ListDatastores()
	if err != nil {
		return nil, "", err
	}

	if len(datastores) == 0 {
		return nil, "", fmt.Errorf("datastore cluster '%s' contains no available datastores", clusterName)
	}

	vmSpec := types.VirtualMachineConfigSpec{
		Name:     fmt.Sprintf("packer-placement-request-%d", time.Now().UnixNano()),
		NumCPUs:  1,
		MemoryMB: 512,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", clusterName),
		},
	}

	resourcePoolRef := resourcePoolFromDatastores(d, datastores)

	placementResult, err := d.RequestStoragePlacement(cluster.Reference(), vmSpec, resourcePoolRef)
	if err == nil && placementResult != nil && len(placementResult.Recommendations) > 0 {
		recommendation := placementResult.Recommendations[0]

		if len(recommendation.Action) > 0 {
			for _, action := range recommendation.Action {
				if relocateAction, ok := action.(*types.StoragePlacementAction); ok {
					ds, err := d.resolveDatastoreRef(relocateAction.Destination)
					if err != nil || relocateAction.Destination.Type == "" {
						continue
					}
					log.Printf("[INFO] Storage DRS recommended datastore '%s' for cluster '%s'",
						ds.Name(), clusterName)
					return ds, SelectionMethodDRS, nil
				}
			}
		}
	}

	if err != nil {
		log.Printf("[WARN] Storage DRS failed for cluster '%s': %s. Using first-available fallback.", clusterName, err)
	}

	return datastores[0], SelectionMethodFallback, nil
}

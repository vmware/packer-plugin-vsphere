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

// SelectDatastoresForDisks requests Storage DRS recommendations for multiple
// disks at once. This allows Storage DRS to make optimal placement decisions.
// Returns a slice of datastores (one per disk), selection method, and any
// error.
func (d *VCenterDriver) SelectDatastoresForDisks(
	clusterName string,
	disks []Disk,
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

	// Create a virtual machine spec with multiple disks for Storage DRS to
	// evaluate for placement.
	vmSpec := types.VirtualMachineConfigSpec{
		Name:     fmt.Sprintf("packer-placement-request-%d", time.Now().UnixNano()),
		NumCPUs:  1,
		MemoryMB: 512,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", clusterName),
		},
	}

	// Add disk device specs to the virtual machine config using actual disk
	// configurations.
	deviceList := object.VirtualDeviceList{}
	controller, err := deviceList.CreateSCSIController("pvscsi")
	if err != nil {
		return nil, "", fmt.Errorf("error creating controller for DRS request: %s", err)
	}
	deviceList = append(deviceList, controller)

	for i, diskConfig := range disks {
		disk := &types.VirtualDisk{
			VirtualDevice: types.VirtualDevice{
				Key: int32(-100 - i),
				Backing: &types.VirtualDiskFlatVer2BackingInfo{
					DiskMode:        string(types.VirtualDiskModePersistent),
					ThinProvisioned: types.NewBool(diskConfig.DiskThinProvisioned),
					EagerlyScrub:    types.NewBool(diskConfig.DiskEagerlyScrub),
				},
			},
			CapacityInKB: diskConfig.DiskSize * 1024,
		}
		deviceList.AssignController(disk, controller.(types.BaseVirtualController))
		deviceList = append(deviceList, disk)
	}

	deviceSpecs, err := deviceList.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return nil, "", fmt.Errorf("error creating device specs for DRS request: %s", err)
	}
	vmSpec.DeviceChange = deviceSpecs

	// Get resource pool for the Storage DRS request.
	var resourcePoolRef *types.ManagedObjectReference
	if len(datastores) > 0 {
		dsInfo, err := datastores[0].Info("host")
		if err == nil && len(dsInfo.Host) > 0 {
			hostRef := dsInfo.Host[0].Key
			host := object.NewHostSystem(d.Client.Client, hostRef)
			hostInfo, err := host.ResourcePool(d.Ctx)
			if err == nil {
				ref := hostInfo.Reference()
				resourcePoolRef = &ref
			}
		}
	}

	placementResult, err := d.RequestStoragePlacement(cluster.Reference(), vmSpec, resourcePoolRef)
	if err == nil && placementResult != nil && len(placementResult.Recommendations) > 0 {
		recommendation := placementResult.Recommendations[0]

		if len(recommendation.Action) > 0 {
			// Storage DRS typically returns one action when all disks should go
			// to the same datastore.
			var recommendedDatastore Datastore

			for _, action := range recommendation.Action {
				if relocateAction, ok := action.(*types.StoragePlacementAction); ok {
					datastoreObj := object.NewDatastore(d.Client.Client, relocateAction.Destination)
					dsDriver := &DatastoreDriver{
						ds:     datastoreObj,
						driver: d,
					}
					info, err := dsDriver.Info("name")
					if err != nil {
						log.Printf("[WARN] Failed to get datastore name: %s", err)
						continue
					}

					ds, err := d.Finder.Datastore(d.Ctx, info.Name)
					if err != nil {
						log.Printf("[WARN] Failed to find datastore '%s': %s. Using direct reference.", info.Name, err)
						recommendedDatastore = dsDriver
					} else {
						recommendedDatastore = &DatastoreDriver{ds: ds, driver: d}
					}
					break
				}
			}

			if recommendedDatastore != nil {
				result := make([]Datastore, len(disks))
				for i := range len(disks) {
					result[i] = recommendedDatastore
				}
				return result, SelectionMethodDRS, nil
			}
		}
	}

	// Fallback: Return first available datastore for all disks.
	if err != nil {
		log.Printf("[WARN] Storage DRS failed for cluster '%s': %s. Using first-available fallback.", clusterName, err)
	}
	result := make([]Datastore, len(disks))
	for i := range len(disks) {
		result[i] = datastores[0]
	}
	return result, SelectionMethodFallback, nil
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

	// Create a minimal virtual machine spec for the Storage DRS placement
	// request. Use a timestamp to make each request unique for proper Storage
	// DRS evaluation.
	vmSpec := types.VirtualMachineConfigSpec{
		Name:     fmt.Sprintf("packer-placement-request-%d", time.Now().UnixNano()),
		NumCPUs:  1,
		MemoryMB: 512,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", clusterName),
		},
	}

	// Storage DRS requires a resource pool. Return one from the first
	// datastore's host.
	var resourcePoolRef *types.ManagedObjectReference
	if len(datastores) > 0 {
		dsInfo, err := datastores[0].Info("host")
		if err == nil && len(dsInfo.Host) > 0 {
			hostRef := dsInfo.Host[0].Key
			host := object.NewHostSystem(d.Client.Client, hostRef)
			hostInfo, err := host.ResourcePool(d.Ctx)
			if err == nil {
				ref := hostInfo.Reference()
				resourcePoolRef = &ref
			}
		}
	}

	placementResult, err := d.RequestStoragePlacement(cluster.Reference(), vmSpec, resourcePoolRef)
	if err == nil && placementResult != nil && len(placementResult.Recommendations) > 0 {
		recommendation := placementResult.Recommendations[0]

		if len(recommendation.Action) > 0 {
			for _, action := range recommendation.Action {
				if relocateAction, ok := action.(*types.StoragePlacementAction); ok {
					datastoreObj := object.NewDatastore(d.Client.Client, relocateAction.Destination)
					dsDriver := &DatastoreDriver{
						ds:     datastoreObj,
						driver: d,
					}
					info, err := dsDriver.Info("name")
					if err != nil {
						log.Printf("[WARN] Failed to get datastore name: %s", err)
						continue
					}
					log.Printf("[INFO] Storage DRS recommended datastore '%s' for cluster '%s'",
						info.Name, clusterName)

					ds, err := d.Finder.Datastore(d.Ctx, info.Name)
					if err != nil {
						log.Printf("[WARN] Failed to find datastore '%s': %s. Using direct reference.", info.Name, err)
						return dsDriver, SelectionMethodDRS, nil
					}

					return &DatastoreDriver{
						ds:     ds,
						driver: d,
					}, SelectionMethodDRS, nil
				}
			}
		}
	}

	if err != nil {
		log.Printf("[WARN] Storage DRS failed for cluster '%s': %s. Using first-available fallback.", clusterName, err)
	}

	return datastores[0], SelectionMethodFallback, nil
}

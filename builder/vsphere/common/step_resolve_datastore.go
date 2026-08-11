// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// DatastoreClusterDriver is an interface for drivers that support datastore cluster operations.
type DatastoreClusterDriver interface {
	driver.Driver
	SelectDatastoreFromCluster(clusterName string) (driver.Datastore, string, error)
}

// StepResolveDatastore resolves a datastore from either a direct datastore
// name or a datastore cluster name. When a datastore cluster is specified,
// it uses Storage DRS to select the optimal datastore. When neither is set
// but a storage policy is provided, PBM selects a compliant datastore.
type StepResolveDatastore struct {
	// Datastore is the name of a specific datastore to use.
	Datastore string
	// DatastoreCluster is the name of a datastore cluster to use.
	// When specified, Storage DRS will select the optimal datastore.
	DatastoreCluster string
	// StoragePolicy is an optional storage policy name used for PBM placement
	// when Datastore and DatastoreCluster are both empty.
	StoragePolicy string
	// Host scopes PBM candidate datastores when Cluster is empty.
	Host string
	// Cluster scopes PBM candidate datastores when set.
	Cluster string
	// DiskCount is the number of disks that will be created.
	// When using a datastore cluster with multiple disks, this step will be skipped
	// to avoid redundant DRS calls (per-disk DRS calls will be made later).
	DiskCount int
}

// Run resolves a datastore from either a direct datastore name or a datastore cluster.
// When using a datastore cluster, Storage DRS selects the optimal datastore.
// For multi-disk configurations with datastore clusters, this initial selection is used
// for non-disk operations (ISO uploads, etc.), while per-disk DRS calls are made separately.
// When only StoragePolicy is set, PBM selects a compatible datastore.
func (s *StepResolveDatastore) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	d := state.Get("driver").(driver.Driver)

	var ds driver.Datastore
	var err error
	var selectionMethod string

	switch {
	case s.Datastore != "":
		log.Printf("[INFO] Using datastore '%s'", s.Datastore)
		ds, err = d.FindDatastore(s.Datastore, "")
		if err != nil {
			state.Put("error", fmt.Errorf("error finding datastore '%s': %s", s.Datastore, err))
			return multistep.ActionHalt
		}
		selectionMethod = "direct"

	case s.DatastoreCluster != "":
		clusterDriver, ok := d.(DatastoreClusterDriver)
		if !ok {
			state.Put("error", fmt.Errorf("driver does not support datastore cluster operations"))
			return multistep.ActionHalt
		}

		ds, selectionMethod, err = clusterDriver.SelectDatastoreFromCluster(s.DatastoreCluster)
		if err != nil {
			state.Put("error", fmt.Errorf("error resolving datastore from cluster '%s': %s", s.DatastoreCluster, err))
			return multistep.ActionHalt
		}

		if s.DiskCount > 1 {
			log.Printf("[INFO] Selected datastore '%s' from cluster '%s' for non-disk operations (per-disk placement will be requested separately)", ds.Name(), s.DatastoreCluster)
		} else {
			if selectionMethod == driver.SelectionMethodDRS {
				log.Printf("[INFO] Storage DRS selected datastore '%s' from cluster '%s'", ds.Name(), s.DatastoreCluster)
			} else {
				log.Printf("[INFO] Selected datastore '%s' from cluster '%s' (first available)", ds.Name(), s.DatastoreCluster)
			}
		}

	case s.StoragePolicy != "":
		policyID, err := d.FindStoragePolicyID(s.StoragePolicy)
		if err != nil {
			state.Put("error", fmt.Errorf("error resolving storage policy %q: %v", s.StoragePolicy, err))
			return multistep.ActionHalt
		}

		ds, err = d.FindCompatibleDatastore(policyID, s.Host, s.Cluster)
		if err != nil {
			state.Put("error", fmt.Errorf("error resolving datastore for storage policy %q: %v", s.StoragePolicy, err))
			return multistep.ActionHalt
		}
		selectionMethod = driver.SelectionMethodStoragePolicy
		state.Put("storage_policy_id", policyID)
		log.Printf("[INFO] Storage policy %q selected datastore '%s'", s.StoragePolicy, ds.Name())

	default:
		return multistep.ActionContinue
	}

	state.Put("datastore", ds)
	state.Put("datastore_selection_method", selectionMethod)

	return multistep.ActionContinue
}

// Cleanup performs any necessary cleanup.
func (s *StepResolveDatastore) Cleanup(state multistep.StateBag) {}

// ResolveMultiDiskDatastorePlacement requests per-disk Storage DRS placement when a
// datastore cluster is configured with multiple disks. It is the companion to
// StepResolveDatastore for VM create/clone steps that attach more than one disk.
func ResolveMultiDiskDatastorePlacement(
	ui packersdk.Ui,
	d driver.Driver,
	datastoreCluster string,
	input driver.StoragePlacementInput,
	primaryDatastore driver.Datastore,
	datastoreName string,
) (string, []driver.DiskDatastore) {
	if datastoreCluster == "" || len(input.StorageConfig.Storage) <= 1 {
		return datastoreName, nil
	}

	dsSelector, ok := d.(driver.DatastoreSelector)
	if !ok {
		return datastoreName, nil
	}

	ui.Sayf("Requesting Storage DRS recommendations for %d disks...", len(input.StorageConfig.Storage))

	diskDatastores, method, err := dsSelector.SelectDatastoresForDisks(datastoreCluster, input)
	if err != nil {
		ui.Sayf("Warning: Failed to get Storage DRS recommendations: %s. Using primary datastore.", err)
		if primaryDatastore == nil {
			return datastoreName, nil
		}

		placements := make([]driver.DiskDatastore, 0, len(input.StorageConfig.Storage))
		for i := 0; i < len(input.StorageConfig.Storage); i++ {
			placements = append(placements, driver.DiskDatastoreFrom(primaryDatastore))
		}
		return datastoreName, placements
	}

	if len(diskDatastores) > 0 {
		datastoreName = diskDatastores[0].Name()
	}

	placements := make([]driver.DiskDatastore, 0, len(diskDatastores))
	for i, ds := range diskDatastores {
		if method == driver.SelectionMethodDRS {
			log.Printf("[INFO] Disk %d: Storage DRS selected datastore '%s'", i+1, ds.Name())
		} else {
			log.Printf("[INFO] Disk %d: Using first available datastore '%s'", i+1, ds.Name())
		}
		placements = append(placements, driver.DiskDatastoreFrom(ds))
	}

	return datastoreName, placements
}

// ResolveStoragePolicyDatastorePlacement selects a PBM-compatible datastore for
// each disk that has a StoragePolicyID. Disks without a policy use primaryDatastore.
// Results are cached by policy ID so repeated policies share one PBM lookup.
//
// When seedPolicyID is set (from StepResolveDatastore) and primaryDatastore is
// non-nil, that pairing seeds the cache so the first policy is not looked up twice.
//
// Skipped when an explicit datastore or datastoreCluster is configured (those
// own placement). When both are omitted, StepResolveDatastore places VM home
// from the first policy and this helper places each disk from its own policy.
func ResolveStoragePolicyDatastorePlacement(
	d driver.Driver,
	host, cluster string,
	disks []driver.Disk,
	primaryDatastore driver.Datastore,
	datastoreName, datastore, datastoreCluster, seedPolicyID string,
) (string, []driver.DiskDatastore, error) {
	if datastore != "" || datastoreCluster != "" {
		return datastoreName, nil, nil
	}

	hasPolicy := false
	for _, disk := range disks {
		if disk.StoragePolicyID != "" {
			hasPolicy = true
			break
		}
	}
	if !hasPolicy {
		return datastoreName, nil, nil
	}

	log.Printf("[INFO] Resolving datastores for %d disks via storage policies...", len(disks))

	cache := make(map[string]driver.Datastore)
	if seedPolicyID != "" && primaryDatastore != nil {
		cache[seedPolicyID] = primaryDatastore
	}
	placements := make([]driver.DiskDatastore, 0, len(disks))

	for i, disk := range disks {
		var ds driver.Datastore

		if disk.StoragePolicyID == "" {
			if primaryDatastore == nil {
				return "", nil, fmt.Errorf("disk %d has no storage_policy and no primary datastore is available for placement", i+1)
			}
			ds = primaryDatastore
			log.Printf("[INFO] Disk %d: no storage_policy; using primary datastore '%s'", i+1, ds.Name())
		} else if cached, ok := cache[disk.StoragePolicyID]; ok {
			ds = cached
			log.Printf("[INFO] Disk %d: reusing datastore '%s' for storage policy", i+1, ds.Name())
		} else {
			var err error
			ds, err = d.FindCompatibleDatastore(disk.StoragePolicyID, host, cluster)
			if err != nil {
				return "", nil, fmt.Errorf("error resolving datastore for disk %d storage policy profile %q: %v", i+1, disk.StoragePolicyID, err)
			}
			cache[disk.StoragePolicyID] = ds
			log.Printf("[INFO] Disk %d: storage policy selected datastore '%s'", i+1, ds.Name())
		}

		placements = append(placements, driver.DiskDatastoreFrom(ds))
	}

	if datastoreName == "" {
		if primaryDatastore != nil {
			datastoreName = primaryDatastore.Name()
		} else if len(placements) > 0 {
			datastoreName = placements[0].Name
		}
	}

	return datastoreName, placements, nil
}

// StoragePolicyIDFromState returns the profile UUID stored by StepResolveDatastore
// when PBM selected the primary datastore, or empty if unset.
func StoragePolicyIDFromState(state multistep.StateBag) string {
	id, _ := state.Get("storage_policy_id").(string)
	return id
}

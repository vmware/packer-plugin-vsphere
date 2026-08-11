// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"strings"

	"github.com/vmware/govmomi/pbm"
	pbmt "github.com/vmware/govmomi/pbm/types"
	"github.com/vmware/govmomi/vim25/types"
)

// SelectionMethodStoragePolicy indicates the datastore was selected via PBM
// compatibility with a storage policy.
const SelectionMethodStoragePolicy = "storage_policy"

func (d *VCenterDriver) ensurePBMClient() (*pbm.Client, error) {
	if d.pbmClient == nil {
		c, err := pbm.NewClient(d.Ctx, d.VimClient)
		if err != nil {
			return nil, fmt.Errorf("error initializing PBM client: %v", err)
		}
		d.pbmClient = c
	}
	return d.pbmClient, nil
}

// FindCompatibleDatastore finds a datastore compatible with the given storage
// policy profile UUID using the PBM PlacementSolver. Candidate hubs are scoped
// to the configured cluster when set, otherwise to the host's datastores.
// When multiple datastores are compatible, the first is selected.
func (d *VCenterDriver) FindCompatibleDatastore(policyID, host, cluster string) (Datastore, error) {
	if policyID == "" {
		return nil, fmt.Errorf("storage policy ID is required")
	}

	pc, err := d.ensurePBMClient()
	if err != nil {
		return nil, err
	}

	hubs, nameByHubID, err := d.placementHubs(pc, host, cluster)
	if err != nil {
		return nil, err
	}
	if len(hubs) == 0 {
		return nil, fmt.Errorf("no candidate datastores found for storage policy placement")
	}

	req := []pbmt.BasePbmPlacementRequirement{
		&pbmt.PbmPlacementCapabilityProfileRequirement{
			ProfileId: pbmt.PbmProfileId{UniqueId: policyID},
		},
	}

	res, err := pc.CheckRequirements(d.Ctx, hubs, nil, req)
	if err != nil {
		return nil, fmt.Errorf("error checking storage policy compatibility: %v", err)
	}

	compatible := res.CompatibleDatastores()
	if len(compatible) == 0 {
		return nil, fmt.Errorf("no datastore compatible with storage policy profile %q", policyID)
	}

	hub := compatible[0]
	name := nameByHubID[hub.HubId]
	if name == "" {
		name, err = d.GetDatastoreName(hub.HubId)
		if err != nil {
			return nil, fmt.Errorf("error resolving compatible datastore name: %v", err)
		}
	}

	return d.FindDatastore(name, host)
}

// placementHubs builds PBM placement hubs scoped to the cluster or host.
func (d *VCenterDriver) placementHubs(pc *pbm.Client, host, cluster string) ([]pbmt.PbmPlacementHub, map[string]string, error) {
	if cluster != "" {
		c, err := d.FindCluster(cluster)
		if err != nil {
			return nil, nil, fmt.Errorf("error finding cluster %q for storage policy placement: %v", cluster, err)
		}
		dsMap, err := pc.DatastoreMap(d.Ctx, d.VimClient, c.cluster.Reference())
		if err != nil {
			return nil, nil, fmt.Errorf("error listing cluster datastores for storage policy placement: %v", err)
		}
		return dsMap.PlacementHub, dsMap.Name, nil
	}

	if host == "" {
		return nil, nil, fmt.Errorf("host or cluster is required for storage policy placement")
	}

	h, err := d.FindHost(host)
	if err != nil {
		return nil, nil, fmt.Errorf("error finding host %q for storage policy placement: %v", host, err)
	}
	info, err := h.Info("datastore")
	if err != nil {
		return nil, nil, fmt.Errorf("error listing host datastores for storage policy placement: %v", err)
	}

	nameByHubID := make(map[string]string, len(info.Datastore))
	hubs := make([]pbmt.PbmPlacementHub, 0, len(info.Datastore))
	for i := range info.Datastore {
		ref := info.Datastore[i]
		hubs = append(hubs, pbmt.PbmPlacementHub{
			HubType: ref.Type,
			HubId:   ref.Value,
		})
		ds := d.NewDatastore(&ref)
		inf, err := ds.Info("name")
		if err != nil {
			return nil, nil, fmt.Errorf("error reading datastore name for storage policy placement: %v", err)
		}
		nameByHubID[ref.Value] = inf.Name
	}

	return hubs, nameByHubID, nil
}

// DiskStoragePlacement describes the storage policy and datastore backing for
// one virtual disk, in device order.
type DiskStoragePlacement struct {
	PolicyName    string
	DatastoreName string
}

// DiskStoragePlacements returns the attached storage policy name and datastore
// name for each virtual disk on virtual machine, in device order. Disks without
// associated storage policy report an empty PolicyName.
func (d *VCenterDriver) DiskStoragePlacements(vm VirtualMachine) ([]DiskStoragePlacement, error) {
	pc, err := d.ensurePBMClient()
	if err != nil {
		return nil, err
	}

	devices, err := vm.Devices()
	if err != nil {
		return nil, fmt.Errorf("error reading VM devices: %v", err)
	}

	vmRef := vm.Reference().Value
	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	out := make([]DiskStoragePlacement, 0, len(disks))

	for _, device := range disks {
		disk := device.(*types.VirtualDisk)
		placement := DiskStoragePlacement{}

		dsName, err := diskDatastoreName(d, disk)
		if err != nil {
			return nil, err
		}
		placement.DatastoreName = dsName

		entity := pbmt.PbmServerObjectRef{
			ObjectType: string(pbmt.PbmObjectTypeVirtualDiskId),
			Key:        fmt.Sprintf("%s:%d", vmRef, disk.Key),
		}
		ids, err := pc.QueryAssociatedProfile(d.Ctx, entity)
		if err != nil {
			return nil, fmt.Errorf("error querying storage policy for disk key %d: %v", disk.Key, err)
		}
		if len(ids) > 0 {
			name, err := pc.GetProfileNameByID(d.Ctx, ids[0].UniqueId)
			if err != nil {
				return nil, fmt.Errorf("error resolving storage policy name for disk key %d: %v", disk.Key, err)
			}
			placement.PolicyName = name
		}

		out = append(out, placement)
	}

	return out, nil
}

func diskDatastoreName(d *VCenterDriver, disk *types.VirtualDisk) (string, error) {
	var dsRef *types.ManagedObjectReference
	var fileName string

	switch b := disk.Backing.(type) {
	case *types.VirtualDiskFlatVer2BackingInfo:
		dsRef = b.Datastore
		fileName = b.FileName
	case *types.VirtualDiskSparseVer2BackingInfo:
		dsRef = b.Datastore
		fileName = b.FileName
	case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
		dsRef = b.Datastore
		fileName = b.FileName
	case *types.VirtualDiskSeSparseBackingInfo:
		dsRef = b.Datastore
		fileName = b.FileName
	default:
		return "", fmt.Errorf("unsupported disk backing type %T", disk.Backing)
	}

	if dsRef != nil {
		ds := d.NewDatastore(dsRef)
		info, err := ds.Info("name")
		if err != nil {
			return "", fmt.Errorf("error reading disk datastore name: %v", err)
		}
		return info.Name, nil
	}

	// Fall back to parsing "[datastore] path" from FileName.
	if len(fileName) > 2 && fileName[0] == '[' {
		if end := strings.IndexByte(fileName, ']'); end > 1 {
			return fileName[1:end], nil
		}
	}
	return "", fmt.Errorf("disk has no datastore backing")
}

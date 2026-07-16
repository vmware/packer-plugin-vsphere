// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	_ "github.com/vmware/govmomi/vapi/simulator"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
)

type Tag struct {
	Category string
	Name     string
}

type SimulatedVmConfig struct {
	Name         string
	Tags         []Tag
	Template     bool
	CreationTime time.Time
}

// SimulatedDatastoreConfig configures an existing simulator datastore by
// inventory index (order returned by DatastoreList "*").
type SimulatedDatastoreConfig struct {
	Name      string
	Capacity  *int64 // bytes; nil leaves the simulator default
	FreeSpace *int64 // bytes; nil leaves the simulator default
	Tags      []Tag
}

// SimulatedDatastoreClusterConfig configures an existing simulator storage pod
// (datastore cluster) by DatastoreClusterList "*" order. MemberIndexes select
// datastores by DatastoreList "*" order to move into the pod.
type SimulatedDatastoreClusterConfig struct {
	Name          string
	MemberIndexes []int
	Tags          []Tag
}

// SimulatedHostConfig configures an existing simulator host by HostSystemList
// "*" order. MemoryCapacity is bytes; MemoryUsageMB is quickStats usage in MB.
type SimulatedHostConfig struct {
	Name           string
	MemoryCapacity *int64 // bytes; nil leaves the simulator default
	MemoryUsageMB  *int32 // MB; nil leaves the simulator default
	Tags           []Tag
}

// SimulatedComputeClusterConfig configures an existing simulator compute
// cluster by ClusterComputeResourceList "*" order.
type SimulatedComputeClusterConfig struct {
	Name string
	Tags []Tag
}

// SimulatedResourcePoolConfig creates a nested resource pool under a compute
// cluster root. Path is relative to the cluster Resources pool
// (e.g. "rp-production" or "rp-production/rp-linux"). ClusterIndex selects
// ClusterComputeResourceList "*" order.
type SimulatedResourcePoolConfig struct {
	Path         string
	ClusterIndex int
	Tags         []Tag
}

type simulatorContext struct {
	Model      *simulator.Model
	Server     *simulator.Server
	Ctx        context.Context
	Client     *govmomi.Client
	RestClient *rest.Client
	Finder     *find.Finder
	Datacenter *object.Datacenter
}

// NewSimulator creates a new vCenter simulator with the specified model.
func NewSimulator(model *simulator.Model) (*simulatorContext, error) {
	ctx := context.Background()
	if model == nil {
		return nil, fmt.Errorf("model has not been initialized")
	}

	err := model.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create simulator model: %w", err)
	}
	model.Service.RegisterEndpoints = true
	model.Service.TLS = new(tls.Config)

	server := model.Service.NewServer()

	u, err := url.Parse(server.URL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse simulator URL: %w", err)
	}
	password, _ := simulator.DefaultLogin.Password()
	u.User = url.UserPassword(simulator.DefaultLogin.Username(), password)

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SOAP simulator: %w", err)
	}

	restClient := rest.NewClient(client.Client)
	err = restClient.Login(ctx, simulator.DefaultLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to login to REST simulator: %w", err)
	}

	finder := find.NewFinder(client.Client, false)
	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list datacenters: %w", err)
	}
	if len(dcs) == 0 {
		return nil, fmt.Errorf("datacenters were not found in the simulator: %w", err)
	}
	finder.SetDatacenter(dcs[0])

	return &simulatorContext{
		Ctx:        ctx,
		Server:     server,
		Model:      model,
		Client:     client,
		Finder:     finder,
		RestClient: restClient,
		Datacenter: dcs[0],
	}, nil
}

func (sim *simulatorContext) Stop() {
	if sim.Model != nil {
		sim.Model.Remove()
	}
	if sim.Server != nil {
		sim.Server.Close()
	}
}

// ApplyVmConfiguration applies virtual machines in the simulator according to the provided configurations.
func (sim *simulatorContext) ApplyVmConfiguration(vmsConfig []SimulatedVmConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	vms, err := sim.Finder.VirtualMachineList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list virtual machines in cluster: %w", err)
	}

	for i := 0; i < len(vmsConfig); i++ {
		vmConfig := types.VirtualMachineConfigSpec{
			Name: vmsConfig[i].Name,
		}

		if !vmsConfig[i].CreationTime.IsZero() {
			vmConfig.CreateDate = &vmsConfig[i].CreationTime
		}

		if vmsConfig[i].Name != "" {
			task, err := vms[i].Reconfigure(sim.Ctx, vmConfig)
			if err != nil {
				return fmt.Errorf("failed to issue rename of virtual machine command: %w", err)
			}
			if err = task.Wait(sim.Ctx); err != nil {
				return fmt.Errorf("failed to rename virtual machine: %w", err)
			}
		}

		if vmsConfig[i].Template {
			err = markSimulatedVmAsTemplate(sim.Ctx, vms[i])
			if err != nil {
				return fmt.Errorf("failed to convert to templates: %w", err)
			}
		}

		if vmsConfig[i].Tags != nil {
			for _, tag := range vmsConfig[i].Tags {
				catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
				if err != nil {
					return fmt.Errorf("failed to ensure category exists: %w", err)
				}
				tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
				if err != nil {
					return fmt.Errorf("failed to ensure tag exists: %w", err)
				}
				err = tagMan.AttachTag(sim.Ctx, tagID, vms[i].Reference())
				if err != nil {
					return fmt.Errorf("failed to attach tag to virtual machine: %w", err)
				}
			}
		}
	}

	return nil
}

// ApplyDatastoreConfiguration updates existing simulator datastores according
// to the provided configurations, matched by Finder.DatastoreList order.
// Capacity and FreeSpace mutations update Summary and Info; do not call
// simulator RefreshDatastore afterward or those values are reset.
func (sim *simulatorContext) ApplyDatastoreConfiguration(dsConfigs []SimulatedDatastoreConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	datastores, err := sim.Finder.DatastoreList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list datastores in simulator: %w", err)
	}
	if len(dsConfigs) > len(datastores) {
		return fmt.Errorf("requested %d datastore configurations but simulator has %d", len(dsConfigs), len(datastores))
	}

	for i, cfg := range dsConfigs {
		ref := datastores[i].Reference()
		simDs, ok := sim.Model.Map().Get(ref).(*simulator.Datastore)
		if !ok || simDs == nil {
			return fmt.Errorf("failed to resolve simulator datastore for %s", ref.Value)
		}

		if cfg.Name != "" {
			simDs.Name = cfg.Name
			simDs.Summary.Name = cfg.Name
		}
		if cfg.Capacity != nil {
			simDs.Summary.Capacity = *cfg.Capacity
		}
		if cfg.FreeSpace != nil {
			simDs.Summary.FreeSpace = *cfg.FreeSpace
			if info := simDs.Info.GetDatastoreInfo(); info != nil {
				info.FreeSpace = *cfg.FreeSpace
			}
		}

		for _, tag := range cfg.Tags {
			catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
			if err != nil {
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure tag exists: %w", err)
			}
			if err := tagMan.AttachTag(sim.Ctx, tagID, ref); err != nil {
				return fmt.Errorf("failed to attach tag to datastore: %w", err)
			}
		}
	}

	return nil
}

// ApplyDatastoreClusterConfiguration renames simulator storage pods, moves
// selected datastores into them, and attaches tags. MemberIndexes refer to the
// DatastoreList "*" order used by ApplyDatastoreConfiguration.
func (sim *simulatorContext) ApplyDatastoreClusterConfiguration(clusterConfigs []SimulatedDatastoreClusterConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	clusters, err := sim.Finder.DatastoreClusterList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list datastore clusters in simulator: %w", err)
	}
	if len(clusterConfigs) > len(clusters) {
		return fmt.Errorf("requested %d datastore cluster configurations but simulator has %d", len(clusterConfigs), len(clusters))
	}

	datastores, err := sim.Finder.DatastoreList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list datastores in simulator: %w", err)
	}

	for i, cfg := range clusterConfigs {
		cluster := clusters[i]
		ref := cluster.Reference()
		simPod, ok := sim.Model.Map().Get(ref).(*simulator.StoragePod)
		if !ok || simPod == nil {
			return fmt.Errorf("failed to resolve simulator storage pod for %s", ref.Value)
		}

		if cfg.Name != "" {
			simPod.Name = cfg.Name
			if simPod.Summary != nil {
				simPod.Summary.Name = cfg.Name
			}
		}

		if len(cfg.MemberIndexes) > 0 {
			members := make([]types.ManagedObjectReference, 0, len(cfg.MemberIndexes))
			for _, idx := range cfg.MemberIndexes {
				if idx < 0 || idx >= len(datastores) {
					return fmt.Errorf("datastore member index %d out of range (have %d)", idx, len(datastores))
				}
				members = append(members, datastores[idx].Reference())
			}
			task, err := cluster.MoveInto(sim.Ctx, members)
			if err != nil {
				return fmt.Errorf("failed to move datastores into cluster %s: %w", cluster.Name(), err)
			}
			if err := task.Wait(sim.Ctx); err != nil {
				return fmt.Errorf("failed waiting to move datastores into cluster %s: %w", cluster.Name(), err)
			}
		}

		for _, tag := range cfg.Tags {
			catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
			if err != nil {
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure tag exists: %w", err)
			}
			if err := tagMan.AttachTag(sim.Ctx, tagID, ref); err != nil {
				return fmt.Errorf("failed to attach tag to datastore cluster: %w", err)
			}
		}
	}

	return nil
}

// ApplyHostConfiguration updates existing simulator hosts according to the
// provided configurations, matched by Finder.HostSystemList order.
func (sim *simulatorContext) ApplyHostConfiguration(hostConfigs []SimulatedHostConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	hosts, err := sim.Finder.HostSystemList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list hosts in simulator: %w", err)
	}
	if len(hostConfigs) > len(hosts) {
		return fmt.Errorf("requested %d host configurations but simulator has %d", len(hostConfigs), len(hosts))
	}

	for i, cfg := range hostConfigs {
		ref := hosts[i].Reference()
		simHost, ok := sim.Model.Map().Get(ref).(*simulator.HostSystem)
		if !ok || simHost == nil {
			return fmt.Errorf("failed to resolve simulator host for %s", ref.Value)
		}

		if cfg.Name != "" {
			simHost.Name = cfg.Name
			simHost.Summary.Config.Name = cfg.Name
		}
		if cfg.MemoryCapacity != nil {
			if simHost.Summary.Hardware == nil {
				simHost.Summary.Hardware = &types.HostHardwareSummary{}
			}
			simHost.Summary.Hardware.MemorySize = *cfg.MemoryCapacity
		}
		if cfg.MemoryUsageMB != nil {
			simHost.Summary.QuickStats.OverallMemoryUsage = *cfg.MemoryUsageMB
		}

		for _, tag := range cfg.Tags {
			catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
			if err != nil {
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure tag exists: %w", err)
			}
			if err := tagMan.AttachTag(sim.Ctx, tagID, ref); err != nil {
				return fmt.Errorf("failed to attach tag to host: %w", err)
			}
		}
	}

	return nil
}

// ApplyComputeClusterConfiguration updates existing simulator compute clusters
// according to the provided configurations, matched by
// Finder.ClusterComputeResourceList order.
func (sim *simulatorContext) ApplyComputeClusterConfiguration(clusterConfigs []SimulatedComputeClusterConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	clusters, err := sim.Finder.ClusterComputeResourceList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list compute clusters in simulator: %w", err)
	}
	if len(clusterConfigs) > len(clusters) {
		return fmt.Errorf("requested %d compute cluster configurations but simulator has %d", len(clusterConfigs), len(clusters))
	}

	for i, cfg := range clusterConfigs {
		ref := clusters[i].Reference()
		simCluster, ok := sim.Model.Map().Get(ref).(*simulator.ClusterComputeResource)
		if !ok || simCluster == nil {
			return fmt.Errorf("failed to resolve simulator compute cluster for %s", ref.Value)
		}

		if cfg.Name != "" {
			simCluster.Name = cfg.Name
		}

		for _, tag := range cfg.Tags {
			catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
			if err != nil {
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure tag exists: %w", err)
			}
			if err := tagMan.AttachTag(sim.Ctx, tagID, ref); err != nil {
				return fmt.Errorf("failed to attach tag to compute cluster: %w", err)
			}
		}
	}

	return nil
}

// ApplyResourcePoolConfiguration creates nested resource pools under compute
// cluster roots and attaches tags.
func (sim *simulatorContext) ApplyResourcePoolConfiguration(poolConfigs []SimulatedResourcePoolConfig) error {
	tagMan := tags.NewManager(sim.RestClient)

	clusters, err := sim.Finder.ClusterComputeResourceList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list compute clusters in simulator: %w", err)
	}

	for _, cfg := range poolConfigs {
		if cfg.Path == "" {
			return fmt.Errorf("resource pool path is required")
		}
		if cfg.ClusterIndex < 0 || cfg.ClusterIndex >= len(clusters) {
			return fmt.Errorf("cluster index %d out of range (have %d)", cfg.ClusterIndex, len(clusters))
		}

		root, err := clusters[cfg.ClusterIndex].ResourcePool(sim.Ctx)
		if err != nil {
			return fmt.Errorf("failed to get root resource pool for cluster index %d: %w", cfg.ClusterIndex, err)
		}

		parent := root
		segments := strings.Split(cfg.Path, "/")
		var current *object.ResourcePool
		for _, segment := range segments {
			if segment == "" || segment == "." {
				return fmt.Errorf("invalid resource pool path %q", cfg.Path)
			}
			current, err = ensureChildResourcePool(sim, parent, segment)
			if err != nil {
				return err
			}
			parent = current
		}

		ref := current.Reference()
		for _, tag := range cfg.Tags {
			catID, err := ensureCategory(sim.Ctx, tagMan, tag.Category)
			if err != nil {
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			tagID, err := ensureTag(sim.Ctx, tagMan, catID, tag.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure tag exists: %w", err)
			}
			if err := tagMan.AttachTag(sim.Ctx, tagID, ref); err != nil {
				return fmt.Errorf("failed to attach tag to resource pool: %w", err)
			}
		}
	}

	return nil
}

func ensureChildResourcePool(sim *simulatorContext, parent *object.ResourcePool, name string) (*object.ResourcePool, error) {
	parentPath := parent.InventoryPath
	if parentPath == "" {
		element, err := sim.Finder.Element(sim.Ctx, parent.Reference())
		if err != nil {
			return nil, fmt.Errorf("failed to resolve inventory path for parent resource pool: %w", err)
		}
		parentPath = element.Path
	}

	searchPath := parentPath + "/" + name
	existing, err := sim.Finder.ResourcePool(sim.Ctx, searchPath)
	if err == nil {
		return existing, nil
	}

	created, err := parent.Create(sim.Ctx, name, types.DefaultResourceConfigSpec())
	if err != nil {
		return nil, fmt.Errorf("failed to create resource pool %q under %s: %w", name, parentPath, err)
	}
	created.InventoryPath = searchPath
	return created, nil
}

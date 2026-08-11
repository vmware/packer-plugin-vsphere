// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vcsim

import (
	"bytes"
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
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	_ "github.com/vmware/govmomi/vapi/simulator"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
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

// SimulatedNetworkConfig configures an existing simulator network by supported
// NetworkList order (Network, DistributedVirtualPortgroup, OpaqueNetwork —
// DistributedVirtualSwitch entries are skipped).
type SimulatedNetworkConfig struct {
	Name string
	Tags []Tag
}

// SimulatedContentLibraryConfig creates a local content library backed by the
// datastore at DatastoreIndex (DatastoreList "*" order) and attaches tags.
type SimulatedContentLibraryConfig struct {
	Name           string
	DatastoreIndex int
	Tags           []Tag
}

// SimulatedContentLibraryItemConfig creates a content library item in an
// existing content library (matched by Library name) and optionally uploads
// files so the item resolves to a content library path. Items are created in
// slice order, giving later items a more recent last modified time.
type SimulatedContentLibraryItemConfig struct {
	Library string
	Name    string
	Type    string
	Files   []string
	Tags    []Tag
}

// Simulator is a running govmomi vcsim instance with SOAP and REST clients.
type Simulator struct {
	Model      *simulator.Model
	Server     *simulator.Server
	Ctx        context.Context
	Client     *govmomi.Client
	RestClient *rest.Client
	Finder     *find.Finder
	Datacenter *object.Datacenter

	// restLoginErr is set when CIS REST login fails (common on ESX-only models).
	restLoginErr error
}

// NewSimulator creates a new vCenter simulator with the specified model.
func NewSimulator(model *simulator.Model) (*Simulator, error) {
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
	// REST is available on VPX models; ESX-only models may not expose the CIS session API.
	var restLoginErr error
	if err = restClient.Login(ctx, simulator.DefaultLogin); err != nil {
		restLoginErr = err
		restClient = nil
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

	return &Simulator{
		Ctx:          ctx,
		Server:       server,
		Model:        model,
		Client:       client,
		Finder:       finder,
		RestClient:   restClient,
		Datacenter:   dcs[0],
		restLoginErr: restLoginErr,
	}, nil
}

// NewVPXSimulator creates a VPX simulator with a single pre-created VM.
// Matches the former driver/common NewVCenterSimulator helper.
func NewVPXSimulator() (*Simulator, error) {
	model := simulator.VPX()
	model.Machine = 1
	return NewSimulator(model)
}

func (sim *Simulator) Stop() {
	if sim.Model != nil {
		sim.Model.Remove()
	}
	if sim.Server != nil {
		sim.Server.Close()
	}
}

// requireREST ensures the CIS REST session is available (VPX models).
// ESX-only models may leave RestClient nil; the original login error is preserved.
func (sim *Simulator) requireREST() error {
	if sim.RestClient != nil {
		return nil
	}
	if sim.restLoginErr != nil {
		return fmt.Errorf("REST client is unavailable; use a VPX simulator model for tags and content library APIs: %w", sim.restLoginErr)
	}
	return fmt.Errorf("REST client is unavailable; use a VPX simulator model for tags and content library APIs")
}

// ApplyVmConfiguration applies virtual machines in the simulator according to the provided configurations.
func (sim *Simulator) ApplyVmConfiguration(vmsConfig []SimulatedVmConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
	tagMan := tags.NewManager(sim.RestClient)

	vms, err := sim.Finder.VirtualMachineList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list virtual machines in cluster: %w", err)
	}

	for i := range vmsConfig {
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
func (sim *Simulator) ApplyDatastoreConfiguration(dsConfigs []SimulatedDatastoreConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
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
func (sim *Simulator) ApplyDatastoreClusterConfiguration(clusterConfigs []SimulatedDatastoreClusterConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
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
func (sim *Simulator) ApplyHostConfiguration(hostConfigs []SimulatedHostConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
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
func (sim *Simulator) ApplyComputeClusterConfiguration(clusterConfigs []SimulatedComputeClusterConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
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
func (sim *Simulator) ApplyResourcePoolConfiguration(poolConfigs []SimulatedResourcePoolConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
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
		for _, segment := range segments {
			if segment == "" || segment == "." {
				return fmt.Errorf("invalid resource pool path %q", cfg.Path)
			}
			parent, err = ensureChildResourcePool(sim, parent, segment)
			if err != nil {
				return err
			}
		}

		ref := parent.Reference()
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

// ApplyNetworkConfiguration updates existing simulator networks according to
// the provided configurations, matched by supported NetworkList order.
func (sim *Simulator) ApplyNetworkConfiguration(networkConfigs []SimulatedNetworkConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
	tagMan := tags.NewManager(sim.RestClient)

	all, err := sim.Finder.NetworkList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list networks in simulator: %w", err)
	}

	networks := make([]object.NetworkReference, 0, len(all))
	for _, n := range all {
		switch n.Reference().Type {
		case "Network", "DistributedVirtualPortgroup", "OpaqueNetwork":
			networks = append(networks, n)
		}
	}

	if len(networkConfigs) > len(networks) {
		return fmt.Errorf("requested %d network configurations but simulator has %d supported networks", len(networkConfigs), len(networks))
	}

	for i, cfg := range networkConfigs {
		ref := networks[i].Reference()
		obj := sim.Model.Map().Get(ref)
		if obj == nil {
			return fmt.Errorf("failed to resolve simulator network for %s", ref.Value)
		}

		if cfg.Name != "" {
			switch net := obj.(type) {
			case *mo.Network:
				net.Name = cfg.Name
				if s, ok := net.Summary.(*types.NetworkSummary); ok && s != nil {
					s.Name = cfg.Name
				}
			case *simulator.DistributedVirtualPortgroup:
				net.Name = cfg.Name
				net.Config.Name = cfg.Name
			case *mo.OpaqueNetwork:
				net.Name = cfg.Name
				if s, ok := net.Summary.(*types.OpaqueNetworkSummary); ok && s != nil {
					s.Name = cfg.Name
				}
			default:
				return fmt.Errorf("unsupported simulator network type %T for %s", obj, ref.Value)
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
				return fmt.Errorf("failed to attach tag to network: %w", err)
			}
		}
	}

	return nil
}

// ApplyContentLibraryConfiguration creates local content libraries backed by
// simulator datastores and attaches tags.
func (sim *Simulator) ApplyContentLibraryConfiguration(libraryConfigs []SimulatedContentLibraryConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
	tagMan := tags.NewManager(sim.RestClient)
	libMan := library.NewManager(sim.RestClient)

	datastores, err := sim.Finder.DatastoreList(sim.Ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to list datastores in simulator: %w", err)
	}
	if len(datastores) == 0 {
		return fmt.Errorf("simulator has no datastores to back a content library")
	}

	for _, cfg := range libraryConfigs {
		if cfg.Name == "" {
			return fmt.Errorf("content library name is required")
		}
		if cfg.DatastoreIndex < 0 || cfg.DatastoreIndex >= len(datastores) {
			return fmt.Errorf("datastore index %d out of range (have %d)", cfg.DatastoreIndex, len(datastores))
		}

		id, err := libMan.CreateLibrary(sim.Ctx, library.Library{
			Name: cfg.Name,
			Type: "LOCAL",
			Storage: []library.StorageBacking{{
				DatastoreID: datastores[cfg.DatastoreIndex].Reference().Value,
				Type:        "DATASTORE",
			}},
		})
		if err != nil {
			return fmt.Errorf("failed to create content library %q: %w", cfg.Name, err)
		}

		ref := types.ManagedObjectReference{Type: "com.vmware.content.Library", Value: id}
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
				return fmt.Errorf("failed to attach tag to content library: %w", err)
			}
		}
	}

	return nil
}

// ApplyContentLibraryItemConfiguration creates content library items in
// existing content libraries, attaches tags, and uploads any requested files.
func (sim *Simulator) ApplyContentLibraryItemConfiguration(itemConfigs []SimulatedContentLibraryItemConfig) error {
	if err := sim.requireREST(); err != nil {
		return err
	}
	tagMan := tags.NewManager(sim.RestClient)
	libMan := library.NewManager(sim.RestClient)

	for _, cfg := range itemConfigs {
		if cfg.Library == "" {
			return fmt.Errorf("content library item requires a library name")
		}
		if cfg.Name == "" {
			return fmt.Errorf("content library item name is required")
		}

		lib, err := libMan.GetLibraryByName(sim.Ctx, cfg.Library)
		if err != nil {
			return fmt.Errorf("failed to find content library %q: %w", cfg.Library, err)
		}

		itemID, err := libMan.CreateLibraryItem(sim.Ctx, library.Item{
			Name:      cfg.Name,
			Type:      cfg.Type,
			LibraryID: lib.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to create content library item %q: %w", cfg.Name, err)
		}

		ref := types.ManagedObjectReference{Type: "com.vmware.content.library.Item", Value: itemID}
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
				return fmt.Errorf("failed to attach tag to content library item: %w", err)
			}
		}

		if len(cfg.Files) == 0 {
			continue
		}

		sessionID, err := libMan.CreateLibraryItemUpdateSession(sim.Ctx, library.Session{LibraryItemID: itemID})
		if err != nil {
			return fmt.Errorf("failed to create update session for item %q: %w", cfg.Name, err)
		}

		for _, fileName := range cfg.Files {
			content := []byte(fmt.Sprintf("simulated content for %s/%s", cfg.Name, fileName))
			update, err := libMan.AddLibraryItemFile(sim.Ctx, sessionID, library.UpdateFile{
				Name:       fileName,
				SourceType: "PUSH",
				Size:       int64(len(content)),
			})
			if err != nil {
				return fmt.Errorf("failed to add file %q to item %q: %w", fileName, cfg.Name, err)
			}

			u, err := url.Parse(update.UploadEndpoint.URI)
			if err != nil {
				return fmt.Errorf("failed to parse upload endpoint for %q: %w", fileName, err)
			}

			p := soap.DefaultUpload
			p.ContentLength = int64(len(content))
			if err := libMan.Upload(sim.Ctx, bytes.NewReader(content), u, &p); err != nil {
				return fmt.Errorf("failed to upload file %q to item %q: %w", fileName, cfg.Name, err)
			}
		}

		if err := libMan.CompleteLibraryItemUpdateSession(sim.Ctx, sessionID); err != nil {
			return fmt.Errorf("failed to complete update session for item %q: %w", cfg.Name, err)
		}
	}

	return nil
}

func ensureChildResourcePool(sim *Simulator, parent *object.ResourcePool, name string) (*object.ResourcePool, error) {
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

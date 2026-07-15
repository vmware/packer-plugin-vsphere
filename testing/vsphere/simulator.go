// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
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

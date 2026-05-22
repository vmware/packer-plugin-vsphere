// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type CloneConfig

package clone

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type CloneConfig struct {
	// The name of the source virtual machine to clone.
	Template string `mapstructure:"template"`
	// The size of the primary disk in MiB. Cannot be used with `linked_clone`.
	// -> **Note:** Only the primary disk size can be specified. Additional
	// disks are not supported.
	DiskSize int64 `mapstructure:"disk_size"`
	// Create the virtual machine as a linked clone from the latest snapshot.
	// Defaults to `false`. Cannot be used with `disk_size`.`
	LinkedClone bool `mapstructure:"linked_clone"`
	// The network to which the virtual machine will connect.
	//
	// For example:
	//
	// - Name: `<NetworkName>`
	// - Inventory Path: `/<DatacenterName>/<FolderName>/<NetworkName>`
	// - Managed Object ID (Port Group): `Network:network-<xxxxx>`
	// - Managed Object ID (Distributed Port Group): `DistributedVirtualPortgroup::dvportgroup-<xxxxx>`
	// - Logical Switch UUID: `<uuid>`
	// - Segment ID: `/infra/segments/<SegmentID>`
	//
	// ~> **Note:** If more than one network resolves to the same name, either
	// the inventory path to network or an ID must be provided.
	//
	// ~> **Note:** If no network is specified, provide `host` to allow the
	// plugin to search for an available network.
	Network string `mapstructure:"network"`
	// The network card MAC address. For example `00:50:56:00:00:00`.
	// If set, the `network` must be also specified.
	MacAddress string `mapstructure:"mac_address"`
	// The annotations for the virtual machine.
	Notes string `mapstructure:"notes"`
	// Destroy the virtual machine after the build is complete.
	// Defaults to `false`.
	Destroy bool `mapstructure:"destroy"`
	// The vApp Options for the virtual machine. For more information, refer to
	// the [vApp Options Configuration](/packer/plugins/builders/vmware/vsphere-clone#vapp-options-configuration)
	// section.
	VAppConfig    common.VAppConfig    `mapstructure:"vapp"`
	StorageConfig common.StorageConfig `mapstructure:",squash"`
}

func (c *CloneConfig) Prepare() []error {
	var errs []error
	errs = append(errs, c.StorageConfig.Prepare()...)

	if c.Template == "" {
		errs = append(errs, fmt.Errorf("'template' is required"))
	}

	if c.LinkedClone && c.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'linked_clone' and 'disk_size' cannot be used together"))
	}

	if c.MacAddress != "" && c.Network == "" {
		errs = append(errs, fmt.Errorf("'network' is required when 'mac_address' is specified"))
	}

	return errs
}

type StepCloneVM struct {
	Config        *CloneConfig
	Location      *common.LocationConfig
	Force         bool
	GeneratedData *packerbuilderdata.GeneratedData
}

func (s *StepCloneVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)
	vmPath := path.Join(s.Location.Folder, s.Location.VMName)

	ui.Say("Finding virtual machine to clone...")
	template, err := d.FindVM(s.Config.Template)
	if err != nil {
		state.Put("error", fmt.Errorf("error finding virtual machine to clone: %s", err))
		return multistep.ActionHalt
	}

	err = d.PreCleanVM(ui, vmPath, s.Force, s.Location.Cluster, s.Location.Host, s.Location.ResourcePool)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Cloning virtual machine...")
	var disks []driver.Disk
	for _, disk := range s.Config.StorageConfig.Storage {
		disks = append(disks, driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
		})
	}

	datastoreName := s.Location.Datastore
	var primaryDatastore driver.Datastore
	if ds, ok := state.GetOk("datastore"); ok {
		primaryDatastore = ds.(driver.Datastore)
		datastoreName = primaryDatastore.Name()
	}

	// If no datastore was resolved and no datastore was specified, return an error.
	if datastoreName == "" && s.Location.DatastoreCluster == "" {
		state.Put("error", fmt.Errorf("no datastore specified and no datastore resolved from cluster"))
		return multistep.ActionHalt
	}

	// Handle multi-disk placement when using a datastore cluster.
	var datastoreRefs []*types.ManagedObjectReference
	if s.Location.DatastoreCluster != "" && len(disks) > 1 {
		if vcDriver, ok := d.(*driver.VCenterDriver); ok {
			// Request Storage DRS recommendations for all disks at once for optimal placement.
			ui.Sayf("Requesting Storage DRS recommendations for %d disks...", len(disks))

			diskDatastores, method, err := vcDriver.SelectDatastoresForDisks(s.Location.DatastoreCluster, disks)
			if err != nil {
				ui.Errorf("Warning: Failed to get Storage DRS recommendations: %s. Using primary datastore.", err)
				if primaryDatastore != nil {
					ref := primaryDatastore.Reference()
					for i := 0; i < len(disks); i++ {
						datastoreRefs = append(datastoreRefs, &ref)
					}
				}
			} else {
				// Use the first disk's datastore as the primary datastore.
				if len(diskDatastores) > 0 {
					datastoreName = diskDatastores[0].Name()
				}

				for i, ds := range diskDatastores {
					ref := ds.Reference()
					if method == driver.SelectionMethodDRS {
						log.Printf("[INFO] Disk %d: Storage DRS selected datastore '%s'", i+1, ds.Name())
					} else {
						log.Printf("[INFO] Disk %d: Using first available datastore '%s'", i+1, ds.Name())
					}
					datastoreRefs = append(datastoreRefs, &ref)
				}
			}
		}
	}

	vm, err := template.Clone(ctx, &driver.CloneConfig{
		Name:            s.Location.VMName,
		Folder:          s.Location.Folder,
		Cluster:         s.Location.Cluster,
		Host:            s.Location.Host,
		ResourcePool:    s.Location.ResourcePool,
		Datastore:       datastoreName,
		LinkedClone:     s.Config.LinkedClone,
		Network:         s.Config.Network,
		MacAddress:      strings.ToLower(s.Config.MacAddress),
		Annotation:      s.Config.Notes,
		VAppProperties:  s.Config.VAppConfig.Properties,
		PrimaryDiskSize: s.Config.DiskSize,
		StorageConfig: driver.StorageConfig{
			DiskControllerType: s.Config.StorageConfig.DiskControllerType,
			Storage:            disks,
			DatastoreRefs:      datastoreRefs,
		},
	})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	if vm == nil {
		return multistep.ActionHalt
	}
	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

func (s *StepCloneVM) Cleanup(state multistep.StateBag) {
	common.CleanupVM(state)
}

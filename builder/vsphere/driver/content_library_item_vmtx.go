// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"log"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/types"
)

func buildVmtxLibraryItemDeploySpec(config *ContentLibraryDeployConfig, target *contentLibraryDeployTarget) vcenter.DeployTemplate {
	storage := &vcenter.DiskStorage{
		Datastore: target.datastoreID,
		StoragePolicy: &vcenter.StoragePolicy{
			Type: "USE_SOURCE_POLICY",
		},
	}

	return vcenter.DeployTemplate{
		Name:          config.Name,
		Description:   config.Annotation,
		DiskStorage:   storage,
		VMHomeStorage: storage,
		Placement: &vcenter.Placement{
			ResourcePool: target.ResourcePoolID,
			Host:         target.HostID,
			Folder:       target.FolderID,
		},
	}
}

func (d *VCenterDriver) deployVmtxLibraryItem(ctx context.Context, item *library.Item, config *ContentLibraryDeployConfig, target *contentLibraryDeployTarget) (VirtualMachine, error) {
	deploy := buildVmtxLibraryItemDeploySpec(config, target)

	m := vcenter.NewManager(d.RestClient.client)
	ref, err := m.DeployTemplateLibraryItem(ctx, item.ID, deploy)
	if err != nil {
		return nil, fmt.Errorf("content library item VM template deployment failed: %s", err)
	}

	vmDriver := d.NewVM(ref).(*VirtualMachineDriver)
	if err := d.applyVmtxLibraryItemPostDeploy(ctx, vmDriver, config); err != nil {
		return nil, fmt.Errorf("failed to apply post-deploy virtual machine configuration: %s", err)
	}

	log.Printf("[INFO] Successfully deployed virtual machine from content library item VM template '%s'", item.Name)
	return vmDriver, nil
}

func (d *VCenterDriver) applyVmtxLibraryItemPostDeploy(ctx context.Context, vm *VirtualMachineDriver, config *ContentLibraryDeployConfig) error {
	var configSpec types.VirtualMachineConfigSpec
	needsReconfigure := false

	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return fmt.Errorf("error finding virtual machine devices: %s", err)
	}

	if config.PrimaryDiskSize > 0 {
		deviceResizeSpec, err := vm.ResizeDisk(config.PrimaryDiskSize)
		if err != nil {
			return fmt.Errorf("failed to resize primary disk: %s", err)
		}
		configSpec.DeviceChange = append(configSpec.DeviceChange, deviceResizeSpec...)
		needsReconfigure = true
	}

	virtualDisks := devices.SelectByType((*types.VirtualDisk)(nil))
	virtualControllers := devices.SelectByType((*types.VirtualController)(nil))
	existingDevices := object.VirtualDeviceList{}
	existingDevices = append(existingDevices, virtualDisks...)
	existingDevices = append(existingDevices, virtualControllers...)

	if len(config.StorageConfig.Storage) > 0 || len(config.StorageConfig.DiskControllerType) > 0 {
		storageConfigSpec, err := config.StorageConfig.AddStorageDevices(existingDevices)
		if err != nil {
			return fmt.Errorf("failed to add storage devices: %s", err)
		}
		configSpec.DeviceChange = append(configSpec.DeviceChange, storageConfigSpec...)
		needsReconfigure = true
	}

	if config.Network != "" {
		net, err := vm.driver.FindNetwork(config.Network)
		if err != nil {
			return fmt.Errorf("error finding network: %s", err)
		}
		backing, err := net.network.EthernetCardBackingInfo(ctx)
		if err != nil {
			return fmt.Errorf("error finding ethernet card backing info: %s", err)
		}

		devices, err := vm.vm.Device(ctx)
		if err != nil {
			return fmt.Errorf("error finding virtual machine devices: %s", err)
		}

		adapter, err := findNetworkAdapter(devices)
		if err != nil {
			return fmt.Errorf("error finding network adapter: %s", err)
		}

		current := adapter.GetVirtualEthernetCard()
		current.Backing = backing
		if config.MacAddress != "" {
			current.AddressType = string(types.VirtualEthernetCardMacTypeManual)
			current.MacAddress = config.MacAddress
		}

		configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
			Device:    adapter.(types.BaseVirtualDevice),
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		})
		needsReconfigure = true
	}

	if len(config.VAppProperties) > 0 {
		vAppConfig, err := buildVAppConfigSpec(ctx, vm, false, config.VAppProperties)
		if err != nil {
			return fmt.Errorf("error updating VAppConfig: %s", err)
		}
		configSpec.VAppConfig = vAppConfig
		needsReconfigure = true
	}

	if !needsReconfigure {
		return nil
	}

	return vm.Reconfigure(configSpec)
}

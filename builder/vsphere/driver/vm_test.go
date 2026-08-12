// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestVirtualMachineDriver_Configure(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	// Happy test
	hardwareConfig := &HardwareConfig{
		CPUs:                  1,
		CpuCores:              1,
		CPUReservation:        2500,
		CPULimit:              1,
		RAM:                   1024,
		RAMReserveAll:         true,
		VideoRAM:              512,
		VGPUProfile:           "grid_m10-8q",
		Firmware:              "efi-secure",
		ForceBIOSSetup:        true,
		BootDelay:             5000,
		VTPMEnabled:           true,
		VirtualPrecisionClock: "ntp",
	}
	if err := vm.Configure(hardwareConfig); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(newSimulatorDriver(sim).Ctx)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if props.Config.BootOptions == nil {
		t.Fatal("expected boot options to be set")
	}
	if props.Config.BootOptions.BootDelay != hardwareConfig.BootDelay {
		t.Fatalf("unexpected boot delay: expected '%d', but returned '%d'", hardwareConfig.BootDelay, props.Config.BootOptions.BootDelay)
	}
}

func TestVirtualMachineDriver_ConfigureBootDelayOnly(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	const bootDelay int64 = 5000
	if err := vm.Configure(&HardwareConfig{BootDelay: bootDelay}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(newSimulatorDriver(sim).Ctx)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if props.Config.BootOptions == nil {
		t.Fatal("expected boot options to be set")
	}
	if props.Config.BootOptions.BootDelay != bootDelay {
		t.Fatalf("unexpected boot delay: expected '%d', but returned '%d'", bootDelay, props.Config.BootOptions.BootDelay)
	}
}

func TestVirtualMachineDriver_SetBootOrderPreservesBootDelay(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	const bootDelay int64 = 5000
	if err := vm.Configure(&HardwareConfig{BootDelay: bootDelay}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if err := vm.SetBootOrder([]string{"disk", "cdrom"}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(newSimulatorDriver(sim).Ctx)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if props.Config.BootOptions == nil {
		t.Fatal("expected boot options to be set")
	}
	if props.Config.BootOptions.BootDelay != bootDelay {
		t.Fatalf("unexpected boot delay: expected '%d', but returned '%d'", bootDelay, props.Config.BootOptions.BootDelay)
	}
}

func TestVirtualMachineDriver_CreateVMWithMultipleDisks(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)

	config := &CreateConfig{
		Name:      "mock name",
		Host:      "DC0_H0",
		Datastore: datastore.Name,
		NICs: []NIC{
			{
				Network:     "VM Network",
				NetworkCard: "vmxnet3",
			},
		},
		StorageConfig: StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []Disk{
				{
					DiskSize:            3072,
					DiskThinProvisioned: true,
					ControllerIndex:     0,
				},
				{
					DiskSize:            20480,
					DiskThinProvisioned: true,
					ControllerIndex:     0,
				},
			},
		},
	}

	vm, err := newSimulatorDriver(sim).CreateVM(config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err := vm.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var disks []*types.VirtualDisk
	for _, device := range devices {
		switch d := device.(type) {
		case *types.VirtualDisk:
			disks = append(disks, d)
		}
	}

	if len(disks) != 2 {
		t.Fatalf("unexpected result: expected '2', but returned %d", len(disks))
	}
}

// TestVirtualMachineDriver_CreateVM_WithStoragePolicy verifies that CreateVM
// succeeds when a disk carries a StoragePolicyID, exercising the VmProfile
// code path in the VM config spec.
func TestVirtualMachineDriver_CreateVM_WithStoragePolicy(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)

	config := &CreateConfig{
		Name:      "mock-vm-with-policy",
		Host:      "DC0_H0",
		Datastore: datastore.Name,
		NICs: []NIC{
			{
				Network:     "VM Network",
				NetworkCard: "vmxnet3",
			},
		},
		StorageConfig: StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []Disk{
				{
					DiskSize:            10240,
					DiskThinProvisioned: true,
					ControllerIndex:     0,
					StoragePolicyID:     "aaaabbbb-cccc-dddd-eeee-ffffffffffff",
				},
				{
					DiskSize:        20480,
					ControllerIndex: 0,
					// No policy on this disk — VmProfile should still be set
					// from the first disk's policy.
				},
			},
		},
	}

	vm, err := newSimulatorDriver(sim).CreateVM(config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if vm == nil {
		t.Fatal("expected a VM, got nil")
	}
}

func TestVirtualMachineDriver_CloneWithPrimaryDiskResize(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	vm, _ := mustPreCreatedVM(t, sim)

	config := &CloneConfig{
		Name:            "mock name",
		Host:            "DC0_H0",
		Datastore:       datastore.Name,
		PrimaryDiskSize: 204800,
		StorageConfig: StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []Disk{
				{
					DiskSize:            3072,
					DiskThinProvisioned: true,
					ControllerIndex:     0,
				},
				{
					DiskSize:            20480,
					DiskThinProvisioned: true,
					ControllerIndex:     0,
				},
			},
		},
	}

	clonedVM, err := vm.Clone(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err := clonedVM.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var disks []*types.VirtualDisk
	for _, device := range devices {
		switch d := device.(type) {
		case *types.VirtualDisk:
			disks = append(disks, d)
		}
	}

	if len(disks) != 3 {
		t.Fatalf("unexpected result: expected '3', but returned '%d'", len(disks))
	}

	if disks[0].CapacityInKB != config.PrimaryDiskSize*1024 {
		t.Fatalf("unexpected result: expected '%d', but returned '%d'", config.PrimaryDiskSize*1024, disks[0].CapacityInKB)
	}
	if disks[1].CapacityInKB != config.StorageConfig.Storage[0].DiskSize*1024 {
		t.Fatalf("unexpected result: expected '%d', but returned '%d'", config.StorageConfig.Storage[0].DiskSize*1024, disks[1].CapacityInKB)
	}
	if disks[2].CapacityInKB != config.StorageConfig.Storage[1].DiskSize*1024 {
		t.Fatalf("unexpected result: expected '%d', but returned '%d'", config.StorageConfig.Storage[1].DiskSize*1024, disks[2].CapacityInKB)
	}
}

func TestVirtualMachineDriver_CloneWithExplicitControllerUnit(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	vm, _ := mustPreCreatedVM(t, sim)

	config := &CloneConfig{
		Name:      "mock name",
		Host:      "DC0_H0",
		Datastore: datastore.Name,
		StorageConfig: StorageConfig{
			Storage: []Disk{
				{
					DiskSize:       4096,
					ControllerUnit: "scsi0:1",
				},
			},
		},
	}

	clonedVM, err := vm.Clone(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err := clonedVM.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var disks []*types.VirtualDisk
	for _, device := range devices {
		if disk, ok := device.(*types.VirtualDisk); ok {
			disks = append(disks, disk)
		}
	}

	if len(disks) != 2 {
		t.Fatalf("unexpected result: expected '2', but returned '%d'", len(disks))
	}

	var added *types.VirtualDisk
	for _, disk := range disks {
		if disk.UnitNumber != nil && *disk.UnitNumber == 1 {
			added = disk
			break
		}
	}
	if added == nil {
		t.Fatal("expected to find disk attached at scsi0:1")
	}
	if added.CapacityInKB != 4096*1024 {
		t.Fatalf("unexpected result: expected '%d', but returned '%d'", 4096*1024, added.CapacityInKB)
	}

	controllers := devices.SelectByType((*types.VirtualSCSIController)(nil))
	if len(controllers) != 1 {
		t.Fatalf("unexpected result: expected '1' SCSI controller, but returned '%d'", len(controllers))
	}
}

func TestVirtualMachineDriver_CloneWithExplicitUnitNewController(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	vm, _ := mustPreCreatedVM(t, sim)

	config := &CloneConfig{
		Name:      "mock name",
		Host:      "DC0_H0",
		Datastore: datastore.Name,
		StorageConfig: StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []Disk{
				{
					DiskSize:       4096,
					ControllerUnit: "scsi1:0",
				},
			},
		},
	}

	clonedVM, err := vm.Clone(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err := clonedVM.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	controllers := devices.SelectByType((*types.VirtualSCSIController)(nil))
	if len(controllers) != 2 {
		t.Fatalf("unexpected result: expected '2' SCSI controllers, but returned '%d'", len(controllers))
	}

	var added *types.VirtualDisk
	for _, device := range devices {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		if disk.UnitNumber != nil && *disk.UnitNumber == 0 {
			controllerKey := disk.GetVirtualDevice().ControllerKey
			for _, controllerDevice := range devices {
				controller, ok := controllerDevice.(types.BaseVirtualSCSIController)
				if !ok {
					continue
				}
				if controller.GetVirtualSCSIController().Key == controllerKey &&
					controller.GetVirtualSCSIController().BusNumber == 1 {
					added = disk
					break
				}
			}
		}
	}
	if added == nil {
		t.Fatal("expected to find disk attached at scsi1:0")
	}
}

func TestVirtualMachineDriver_CloneWithMixedLegacyAndExplicit(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	vm, _ := mustPreCreatedVM(t, sim)

	config := &CloneConfig{
		Name:      "mock name",
		Host:      "DC0_H0",
		Datastore: datastore.Name,
		StorageConfig: StorageConfig{
			DiskControllerType: []string{"pvscsi", "pvscsi"},
			Storage: []Disk{
				{
					DiskSize:        2048,
					ControllerIndex: 0,
				},
				{
					DiskSize:       4096,
					ControllerUnit: "scsi0:1",
				},
			},
		},
	}

	clonedVM, err := vm.Clone(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err := clonedVM.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var explicitDisk *types.VirtualDisk
	for _, device := range devices {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		if disk.UnitNumber != nil && *disk.UnitNumber == 1 {
			explicitDisk = disk
			break
		}
	}
	if explicitDisk == nil {
		t.Fatal("expected explicit disk at scsi0:1")
	}
}

func TestVirtualMachineDriver_CloneWithMacAddress(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	vm, _ := mustPreCreatedVM(t, sim)

	devices, err := vm.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	adapter, err := findNetworkAdapter(devices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	network := adapter.GetVirtualEthernetCard()
	oldMacAddress := network.MacAddress

	newMacAddress := "d4:b4:d4:96:70:26"
	config := &CloneConfig{
		Name:       "mock name",
		Host:       "DC0_H0",
		Datastore:  datastore.Name,
		Network:    "/DC0/network/VM Network",
		MacAddress: newMacAddress,
	}

	ctx := context.Background()
	clonedVM, err := vm.Clone(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	devices, err = clonedVM.Devices()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	adapter, err = findNetworkAdapter(devices)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	network = adapter.GetVirtualEthernetCard()
	if network.AddressType != string(types.VirtualEthernetCardMacTypeManual) {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", types.VirtualEthernetCardMacTypeManual, network.AddressType)
	}
	if network.MacAddress == oldMacAddress {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", newMacAddress, network.MacAddress)
	}
	if network.MacAddress != newMacAddress {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", newMacAddress, network.MacAddress)
	}
}

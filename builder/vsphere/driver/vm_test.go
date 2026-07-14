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
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	vm, _ := sim.ChooseSimulatorPreCreatedVM()

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
	if err = vm.Configure(hardwareConfig); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(sim.driver.Ctx)
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
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	vm, _ := sim.ChooseSimulatorPreCreatedVM()

	const bootDelay int64 = 5000
	if err = vm.Configure(&HardwareConfig{BootDelay: bootDelay}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(sim.driver.Ctx)
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
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	vm, _ := sim.ChooseSimulatorPreCreatedVM()

	const bootDelay int64 = 5000
	if err = vm.Configure(&HardwareConfig{BootDelay: bootDelay}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if err = vm.SetBootOrder([]string{"disk", "cdrom"}); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	props, err := vm.Properties(sim.driver.Ctx)
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
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	_, datastore := sim.ChooseSimulatorPreCreatedDatastore()

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

	vm, err := sim.driver.CreateVM(config)
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

func TestVirtualMachineDriver_CloneWithPrimaryDiskResize(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	_, datastore := sim.ChooseSimulatorPreCreatedDatastore()
	vm, _ := sim.ChooseSimulatorPreCreatedVM()

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

	clonedVM, err := vm.Clone(context.TODO(), config)
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

func TestVirtualMachineDriver_CloneWithMacAddress(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	defer sim.Close()

	_, datastore := sim.ChooseSimulatorPreCreatedDatastore()
	vm, _ := sim.ChooseSimulatorPreCreatedVM()

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

	ctx := context.TODO()
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

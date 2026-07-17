// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware/govmomi/vim25/types"
)

func TestVirtualMachineDriver_FindAndAddSATAController(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	_, err := vm.FindSATAController()
	if err != nil && !strings.Contains(err.Error(), "no available SATA controller") {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if err == nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if err := vm.AddSATAController(); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	sc, err := vm.FindSATAController()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if sc == nil {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "sata controller", sc)
	}
}

func TestVirtualMachineDriver_CreateAndRemoveCdrom(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	// Add the SATA controller.
	if err := vm.AddSATAController(); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	// Verify if the SATA controller was created.
	sc, err := vm.FindSATAController()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if sc == nil {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "sata controller", sc)
	}

	// Create a CD-ROM.
	controller := sc.GetVirtualController()
	cdrom, err := vm.CreateCdrom(controller)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if cdrom == nil {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "cd-rom", cdrom)
	}

	// Verify if the CD-ROM was created.
	cdroms, err := vm.CdromDevices()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if len(cdroms) != 1 {
		t.Fatalf("unexpected result: expected '1', but returned '%d'", len(cdroms))
	}

	// Remove the CD-ROM.
	err = vm.RemoveCdroms()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	// Verify if the CD-ROM was removed.
	cdroms, err = vm.CdromDevices()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if len(cdroms) != 0 {
		t.Fatalf("unexpected result: expected '0', but returned '%d'", len(cdroms))
	}
}

func TestVirtualMachineDriver_EjectCdrom(t *testing.T) {
	sim := mustVPXSimulator(t)

	vm, _ := mustPreCreatedVM(t, sim)

	// Add the SATA controller.
	if err := vm.AddSATAController(); err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	// Verify if the SATA controller was created.
	sc, err := vm.FindSATAController()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if sc == nil {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "sata controller", sc)
	}

	// Create the CD-ROM.
	controller := sc.GetVirtualController()
	cdrom, err := vm.CreateCdrom(controller)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if cdrom == nil {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "cd-rom", cdrom)
	}

	// Verify if the CD-ROM was created.
	cdroms, err := vm.CdromDevices()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if len(cdroms) != 1 {
		t.Fatalf("unexpected result: expected '1', but returned '%d'", len(cdroms))
	}

	// Remove the CD-ROM.
	err = vm.EjectCdroms()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	// Verify if the CD-ROM was removed.
	cdroms, err = vm.CdromDevices()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if len(cdroms) != 1 {
		t.Fatalf("unexpected result: expected '1', but returned '%d'", len(cdroms))
	}
	cd, ok := cdroms[0].(*types.VirtualCdrom)
	if !ok {
		t.Fatalf("unexpected result: expected '%s', but returned '%v'", "cdrom", cd)
	}
	if diff := cmp.Diff(cd.Backing, &types.VirtualCdromRemotePassthroughBackingInfo{}); diff != "" {
		t.Fatalf("unexpected result: '%s'", diff)
	}
	if diff := cmp.Diff(cd.Connectable, &types.VirtualDeviceConnectInfo{}); diff != "" {
		t.Fatalf("unexpected result: '%s'", diff)
	}
}

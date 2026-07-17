// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

func newSimulatorDriver(s *vcsim.Simulator) *driver.VCenterDriver {
	user := s.Server.URL.User
	if user == nil {
		user = simulator.DefaultLogin
	}
	return driver.NewVCenterDriver(s.Ctx, s.Client, s.Client.Client, user, s.Finder, s.Datacenter)
}

func mustVPXSimulator(t *testing.T) *vcsim.Simulator {
	t.Helper()
	sim, err := vcsim.NewVPXSimulator()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	t.Cleanup(sim.Stop)
	return sim
}

func mustPreCreatedVM(t *testing.T, s *vcsim.Simulator) (driver.VirtualMachine, *simulator.VirtualMachine) {
	t.Helper()
	machine, ok := s.Model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	if !ok || machine == nil {
		t.Fatal("no VirtualMachine found in simulator inventory")
	}
	ref := machine.Reference()
	return newSimulatorDriver(s).NewVM(&ref), machine
}

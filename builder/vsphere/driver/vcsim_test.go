// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

// newSimulatorDriver returns a VCenterDriver connected to the shared testing/vcsim instance.
func newSimulatorDriver(s *vcsim.Simulator) *VCenterDriver {
	user := s.Server.URL.User
	if user == nil {
		user = simulator.DefaultLogin
	}
	return NewVCenterDriver(s.Ctx, s.Client, s.Client.Client, user, s.Finder, s.Datacenter)
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

func mustCustomSimulator(t *testing.T, model *simulator.Model) *vcsim.Simulator {
	t.Helper()
	sim, err := vcsim.NewSimulator(model)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	t.Cleanup(sim.Stop)
	return sim
}

func mustPreCreatedVM(t *testing.T, s *vcsim.Simulator) (VirtualMachine, *simulator.VirtualMachine) {
	t.Helper()
	machine, ok := s.Model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	if !ok || machine == nil {
		t.Fatal("no VirtualMachine found in simulator inventory")
	}
	ref := machine.Reference()
	return newSimulatorDriver(s).NewVM(&ref), machine
}

func mustPreCreatedDatastore(t *testing.T, s *vcsim.Simulator) (Datastore, *simulator.Datastore) {
	t.Helper()
	ds, ok := s.Model.Map().Any("Datastore").(*simulator.Datastore)
	if !ok || ds == nil {
		t.Fatal("no Datastore found in simulator inventory")
	}
	ref := ds.Reference()
	return newSimulatorDriver(s).NewDatastore(&ref), ds
}

func mustPreCreatedHost(t *testing.T, s *vcsim.Simulator) (*Host, *simulator.HostSystem) {
	t.Helper()
	h, ok := s.Model.Map().Any("HostSystem").(*simulator.HostSystem)
	if !ok || h == nil {
		t.Fatal("no HostSystem found in simulator inventory")
	}
	ref := h.Reference()
	return newSimulatorDriver(s).NewHost(&ref), h
}

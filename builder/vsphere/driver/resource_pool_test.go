// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestVCenterDriver_FindResourcePool(t *testing.T) {
	sim := mustVPXSimulator(t)

	t.Run("empty name with host", func(t *testing.T) {
		res, err := newSimulatorDriver(sim).FindResourcePool("", "DC0_H0", "")
		if err != nil {
			t.Fatalf("unexpected error: '%s'", err)
		}
		if res == nil {
			t.Fatalf("unexpected result: expected resource pool, but returned 'nil'")
		}
		expectedResourcePool := "Resources"
		if res.pool.Name() != expectedResourcePool {
			t.Fatalf("unexpected result: expected '%s', but returned '%s'", expectedResourcePool, res.pool.Name())
		}
	})

	t.Run("empty name with cluster", func(t *testing.T) {
		res, err := newSimulatorDriver(sim).FindResourcePool("DC0_C0", "", "")
		if err != nil {
			t.Fatalf("unexpected error: '%s'", err)
		}
		if res == nil {
			t.Fatalf("unexpected result: expected resource pool, but returned 'nil'")
		}
		if res.pool.Name() != "Resources" {
			t.Fatalf("unexpected result: expected 'Resources', but returned '%s'", res.pool.Name())
		}
	})

	t.Run("relative path", func(t *testing.T) {
		res, err := newSimulatorDriver(sim).FindResourcePool("DC0_C0", "", "foo")

		if err == nil {
			t.Fatalf("expected error when using unknown relative resource pool path 'foo', but got none")
		}
		if res != nil {
			t.Fatalf("unexpected result: expected no resource pool for unknown path 'foo', but got one")
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		res, err := newSimulatorDriver(sim).FindResourcePool("", "", "/DC0/host/DC0_H0/Resources")
		if err != nil {
			t.Fatalf("unexpected error: '%s'", err)
		}
		if res == nil || res.pool == nil {
			t.Fatalf("unexpected result: expected resource pool, but returned 'nil'")
		}
	})

	t.Run("whitespace trimming", func(t *testing.T) {
		res, err := newSimulatorDriver(sim).FindResourcePool("", "DC0_H0", "  ")
		if err != nil {
			t.Fatalf("unexpected error: '%s'", err)
		}
		if res == nil {
			t.Fatalf("unexpected result: expected resource pool, but returned 'nil'")
		}
	})
}

func TestVCenterDriver_FindResourcePoolStandaloneESX(t *testing.T) {
	// Standalone ESX host without a vCenter instance
	model := simulator.ESX()
	defer model.Remove()

	opts := simulator.VPX()
	model.Datastore = opts.Datastore
	model.Machine = opts.Machine
	model.Autostart = opts.Autostart
	model.DelayConfig.Delay = opts.DelayConfig.Delay
	model.DelayConfig.MethodDelay = opts.DelayConfig.MethodDelay
	model.DelayConfig.DelayJitter = opts.DelayConfig.DelayJitter

	sim := mustCustomSimulator(t, model)

	res, err := newSimulatorDriver(sim).FindResourcePool("", "localhost.localdomain", "")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if res == nil {
		t.Fatalf("unexpected result: expected resource pool, but returned nil")
	}
	expectedResourcePool := "Resources"
	if res.pool.Name() != expectedResourcePool {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", expectedResourcePool, res.pool.Name())
	}

	// Invalid resource name should look for default resource pool
	res, err = newSimulatorDriver(sim).FindResourcePool("", "localhost.localdomain", "invalid")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if res == nil {
		t.Fatalf("unexpected result: expected non-nil resource pool, but returned nil")
	}
	if res.pool.Name() != expectedResourcePool {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", expectedResourcePool, res.pool.Name())
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"sort"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/testing/vcsim"
)

// allSimulatorDatastores returns every Datastore in the simulator inventory,
// sorted by reference value for deterministic ordering across test runs.
func allSimulatorDatastores(t *testing.T, s *vcsim.Simulator) []*simulator.Datastore {
	t.Helper()

	var datastores []*simulator.Datastore
	for _, entity := range s.Model.Map().All("Datastore") {
		if ds, ok := entity.(*simulator.Datastore); ok {
			datastores = append(datastores, ds)
		}
	}

	sort.Slice(datastores, func(i, j int) bool {
		return datastores[i].Reference().Value < datastores[j].Reference().Value
	})

	return datastores
}

func TestAllResolved(t *testing.T) {
	testCases := []struct {
		name     string
		resolved []bool
		want     bool
	}{
		{name: "empty slice", resolved: nil, want: true},
		{name: "all resolved", resolved: []bool{true, true, true}, want: true},
		{name: "one unresolved", resolved: []bool{true, false, true}, want: false},
		{name: "none resolved", resolved: []bool{false, false}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allResolved(tc.resolved); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestDuplicateDatastore(t *testing.T) {
	ds := &DatastoreMock{NameReturn: "ds-1"}

	result := duplicateDatastore(ds, 3)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	for i, got := range result {
		if got != ds {
			t.Fatalf("entry %d: expected the same datastore reference, got %#v", i, got)
		}
	}
}

func TestDuplicateDatastoreZeroCount(t *testing.T) {
	result := duplicateDatastore(&DatastoreMock{}, 0)
	if len(result) != 0 {
		t.Fatalf("expected an empty slice, got %d entries", len(result))
	}
}

func TestResolveDatastoreRef(t *testing.T) {
	sim := mustVPXSimulator(t)
	d := newSimulatorDriver(sim)

	_, simDS := mustPreCreatedDatastore(t, sim)
	ref := simDS.Reference()

	resolved, err := d.resolveDatastoreRef(ref)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if resolved.Reference().Value != ref.Value {
		t.Fatalf("expected resolved reference %q, got %q", ref.Value, resolved.Reference().Value)
	}
	if resolved.Name() != simDS.Name {
		t.Fatalf("expected resolved name %q, got %q", simDS.Name, resolved.Name())
	}
}

func TestDatastoresForNewDisks(t *testing.T) {
	model := simulator.VPX()
	model.Datastore = 2
	sim := mustCustomSimulator(t, model)
	d := newSimulatorDriver(sim)

	datastores := allSimulatorDatastores(t, sim)
	if len(datastores) < 2 {
		t.Fatalf("expected at least 2 datastores in simulator inventory, got %d", len(datastores))
	}
	ds0Ref := datastores[0].Reference()
	ds1Ref := datastores[1].Reference()
	fallback := d.NewDatastore(&ds0Ref)

	t.Run("fully resolved via per-disk locators", func(t *testing.T) {
		recommendation := types.ClusterRecommendation{
			Action: []types.BaseClusterAction{
				&types.StoragePlacementAction{
					RelocateSpec: types.VirtualMachineRelocateSpec{
						Disk: []types.VirtualMachineRelocateSpecDiskLocator{
							{DiskId: 2000, Datastore: ds0Ref},
							{DiskId: 2001, Datastore: ds1Ref},
						},
					},
				},
			},
		}

		result, ok := d.datastoresForNewDisks(recommendation, []int32{2000, 2001}, fallback)
		if !ok {
			t.Fatal("expected all disks to resolve")
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 datastores, got %d", len(result))
		}
		if result[0].Name() != datastores[0].Name {
			t.Fatalf("expected disk 2000 on %q, got %q", datastores[0].Name, result[0].Name())
		}
		if result[1].Name() != datastores[1].Name {
			t.Fatalf("expected disk 2001 on %q, got %q", datastores[1].Name, result[1].Name())
		}
	})

	t.Run("single destination-only action fills first gap then backfills the rest", func(t *testing.T) {
		recommendation := types.ClusterRecommendation{
			Action: []types.BaseClusterAction{
				&types.StoragePlacementAction{Destination: ds1Ref},
			},
		}

		result, ok := d.datastoresForNewDisks(recommendation, []int32{3000, 3001}, fallback)
		if !ok {
			t.Fatal("expected resolution to succeed via gap-filling")
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 datastores, got %d", len(result))
		}
		if result[0].Name() != datastores[1].Name {
			t.Fatalf("expected first disk resolved from the destination-only action, got %q", result[0].Name())
		}
		// The second disk key has no direct recommendation, so it must be
		// backfilled with the first resolved datastore.
		if result[1].Name() != datastores[1].Name {
			t.Fatalf("expected unresolved disk to be backfilled with %q, got %q", datastores[1].Name, result[1].Name())
		}
	})

	t.Run("partially resolved disk keys fall back to the first resolved datastore", func(t *testing.T) {
		recommendation := types.ClusterRecommendation{
			Action: []types.BaseClusterAction{
				&types.StoragePlacementAction{
					RelocateSpec: types.VirtualMachineRelocateSpec{
						Disk: []types.VirtualMachineRelocateSpecDiskLocator{
							{DiskId: 4000, Datastore: ds1Ref},
						},
					},
				},
			},
		}

		result, ok := d.datastoresForNewDisks(recommendation, []int32{4000, 4999}, fallback)
		if !ok {
			t.Fatal("expected resolution to succeed via the gap-filling fallback")
		}
		if result[0].Name() != datastores[1].Name {
			t.Fatalf("expected disk 4000 resolved from the recommendation, got %q", result[0].Name())
		}
		if result[1].Name() != datastores[1].Name {
			t.Fatalf("expected unmatched disk key 4999 to be backfilled with %q, got %q", datastores[1].Name, result[1].Name())
		}
	})

	t.Run("empty newDiskKeys returns a single fallback datastore", func(t *testing.T) {
		result, ok := d.datastoresForNewDisks(types.ClusterRecommendation{}, nil, fallback)
		if !ok {
			t.Fatal("expected success for empty newDiskKeys")
		}
		if len(result) != 1 || result[0] != fallback {
			t.Fatalf("expected a single fallback datastore, got %#v", result)
		}
	})

	t.Run("no actions resolve anything and reports failure", func(t *testing.T) {
		_, ok := d.datastoresForNewDisks(types.ClusterRecommendation{}, []int32{5000, 5001}, fallback)
		if ok {
			t.Fatal("expected resolution to fail when nothing can be resolved")
		}
	})
}

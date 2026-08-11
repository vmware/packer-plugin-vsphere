// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestStepResolveDatastore_Run(t *testing.T) {
	tc := []struct {
		name              string
		step              *StepResolveDatastore
		driverMock        *VCenterDriverMock
		expectedAction    multistep.StepAction
		expectedDatastore string
		expectedMethod    string
		expectError       bool
		errorContains     string
	}{
		{
			name: "Resolve from direct datastore",
			step: &StepResolveDatastore{
				Datastore: "test-datastore",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.DatastoreMock = &driver.DatastoreMock{
					NameReturn: "test-datastore",
				}
				return m
			}(),
			expectedAction:    multistep.ActionContinue,
			expectedDatastore: "test-datastore",
			expectedMethod:    "direct",
			expectError:       false,
		},
		{
			name: "Resolve from datastore cluster with DRS",
			step: &StepResolveDatastore{
				DatastoreCluster: "test-cluster",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.SelectDatastoreReturn = &driver.DatastoreMock{
					NameReturn: "cluster-datastore-1",
				}
				m.SelectDatastoreMethod = driver.SelectionMethodDRS
				return m
			}(),
			expectedAction:    multistep.ActionContinue,
			expectedDatastore: "cluster-datastore-1",
			expectedMethod:    driver.SelectionMethodDRS,
			expectError:       false,
		},
		{
			name: "Resolve from datastore cluster with fallback",
			step: &StepResolveDatastore{
				DatastoreCluster: "test-cluster",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.SelectDatastoreReturn = &driver.DatastoreMock{
					NameReturn: "cluster-datastore-1",
				}
				m.SelectDatastoreMethod = driver.SelectionMethodFallback
				return m
			}(),
			expectedAction:    multistep.ActionContinue,
			expectedDatastore: "cluster-datastore-1",
			expectedMethod:    driver.SelectionMethodFallback,
			expectError:       false,
		},
		{
			name: "Error finding direct datastore",
			step: &StepResolveDatastore{
				Datastore: "missing-datastore",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.FindDatastoreErr = fmt.Errorf("datastore not found")
				return m
			}(),
			expectedAction: multistep.ActionHalt,
			expectError:    true,
			errorContains:  "error finding datastore 'missing-datastore'",
		},
		{
			name: "Error selecting from datastore cluster",
			step: &StepResolveDatastore{
				DatastoreCluster: "missing-cluster",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.SelectDatastoreErr = fmt.Errorf("cluster not found")
				return m
			}(),
			expectedAction: multistep.ActionHalt,
			expectError:    true,
			errorContains:  "error resolving datastore from cluster 'missing-cluster'",
		},
		{
			name: "Resolve from storage policy via PBM",
			step: &StepResolveDatastore{
				StoragePolicy: "gold-policy",
				Host:          "esxi-1",
				Cluster:       "cluster-1",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.FindStoragePolicyIDResult = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"
				m.FindCompatibleDatastoreResult = &driver.DatastoreMock{
					NameReturn: "policy-datastore",
				}
				return m
			}(),
			expectedAction:    multistep.ActionContinue,
			expectedDatastore: "policy-datastore",
			expectedMethod:    driver.SelectionMethodStoragePolicy,
			expectError:       false,
		},
		{
			name: "Explicit datastore wins over storage policy",
			step: &StepResolveDatastore{
				Datastore:     "explicit-datastore",
				StoragePolicy: "gold-policy",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.DatastoreMock = &driver.DatastoreMock{
					NameReturn: "explicit-datastore",
				}
				return m
			}(),
			expectedAction:    multistep.ActionContinue,
			expectedDatastore: "explicit-datastore",
			expectedMethod:    "direct",
			expectError:       false,
		},
		{
			name: "Error when storage policy not found",
			step: &StepResolveDatastore{
				StoragePolicy: "missing-policy",
				Host:          "esxi-1",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.FindStoragePolicyIDErr = fmt.Errorf("not found")
				return m
			}(),
			expectedAction: multistep.ActionHalt,
			expectError:    true,
			errorContains:  "error resolving storage policy \"missing-policy\"",
		},
		{
			name: "Error when no compatible datastore for storage policy",
			step: &StepResolveDatastore{
				StoragePolicy: "gold-policy",
				Cluster:       "cluster-1",
			},
			driverMock: func() *VCenterDriverMock {
				m := NewVCenterDriverMock()
				m.FindStoragePolicyIDResult = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"
				m.FindCompatibleDatastoreErr = fmt.Errorf("no datastore compatible")
				return m
			}(),
			expectedAction: multistep.ActionHalt,
			expectError:    true,
			errorContains:  "error resolving datastore for storage policy \"gold-policy\"",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			state := basicStateBag(nil)
			state.Put("driver", c.driverMock)

			action := c.step.Run(context.TODO(), state)
			if action != c.expectedAction {
				t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", c.expectedAction, action)
			}

			if c.expectError {
				err, ok := state.Get("error").(error)
				if !ok {
					t.Fatal("expected error in state bag, but none found")
				}
				if !strings.Contains(err.Error(), c.errorContains) {
					t.Fatalf("unexpected error: expected to contain '%s', but got '%s'", c.errorContains, err.Error())
				}
			} else {
				if _, ok := state.GetOk("error"); ok {
					t.Fatal("unexpected error in state bag")
				}

				ds, ok := state.Get("datastore").(driver.Datastore)
				if !ok {
					t.Fatal("expected datastore in state bag, but none found")
				}
				if ds.Name() != c.expectedDatastore {
					t.Fatalf("unexpected datastore: expected '%s', but got '%s'", c.expectedDatastore, ds.Name())
				}

				method, ok := state.Get("datastore_selection_method").(string)
				if !ok {
					t.Fatal("expected datastore_selection_method in state bag, but none found")
				}
				if method != c.expectedMethod {
					t.Fatalf("unexpected selection method: expected '%s', but got '%s'", c.expectedMethod, method)
				}

				if c.step.Datastore == "" && c.step.DatastoreCluster == "" && c.step.StoragePolicy != "" {
					if got := StoragePolicyIDFromState(state); got != c.driverMock.FindStoragePolicyIDResult {
						t.Fatalf("unexpected storage_policy_id: expected %q, got %q", c.driverMock.FindStoragePolicyIDResult, got)
					}
				} else if _, ok := state.GetOk("storage_policy_id"); ok {
					t.Fatal("expected storage_policy_id unset when not using PBM placement")
				}
			}

			// Verify mock was called correctly
			if c.step.Datastore != "" && !c.driverMock.FindDatastoreCalled {
				t.Fatal("expected FindDatastore to be called, but it wasn't")
			}
			if c.step.DatastoreCluster != "" && !c.driverMock.SelectDatastoreCalled {
				t.Fatal("expected SelectDatastoreFromCluster to be called, but it wasn't")
			}
			if c.step.Datastore == "" && c.step.DatastoreCluster == "" && c.step.StoragePolicy != "" {
				if !c.driverMock.FindStoragePolicyIDCalled {
					t.Fatal("expected FindStoragePolicyID to be called, but it wasn't")
				}
				if !c.expectError && !c.driverMock.FindCompatibleDatastoreCalled {
					t.Fatal("expected FindCompatibleDatastore to be called, but it wasn't")
				}
			}
			if c.step.Datastore != "" && c.driverMock.FindCompatibleDatastoreCalled {
				t.Fatal("expected FindCompatibleDatastore not to be called when datastore is set")
			}
		})
	}
}

func TestStepResolveDatastore_NoDatastoreOrPolicyContinues(t *testing.T) {
	state := basicStateBag(nil)
	state.Put("driver", NewVCenterDriverMock())

	step := &StepResolveDatastore{}
	if action := step.Run(context.TODO(), state); action != multistep.ActionContinue {
		t.Fatalf("unexpected action: %#v", action)
	}
	if _, ok := state.GetOk("datastore"); ok {
		t.Fatal("expected no datastore in state when nothing is configured")
	}
}

func TestStorageConfig_FirstStoragePolicyName(t *testing.T) {
	cfg := &StorageConfig{
		Storage: []DiskConfig{
			{DiskSize: 1024},
			{DiskSize: 2048, StoragePolicyName: "gold"},
			{DiskSize: 4096, StoragePolicyName: "silver"},
		},
	}
	if got := cfg.FirstStoragePolicyName(); got != "gold" {
		t.Fatalf("unexpected first storage policy: got %q, want %q", got, "gold")
	}
	if got := (&StorageConfig{}).FirstStoragePolicyName(); got != "" {
		t.Fatalf("expected empty policy name, got %q", got)
	}
}

func TestStepResolveDatastore_Cleanup(t *testing.T) {
	step := &StepResolveDatastore{}
	state := basicStateBag(nil)

	// Cleanup should be a no-op
	step.Cleanup(state)
}

func TestResolveMultiDiskDatastorePlacement(t *testing.T) {
	ui := &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	}

	mock := NewVCenterDriverMock()
	ds1 := &driver.DatastoreMock{NameReturn: "datastore-1"}
	ds2 := &driver.DatastoreMock{NameReturn: "datastore-2"}
	mock.SelectDatastoresForDisksReturn = []driver.Datastore{ds1, ds2}
	mock.SelectDatastoresForDisksMethod = driver.SelectionMethodDRS

	input := driver.StoragePlacementInput{
		StorageConfig: driver.StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []driver.Disk{
				{DiskSize: 4096, ControllerUnit: "scsi0:1"},
				{DiskSize: 8192, ControllerUnit: "scsi0:2"},
			},
		},
	}

	datastoreName, placements := ResolveMultiDiskDatastorePlacement(
		ui, mock, "test-cluster", input, ds1, "initial-datastore",
	)

	if !mock.SelectDatastoresForDisksCalled {
		t.Fatal("expected SelectDatastoresForDisks to be called")
	}
	if len(mock.SelectDatastoresForDisksInput.StorageConfig.Storage) != 2 {
		t.Fatalf("expected placement input with 2 disks, got %d", len(mock.SelectDatastoresForDisksInput.StorageConfig.Storage))
	}
	if datastoreName != "datastore-1" {
		t.Fatalf("unexpected datastore name: %q", datastoreName)
	}
	if len(placements) != 2 {
		t.Fatalf("expected 2 placements, got %d", len(placements))
	}
	if placements[0].Name != "datastore-1" || placements[1].Name != "datastore-2" {
		t.Fatalf("unexpected placements: %+v", placements)
	}
}

func TestResolveMultiDiskDatastorePlacementSkipsSingleDisk(t *testing.T) {
	mock := NewVCenterDriverMock()
	input := driver.StoragePlacementInput{
		StorageConfig: driver.StorageConfig{
			Storage: []driver.Disk{{DiskSize: 4096}},
		},
	}

	_, placements := ResolveMultiDiskDatastorePlacement(
		&packersdk.BasicUi{Reader: new(bytes.Buffer), Writer: new(bytes.Buffer)},
		mock, "test-cluster", input, nil, "initial-datastore",
	)

	if mock.SelectDatastoresForDisksCalled {
		t.Fatal("expected SelectDatastoresForDisks not to be called for single disk")
	}
	if placements != nil {
		t.Fatal("expected nil placements for single disk")
	}
}

func TestResolveStoragePolicyDatastorePlacement_PerDisk(t *testing.T) {
	blueDS := &driver.DatastoreMock{NameReturn: "blue-ds"}
	greenDS := &driver.DatastoreMock{NameReturn: "green-ds"}
	redDS := &driver.DatastoreMock{NameReturn: "red-ds"}

	mock := NewVCenterDriverMock()
	mock.FindCompatibleDatastoreByPolicy = map[string]driver.Datastore{
		"policy-blue":  blueDS,
		"policy-green": greenDS,
		"policy-red":   redDS,
	}

	disks := []driver.Disk{
		{DiskSize: 4096, StoragePolicyID: "policy-blue"},
		{DiskSize: 4096, StoragePolicyID: "policy-green"},
		{DiskSize: 4096, StoragePolicyID: "policy-red"},
	}

	name, placements, err := ResolveStoragePolicyDatastorePlacement(
		mock, "esxi-1", "cluster-1", disks, blueDS, "blue-ds", "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "blue-ds" {
		t.Fatalf("expected primary datastore name blue-ds, got %q", name)
	}
	if len(placements) != 3 {
		t.Fatalf("expected 3 placements, got %d", len(placements))
	}
	want := []string{"blue-ds", "green-ds", "red-ds"}
	for i, w := range want {
		if placements[i].Name != w || placements[i].Ref == nil || placements[i].Ref.Value != w {
			t.Fatalf("placement %d: unexpected %+v, want %q", i, placements[i], w)
		}
	}
	if len(mock.FindCompatibleDatastoreCalls) != 3 {
		t.Fatalf("expected 3 PBM lookups, got %d: %v", len(mock.FindCompatibleDatastoreCalls), mock.FindCompatibleDatastoreCalls)
	}
}

func TestResolveStoragePolicyDatastorePlacement_CachesSamePolicy(t *testing.T) {
	goldDS := &driver.DatastoreMock{NameReturn: "gold-ds"}
	mock := NewVCenterDriverMock()
	mock.FindCompatibleDatastoreByPolicy = map[string]driver.Datastore{
		"policy-gold": goldDS,
	}

	disks := []driver.Disk{
		{DiskSize: 4096, StoragePolicyID: "policy-gold"},
		{DiskSize: 8192, StoragePolicyID: "policy-gold"},
	}

	_, placements, err := ResolveStoragePolicyDatastorePlacement(mock, "esxi-1", "", disks, goldDS, "gold-ds", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(placements) != 2 {
		t.Fatalf("expected 2 placements, got %d", len(placements))
	}
	if len(mock.FindCompatibleDatastoreCalls) != 1 {
		t.Fatalf("expected 1 cached PBM lookup, got %d", len(mock.FindCompatibleDatastoreCalls))
	}
}

func TestResolveStoragePolicyDatastorePlacement_SeedsPrimaryPolicy(t *testing.T) {
	blueDS := &driver.DatastoreMock{NameReturn: "blue-ds"}
	greenDS := &driver.DatastoreMock{NameReturn: "green-ds"}
	mock := NewVCenterDriverMock()
	mock.FindCompatibleDatastoreByPolicy = map[string]driver.Datastore{
		"policy-green": greenDS,
	}

	disks := []driver.Disk{
		{DiskSize: 4096, StoragePolicyID: "policy-blue"},
		{DiskSize: 4096, StoragePolicyID: "policy-green"},
	}

	_, placements, err := ResolveStoragePolicyDatastorePlacement(
		mock, "esxi-1", "", disks, blueDS, "blue-ds", "", "", "policy-blue",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if placements[0].Name != "blue-ds" || placements[1].Name != "green-ds" {
		t.Fatalf("unexpected placements: %+v", placements)
	}
	if len(mock.FindCompatibleDatastoreCalls) != 1 || mock.FindCompatibleDatastoreCalls[0] != "policy-green" {
		t.Fatalf("expected only green PBM lookup, got %v", mock.FindCompatibleDatastoreCalls)
	}
}

func TestResolveStoragePolicyDatastorePlacement_NoPolicyDiskUsesPrimary(t *testing.T) {
	primary := &driver.DatastoreMock{NameReturn: "primary-ds"}
	greenDS := &driver.DatastoreMock{NameReturn: "green-ds"}
	mock := NewVCenterDriverMock()
	mock.FindCompatibleDatastoreByPolicy = map[string]driver.Datastore{
		"policy-green": greenDS,
	}

	disks := []driver.Disk{
		{DiskSize: 4096},
		{DiskSize: 4096, StoragePolicyID: "policy-green"},
	}

	_, placements, err := ResolveStoragePolicyDatastorePlacement(mock, "esxi-1", "", disks, primary, "primary-ds", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if placements[0].Name != "primary-ds" || placements[1].Name != "green-ds" {
		t.Fatalf("unexpected placements: %+v", placements)
	}
}

func TestResolveStoragePolicyDatastorePlacement_SkipsDatastoreCluster(t *testing.T) {
	mock := NewVCenterDriverMock()
	disks := []driver.Disk{{DiskSize: 4096, StoragePolicyID: "policy-blue"}}

	_, placements, err := ResolveStoragePolicyDatastorePlacement(
		mock, "esxi-1", "", disks, nil, "ds", "", "my-cluster", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if placements != nil {
		t.Fatal("expected nil placements when datastore_cluster is set")
	}
	if mock.FindCompatibleDatastoreCalled {
		t.Fatal("expected FindCompatibleDatastore not to be called for datastore_cluster")
	}
}

func TestResolveStoragePolicyDatastorePlacement_SkipsExplicitDatastore(t *testing.T) {
	mock := NewVCenterDriverMock()
	disks := []driver.Disk{{DiskSize: 4096, StoragePolicyID: "policy-blue"}}

	_, placements, err := ResolveStoragePolicyDatastorePlacement(
		mock, "esxi-1", "", disks, nil, "explicit-ds", "explicit-ds", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if placements != nil {
		t.Fatal("expected nil placements when explicit datastore is set")
	}
	if mock.FindCompatibleDatastoreCalled {
		t.Fatal("expected FindCompatibleDatastore not to be called for explicit datastore")
	}
}

func TestResolveStoragePolicyDatastorePlacement_ErrorPropagates(t *testing.T) {
	mock := NewVCenterDriverMock()
	mock.FindCompatibleDatastoreErr = fmt.Errorf("no compatible datastore")
	disks := []driver.Disk{{DiskSize: 4096, StoragePolicyID: "policy-blue"}}

	_, _, err := ResolveStoragePolicyDatastorePlacement(
		mock, "esxi-1", "", disks, nil, "", "", "", "",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "policy-blue") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// VCenterDriverMock embeds DriverMock and adds VCenterDriver-specific methods for testing
type VCenterDriverMock struct {
	*driver.DriverMock

	SelectDatastoreCalled bool
	SelectDatastoreReturn driver.Datastore
	SelectDatastoreMethod string
	SelectDatastoreErr    error

	SelectDatastoresForDisksCalled bool
	SelectDatastoresForDisksInput  driver.StoragePlacementInput
	SelectDatastoresForDisksReturn []driver.Datastore
	SelectDatastoresForDisksMethod string
	SelectDatastoresForDisksErr    error
}

// NewVCenterDriverMock creates a new VCenterDriverMock
func NewVCenterDriverMock() *VCenterDriverMock {
	return &VCenterDriverMock{
		DriverMock: driver.NewDriverMock(),
	}
}

// SelectDatastoreFromCluster mocks the VCenterDriver method
func (d *VCenterDriverMock) SelectDatastoreFromCluster(clusterName string) (driver.Datastore, string, error) {
	d.SelectDatastoreCalled = true
	return d.SelectDatastoreReturn, d.SelectDatastoreMethod, d.SelectDatastoreErr
}

// SelectDatastoresForDisks mocks multi-disk Storage DRS placement.
func (d *VCenterDriverMock) SelectDatastoresForDisks(clusterName string, input driver.StoragePlacementInput) ([]driver.Datastore, string, error) {
	d.SelectDatastoresForDisksCalled = true
	d.SelectDatastoresForDisksInput = input
	if d.SelectDatastoresForDisksErr != nil {
		return nil, "", d.SelectDatastoresForDisksErr
	}
	return d.SelectDatastoresForDisksReturn, d.SelectDatastoresForDisksMethod, nil
}

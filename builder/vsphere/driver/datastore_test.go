// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestDatastoreIsoPath(t *testing.T) {
	tc := []struct {
		isoPath  string
		filePath string
		valid    bool
	}{
		{
			isoPath:  "[datastore] dir/subdir/file",
			filePath: "dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "[] dir/subdir/file",
			filePath: "dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "dir/subdir/file",
			filePath: "dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "[datastore] /dir/subdir/file",
			filePath: "/dir/subdir/file",
			valid:    true,
		},
		{
			isoPath: "/dir/subdir/file [datastore] ",
			valid:   false,
		},
		{
			isoPath: "[datastore][] /dir/subdir/file",
			valid:   false,
		},
		{
			isoPath: "[data/store] /dir/subdir/file",
			valid:   false,
		},
		{
			isoPath:  "[data store] /dir/sub dir/file",
			filePath: "/dir/sub dir/file",
			valid:    true,
		},
		{
			isoPath:  "   [datastore] /dir/subdir/file",
			filePath: "/dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "[datastore]    /dir/subdir/file",
			filePath: "/dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "[datastore] /dir/subdir/file     ",
			filePath: "/dir/subdir/file",
			valid:    true,
		},
		{
			isoPath:  "[привѣ́тъ] /привѣ́тъ/привѣ́тъ/привѣ́тъ",
			filePath: "/привѣ́тъ/привѣ́тъ/привѣ́тъ",
			valid:    true,
		},
		// Test case for #9846
		{
			isoPath:  "[ISO-StorageLun9] Linux/rhel-8.0-x86_64-dvd.iso",
			filePath: "Linux/rhel-8.0-x86_64-dvd.iso",
			valid:    true,
		},
	}

	for i, c := range tc {
		dsIsoPath := &DatastoreIsoPath{path: c.isoPath}
		if dsIsoPath.Validate() != c.valid {
			t.Fatalf("%d expected '%s' to be '%t', but returned '%t'", i, c.isoPath, c.valid, !c.valid)
		}
		if !c.valid {
			continue
		}
		filePath := dsIsoPath.GetFilePath()
		if filePath != c.filePath {
			t.Fatalf("%d expected '%s', but returned '%s'", i, c.filePath, filePath)
		}
	}
}

func TestVCenterDriver_FindDatastore(t *testing.T) {
	sim := mustVPXSimulator(t)

	_, datastore := mustPreCreatedDatastore(t, sim)
	_, host := mustPreCreatedHost(t, sim)

	tc := []struct {
		name       string
		datastore  string
		host       string
		fail       bool
		errMessage string
	}{
		{
			name:      "should find datastore when name is provided",
			datastore: datastore.Name,
			fail:      false,
		},
		{
			name: "should find datastore when only host is provided",
			host: host.Name,
			fail: false,
		},
		{
			name:      "should not find invalid datastore",
			datastore: "invalid",
			fail:      true,
		},
		{
			name: "should not find invalid host",
			host: "invalid",
			fail: true,
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			ds, err := newSimulatorDriver(sim).FindDatastore(c.datastore, c.host)
			if c.fail {
				if err == nil {
					t.Fatal("unexpected success: expected failure")
				}
				if c.errMessage != "" && err.Error() != c.errMessage {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.errMessage, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: '%s'", err)
				}
				if ds == nil {
					t.Fatalf("unexpected result: expected '%s', but returned '%s'", c.datastore, ds)
				}
			}
		})
	}
}

func TestVCenterDriver_MultipleDatastoreError(t *testing.T) {
	model := simulator.ESX()
	model.Datastore = 2
	sim := mustCustomSimulator(t, model)

	_, host := mustPreCreatedHost(t, sim)

	_, err := newSimulatorDriver(sim).FindDatastore("", host.Name)
	if err == nil {
		t.Fatal("unexpected success: expected failure")
	}
	if err.Error() != "host has multiple datastores; specify the datastore name" {
		t.Fatalf("unexpected error: '%s'", err)
	}
}

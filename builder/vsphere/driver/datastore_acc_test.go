// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDatastoreAcc(t *testing.T) {
	t.Skip("Acceptance tests not configured yet.")
	d := newTestDriver(t)
	ds, err := d.FindDatastore("datastore1", "")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	info, err := ds.Info("name")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if info.Name != "datastore1" {
		t.Errorf("unexpected result: expected '%s', but returned '%s'", "datastore1", info.Name)
	}
}

func TestFileUpload(t *testing.T) {
	t.Skip("Acceptance tests not configured yet.")
	dsName := "datastore1"
	hostName := "esx01.example.com"

	fileName := fmt.Sprintf("test-%v", time.Now().Unix())
	tmpFile, err := os.CreateTemp("", fileName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	d := newTestDriver(t)
	ds, err := d.FindDatastore(dsName, hostName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	err = ds.UploadFile(tmpFile.Name(), fileName, hostName, true)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if ds.FileExists(fileName) != true {
		t.Fatalf("unexpected result: expected 'true', but returned '%t'", ds.FileExists(fileName))
	}

	err = ds.Delete(fileName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
}

func TestFileUploadDRS(t *testing.T) {
	t.Skip("Acceptance tests not configured yet.")
	dsName := "datastore3"
	hostName := ""

	fileName := fmt.Sprintf("test-%v", time.Now().Unix())
	tmpFile, err := os.CreateTemp("", fileName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	d := newTestDriver(t)
	ds, err := d.FindDatastore(dsName, hostName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	err = ds.UploadFile(tmpFile.Name(), fileName, hostName, false)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if ds.FileExists(fileName) != false {
		t.Fatalf("unexpected error: expected 'true', but returned '%t'", ds.FileExists(fileName))
	}

	err = ds.Delete(fileName)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
}

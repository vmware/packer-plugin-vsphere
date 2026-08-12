// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestStepRemoteUpload_Run(t *testing.T) {
	state := basicStateBag(nil)
	dsMock := driver.DatastoreMock{
		DirExistsReturn: false,
	}
	driverMock := driver.NewDriverMock()
	driverMock.DatastoreMock = &dsMock
	state.Put("driver", driverMock)
	state.Put("iso_path", "[datastore] iso/path")

	step := &StepRemoteUpload{
		Datastore:                  "datastore",
		Host:                       "host",
		SetHostForDatastoreUploads: false,
	}

	if action := step.Run(context.Background(), state); action == multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	if !driverMock.FindDatastoreCalled {
		t.Fatalf("unexpected result: '%s' should be called", "FindDatastore")
	}
	if !driverMock.DatastoreMock.FileExistsCalled {
		t.Fatalf("unexpected result: '%s' should be called", "FileExists")
	}
	if !driverMock.DatastoreMock.MakeDirectoryCalled {
		t.Fatalf("unexpected result: '%s' should be called", "MakeDirectory")
	}
	if !driverMock.DatastoreMock.UploadFileCalled {
		t.Fatalf("unexpected result: '%s' should be called", "UploadFile")
	}
	remotePath, ok := state.GetOk("iso_remote_path")
	if !ok {
		t.Fatalf("unexpected state: '%s' not found", "iso_remote_path")
	}
	expectedRemovePath := fmt.Sprintf("[%s] packer_cache/path", driverMock.DatastoreMock.Name())
	if remotePath != expectedRemovePath {
		t.Fatalf("unexpected result: expected '%s', but returned '%s' for '%s'", expectedRemovePath, remotePath, "iso_remote_path")
	}
}

func TestStepRemoteUpload_SkipRun(t *testing.T) {
	state := basicStateBag(nil)
	driverMock := driver.NewDriverMock()
	state.Put("driver", driverMock)

	step := &StepRemoteUpload{}

	if action := step.Run(context.Background(), state); action == multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	if driverMock.FindDatastoreCalled {
		t.Fatalf("unexpected result: '%s' should not be called", "FindDatastore")
	}
	if _, ok := state.GetOk("iso_remote_path"); ok {
		t.Fatalf("unexpected state: '%s' should not be found", "iso_remote_path")
	}
}

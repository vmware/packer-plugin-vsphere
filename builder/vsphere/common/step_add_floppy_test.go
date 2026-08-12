// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestStepAddFloppy_Run(t *testing.T) {
	tc := []struct {
		name               string
		floppyPath         string
		uploadedPath       string
		step               *StepAddFloppy
		expectedAction     multistep.StepAction
		vmMock             *driver.VirtualMachineMock
		expectedVmMock     *driver.VirtualMachineMock
		driverMock         *driver.DriverMock
		expectedDriverMock *driver.DriverMock
		dsMock             *driver.DatastoreMock
		expectedDsMock     *driver.DatastoreMock
		fail               bool
		errMessage         string
	}{
		{
			name:         "Add floppy from state floppy path",
			floppyPath:   "floppy/path",
			uploadedPath: "vm/dir/packer-*.flp",
			step: &StepAddFloppy{
				Config:                     new(FloppyConfig),
				Datastore:                  "datastore",
				Host:                       "host",
				SetHostForDatastoreUploads: true,
			},
			expectedAction: multistep.ActionContinue,
			vmMock: &driver.VirtualMachineMock{
				GetDirResponse: "vm/dir",
			},
			expectedVmMock: &driver.VirtualMachineMock{
				GetDirResponse:     "vm/dir",
				GetDirCalled:       true,
				AddFloppyCalled:    true,
				AddFloppyImagePath: "resolved/path",
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				ResolvePathReturn: "resolved/path",
			},
			expectedDsMock: &driver.DatastoreMock{
				UploadFileCalled:  true,
				UploadFileSrc:     "floppy/path",
				UploadFileDst:     "vm/dir/packer-*.flp",
				UploadFileHost:    "host",
				UploadFileSetHost: true,
				ResolvePathCalled: true,
				ResolvePathReturn: "resolved/path",
			},
			fail: false,
		},
		{
			name:       "State floppy path - find datastore fail",
			floppyPath: "floppy/path",
			step: &StepAddFloppy{
				Config:                     new(FloppyConfig),
				Datastore:                  "datastore",
				Host:                       "host",
				SetHostForDatastoreUploads: true,
			},
			expectedAction: multistep.ActionHalt,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: new(driver.VirtualMachineMock),
			driverMock: &driver.DriverMock{
				FindDatastoreErr: fmt.Errorf("error finding datastore"),
			},
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock:         new(driver.DatastoreMock),
			expectedDsMock: new(driver.DatastoreMock),
			fail:           true,
			errMessage:     "error finding datastore",
		},
		{
			name:       "State floppy path - vm get dir fail",
			floppyPath: "floppy/path",
			step: &StepAddFloppy{
				Config:                     new(FloppyConfig),
				Datastore:                  "datastore",
				Host:                       "host",
				SetHostForDatastoreUploads: true,
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				GetDirErr: fmt.Errorf("fail to get vm dir"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				GetDirCalled: true,
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock:         new(driver.DatastoreMock),
			expectedDsMock: new(driver.DatastoreMock),
			fail:           true,
			errMessage:     "fail to get vm dir",
		},
		{
			name:       "State floppy path - datastore upload file fail",
			floppyPath: "floppy/path",
			step: &StepAddFloppy{
				Config:                     new(FloppyConfig),
				Datastore:                  "datastore",
				Host:                       "host",
				SetHostForDatastoreUploads: true,
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				GetDirResponse: "vm/dir",
			},
			expectedVmMock: &driver.VirtualMachineMock{
				GetDirResponse: "vm/dir",
				GetDirCalled:   true,
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				UploadFileErr: fmt.Errorf("failed to upload file"),
			},
			expectedDsMock: &driver.DatastoreMock{
				UploadFileCalled:  true,
				UploadFileSrc:     "floppy/path",
				UploadFileDst:     "vm/dir/packer-*.flp",
				UploadFileHost:    "host",
				UploadFileSetHost: true,
			},
			fail:       true,
			errMessage: "failed to upload file",
		},
		{
			name:         "State floppy path - vm fail to add floppy",
			floppyPath:   "floppy/path",
			uploadedPath: "vm/dir/packer-*.flp",
			step: &StepAddFloppy{
				Config:                     new(FloppyConfig),
				Datastore:                  "datastore",
				Host:                       "host",
				SetHostForDatastoreUploads: true,
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				GetDirResponse: "vm/dir",
				AddFloppyErr:   fmt.Errorf("failed to add floppy"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				GetDirResponse:     "vm/dir",
				GetDirCalled:       true,
				AddFloppyCalled:    true,
				AddFloppyImagePath: "resolved/path",
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				ResolvePathReturn: "resolved/path",
			},
			expectedDsMock: &driver.DatastoreMock{
				UploadFileCalled:  true,
				UploadFileSrc:     "floppy/path",
				UploadFileDst:     "vm/dir/packer-*.flp",
				UploadFileHost:    "host",
				UploadFileSetHost: true,
				ResolvePathCalled: true,
				ResolvePathReturn: "resolved/path",
			},
			fail:       true,
			errMessage: "failed to add floppy",
		},
		{
			name: "Add floppy from FloppyIMGPath config",
			step: &StepAddFloppy{
				Config: &FloppyConfig{
					FloppyIMGPath: "floppy/image/path",
				},
			},
			expectedAction: multistep.ActionContinue,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: &driver.VirtualMachineMock{
				AddFloppyCalled:    true,
				AddFloppyImagePath: "floppy/image/path",
			},
			driverMock:         new(driver.DriverMock),
			expectedDriverMock: new(driver.DriverMock),
			dsMock:             new(driver.DatastoreMock),
			expectedDsMock:     new(driver.DatastoreMock),
			fail:               false,
		},
		{
			name: "Fail to add floppy from FloppyIMGPath config",
			step: &StepAddFloppy{
				Config: &FloppyConfig{
					FloppyIMGPath: "floppy/image/path",
				},
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				AddFloppyErr: fmt.Errorf("fail to add floppy"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				AddFloppyCalled:    true,
				AddFloppyImagePath: "floppy/image/path",
			},
			driverMock:         new(driver.DriverMock),
			expectedDriverMock: new(driver.DriverMock),
			dsMock:             new(driver.DatastoreMock),
			expectedDsMock:     new(driver.DatastoreMock),
			fail:               true,
			errMessage:         "fail to add floppy",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			state := basicStateBag(nil)
			state.Put("vm", c.vmMock)
			c.driverMock.DatastoreMock = c.dsMock
			state.Put("driver", c.driverMock)

			if c.floppyPath != "" {
				state.Put("floppy_path", c.floppyPath)
			}

			if action := c.step.Run(context.Background(), state); action != c.expectedAction {
				t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", c.expectedAction, action)
			}
			err, ok := state.Get("error").(error)
			if ok {
				if err.Error() != c.errMessage {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.errMessage, err)
				}
			} else {
				if c.fail {
					t.Fatal("unexpected success: expected failure")
				}
			}

			if c.driverMock.DatastoreMock.UploadFileDst != "" {
				pattern := regexp.MustCompile(`vm/dir/packer-(\d{10}|tmp-created-floppy)\.flp`)
				if !pattern.MatchString(c.driverMock.DatastoreMock.UploadFileDst) {
					t.Fatalf("unexpected result: expected '%s' to match pattern '%s'", c.driverMock.DatastoreMock.UploadFileDst, pattern)
				}
				c.driverMock.DatastoreMock.UploadFileDst = "vm/dir/packer-*.flp"
			}

			if diff := cmp.Diff(c.vmMock, c.expectedVmMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected '%s' calls: %s", "VirtualMachine", diff)
			}
			c.expectedDriverMock.DatastoreMock = c.expectedDsMock
			if diff := cmp.Diff(c.driverMock, c.expectedDriverMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected '%s' calls: %s", "Driver", diff)
			}
			if diff := cmp.Diff(c.dsMock, c.expectedDsMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected '%s' calls: %s", "Datastore", diff)
			}
		})
	}
}

func TestStepAddFloppy_Cleanup(t *testing.T) {
	tc := []struct {
		name               string
		uploadedPath       string
		multistepState     string
		step               *StepAddFloppy
		driverMock         *driver.DriverMock
		expectedDriverMock *driver.DriverMock
		dsMock             *driver.DatastoreMock
		expectedDsMock     *driver.DatastoreMock
		fail               bool
		errMessage         string
	}{
		{
			name:           "State cancelled clean up",
			uploadedPath:   "uploaded/path",
			multistepState: multistep.StateCancelled,
			step: &StepAddFloppy{
				Datastore: "datastore",
				Host:      "host",
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				DeleteCalled: true,
			},
			expectedDsMock: &driver.DatastoreMock{
				DeleteCalled: true,
				DeletePath:   "uploaded/path",
			},
			fail: false,
		},
		{
			name:           "State halted clean up",
			uploadedPath:   "uploaded/path",
			multistepState: multistep.StateHalted,
			step: &StepAddFloppy{
				Datastore: "datastore",
				Host:      "host",
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				DeleteCalled: true,
			},
			expectedDsMock: &driver.DatastoreMock{
				DeleteCalled: true,
				DeletePath:   "uploaded/path",
			},
			fail: false,
		},
		{
			name:               "Don't clean up without uploaded path",
			multistepState:     multistep.StateHalted,
			step:               new(StepAddFloppy),
			driverMock:         new(driver.DriverMock),
			expectedDriverMock: new(driver.DriverMock),
			dsMock:             new(driver.DatastoreMock),
			expectedDsMock:     new(driver.DatastoreMock),
			fail:               false,
		},
		{
			name:               "Don't clean up if state is not halted or canceled",
			multistepState:     "",
			step:               new(StepAddFloppy),
			driverMock:         new(driver.DriverMock),
			expectedDriverMock: new(driver.DriverMock),
			dsMock:             new(driver.DatastoreMock),
			expectedDsMock:     new(driver.DatastoreMock),
			fail:               false,
		},
		{
			name:           "Fail because datastore is not found",
			uploadedPath:   "uploaded/path",
			multistepState: multistep.StateHalted,
			step: &StepAddFloppy{
				Datastore: "datastore",
				Host:      "host",
			},
			driverMock: &driver.DriverMock{
				FindDatastoreErr: fmt.Errorf("fail to find datastore"),
			},
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock:         new(driver.DatastoreMock),
			expectedDsMock: new(driver.DatastoreMock),
			fail:           true,
			errMessage:     "fail to find datastore",
		},
		{
			name:           "Fail to delete floppy",
			uploadedPath:   "uploaded/path",
			multistepState: multistep.StateHalted,
			step: &StepAddFloppy{
				Datastore: "datastore",
				Host:      "host",
			},
			driverMock: new(driver.DriverMock),
			expectedDriverMock: &driver.DriverMock{
				FindDatastoreCalled: true,
				FindDatastoreName:   "datastore",
				FindDatastoreHost:   "host",
			},
			dsMock: &driver.DatastoreMock{
				DeleteCalled: true,
				DeleteErr:    fmt.Errorf("failed to delete floppy"),
			},
			expectedDsMock: &driver.DatastoreMock{
				DeleteCalled: true,
				DeletePath:   "uploaded/path",
			},
			fail:       true,
			errMessage: "failed to delete floppy",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			state := basicStateBag(nil)
			c.driverMock.DatastoreMock = c.dsMock
			state.Put("driver", c.driverMock)
			if c.uploadedPath != "" {
				state.Put("uploaded_floppy_path", c.uploadedPath)
			}

			if c.multistepState != "" {
				state.Put(c.multistepState, true)
			}

			c.step.Cleanup(state)
			err, ok := state.Get("error").(error)
			if ok {
				if err.Error() != c.errMessage {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.errMessage, err)
				}
			} else {
				if c.fail {
					t.Fatal("unexpected success: expected failure")
				}
			}

			c.expectedDriverMock.DatastoreMock = c.expectedDsMock
			if diff := cmp.Diff(c.driverMock, c.expectedDriverMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected result: %s", diff)
			}
			if diff := cmp.Diff(c.dsMock, c.expectedDsMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected result: %s", diff)
			}
		})
	}
}

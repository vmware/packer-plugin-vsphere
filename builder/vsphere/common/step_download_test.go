// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"net/url"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// / create mock step
type MockDownloadStep struct {
	RunCalled bool
}

func (s *MockDownloadStep) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	s.RunCalled = true
	return multistep.ActionContinue
}

func (s *MockDownloadStep) Cleanup(state multistep.StateBag) {}

func (s *MockDownloadStep) UseSourceToFindCacheTarget(source string) (*url.URL, string, error) {
	return nil, "sometarget", nil
}

// / start tests
func downloadStepState(exists bool) *multistep.BasicStateBag {
	state := basicStateBag(nil)
	dsMock := &driver.DatastoreMock{
		FileExistsReturn: exists,
	}
	driverMock := &driver.DriverMock{
		DatastoreMock: dsMock,
	}
	state.Put("driver", driverMock)
	return state
}

func TestStepDownload_Run(t *testing.T) {
	testcases := []struct {
		name                     string
		filePresent              bool
		expectedAction           multistep.StepAction
		expectInternalStepCalled bool
		errMessage               string
	}{
		{
			name:                     "Remote iso present; download shouldn't be called",
			filePresent:              true,
			expectedAction:           multistep.ActionContinue,
			expectInternalStepCalled: false,
			errMessage:               "",
		},
		{
			name:                     "Remote iso not present; download should be called",
			filePresent:              false,
			expectedAction:           multistep.ActionContinue,
			expectInternalStepCalled: true,
			errMessage:               "",
		},
	}
	for _, tc := range testcases {
		internalStep := &MockDownloadStep{}
		state := downloadStepState(tc.filePresent)
		step := &StepDownload{
			DownloadStep: internalStep,
			Url:          []string{"https://path/to/fake-url.iso"},
			Datastore:    "datastore-mock",
			Host:         "fake-host",
		}
		stepAction := step.Run(context.TODO(), state)
		if stepAction != tc.expectedAction {
			t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", tc.expectedAction, stepAction)
		}
		if tc.expectInternalStepCalled != internalStep.RunCalled {
			if tc.expectInternalStepCalled {
				t.Fatalf("unexpected result: expected '%s' to be called", tc.name)
			}

			t.Fatalf("unexpected result: expected '%s' not to be called", tc.name)
		}
	}
}

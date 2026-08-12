// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestStepRemoveNetworkAdapter_Run(t *testing.T) {
	tc := []struct {
		name           string
		step           *StepRemoveNetworkAdapter
		expectedAction multistep.StepAction
		vmMock         *driver.VirtualMachineMock
		expectedVmMock *driver.VirtualMachineMock
		errMessage     string
	}{
		{
			name: "Successfully remove network adapters.",
			step: &StepRemoveNetworkAdapter{
				Config: &RemoveNetworkAdapterConfig{
					RemoveNetworkAdapter: true,
				},
			},
			expectedAction: multistep.ActionContinue,
			vmMock: &driver.VirtualMachineMock{
				RemoveNetworkAdaptersCalled: true,
			},
			expectedVmMock: &driver.VirtualMachineMock{
				RemoveNetworkAdaptersCalled: true,
			},
		},
		{
			name: "Fail to remove network adapters.",
			step: &StepRemoveNetworkAdapter{
				Config: &RemoveNetworkAdapterConfig{
					RemoveNetworkAdapter: true,
				},
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				RemoveNetworkAdaptersCalled: true,
				RemoveNetworkAdaptersErr:    fmt.Errorf("failed to remove network adapters"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				RemoveNetworkAdaptersCalled: true,
			},
			errMessage: "error removing network adapters: failed to remove network adapters",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			state := basicStateBag(nil)
			state.Put("vm", c.vmMock)

			if action := c.step.Run(context.Background(), state); action != c.expectedAction {
				t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", c.expectedAction, action)
			}
			err, ok := state.Get("error").(error)
			if ok {
				if err.Error() != c.errMessage {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.errMessage, err)
				}
			} else if c.errMessage != "" {
				t.Fatalf("unexpected success, expected error: '%s'", c.errMessage)
			}

			if diff := cmp.Diff(c.vmMock, c.expectedVmMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected '%s' calls: %s", "VirtualMachine", diff)
			}
		})
	}
}

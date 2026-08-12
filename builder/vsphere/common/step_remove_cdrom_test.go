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

func TestStepRemoveCDRom_Run(t *testing.T) {
	tc := []struct {
		name           string
		step           *StepRemoveCDRom
		expectedAction multistep.StepAction
		vmMock         *driver.VirtualMachineMock
		expectedVmMock *driver.VirtualMachineMock
		fail           bool
		errMessage     string
	}{
		{
			name: "Successfully eject CD-ROM devices",
			step: &StepRemoveCDRom{
				Config: &RemoveCDRomConfig{},
			},
			expectedAction: multistep.ActionContinue,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: &driver.VirtualMachineMock{
				EjectCdromsCalled: true,
			},
			fail: false,
		},
		{
			name: "Fail to eject CD-ROM devices",
			step: &StepRemoveCDRom{
				Config: &RemoveCDRomConfig{},
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				EjectCdromsErr: fmt.Errorf("failed to eject cdrom media"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				EjectCdromsCalled: true,
			},
			fail:       true,
			errMessage: "error ejecting cdrom media: failed to eject cdrom media",
		},
		{
			name: "Successfully eject and delete CD-ROM devices",
			step: &StepRemoveCDRom{
				Config: &RemoveCDRomConfig{
					RemoveCdrom: true,
				},
			},
			expectedAction: multistep.ActionContinue,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: &driver.VirtualMachineMock{
				RemoveCdromsCalled: true,
				EjectCdromsCalled:  true,
			},
			fail: false,
		},
		{
			name: "Fail to delete CD-ROM devices",
			step: &StepRemoveCDRom{
				Config: &RemoveCDRomConfig{
					RemoveCdrom: true,
				},
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				RemoveCdromsErr: fmt.Errorf("failed to delete cdrom devices"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				EjectCdromsCalled:  true,
				RemoveCdromsCalled: true,
			},
			fail:       true,
			errMessage: "error removing cdrom: failed to delete cdrom devices",
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
			} else {
				if c.fail {
					t.Fatal("unexpected success: expected failure")
				}
			}

			if diff := cmp.Diff(c.vmMock, c.expectedVmMock,
				cmpopts.IgnoreInterfaces(struct{ error }{})); diff != "" {
				t.Fatalf("unexpected '%s' calls: %s", "VirtualMachine", diff)
			}
		})
	}
}

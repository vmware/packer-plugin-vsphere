// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"bytes"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func cleanupTestState(mockVM driver.VirtualMachine) multistep.StateBag {
	state := new(multistep.BasicStateBag)
	state.Put("vm", mockVM)
	state.Put("ui", &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	})
	return state
}

func Test_CleanupVM(t *testing.T) {
	type testCase struct {
		Reason        string
		ExtraState    map[string]any
		ExpectDestroy bool
	}
	testCases := []testCase{
		{
			"if cancelled, we should destroy the VM",
			map[string]any{multistep.StateCancelled: true},
			true,
		},
		{
			"if halted, we should destroy the VM",
			map[string]any{multistep.StateHalted: true},
			true,
		},
		{
			"if destroy flag is set, we should destroy the VM",
			map[string]any{"destroy_vm": true},
			true,
		},
		{
			"if none of the above flags are set, we should not destroy the VM",
			map[string]any{},
			false,
		},
	}
	for _, tc := range testCases {
		mockVM := &driver.VirtualMachineMock{}
		state := cleanupTestState(mockVM)
		for k, v := range tc.ExtraState {
			state.Put(k, v)
		}
		CleanupVM(state)
		if mockVM.DestroyCalled != tc.ExpectDestroy {
			t.Fatalf("unexpected result: expected '%s' to be called", "Destroy")
		}
	}

}

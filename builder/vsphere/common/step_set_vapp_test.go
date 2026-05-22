// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type vAppVMMock struct {
	driver.VirtualMachineMock
	setCalled bool
	props     map[string]string
}

func (m *vAppVMMock) SetVAppProperties(ctx context.Context, props map[string]string) error {
	m.setCalled = true
	m.props = props
	return nil
}

func TestStepSetVAppSkipsWhenInactive(t *testing.T) {
	state := basicStateBag(&strings.Builder{})
	state.Put("vm", &vAppVMMock{})

	step := &StepSetVApp{Config: &VAppConfig{}}
	if step.Run(context.Background(), state) != multistep.ActionContinue {
		t.Fatal("expected continue")
	}
}

func TestStepSetVAppAppliesProperties(t *testing.T) {
	vm := &vAppVMMock{}
	state := basicStateBag(&strings.Builder{})
	state.Put("vm", vm)

	step := &StepSetVApp{
		Config: &VAppConfig{
			Properties: map[string]string{"hostname": "host1"},
		},
	}
	if step.Run(context.Background(), state) != multistep.ActionContinue {
		t.Fatal("expected continue")
	}
	if !vm.setCalled {
		t.Fatal("expected SetVAppProperties to be called")
	}
	if vm.props["hostname"] != "host1" {
		t.Fatalf("unexpected properties: %#v", vm.props)
	}
}

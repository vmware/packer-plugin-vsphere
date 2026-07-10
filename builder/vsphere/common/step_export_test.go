// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/types"
)

func TestValidateExportOptions(t *testing.T) {
	available := []types.OvfOptionInfo{
		{Option: "mac"},
		{Option: "uuid"},
		{Option: "extraconfig"},
		{Option: "nodevicesubtypes"},
	}

	tests := []struct {
		name          string
		requested     []string
		available     []types.OvfOptionInfo
		expectError   bool
		errorContains []string
	}{
		{
			name:        "no requested options",
			requested:   nil,
			available:   available,
			expectError: false,
		},
		{
			name:        "all requested options are valid",
			requested:   []string{"mac", "uuid"},
			available:   available,
			expectError: false,
		},
		{
			name:          "single unknown option",
			requested:     []string{"uuidx"},
			available:     available,
			expectError:   true,
			errorContains: []string{"unknown export options: uuidx", "available options: mac, uuid, extraconfig, nodevicesubtypes"},
		},
		{
			name:          "multiple unknown options",
			requested:     []string{"mac", "bad1", "bad2"},
			available:     available,
			expectError:   true,
			errorContains: []string{"unknown export options: bad1, bad2", "available options: mac, uuid, extraconfig, nodevicesubtypes"},
		},
		{
			name:          "unknown option with empty available list",
			requested:     []string{"mac"},
			available:     nil,
			expectError:   true,
			errorContains: []string{"unknown export options: mac", "available options:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExportOptions(tt.requested, tt.available)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				for _, want := range tt.errorContains {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("expected error to contain %q, got %q", want, err.Error())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

type exportStepVMMock struct {
	exportOptions          []types.OvfOptionInfo
	exportOptionsErr       error
	exportCalled           bool
	getExportOptionsCalled bool
}

func (m *exportStepVMMock) Export() (*nfc.Lease, error) {
	m.exportCalled = true
	return nil, fmt.Errorf("export should not be called")
}

func (m *exportStepVMMock) NewOvfManager() *ovf.Manager {
	return nil
}

func (m *exportStepVMMock) GetOvfExportOptions(_ *ovf.Manager) ([]types.OvfOptionInfo, error) {
	m.getExportOptionsCalled = true
	return m.exportOptions, m.exportOptionsErr
}

func (m *exportStepVMMock) CreateDescriptor(_ *ovf.Manager, _ types.OvfCreateDescriptorParams) (*types.OvfCreateDescriptorResult, error) {
	return nil, fmt.Errorf("create descriptor should not be called")
}

func TestStepExport_Run_unknownExportOptions(t *testing.T) {
	vmMock := &exportStepVMMock{
		exportOptions: []types.OvfOptionInfo{
			{Option: "mac"},
			{Option: "uuid"},
		},
	}

	step := &StepExport{
		Name:    "test-vm",
		Options: []string{"mac", "uuidx"},
	}

	var errorBuf strings.Builder
	state := basicStateBag(&errorBuf)
	state.Put("vm", vmMock)

	action := step.Run(context.Background(), state)
	if action != multistep.ActionHalt {
		t.Fatalf("expected ActionHalt, got %v", action)
	}
	if !vmMock.getExportOptionsCalled {
		t.Fatal("expected GetOvfExportOptions to be called")
	}
	if vmMock.exportCalled {
		t.Fatal("expected Export not to be called when export options are invalid")
	}

	err, ok := state.Get("error").(error)
	if !ok {
		t.Fatal("expected error in state bag")
	}
	if !strings.Contains(err.Error(), "unknown export options: uuidx") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "available options: mac, uuid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

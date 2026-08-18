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

func TestStepRemoveVTPM_Run(t *testing.T) {
	tc := []struct {
		name           string
		step           *StepRemoveVTPM
		expectedAction multistep.StepAction
		vmMock         *driver.VirtualMachineMock
		expectedVmMock *driver.VirtualMachineMock
		errMessage     string
	}{
		{
			name: "Skip when remove_vtpm is false.",
			step: &StepRemoveVTPM{
				Config: &RemoveVTPMConfig{
					RemoveVTPM: false,
				},
			},
			expectedAction: multistep.ActionContinue,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: new(driver.VirtualMachineMock),
		},
		{
			name: "Successfully remove vTPM.",
			step: &StepRemoveVTPM{
				Config: &RemoveVTPMConfig{
					RemoveVTPM: true,
				},
			},
			expectedAction: multistep.ActionContinue,
			vmMock:         new(driver.VirtualMachineMock),
			expectedVmMock: &driver.VirtualMachineMock{
				RemoveVTPMCalled: true,
			},
		},
		{
			name: "Fail to remove vTPM.",
			step: &StepRemoveVTPM{
				Config: &RemoveVTPMConfig{
					RemoveVTPM: true,
				},
			},
			expectedAction: multistep.ActionHalt,
			vmMock: &driver.VirtualMachineMock{
				RemoveVTPMErr: fmt.Errorf("failed to remove vTPM"),
			},
			expectedVmMock: &driver.VirtualMachineMock{
				RemoveVTPMCalled: true,
			},
			errMessage: "error removing vTPM: failed to remove vTPM",
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

func TestRemoveVTPMConfig_Prepare(t *testing.T) {
	const (
		warnExport = "vTPM is enabled and OVF/OVA export is configured; this will fail unless 'remove_vtpm' is true"
		warnCL     = "vTPM is enabled and content library OVF template import is configured; this will fail unless 'remove_vtpm' is true"
		warnBoth   = "vTPM is enabled and OVF/OVA export is configured and content library OVF template import is configured; this will fail unless 'remove_vtpm' is true"
	)
	tc := []struct {
		name              string
		config            RemoveVTPMConfig
		vtpmEnabled       bool
		exportOVF         bool
		contentLibraryOVF bool
		wantWarning       string
	}{
		{
			name:        "No warning when vTPM is disabled.",
			vtpmEnabled: false,
			exportOVF:   true,
		},
		{
			name:        "No warning when neither OVF path is configured.",
			vtpmEnabled: true,
		},
		{
			name: "No warning when remove_vtpm is true.",
			config: RemoveVTPMConfig{
				RemoveVTPM: true,
			},
			vtpmEnabled:       true,
			exportOVF:         true,
			contentLibraryOVF: true,
		},
		{
			name:        "Warn for OVF/OVA export.",
			vtpmEnabled: true,
			exportOVF:   true,
			wantWarning: warnExport,
		},
		{
			name:              "Warn for content library OVF template.",
			vtpmEnabled:       true,
			contentLibraryOVF: true,
			wantWarning:       warnCL,
		},
		{
			name:              "Warn for export and content library OVF template.",
			vtpmEnabled:       true,
			exportOVF:         true,
			contentLibraryOVF: true,
			wantWarning:       warnBoth,
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			warnings := c.config.Prepare(c.vtpmEnabled, c.exportOVF, c.contentLibraryOVF)
			if c.wantWarning == "" {
				if len(warnings) != 0 {
					t.Fatalf("expected no warnings, got %#v", warnings)
				}
				return
			}
			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %#v", warnings)
			}
			if warnings[0] != c.wantWarning {
				t.Fatalf("unexpected warning: %s", warnings[0])
			}
		})
	}
}

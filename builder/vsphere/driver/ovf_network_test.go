// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestBuildOvfNetworkMappings(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name         string
		ovfNetworks  []types.OvfNetworkInfo
		network      string
		expectError  bool
		errorMsg     string
		expectMapped int
		expectNames  []string
	}{
		{
			name:         "No OVF networks",
			ovfNetworks:  nil,
			network:      "VM Network",
			expectMapped: 0,
		},
		{
			name: "Single OVF network mapped",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Management Network"},
			},
			network:      "VM Network",
			expectMapped: 1,
			expectNames:  []string{"Management Network"},
		},
		{
			name: "Multiple OVF networks mapped to same vSphere network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Management Network"},
				{Name: "VM Network"},
			},
			network:      "VM Network",
			expectMapped: 2,
			expectNames:  []string{"Management Network", "VM Network"},
		},
		{
			name: "OVF networks require configured network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Guest Network"},
			},
			network:     "",
			expectError: true,
			errorMsg:    "OVF requires network mapping",
		},
		{
			name: "Invalid vSphere network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Guest Network"},
			},
			network:     "nonexistent-network",
			expectError: true,
			errorMsg:    "error finding network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappings, err := driver.buildOvfNetworkMappings(tt.ovfNetworks, tt.network)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if len(mappings) != tt.expectMapped {
				t.Fatalf("expected %d network mappings, got %d", tt.expectMapped, len(mappings))
			}

			for i, expectedName := range tt.expectNames {
				if mappings[i].Name != expectedName {
					t.Errorf("mapping[%d].Name = %q, want %q", i, mappings[i].Name, expectedName)
				}
			}
		})
	}
}

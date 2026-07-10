// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/vapi/vcenter"
)

func TestValidateContentLibraryDeploymentOption(t *testing.T) {
	tests := []struct {
		name        string
		option      string
		params      []vcenter.AdditionalParams
		expectError bool
		errorMsg    string
	}{
		{
			name:   "Valid deployment option",
			option: "small",
			params: []vcenter.AdditionalParams{
				{
					Class: vcenter.ClassDeploymentOptionParams,
					DeploymentOptions: []vcenter.DeploymentOption{
						{Key: "small"},
						{Key: "medium"},
					},
				},
			},
			expectError: false,
		},
		{
			name:   "Invalid deployment option",
			option: "xlarge",
			params: []vcenter.AdditionalParams{
				{
					Class: vcenter.ClassDeploymentOptionParams,
					DeploymentOptions: []vcenter.DeploymentOption{
						{Key: "small"},
						{Key: "medium"},
					},
				},
			},
			expectError: true,
			errorMsg:    "deployment option 'xlarge' not found in OVF",
		},
		{
			name:        "No deployment option params",
			option:      "small",
			params:      nil,
			expectError: true,
			errorMsg:    "deployment option 'small' specified but OVF does not define any deployment options",
		},
		{
			name:   "Only non-deployment option params",
			option: "small",
			params: []vcenter.AdditionalParams{
				{
					Class: vcenter.ClassPropertyParams,
				},
			},
			expectError: true,
			errorMsg:    "deployment option 'small' specified but OVF does not define any deployment options",
		},
		{
			name:   "Deployment option params with empty options list",
			option: "small",
			params: []vcenter.AdditionalParams{
				{
					Class:             vcenter.ClassDeploymentOptionParams,
					DeploymentOptions: nil,
				},
			},
			expectError: true,
			errorMsg:    "deployment option 'small' specified but OVF does not define any deployment options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContentLibraryDeploymentOption(tt.option, tt.params)
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
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateContentLibraryOvfConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *ContentLibraryDeployConfig
		filter      vcenter.FilterResponse
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid deployment option and vApp properties",
			config: &ContentLibraryDeployConfig{
				DeploymentOption: "small",
				VAppProperties: map[string]string{
					"hostname": "example",
				},
			},
			filter: vcenter.FilterResponse{
				AdditionalParams: []vcenter.AdditionalParams{
					{
						Class: vcenter.ClassDeploymentOptionParams,
						DeploymentOptions: []vcenter.DeploymentOption{
							{Key: "small"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty vApp property key",
			config: &ContentLibraryDeployConfig{
				VAppProperties: map[string]string{
					"": "value",
				},
			},
			expectError: true,
			errorMsg:    "vApp property key cannot be empty",
		},
		{
			name: "vApp property key too long",
			config: &ContentLibraryDeployConfig{
				VAppProperties: map[string]string{
					strings.Repeat("a", 256): "value",
				},
			},
			expectError: true,
			errorMsg:    "exceeds maximum length of 255 characters",
		},
		{
			name: "vApp property value too long",
			config: &ContentLibraryDeployConfig{
				VAppProperties: map[string]string{
					"hostname": strings.Repeat("a", 65536),
				},
			},
			expectError: true,
			errorMsg:    "exceeds maximum length of 65535 characters",
		},
		{
			name: "Deployment option with no Filter options",
			config: &ContentLibraryDeployConfig{
				DeploymentOption: "small",
			},
			filter:      vcenter.FilterResponse{},
			expectError: true,
			errorMsg:    "deployment option 'small' specified but OVF does not define any deployment options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContentLibraryOvfConfig(tt.config, tt.filter)
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
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildContentLibraryOvfNetworkMappings(t *testing.T) {
	d := &VCenterDriver{}

	t.Run("No OVF networks", func(t *testing.T) {
		mappings, err := d.buildContentLibraryOvfNetworkMappings("VM Network", vcenter.FilterResponse{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mappings != nil {
			t.Fatalf("expected nil mappings, got %#v", mappings)
		}
	})

	t.Run("OVF networks require network config", func(t *testing.T) {
		_, err := d.buildContentLibraryOvfNetworkMappings("", vcenter.FilterResponse{
			Networks: []string{"VM Network", "Management"},
		})
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if !strings.Contains(err.Error(), "OVF requires network mapping for VM Network, Management") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

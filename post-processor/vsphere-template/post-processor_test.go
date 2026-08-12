// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"testing"
)

func getTestConfig() Config {
	return Config{
		Username: "administrator@vsphere.local",
		Password: "password",
		Host:     "vc01.example.com",
	}
}

func TestConfigure_Valid(t *testing.T) {
	var p PostProcessor

	config := getTestConfig()

	err := p.Configure(config)
	if err != nil {
		t.Errorf("error: %s", err)
	}
}

func TestConfigure_ReregisterVM_Default(t *testing.T) {
	var p PostProcessor

	config := getTestConfig()

	err := p.Configure(config)
	if err != nil {
		t.Errorf("error: %s", err)
	}

	if p.config.ReregisterVM.False() {
		t.Errorf("error: should be unset, not false")
	}
}

func TestConfigure_Override(t *testing.T) {
	trueVal := true
	falseVal := false
	tests := []struct {
		name     string
		override *bool
		expected bool
	}{
		{
			name:     "default",
			override: nil,
			expected: false,
		},
		{
			name:     "true",
			override: &trueVal,
			expected: true,
		},
		{
			name:     "false",
			override: &falseVal,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p PostProcessor
			config := getTestConfig()

			if tt.override != nil {
				config.Override = *tt.override
			}

			err := p.Configure(config)
			if err != nil {
				t.Errorf("error: %s", err)
			}

			if p.config.Override != tt.expected {
				t.Errorf("expected override to be %v, got %v", tt.expected, p.config.Override)
			}
		})
	}
}

func TestConfigure_Tags(t *testing.T) {
	tests := []struct {
		name        string
		tags        []string
		tagBlocks   []map[string]any
		expectError bool
	}{
		{
			name:        "no tags",
			tags:        nil,
			tagBlocks:   nil,
			expectError: false,
		},
		{
			name: "valid tag IDs",
			tags: []string{
				"urn:vmomi:InventoryServiceTag:12345678-1234-1234-1234-123456789012:GLOBAL",
			},
			tagBlocks:   nil,
			expectError: false,
		},
		{
			name: "valid tag blocks",
			tags: nil,
			tagBlocks: []map[string]any{
				{"category": "environment", "name": "production"},
			},
			expectError: false,
		},
		{
			name: "invalid tag ID format",
			tags: []string{
				"invalid-tag-id",
			},
			tagBlocks:   nil,
			expectError: true,
		},
		{
			name: "empty tag ID",
			tags: []string{
				"",
			},
			tagBlocks:   nil,
			expectError: true,
		},
		{
			name: "tag block missing category",
			tags: nil,
			tagBlocks: []map[string]any{
				{"name": "production"},
			},
			expectError: true,
		},
		{
			name: "tag block missing name",
			tags: nil,
			tagBlocks: []map[string]any{
				{"category": "environment"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p PostProcessor
			config := getTestConfig()

			// Build raw config
			rawConfig := map[string]any{
				"username": config.Username,
				"password": config.Password,
				"host":     config.Host,
			}

			if tt.tags != nil {
				rawConfig["tags"] = tt.tags
			}

			if tt.tagBlocks != nil {
				rawConfig["tag"] = tt.tagBlocks
			}

			err := p.Configure(rawConfig)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

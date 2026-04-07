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
			override: &[]bool{true}[0],
			expected: true,
		},
		{
			name:     "false",
			override: &[]bool{false}[0],
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

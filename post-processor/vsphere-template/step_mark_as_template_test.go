// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/template/config"
)

func TestStepMarkAsTemplate_Override(t *testing.T) {
	tests := []struct {
		name     string
		override bool
		expected bool
	}{
		{
			name:     "default",
			override: false,
			expected: false,
		},
		{
			name:     "enabled",
			override: true,
			expected: true,
		},
		{
			name:     "disabled",
			override: false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PostProcessor{
				config: Config{
					Override: tt.override,
				},
			}

			artifact := &mockArtifact{
				builderId: "vsphere-iso",
				id:        "test-vm",
			}

			step := NewStepMarkAsTemplate(artifact, p)

			if step.Override != tt.expected {
				t.Errorf("Expected Override to be %v, got %v", tt.expected, step.Override)
			}
		})
	}
}

func TestStepMarkAsTemplate_TemplateName(t *testing.T) {
	tests := []struct {
		name         string
		vmName       string
		templateName string
		expected     string
	}{
		{
			name:         "Use template name when provided",
			vmName:       "test-vm",
			templateName: "custom-template",
			expected:     "custom-template",
		},
		{
			name:         "Use VM name when template name is empty",
			vmName:       "test-vm",
			templateName: "",
			expected:     "test-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &StepMarkAsTemplate{
				VMName:       tt.vmName,
				TemplateName: tt.templateName,
				Override:     false,
				ReregisterVM: config.TriFalse,
			}

			templateName := step.VMName
			if step.TemplateName != "" {
				templateName = step.TemplateName
			}

			if templateName != tt.expected {
				t.Errorf("Expected template name to be %s, got %s", tt.expected, templateName)
			}
		})
	}
}

type mockArtifact struct {
	builderId string
	id        string
	state     map[string]any
}

func (m *mockArtifact) BuilderId() string {
	return m.builderId
}

func (m *mockArtifact) Files() []string {
	return []string{}
}

func (m *mockArtifact) Id() string {
	return m.id
}

func (m *mockArtifact) String() string {
	return m.id
}

func (m *mockArtifact) State(name string) any {
	if m.state == nil {
		return nil
	}
	return m.state[name]
}

func (m *mockArtifact) Destroy() error {
	return nil
}

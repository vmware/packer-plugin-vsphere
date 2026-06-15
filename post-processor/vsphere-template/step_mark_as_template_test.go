// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/template/config"
)

func TestDatastorePath(t *testing.T) {
	tests := []struct {
		name        string
		disk        string
		wantDS      string
		wantPathSfx string // suffix that vmxPath should end with
		wantErr     bool
	}{
		{
			name:        "valid disk path",
			disk:        "[datastore01] vm/test-vm/test-vm.vmdk",
			wantDS:      "datastore01",
			wantPathSfx: "vm/test-vm/test-vm.vmx",
			wantErr:     false,
		},
		{
			name:    "missing datastore brackets",
			disk:    "vm/test-vm/test-vm.vmdk",
			wantErr: true,
		},
		{
			name:    "empty datastore name",
			disk:    "[] vm/test-vm/test-vm.vmdk",
			wantErr: true,
		},
		{
			name:    "no space separator",
			disk:    "[datastore01]vm/test-vm/test-vm.vmdk",
			wantErr: true,
		},
		{
			name:    "only datastore bracket no path",
			disk:    "[datastore01] ",
			wantErr: true,
		},
		{
			name:        "disk path with leading whitespace after bracket",
			disk:        "[datastore01]   vm/test-vm/test-vm.vmdk",
			wantDS:      "datastore01",
			wantPathSfx: "vm/test-vm/test-vm.vmx",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := datastorePathFromDisk(tt.disk, "test-vm")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if result.Datastore != tt.wantDS {
				t.Errorf("expected datastore %q, got %q", tt.wantDS, result.Datastore)
			}
			if tt.wantPathSfx != "" && result.Path[len(result.Path)-len(tt.wantPathSfx):] != tt.wantPathSfx {
				t.Errorf("expected path suffix %q, got path %q", tt.wantPathSfx, result.Path)
			}
		})
	}
}

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

			if got := step.getEffectiveTemplateName(); got != tt.expected {
				t.Errorf("expected template name %q, got %q", tt.expected, got)
			}
		})
	}
}

type mockArtifact struct {
	builderId string
	id        string
	state     map[string]interface{}
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

func (m *mockArtifact) State(name string) interface{} {
	if m.state == nil {
		return nil
	}
	return m.state[name]
}

func (m *mockArtifact) Destroy() error {
	return nil
}

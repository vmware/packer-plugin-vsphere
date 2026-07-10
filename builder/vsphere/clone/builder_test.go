// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestCloneBuilder_ImplementsBuilder(t *testing.T) {
	var _ packersdk.Builder = &Builder{}
}

func TestSourceArtifactStateData(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantKey string
		wantVal string
	}{
		{
			name: "content library source",
			config: &Config{
				CloneConfig: CloneConfig{
					ContentLibrarySource: &ContentLibrarySourceConfig{
						Library: "Example Content Library",
						Name:    "example-template",
					},
				},
			},
			wantKey: "source_content_library",
			wantVal: "Example Content Library/example-template",
		},
		{
			name: "template source",
			config: &Config{
				CloneConfig: CloneConfig{
					Template: "inventory-template",
				},
			},
			wantKey: "source_template",
			wantVal: "inventory-template",
		},
		{
			name: "ovf path source",
			config: &Config{
				CloneConfig: CloneConfig{
					OvfSource: &OvfSourceConfig{
						Path: "./artifacts/example.ovf",
					},
				},
			},
			wantKey: "source_ovf_path",
			wantVal: "./artifacts/example.ovf",
		},
		{
			name: "ovf url source",
			config: &Config{
				CloneConfig: CloneConfig{
					OvfSource: &OvfSourceConfig{
						URL: "https://packages.example.com/artifacts/example.ovf",
					},
				},
			},
			wantKey: "source_ovf_url",
			wantVal: "https://packages.example.com/artifacts/example.ovf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sourceArtifactStateData(tt.config)
			if len(got) != 1 {
				t.Fatalf("expected 1 source key, got %#v", got)
			}
			value, ok := got[tt.wantKey]
			if !ok {
				t.Fatalf("expected key %q in %#v", tt.wantKey, got)
			}
			if value != tt.wantVal {
				t.Fatalf("expected %q, got %#v", tt.wantVal, value)
			}
		})
	}
}

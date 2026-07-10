// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"strings"
	"testing"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
)

func TestCloneConfig_prepareStorage(t *testing.T) {
	testCases := []struct {
		name           string
		config         *CloneConfig
		fail           bool
		expectedErrMsg string
	}{
		{
			name: "explicit unit without disk_controller_type",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:           4096,
							DiskControllerUnit: "scsi0:1",
						},
					},
				},
			},
		},
		{
			name: "invalid controller type",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"invalid_type"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "unsupported controller type",
		},
		{
			name: "duplicate disk_controller_unit",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi0:1"},
						{DiskSize: 4096, DiskControllerUnit: "scsi0:1"},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "already assigned",
		},
		{
			name: "mutually exclusive unit and index",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            4096,
							DiskControllerUnit:  "scsi0:1",
							DiskControllerIndex: 1,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "mutually exclusive",
		},
		{
			name: "legacy path requires disk_controller_type",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{DiskSize: 4096},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "'disk_controller_type' is required",
		},
		{
			name: "reserved scsi unit 7",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi0:7"},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "unit 7 is reserved",
		},
		{
			name: "pvscsi unit 64 passes prepare",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi1:64"},
					},
				},
			},
		},
		{
			name: "lsilogic unit 64 fails prepare",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"lsilogic"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi1:64"},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "unit must be between 0 and 15 for LSI Logic controllers",
		},
		{
			name: "mixed legacy and explicit",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi", "pvscsi"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096},
						{DiskSize: 4096, DiskControllerUnit: "scsi0:1"},
					},
				},
			},
		},
		{
			// Regression test: both disks share the controller created for
			// scsi0 (from diskControllerTypes[0], "pvscsi"), so the second
			// disk must be validated against PVSCSI's 64-unit limit, not
			// diskControllerTypes[1] ("lsilogic"). Previously this failed
			// because Prepare() advanced its type index once per disk instead
			// of once per distinct new bus.
			name: "multiple disks on shared new bus use the same controller type",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi", "lsilogic"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi0:1"},
						{DiskSize: 4096, DiskControllerUnit: "scsi0:50"},
					},
				},
			},
		},
		{
			// Distinct buses must still consume distinct disk_controller_type
			// entries in order of first appearance: scsi0 gets "pvscsi" and
			// scsi1 gets "lsilogic", so unit 20 on scsi1 correctly fails
			// against LSI Logic's 15-unit limit.
			name: "distinct new buses map to distinct disk_controller_type entries in order",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi", "lsilogic"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi0:1"},
						{DiskSize: 4096, DiskControllerUnit: "scsi1:20"},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "unit must be between 0 and 15 for LSI Logic controllers",
		},
		{
			// Regression test: disk_controller_type entries are consumed in
			// bus-reference order across ALL controller kinds, not per-kind.
			// nvme0 is referenced first here, so it lands on
			// diskControllerTypes[0] ("lsilogic"), a SCSI-family type. This
			// must be rejected at Prepare() time instead of only failing
			// during Clone() with a low-level controller mismatch error.
			name: "new bus referenced out of kind order fails prepare",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"lsilogic", "nvme"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "nvme0:0"},
						{DiskSize: 4096, DiskControllerUnit: "scsi1:0"},
					},
				},
			},
			fail:           true,
			expectedErrMsg: `disk_controller_type[0] is "lsilogic"`,
		},
		{
			// Correctly ordered mixed-kind config: scsi1 is referenced
			// first (index 0, "pvscsi"), sata0 next (index 1, "sata"), and
			// nvme0 last (index 2, "nvme").
			name: "mixed controller kinds referenced in matching order pass prepare",
			config: &CloneConfig{
				Template: "template",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi", "sata", "nvme"},
					Storage: []common.DiskConfig{
						{DiskSize: 4096, DiskControllerUnit: "scsi1:0"},
						{DiskSize: 4096, DiskControllerUnit: "sata0:0"},
						{DiskSize: 4096, DiskControllerUnit: "nvme0:0"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.config.prepareStorage()
			if tc.fail {
				if len(errs) == 0 {
					t.Fatal("expected failure")
				}
				if tc.expectedErrMsg != "" && !strings.Contains(errs[0].Error(), tc.expectedErrMsg) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedErrMsg, errs[0].Error())
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("unexpected error: %s", errs[0])
			}
		})
	}
}

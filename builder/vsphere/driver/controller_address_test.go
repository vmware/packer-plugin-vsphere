// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"strings"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestParseControllerUnit(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    ControllerAddress
		wantErr bool
	}{
		{
			name:  "scsi",
			input: "scsi0:1",
			want:  ControllerAddress{Kind: ControllerKindSCSI, Bus: 0, Unit: 1},
		},
		{
			name:  "nvme",
			input: "nvme1:0",
			want:  ControllerAddress{Kind: ControllerKindNVMe, Bus: 1, Unit: 0},
		},
		{
			name:  "sata",
			input: "sata2:29",
			want:  ControllerAddress{Kind: ControllerKindSATA, Bus: 2, Unit: 29},
		},
		{
			name:    "invalid format",
			input:   "scsi0-1",
			wantErr: true,
		},
		{
			name:    "invalid bus",
			input:   "scsi9:0",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseControllerUnit(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected result: expected %#v, got %#v", tc.want, got)
			}
		})
	}
}

func TestValidateControllerUnitStatic(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "valid pvscsi unit", input: "scsi1:8"},
		{name: "valid nvme unit", input: "nvme0:14"},
		{name: "reserved scsi unit 7", input: "scsi0:7", wantErr: "unit 7 is reserved"},
		{name: "scsi unit too high", input: "scsi0:16", wantErr: "unit must be between 0 and 15 for SCSI controllers"},
		{name: "nvme unit too high", input: "nvme0:15", wantErr: "unit must be between 0 and 14 for NVMe controllers"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateControllerUnitStatic(tc.input)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateControllerUnitStaticForConfig(t *testing.T) {
	testCases := []struct {
		name                string
		input               string
		diskControllerTypes []string
		typeIndex           int
		wantErr             string
	}{
		{
			name:                "pvscsi unit 64 with pvscsi type",
			input:               "scsi1:64",
			diskControllerTypes: []string{"pvscsi"},
			wantErr:             "",
		},
		{
			name:                "pvscsi unit 64 with lsilogic type",
			input:               "scsi1:64",
			diskControllerTypes: []string{"lsilogic"},
			wantErr:             "unit must be between 0 and 15 for LSI Logic controllers",
		},
		{
			name:                "explicit on-demand pvscsi after legacy",
			input:               "scsi2:32",
			diskControllerTypes: []string{"pvscsi", "pvscsi", "pvscsi"},
			// All three disk_controller_type entries are consumed by legacy
			// storage blocks, so this address falls past the end of the list
			// and is validated against the PVSCSI fallback.
			typeIndex: 3,
			wantErr:   "",
		},
		{
			name:                "second disk on same new bus uses same type as first",
			input:               "scsi0:50",
			diskControllerTypes: []string{"pvscsi", "lsilogic"},
			// Both disks addressing scsi0 share the controller created from
			// diskControllerTypes[0] ("pvscsi"), so the second disk must also
			// be validated against PVSCSI's limits, not diskControllerTypes[1].
			typeIndex: 0,
			wantErr:   "",
		},
		{
			name:                "nvme address mapped to a scsi type entry",
			input:               "nvme0:0",
			diskControllerTypes: []string{"lsilogic", "nvme"},
			// nvme0 is referenced before scsi1 in this hypothetical config,
			// so it lands on index 0 ("lsilogic"), a SCSI-family type. That
			// mismatch must be reported clearly instead of silently
			// validating nvme0 against NVMe limits and letting it fail at
			// clone time with a low-level bus mismatch error.
			typeIndex: 0,
			wantErr:   `disk_controller_type[0] is "lsilogic"`,
		},
		{
			name:                "sata address mapped to an nvme type entry",
			input:               "sata0:0",
			diskControllerTypes: []string{"nvme"},
			typeIndex:           0,
			wantErr:             `disk_controller_type[0] is "nvme"`,
		},
		{
			name:                "scsi address mapped to a sata type entry",
			input:               "scsi1:0",
			diskControllerTypes: []string{"sata"},
			typeIndex:           0,
			wantErr:             `disk_controller_type[0] is "sata"`,
		},
		{
			name:                "correctly ordered mixed kinds",
			input:               "nvme0:0",
			diskControllerTypes: []string{"pvscsi", "sata", "nvme"},
			typeIndex:           2,
			wantErr:             "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateControllerUnitStaticForConfig(tc.input, tc.diskControllerTypes, tc.typeIndex)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestListAvailableUnits(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController("lsilogic")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	controllerObj, err := devices.FindDiskController(devices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: devices.NewKey(),
		},
		CapacityInKB: 1024,
	}
	assignDiskAtUnit(devices, disk, controllerObj, 1, true)
	devices = append(devices, disk)

	available := listAvailableUnits(devices, controllerObj)
	if len(available) == 0 {
		t.Fatal("expected available units")
	}
	for _, unit := range available {
		if unit == 1 {
			t.Fatal("unit 1 should be occupied")
		}
		if unit == 7 {
			t.Fatal("unit 7 should be reserved for SCSI")
		}
	}
}

func TestValidateStorageCapacity(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController("lsilogic")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	controllerObj, err := devices.FindDiskController(devices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	for unit := 0; unit <= 15; unit++ {
		if unit == 7 {
			continue
		}
		disk := &types.VirtualDisk{
			VirtualDevice: types.VirtualDevice{
				Key: devices.NewKey(),
			},
			CapacityInKB: 1024,
		}
		assignDiskAtUnit(devices, disk, controllerObj, int32(unit), true)
		devices = append(devices, disk)
	}

	err = validateControllerDiskCapacity(devices, controllerObj, 1)
	if err == nil || !strings.Contains(err.Error(), "supports maximum 15 disks") {
		t.Fatalf("expected capacity error, got %v", err)
	}
}

func TestControllerDisplayName(t *testing.T) {
	testCases := []struct {
		controllerType string
		want           string
	}{
		{controllerType: controllerTypePVSCSI, want: "PVSCSI"},
		{controllerType: controllerTypeLSILogic, want: "LSI Logic"},
		{controllerType: controllerTypeLSILogicSAS, want: "LSI Logic SAS"},
	}

	for _, tc := range testCases {
		t.Run(tc.controllerType, func(t *testing.T) {
			devices := object.VirtualDeviceList{}
			controller, err := devices.CreateSCSIController(tc.controllerType)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			devices = append(devices, controller)

			controllerObj, err := devices.FindDiskController(devices.Name(controller))
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if got := controllerDisplayName(controllerObj); got != tc.want {
				t.Fatalf("unexpected display name: expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDisplayNameForControllerType(t *testing.T) {
	testCases := []struct {
		controllerType string
		want           string
	}{
		{controllerType: controllerTypePVSCSI, want: "PVSCSI"},
		{controllerType: controllerTypeLSILogic, want: "LSI Logic"},
		{controllerType: controllerTypeSCSI, want: "SCSI"},
		{controllerType: "unknown", want: "SCSI"},
	}

	for _, tc := range testCases {
		t.Run(tc.controllerType, func(t *testing.T) {
			if got := displayNameForControllerType(tc.controllerType); got != tc.want {
				t.Fatalf("unexpected display name: expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestFindControllerByBus(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	found := FindControllerByBus(devices, ControllerKindSCSI, 0)
	if found == nil {
		t.Fatal("expected to find scsi0 controller")
	}

	if FindControllerByBus(devices, ControllerKindSCSI, 1) != nil {
		t.Fatal("expected scsi1 controller to be missing")
	}
}

func TestValidateControllerUnitRuntimeOccupied(t *testing.T) {
	devices := object.VirtualDeviceList{}
	controller, err := devices.CreateSCSIController("pvscsi")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	devices = append(devices, controller)

	controllerObj, err := devices.FindDiskController(devices.Name(controller))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: devices.NewKey(),
		},
		CapacityInKB: 1024,
	}
	assignDiskAtUnit(devices, disk, controllerObj, 1, true)
	devices = append(devices, disk)

	err = ValidateControllerUnitRuntime(devices, "scsi0:1", nil)
	if err == nil || !strings.Contains(err.Error(), "already in use") || !strings.Contains(err.Error(), "Available units:") {
		t.Fatalf("expected occupied unit error with available units, got %v", err)
	}
}

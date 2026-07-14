// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestHardwareConfig_Prepare(t *testing.T) {
	tc := []struct {
		name           string
		config         *HardwareConfig
		fail           bool
		expectedErrMsg string
	}{
		{
			name:   "Validate empty config",
			config: &HardwareConfig{},
			fail:   false,
		},
		{
			name: "Validate RAMReservation RAMReserveAll cannot be used together",
			config: &HardwareConfig{
				RAMReservation: 2,
				RAMReserveAll:  true,
			},
			fail:           true,
			expectedErrMsg: "'RAM_reservation' and 'RAM_reserve_all' cannot be used together",
		},
		{
			name: "Invalid firmware",
			config: &HardwareConfig{
				Firmware: "invalid",
			},
			fail:           true,
			expectedErrMsg: "'firmware' must be '', 'bios', 'efi' or 'efi-secure'",
		},
		{
			name: "Validate 'bios' firmware",
			config: &HardwareConfig{
				Firmware: "bios",
			},
			fail: false,
		},
		{
			name: "Validate 'efi' firmware",
			config: &HardwareConfig{
				Firmware: "efi",
			},
			fail: false,
		},
		{
			name: "Validate 'efi-secure' firmware",
			config: &HardwareConfig{
				Firmware: "efi-secure",
			},
			fail: false,
		},
		{
			name: "Validate 'vTPM' and 'efi' firmware",
			config: &HardwareConfig{
				Firmware:    "efi",
				VTPMEnabled: true,
			},
			fail: false,
		},
		{
			name: "Validate 'vTPM' and 'efi-secure' firmware",
			config: &HardwareConfig{
				Firmware:    "efi-secure",
				VTPMEnabled: true,
			},
			fail: false,
		},
		{
			name: "Validate 'vTPM' and unsupported firmware",
			config: &HardwareConfig{
				Firmware:    "bios",
				VTPMEnabled: true,
			},
			fail:           true,
			expectedErrMsg: "'vTPM' could be enabled only when 'firmware' set to 'efi' or 'efi-secure'",
		},
		{
			name: "Validate 'vTPM' and empty firmware",
			config: &HardwareConfig{
				VTPMEnabled: true,
			},
			fail:           true,
			expectedErrMsg: "'vTPM' could be enabled only when 'firmware' set to 'efi' or 'efi-secure'",
		},
		{
			name: "Validate 'precision_clock'",
			config: &HardwareConfig{
				VirtualPrecisionClock: "ntp",
			},
			fail: false,
		},
		{
			name: "Validate 'precision_clock' and invalid option",
			config: &HardwareConfig{
				VirtualPrecisionClock: "invalid",
			},
			fail:           true,
			expectedErrMsg: "'precision_clock' must be '', 'ptp', 'ntp', or 'none'",
		},
		{
			name: "Validate boot_delay",
			config: &HardwareConfig{
				BootDelay: 5000,
			},
			fail: false,
		},
		{
			name: "Validate maximum boot_delay",
			config: &HardwareConfig{
				BootDelay: 10000,
			},
			fail: false,
		},
		{
			name: "Validate negative boot_delay",
			config: &HardwareConfig{
				BootDelay: -1,
			},
			fail:           true,
			expectedErrMsg: "'boot_delay' must be between 0 and 10000 milliseconds",
		},
		{
			name: "Validate boot_delay above maximum",
			config: &HardwareConfig{
				BootDelay: 10001,
			},
			fail:           true,
			expectedErrMsg: "'boot_delay' must be between 0 and 10000 milliseconds",
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			errs := c.config.Prepare()
			if c.fail {
				if len(errs) == 0 {
					t.Fatal("unexpected success: expected failure")
				}
				if errs[0].Error() != c.expectedErrMsg {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.expectedErrMsg, errs[0])
				}
			} else {
				if len(errs) != 0 {
					t.Fatalf("unexpected failure: expected success, but failed: %s", errs[0])
				}
			}
		})
	}
}

func TestStepConfigureHardware_Run(t *testing.T) {
	tc := []struct {
		name            string
		step            *StepConfigureHardware
		action          multistep.StepAction
		configureError  error
		configureCalled bool
		hardwareConfig  *driver.HardwareConfig
	}{
		{
			name:            "Configure hardware",
			step:            basicStepConfigureHardware(),
			action:          multistep.ActionContinue,
			configureError:  nil,
			configureCalled: true,
			hardwareConfig:  driverHardwareConfigFromConfig(basicStepConfigureHardware().Config),
		},
		{
			name:            "Configure hardware even when config is empty (preserves template settings)",
			step:            &StepConfigureHardware{Config: &HardwareConfig{}},
			action:          multistep.ActionContinue,
			configureError:  nil,
			configureCalled: true,
			hardwareConfig:  driverHardwareConfigFromConfig(&HardwareConfig{}),
		},
		{
			name:            "Halt when configure return error",
			step:            basicStepConfigureHardware(),
			action:          multistep.ActionHalt,
			configureError:  errors.New("failed to configure"),
			configureCalled: true,
			hardwareConfig:  driverHardwareConfigFromConfig(basicStepConfigureHardware().Config),
		},
	}
	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			state := basicStateBag(nil)
			vmMock := new(driver.VirtualMachineMock)
			vmMock.ConfigureError = c.configureError
			state.Put("vm", vmMock)

			action := c.step.Run(context.TODO(), state)
			if action != c.action {
				t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", c.action, action)
			}
			if vmMock.ConfigureCalled != c.configureCalled {
				t.Fatalf("unexpected result: expected '%t', but returned '%t'", c.configureCalled, vmMock.ConfigureCalled)
			}
			if diff := cmp.Diff(vmMock.ConfigureHardwareConfig, c.hardwareConfig); diff != "" {
				t.Fatalf("unexpected result: '%s'", diff)
			}

			err, ok := state.GetOk("error")
			containsError := c.configureError != nil
			if containsError != ok {
				t.Fatalf("unexpected result: expected '%t', but returned '%t'", ok, containsError)
			}
			if containsError {
				if !strings.Contains(err.(error).Error(), c.configureError.Error()) {
					t.Fatalf("unexpected error: expected '%s', but returned '%s'", c.configureError.Error(), err.(error).Error())
				}
			}
		})
	}
}

func basicStepConfigureHardware() *StepConfigureHardware {
	return &StepConfigureHardware{
		Config: &HardwareConfig{
			CPUs:           1,
			CpuCores:       1,
			CPUReservation: 1,
			CPULimit:       4000,
			RAM:            1024,
			RAMReserveAll:  true,
			Firmware:       "efi-secure",
			ForceBIOSSetup: true,
			AllowedDevices: []PCIPassthroughAllowedDevice{
				{
					VendorId:    "8086",
					DeviceId:    "100e",
					SubVendorId: "8086",
					SubDeviceId: "100e",
				},
				{
					VendorId:    "8087",
					DeviceId:    "100f",
					SubVendorId: "8087",
					SubDeviceId: "100f",
				},
			},
		},
	}
}

func driverHardwareConfigFromConfig(config *HardwareConfig) *driver.HardwareConfig {

	var allowedDevices []driver.PCIPassthroughAllowedDevice
	for _, device := range config.AllowedDevices {
		allowedDevices = append(allowedDevices, driver.PCIPassthroughAllowedDevice(device))
	}

	return &driver.HardwareConfig{
		CPUs:                  config.CPUs,
		CpuCores:              config.CpuCores,
		CPUReservation:        config.CPUReservation,
		CPULimit:              config.CPULimit,
		RAM:                   config.RAM,
		RAMReservation:        config.RAMReservation,
		RAMReserveAll:         config.RAMReserveAll,
		NestedHV:              config.NestedHV,
		CpuHotAddEnabled:      config.CpuHotAddEnabled,
		MemoryHotAddEnabled:   config.MemoryHotAddEnabled,
		VideoRAM:              config.VideoRAM,
		Displays:              config.Displays,
		AllowedDevices:        allowedDevices,
		VGPUProfile:           config.VGPUProfile,
		Firmware:              config.Firmware,
		ForceBIOSSetup:        config.ForceBIOSSetup,
		BootDelay:             config.BootDelay,
		VTPMEnabled:           config.VTPMEnabled,
		VirtualPrecisionClock: config.VirtualPrecisionClock,
	}
}

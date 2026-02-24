// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type HardwareConfig,PCIPassthroughAllowedDevice

package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// Dynamic DirectPath I/O is the component of the Assignable Hardware framework in VMware vSphere.
// Dynamic DirectPath I/O enables the Assignable Hardware intelligence for passthrough devices and
// the hardware address of the PCIe device is no longer directly mapped to the virtual machine
// configuration. Instead, the attributes, or capabilities, are exposed to the virtual machine.
//
// # JSON
//
// ```json
//
//	{
//	  "pci_passthrough_allowed_device": {
//	    "vendor_id": "8086",
//	    "device_id": "100e",
//	    "sub_device_id": "8086",
//	    "sub_vendor_id": "100e"
//	  }
//	}
//
// ```
//
// # HCL2
//
// ```hcl
//
//	pci_passthrough_allowed_device {
//	  "vendor_id": "8086",
//	  "device_id": "100e",
//	  "sub_device_id": "8086",
//	  "sub_vendor_id": "100e"
//	}
//
// ```
type PCIPassthroughAllowedDevice struct {
	// The sub-vendor ID of the PCI device.
	VendorId string `mapstructure:"vendor_id"`
	// The vendor ID of the PCI device.
	DeviceId string `mapstructure:"device_id"`
	// The sub-vendor ID of the PCI device.
	SubVendorId string `mapstructure:"sub_vendor_id"`
	// The sub-device ID of the PCI device.
	SubDeviceId string `mapstructure:"sub_device_id"`
}

type HardwareConfig struct {
	// The number of virtual CPUs cores for the virtual machine.
	CPUs int32 `mapstructure:"CPUs"`
	// The number of virtual CPU cores per socket for the virtual machine.
	CpuCores int32 `mapstructure:"cpu_cores"`
	// The CPU reservation in MHz.
	CPUReservation int64 `mapstructure:"CPU_reservation"`
	// The upper limit of available CPU resources in MHz.
	CPULimit int64 `mapstructure:"CPU_limit"`
	// Enable CPU hot plug setting for virtual machine. Defaults to `false`
	CpuHotAddEnabled bool `mapstructure:"CPU_hot_plug"`
	// The amount of memory for the virtual machine in MB.
	RAM int64 `mapstructure:"RAM"`
	// The guaranteed minimum allocation of memory for the virtual machine in MB.
	RAMReservation int64 `mapstructure:"RAM_reservation"`
	// Reserve all allocated memory. Defaults to `false`.
	//
	// -> **Note:** May not be used together with `RAM_reservation`.
	RAMReserveAll bool `mapstructure:"RAM_reserve_all"`
	// Enable memory hot add setting for virtual machine. Defaults to `false`.
	MemoryHotAddEnabled bool `mapstructure:"RAM_hot_plug"`
	// The amount of video memory in KB. Defaults to 4096 KB.
	//
	// -> **Note:** Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/virtual-machine-compatibilityvsphere-vm-admin/hardware-features-available-with-virtual-machine-compatibility-levelsvsphere-vm-admin.html)
	// for supported maximums.
	VideoRAM int64 `mapstructure:"video_ram"`
	// The number of video displays. Defaults to `1`.
	//
	//`-> **Note:** Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/virtual-machine-compatibilityvsphere-vm-admin/hardware-features-available-with-virtual-machine-compatibility-levelsvsphere-vm-admin.html)
	// for supported maximums.
	Displays int32 `mapstructure:"displays"`
	// Configure Dynamic DirectPath I/O [PCI Passthrough](#pci-passthrough-configuration) for
	// virtual machine. Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/other-virtual-machine-device-configurationvsphere-vm-admin/add-a-pci-device-to-a-virutal-machinevsphere-vm-admin.html)
	AllowedDevices []PCIPassthroughAllowedDevice `mapstructure:"pci_passthrough_allowed_device"`
	// vGPU profile for accelerated graphics. Refer to the [NVIDIA GRID vGPU documentation](https://docs.nvidia.com/grid/latest/grid-vgpu-user-guide/index.html#configure-vmware-vsphere-vm-with-vgpu)
	// for examples of profile names. Defaults to none.
	VGPUProfile string `mapstructure:"vgpu_profile"`
	// Enable nested hardware virtualization for the virtual machine.
	NestedHV bool `mapstructure:"NestedHV"`
	// The firmware for the virtual machine.
	//
	// The available options for this setting are: 'bios', 'efi', and
	// 'efi-secure'.
	//
	// -> **Note:** Use `efi-secure` for UEFI Secure Boot.
	Firmware string `mapstructure:"firmware"`
	// Force entry into the BIOS setup screen during boot. Defaults to `false`.
	ForceBIOSSetup bool `mapstructure:"force_bios_setup"`
	// Enable virtual trusted platform module (TPM) device for the virtual
	// machine. Defaults to `false`.
	VTPMEnabled bool `mapstructure:"vTPM"`
	// The virtual precision clock device for the virtual machine.
	// Defaults to `none`.
	//
	// The available options for this setting are: `none`, `ntp`, and `ptp`.
	VirtualPrecisionClock string `mapstructure:"precision_clock"`
}

func (c *HardwareConfig) Prepare() []error {
	var errs []error

	if c.RAMReservation > 0 && c.RAMReserveAll {
		errs = append(errs, fmt.Errorf("'RAM_reservation' and 'RAM_reserve_all' cannot be used together"))
	}

	if c.Firmware != "" && c.Firmware != "bios" && c.Firmware != "efi" && c.Firmware != "efi-secure" {
		errs = append(errs, fmt.Errorf("'firmware' must be '', 'bios', 'efi' or 'efi-secure'"))
	}

	if c.VTPMEnabled && c.Firmware != "efi" && c.Firmware != "efi-secure" {
		errs = append(errs, fmt.Errorf("'vTPM' could be enabled only when 'firmware' set to 'efi' or 'efi-secure'"))
	}

	if c.VirtualPrecisionClock != "" && c.VirtualPrecisionClock != "ptp" && c.VirtualPrecisionClock != "ntp" && c.VirtualPrecisionClock != "none" {
		errs = append(errs, fmt.Errorf("'precision_clock' must be '', 'ptp', 'ntp', or 'none'"))
	}

	return errs
}

type StepConfigureHardware struct {
	Config *HardwareConfig
}

func (s *StepConfigureHardware) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(driver.VirtualMachine)

	hasCustomConfig := s.hasCustomHardwareConfig()

	if hasCustomConfig {
		ui.Say("Applying custom hardware configuration...")
	} else {
		ui.Say("Applying hardware configuration...")
	}

	var allowedDevices []driver.PCIPassthroughAllowedDevice
	for _, device := range s.Config.AllowedDevices {
		allowedDevices = append(allowedDevices, driver.PCIPassthroughAllowedDevice(device))
	}

	err := vm.Configure(&driver.HardwareConfig{
		CPUs:                  s.Config.CPUs,
		CpuCores:              s.Config.CpuCores,
		CPUReservation:        s.Config.CPUReservation,
		CPULimit:              s.Config.CPULimit,
		RAM:                   s.Config.RAM,
		RAMReservation:        s.Config.RAMReservation,
		RAMReserveAll:         s.Config.RAMReserveAll,
		NestedHV:              s.Config.NestedHV,
		CpuHotAddEnabled:      s.Config.CpuHotAddEnabled,
		MemoryHotAddEnabled:   s.Config.MemoryHotAddEnabled,
		VideoRAM:              s.Config.VideoRAM,
		Displays:              s.Config.Displays,
		AllowedDevices:        allowedDevices,
		VGPUProfile:           s.Config.VGPUProfile,
		Firmware:              s.Config.Firmware,
		ForceBIOSSetup:        s.Config.ForceBIOSSetup,
		VTPMEnabled:           s.Config.VTPMEnabled,
		VirtualPrecisionClock: s.Config.VirtualPrecisionClock,
	})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

// hasCustomHardwareConfig checks if user provided custom hardware configuration.
func (s *StepConfigureHardware) hasCustomHardwareConfig() bool {
	c := s.Config

	hasCpuConfig := c.CPUs != 0 || c.CpuCores != 0 || c.CPUReservation != 0 || c.CPULimit != 0
	hasMemoryConfig := c.RAM != 0 || c.RAMReservation != 0 || c.RAMReserveAll
	hasHotAddConfig := c.CpuHotAddEnabled || c.MemoryHotAddEnabled
	hasDisplayConfig := c.VideoRAM != 0 || c.Displays != 0
	hasNestedConfig := c.NestedHV
	hasGpuConfig := c.VGPUProfile != ""
	hasFirmwareConfig := c.Firmware != "" || c.ForceBIOSSetup || c.VTPMEnabled
	hasClockConfig := c.VirtualPrecisionClock != ""
	hasDeviceConfig := len(c.AllowedDevices) > 0

	return hasCpuConfig ||
		hasMemoryConfig ||
		hasHotAddConfig ||
		hasDisplayConfig ||
		hasNestedConfig ||
		hasGpuConfig ||
		hasFirmwareConfig ||
		hasClockConfig ||
		hasDeviceConfig
}

func (s *StepConfigureHardware) Cleanup(multistep.StateBag) {}

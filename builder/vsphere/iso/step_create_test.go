// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package iso

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// TestCreateConfig_Prepare tests the Prepare method of CreateConfig for various configurations and ensures validations.
func TestCreateConfig_Prepare(t *testing.T) {
	// Empty config - check defaults
	config := &CreateConfig{
		// Storage is required
		StorageConfig: common.StorageConfig{
			Storage: []common.DiskConfig{
				{
					DiskSize: 32768,
				},
			},
		},
	}
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %s", errs[0])
	}
	if config.GuestOSType != "otherGuest" {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", "otherGuest", config.GuestOSType)
	}
	if len(config.StorageConfig.DiskControllerType) != 1 {
		t.Fatalf("unexpected result: expected '%d', but returned '%d'", 1, len(config.StorageConfig.DiskControllerType))
	}

	// Data validation
	tc := []struct {
		name           string
		config         *CreateConfig
		fail           bool
		expectedErrMsg string
	}{
		{
			name: "Storage validate disk_size",
			config: &CreateConfig{
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:            0,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0].'disk_size' is required",
		},
		{
			name: "Storage validate disk_controller_index",
			config: &CreateConfig{
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskControllerIndex: 3,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0].'disk_controller_index' references an unknown disk controller",
		},
		{
			name: "Storage rejects disk_controller_unit",
			config: &CreateConfig{
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:           32768,
							DiskControllerUnit: "scsi0:1",
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0]: 'disk_controller_unit' is only supported by the vsphere-clone builder",
		},
		{
			name: "USBController validate 'usb' and 'xhci' can be set together",
			config: &CreateConfig{
				USBController: []string{"usb", "xhci"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail: false,
		},
		{
			name: "USBController validate '1' and '0' can be set together",
			config: &CreateConfig{
				USBController: []string{"1", "0"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail: false,
		},
		{
			name: "USBController validate 'true' and 'false' can be set together",
			config: &CreateConfig{
				USBController: []string{"true", "false"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail: false,
		},
		{
			name: "USBController validate 'true' and 'usb' cannot be set together",
			config: &CreateConfig{
				USBController: []string{"true", "usb"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "there can only be one usb controller and one xhci controller",
		},
		{
			name: "USBController validate '1' and 'usb' cannot be set together",
			config: &CreateConfig{
				USBController: []string{"1", "usb"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "there can only be one usb controller and one xhci controller",
		},
		{
			name: "USBController validate 'xhci' cannot be set more that once",
			config: &CreateConfig{
				USBController: []string{"xhci", "xhci"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "there can only be one usb controller and one xhci controller",
		},
		{
			name: "USBController validate unknown value cannot be set",
			config: &CreateConfig{
				USBController: []string{"unknown"},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "usb_controller[0] references an unknown usb controller",
		},
	}

	for _, c := range tc {
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
	}
}

// TestStepCreateVM_Run tests the Run method in the StepCreateVM structure, verifying correct state updates, method call
// sequences, and configuration integrity for virtual machine creation.
func TestStepCreateVM_Run(t *testing.T) {
	state := basicStateBag()
	driverMock := driver.NewDriverMock()
	state.Put("driver", driverMock)
	step := basicStepCreateVM()
	step.Force = true
	vmPath := path.Join(step.Location.Folder, step.Location.VMName)

	if action := step.Run(context.TODO(), state); action == multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Pre clean VM
	if !driverMock.PreCleanVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "PreCleanVM")
	}
	if driverMock.PreCleanForce != step.Force {
		t.Fatalf("unexpected result: expected '%t', but returned '%t'.", step.Force, driverMock.PreCleanForce)
	}
	if driverMock.PreCleanVMPath != vmPath {
		t.Fatalf("unexpected result: expected %s, but returned %s", vmPath, driverMock.PreCleanVMPath)
	}

	if !driverMock.CreateVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "CreateVM")
	}
	if diff := cmp.Diff(driverMock.CreateConfig, driverCreateConfig(step.Config, step.Location)); diff != "" {
		t.Fatalf("unexpected CreateConfig: %s", diff)
	}
	vm, ok := state.GetOk("vm")
	if !ok {
		t.Fatalf("unexpected result: expected '%s' to be in state", "vm")
	}
	if vm != driverMock.VM {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", driverMock.VM, vm)
	}
}

// TestStepCreateVM_RunHalt tests the Run method of StepCreateVM to verify it halts execution when prerequisites fail.
func TestStepCreateVM_RunHalt(t *testing.T) {
	state := basicStateBag()
	step := basicStepCreateVM()

	// PreCleanVM fails
	driverMock := driver.NewDriverMock()
	driverMock.PreCleanShouldFail = true
	state.Put("driver", driverMock)
	if action := step.Run(context.TODO(), state); action != multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionHalt, action)
	}
	if !driverMock.PreCleanVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "PreCleanVM")
	}

	// CreateVM fails
	driverMock = driver.NewDriverMock()
	driverMock.CreateVMShouldFail = true
	state.Put("driver", driverMock)
	if action := step.Run(context.TODO(), state); action != multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionHalt, action)
	}
	if !driverMock.PreCleanVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "PreCleanVM")
	}
	if !driverMock.CreateVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "CreateVM")
	}
	if _, ok := state.GetOk("vm"); ok {
		t.Fatalf("unexpected result: expected '%s' not to be in state", "vm")
	}
}

// TestStepCreateVM_Cleanup tests the Cleanup method of the StepCreateVM to ensure resources are destroyed.
func TestStepCreateVM_Cleanup(t *testing.T) {
	state := basicStateBag()
	step := basicStepCreateVM()
	vm := new(driver.VirtualMachineMock)
	state.Put("vm", vm)

	// Clean up when state is canceled
	state.Put(multistep.StateCancelled, true)
	step.Cleanup(state)
	if !vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "Destroy")
	}
	vm.DestroyCalled = false
	state.Remove(multistep.StateCancelled)

	// Clean up when the state is halted
	state.Put(multistep.StateHalted, true)
	step.Cleanup(state)
	if !vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "Destroy")
	}
	vm.DestroyCalled = false
	state.Remove(multistep.StateHalted)

	// Clean up when state is destroy_vm is set
	state.Put("destroy_vm", true)
	step.Cleanup(state)
	if !vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "Destroy")
	}
	vm.DestroyCalled = false
	state.Remove("destroy_vm")

	// Don't clean up if the state is not set with previous values
	step.Cleanup(state)
	if vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' not to be called", "Destroy")
	}

	// Destroy fail
	errorBuffer := &strings.Builder{}
	ui := &packersdk.BasicUi{
		Reader:      strings.NewReader(""),
		Writer:      io.Discard,
		ErrorWriter: errorBuffer,
	}
	state.Put("ui", ui)
	state.Put(multistep.StateCancelled, true)
	vm.DestroyError = errors.New("destroy failed")

	step.Cleanup(state)
	if !vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "Destroy")
	}
	if !strings.Contains(errorBuffer.String(), vm.DestroyError.Error()) {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", vm.DestroyError.Error(), errorBuffer.String())
	}
	vm.DestroyCalled = false
	state.Remove(multistep.StateCancelled)

	// Should not destroy if VM is not set
	state.Remove("vm")
	state.Put(multistep.StateCancelled, true)
	step.Cleanup(state)
	if vm.DestroyCalled {
		t.Fatalf("unexpected result: expected '%s' not to be called", "Destroy")
	}
}

// basicStepCreateVM initializes and returns a new StepCreateVM with a default configuration and location setup.
func basicStepCreateVM() *StepCreateVM {
	step := &StepCreateVM{
		Config:   createConfig(),
		Location: basicLocationConfig(),
	}
	return step
}

// basicLocationConfig initializes and returns a default LocationConfig with predefined test values for the virtual machine.
func basicLocationConfig() *common.LocationConfig {
	return &common.LocationConfig{
		VMName:       "test-vm",
		Folder:       "test-folder",
		Cluster:      "test-cluster",
		Host:         "test-host",
		ResourcePool: "test-resource-pool",
		Datastore:    "test-datastore",
	}
}

// createConfig initializes and returns a configured instance of CreateConfig with predefined default values.
func createConfig() *CreateConfig {
	return &CreateConfig{
		Version:     1,
		GuestOSType: "ubuntu64Guest",
		StorageConfig: common.StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []common.DiskConfig{
				{
					DiskSize:            32768,
					DiskThinProvisioned: true,
				},
			},
		},
		NICs: []NIC{
			{
				Network:     "VM Network",
				NetworkCard: "vmxnet3",
			},
		},
	}
}

// driverCreateConfig converts CreateConfig and LocationConfig into driver.CreateConfig for virtual machine creation.
// It maps network interfaces, disks, and other configuration details to the required driver.CreateConfig structure.
// resolvedPolicyIDs maps disk index to pre-resolved storage policy UUID (empty string = no policy).
func driverCreateConfig(config *CreateConfig, location *common.LocationConfig, resolvedPolicyIDs ...string) *driver.CreateConfig {
	var networkCards []driver.NIC
	for _, nic := range config.NICs {
		networkCards = append(networkCards, driver.NIC{
			Network:     nic.Network,
			NetworkCard: nic.NetworkCard,
			MacAddress:  strings.ToLower(nic.MacAddress),
			Passthrough: nic.Passthrough,
		})
	}

	var disks []driver.Disk
	for i, disk := range config.StorageConfig.Storage {
		d := driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
		}
		if i < len(resolvedPolicyIDs) {
			d.StoragePolicyID = resolvedPolicyIDs[i]
		}
		disks = append(disks, d)
	}

	return &driver.CreateConfig{
		StorageConfig: driver.StorageConfig{
			DiskControllerType: config.StorageConfig.DiskControllerType,
			Storage:            disks,
		},
		Annotation:    config.Notes,
		Name:          location.VMName,
		Folder:        location.Folder,
		Cluster:       location.Cluster,
		Host:          location.Host,
		ResourcePool:  location.ResourcePool,
		Datastore:     location.Datastore,
		GuestOS:       config.GuestOSType,
		NICs:          networkCards,
		USBController: config.USBController,
		Version:       config.Version,
	}
}

// TestStepCreateVM_Run_WithStoragePolicy verifies that a storage policy name is
// resolved to a UUID and forwarded to CreateVM as StoragePolicyID on the disk.
func TestStepCreateVM_Run_WithStoragePolicy(t *testing.T) {
	const policyName = "gold-policy"
	const policyUUID = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"

	state := basicStateBag()
	driverMock := driver.NewDriverMock()
	driverMock.FindStoragePolicyIDResult = policyUUID
	state.Put("driver", driverMock)

	config := createConfig()
	config.StorageConfig.Storage[0].StoragePolicyName = policyName

	step := &StepCreateVM{Config: config, Location: basicLocationConfig()}

	if action := step.Run(context.TODO(), state); action == multistep.ActionHalt {
		t.Fatalf("unexpected halt: %v", state.Get("error"))
	}

	if !driverMock.FindStoragePolicyIDCalled {
		t.Fatal("expected FindStoragePolicyID to be called")
	}
	if driverMock.FindStoragePolicyIDName != policyName {
		t.Fatalf("expected policy name %q, got %q", policyName, driverMock.FindStoragePolicyIDName)
	}
	if diff := cmp.Diff(driverMock.CreateConfig, driverCreateConfig(config, basicLocationConfig(), policyUUID)); diff != "" {
		t.Fatalf("unexpected CreateConfig: %s", diff)
	}
}

// TestStepCreateVM_Run_StoragePolicyNotFound verifies that a missing storage
// policy causes the step to halt with a descriptive error.
func TestStepCreateVM_Run_StoragePolicyNotFound(t *testing.T) {
	state := basicStateBag()
	driverMock := driver.NewDriverMock()
	driverMock.FindStoragePolicyIDErr = fmt.Errorf("no pbm profile found with name: %q", "nonexistent")
	state.Put("driver", driverMock)

	config := createConfig()
	config.StorageConfig.Storage[0].StoragePolicyName = "nonexistent"

	step := &StepCreateVM{Config: config, Location: basicLocationConfig()}

	if action := step.Run(context.TODO(), state); action != multistep.ActionHalt {
		t.Fatalf("expected ActionHalt, got %v", action)
	}
	if state.Get("error") == nil {
		t.Fatal("expected an error in state, got nil")
	}
}

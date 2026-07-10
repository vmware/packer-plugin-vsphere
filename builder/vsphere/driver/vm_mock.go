// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VirtualMachineMock struct {
	DestroyError  error
	DestroyCalled bool

	ConfigureError          error
	ConfigureCalled         bool
	ConfigureHardwareConfig *HardwareConfig

	ReconfigureCalled bool
	ReconfigureSpec   types.VirtualMachineConfigSpec
	ReconfigureError  error

	FindSATAControllerCalled bool
	FindSATAControllerErr    error

	AddSATAControllerCalled bool
	AddSATAControllerErr    error

	AddCdromCalledTimes int
	AddCdromErr         error
	AddCdromTypes       []string
	AddCdromPaths       []string

	AddFlagCalled            bool
	AddFlagCalledTimes       int
	AddFlagErr               error
	AddFlagVbsEnabledValues  bool
	AddFlagVvtdEnabledValues bool

	GetDirCalled   bool
	GetDirResponse string
	GetDirErr      error

	AddFloppyCalled    bool
	AddFloppyImagePath string
	AddFloppyErr       error

	FloppyDevicesErr    error
	FloppyDevicesReturn object.VirtualDeviceList
	FloppyDevicesCalled bool

	CdromDevicesErr    error
	CdromDevicesList   object.VirtualDeviceList
	CdromDevicesCalled bool

	RemoveDeviceErr       error
	RemoveDeviceCalled    bool
	RemoveDeviceKeepFiles bool
	RemoveDeviceDevices   []types.BaseVirtualDevice

	EjectCdromsCalled bool
	EjectCdromsErr    error

	RemoveCdromsCalled bool
	RemoveCdromsErr    error

	RemoveNCdromsCalled bool
	RemoveNCdromsErr    error

	ReattachCDRomsCalled bool
	ReattachCDRomsErr    error

	RemoveNetworkAdaptersCalled bool
	NetworkAdaptersList         object.VirtualDeviceList
	RemoveNetworkAdaptersErr    error

	CloneCalled    bool
	CloneConfig    *CloneConfig
	CloneError     error
	CloneReturnNil bool
}

func (vm *VirtualMachineMock) Info(params ...string) (*mo.VirtualMachine, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) Devices() (object.VirtualDeviceList, error) {
	return object.VirtualDeviceList{}, nil
}

func (vm *VirtualMachineMock) FloppyDevices() (object.VirtualDeviceList, error) {
	vm.FloppyDevicesCalled = true
	return vm.FloppyDevicesReturn, vm.FloppyDevicesErr
}

func (vm *VirtualMachineMock) CdromDevices() (object.VirtualDeviceList, error) {
	vm.CdromDevicesCalled = true
	return vm.CdromDevicesList, vm.CdromDevicesErr
}

func (vm *VirtualMachineMock) Clone(ctx context.Context, config *CloneConfig) (VirtualMachine, error) {
	vm.CloneCalled = true
	vm.CloneConfig = config
	if vm.CloneReturnNil {
		return nil, vm.CloneError
	}
	return vm, vm.CloneError
}

func (vm *VirtualMachineMock) SetVAppProperties(ctx context.Context, props map[string]string) error {
	return nil
}

func (vm *VirtualMachineMock) AddPublicKeys(ctx context.Context, publicKeys string) error {
	return nil
}

func (vm *VirtualMachineMock) Properties(ctx context.Context) (*mo.VirtualMachine, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) Destroy() error {
	vm.DestroyCalled = true
	if vm.DestroyError != nil {
		return vm.DestroyError
	}
	return nil
}

func (vm *VirtualMachineMock) Configure(config *HardwareConfig) error {
	vm.ConfigureCalled = true
	vm.ConfigureHardwareConfig = config
	if vm.ConfigureError != nil {
		return vm.ConfigureError
	}
	return nil
}

func (vm *VirtualMachineMock) Reconfigure(confSpec types.VirtualMachineConfigSpec) error {
	vm.ReconfigureCalled = true
	vm.ReconfigureSpec = confSpec
	if vm.ReconfigureError != nil {
		return vm.ReconfigureError
	}
	return nil
}

func (vm *VirtualMachineMock) Customize(spec types.CustomizationSpec) error {
	return nil
}

func (vm *VirtualMachineMock) ResizeDisk(diskSize int64) ([]types.BaseVirtualDeviceConfigSpec, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) PowerOn() error {
	return nil
}

func (vm *VirtualMachineMock) WaitForIP(ctx context.Context, ipNet *net.IPNet) (string, error) {
	return "", nil
}

func (vm *VirtualMachineMock) PowerOff() error {
	return nil
}

func (vm *VirtualMachineMock) IsPoweredOff() (bool, error) {
	return false, nil
}

func (vm *VirtualMachineMock) StartShutdown() error {
	return nil
}

func (vm *VirtualMachineMock) WaitForShutdown(ctx context.Context, timeout time.Duration) error {
	return nil
}

func (vm *VirtualMachineMock) CreateSnapshot(name string) error {
	return nil
}

func (vm *VirtualMachineMock) ConvertToTemplate() error {
	return nil
}

func (vm *VirtualMachineMock) IsTemplate() (bool, error) {
	return false, nil
}

func (vm *VirtualMachineMock) ConvertToVirtualMachine(vsphereCluster string, vsphereHost string, vsphereResourcePool string) error {
	return nil
}

func (vm *VirtualMachineMock) ImportOvfToContentLibrary(ovf vcenter.OVF) error {
	return nil
}

func (vm *VirtualMachineMock) ImportToContentLibrary(template vcenter.Template) error {
	return nil
}

func (vm *VirtualMachineMock) FindContentLibraryItem(libraryName string, itemName string) (*library.Item, error) {
	return nil, fmt.Errorf("content library item %s not found", itemName)
}

func (vm *VirtualMachineMock) DeleteContentLibraryItem(itemID string) error {
	return nil
}

func (vm *VirtualMachineMock) GetDir() (string, error) {
	vm.GetDirCalled = true
	return vm.GetDirResponse, vm.GetDirErr
}

func (vm *VirtualMachineMock) AddCdrom(controllerType string, isoPath string) error {
	vm.AddCdromCalledTimes++
	vm.AddCdromTypes = append(vm.AddCdromTypes, controllerType)
	vm.CdromDevicesList = append(vm.CdromDevicesList, nil)
	if isoPath != "" {
		vm.AddCdromPaths = append(vm.AddCdromPaths, isoPath)
	}
	return vm.AddCdromErr
}

func (vm *VirtualMachineMock) AddFloppy(imgPath string) error {
	vm.AddFloppyCalled = true
	vm.AddFloppyImagePath = imgPath
	return vm.AddFloppyErr
}

func (vm *VirtualMachineMock) SetBootOrder(order []string) error {
	return nil
}

func (vm *VirtualMachineMock) RemoveDevice(keepFiles bool, device ...types.BaseVirtualDevice) error {
	vm.RemoveDeviceCalled = true
	vm.RemoveDeviceKeepFiles = keepFiles
	vm.RemoveDeviceDevices = device
	return vm.RemoveDeviceErr
}

func (vm *VirtualMachineMock) addDevice(device types.BaseVirtualDevice) error {
	return nil
}

func (vm *VirtualMachineMock) AddConfigParams(params map[string]string, info *types.ToolsConfigInfo) error {
	return nil
}

func (vm *VirtualMachineMock) AddFlag(ctx context.Context, info *types.VirtualMachineFlagInfo) error {
	vm.AddFlagCalled = true
	vm.AddFlagCalledTimes++
	if info.VbsEnabled != nil {
		vm.AddFlagVbsEnabledValues = *info.VbsEnabled
	}
	if info.VvtdEnabled != nil {
		vm.AddFlagVvtdEnabledValues = *info.VvtdEnabled
	}
	return vm.AddFlagErr
}

func (vm *VirtualMachineMock) Export() (*nfc.Lease, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) CreateDescriptor(m *ovf.Manager, cdp types.OvfCreateDescriptorParams) (*types.OvfCreateDescriptorResult, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) NewOvfManager() *ovf.Manager {
	return nil
}

func (vm *VirtualMachineMock) GetOvfExportOptions(m *ovf.Manager) ([]types.OvfOptionInfo, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) AddSATAController() error {
	vm.AddSATAControllerCalled = true
	return vm.AddSATAControllerErr
}

func (vm *VirtualMachineMock) FindSATAController() (*types.VirtualAHCIController, error) {
	vm.FindSATAControllerCalled = true
	return nil, vm.FindSATAControllerErr
}

func (vm *VirtualMachineMock) CreateCdrom(c *types.VirtualController) (*types.VirtualCdrom, error) {
	return nil, nil
}

func (vm *VirtualMachineMock) RemoveCdroms() error {
	vm.RemoveCdromsCalled = true
	vm.CdromDevicesList = nil
	return vm.RemoveCdromsErr
}

func (vm *VirtualMachineMock) RemoveNCdroms(nCdroms int) error {
	vm.RemoveNCdromsCalled = true
	if nCdroms == 0 {
		return nil
	}
	vm.CdromDevicesList = vm.CdromDevicesList[len(vm.CdromDevicesList)-nCdroms:]
	return vm.RemoveNCdromsErr
}

func (vm *VirtualMachineMock) ReattachCDRoms() error {
	return vm.ReattachCDRomsErr
}

func (vm *VirtualMachineMock) EjectCdroms() error {
	vm.EjectCdromsCalled = true
	vm.AddCdromPaths = nil
	return vm.EjectCdromsErr
}

func (vm *VirtualMachineMock) RemoveNetworkAdapters() error {
	vm.RemoveNetworkAdaptersCalled = true
	vm.NetworkAdaptersList = nil
	return vm.RemoveNetworkAdaptersErr
}

func (vm *VirtualMachineMock) Datacenter() *object.Datacenter {
	return nil
}

func (vm *VirtualMachineMock) Reference() types.ManagedObjectReference {
	return types.ManagedObjectReference{}
}

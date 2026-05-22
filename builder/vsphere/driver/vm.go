// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VirtualMachine interface {
	Info(params ...string) (*mo.VirtualMachine, error)
	Devices() (object.VirtualDeviceList, error)
	CdromDevices() (object.VirtualDeviceList, error)
	FloppyDevices() (object.VirtualDeviceList, error)
	Clone(ctx context.Context, config *CloneConfig) (VirtualMachine, error)
	SetVAppProperties(ctx context.Context, props map[string]string) error
	AddPublicKeys(ctx context.Context, publicKeys string) error
	Properties(ctx context.Context) (*mo.VirtualMachine, error)
	Destroy() error
	Unregister() error
	Configure(config *HardwareConfig) error
	Reconfigure(spec types.VirtualMachineConfigSpec) error
	Customize(spec types.CustomizationSpec) error
	ResizeDisk(diskSize int64) ([]types.BaseVirtualDeviceConfigSpec, error)
	WaitForIP(ctx context.Context, ipNet *net.IPNet, adapterIndex *int) (string, error)
	PowerOn() error
	PowerOff() error
	IsPoweredOff() (bool, error)
	StartShutdown() error
	WaitForShutdown(ctx context.Context, timeout time.Duration) error
	CreateSnapshot(name string) error
	ConvertToTemplate() error
	IsTemplate() (bool, error)
	ConvertToVirtualMachine(vsphereCluster string, vsphereHost string, vsphereResourcePool string) error
	ImportOvfToContentLibrary(ovf vcenter.OVF) error
	ImportToContentLibrary(template vcenter.Template) error
	FindContentLibraryItem(libraryName string, itemName string) (*library.Item, error)
	DeleteContentLibraryItem(itemID string) error
	GetDir() (string, error)
	AddFloppy(imgPath string) error
	SetBootOrder(order []string) error
	RemoveDevice(keepFiles bool, device ...types.BaseVirtualDevice) error
	addDevice(device types.BaseVirtualDevice) error
	AddConfigParams(params map[string]string, info *types.ToolsConfigInfo) error
	AddFlag(ctx context.Context, info *types.VirtualMachineFlagInfo) error
	Export() (*nfc.Lease, error)
	CreateDescriptor(m *ovf.Manager, cdp types.OvfCreateDescriptorParams) (*types.OvfCreateDescriptorResult, error)
	NewOvfManager() *ovf.Manager
	GetOvfExportOptions(m *ovf.Manager) ([]types.OvfOptionInfo, error)
	Datacenter() *object.Datacenter

	AddCdrom(controllerType string, datastoreIsoPath string) error
	CreateCdrom(c *types.VirtualController) (*types.VirtualCdrom, error)
	RemoveCdroms() error
	RemoveNCdroms(nCdroms int) error
	EjectCdroms() error
	AddSATAController() error
	FindSATAController() (*types.VirtualAHCIController, error)

	RemoveNetworkAdapters() error
	RemoveVTPM() error
	Reference() types.ManagedObjectReference
}

type VirtualMachineDriver struct {
	vm     *object.VirtualMachine
	driver *VCenterDriver
}

type CloneConfig struct {
	Name            string
	Folder          string
	Cluster         string
	Host            string
	ResourcePool    string
	Datastore       string
	LinkedClone     bool
	Network         string
	MacAddress      string
	Annotation      string
	VAppProperties  map[string]string
	PrimaryDiskSize int64
	StorageConfig   StorageConfig
}

type PCIPassthroughAllowedDevice struct {
	VendorId    string
	DeviceId    string
	SubVendorId string
	SubDeviceId string
}

type HardwareConfig struct {
	CPUs                  int32
	CpuCores              int32
	CPUReservation        int64
	CPULimit              int64
	RAM                   int64
	RAMReservation        int64
	RAMReserveAll         bool
	NestedHV              bool
	CpuHotAddEnabled      bool
	MemoryHotAddEnabled   bool
	VideoRAM              int64
	Displays              int32
	AllowedDevices        []PCIPassthroughAllowedDevice
	VGPUProfile           string
	Firmware              string
	ForceBIOSSetup        bool
	BootDelay             int64
	VTPMEnabled           bool
	VirtualPrecisionClock string
}

type NIC struct {
	Network     string
	NetworkCard string
	MacAddress  string
	Passthrough *bool
}

type CreateConfig struct {
	Annotation    string
	Name          string
	Folder        string
	Cluster       string
	Host          string
	ResourcePool  string
	Datastore     string
	GuestOS       string
	NICs          []NIC
	USBController []string
	Version       uint
	Firmware      string
	StorageConfig StorageConfig
}

// NewVM creates a new virtual machine object.
func (d *VCenterDriver) NewVM(ref *types.ManagedObjectReference) VirtualMachine {
	return &VirtualMachineDriver{
		vm:     object.NewVirtualMachine(d.Client.Client, *ref),
		driver: d,
	}
}

// FindVM locates a virtual machine by its name.
func (d *VCenterDriver) FindVM(name string) (VirtualMachine, error) {
	vm, err := d.Finder.VirtualMachine(d.Ctx, name)
	if err != nil {
		return nil, err
	}
	return &VirtualMachineDriver{
		vm:     vm,
		driver: d,
	}, nil
}

// PreCleanVM checks for an existing virtual machine at the specified path and optionally forces its removal.
func (d *VCenterDriver) PreCleanVM(ui packersdk.Ui, vmPath string, force bool, vsphereCluster string, vsphereHost string, vsphereResourcePool string) error {
	vm, err := d.FindVM(vmPath)
	if err != nil {
		var notFoundError *find.NotFoundError
		if !errors.As(err, &notFoundError) {
			return fmt.Errorf("error looking up existing virtual machine: %v", err)
		}
	}
	if force && vm != nil {
		ui.Sayf("Removing the existing virtual machine at %s based on use of the '-force' option...", vmPath)

		// Power off the virtual machine if it is powered on.
		_ = vm.PowerOff()

		// Check if the virtual machine is a template and convert it back to a
		// virtual machine if necessary.
		isTemplate, err := vm.IsTemplate()
		if err != nil {
			return fmt.Errorf("error determining if the virtual machine is a template%s: %v", vmPath, err)
		} else if isTemplate {
			ui.Sayf("Attempting to convert the template at %s to a virtual machine...", vmPath)
			err := vm.ConvertToVirtualMachine(vsphereCluster, vsphereHost, vsphereResourcePool)
			if err != nil {
				return fmt.Errorf("error converting template back to virtual machine for cleanup %s: %v", vmPath, err)
			}
		}

		err = vm.Destroy()
		if err != nil {
			return fmt.Errorf("error destroying %s: %v", vmPath, err)
		}
	}
	if !force && vm != nil {
		return fmt.Errorf("%s already exists, you can use -force flag to destroy it: %v", vmPath, err)
	}

	return nil
}

// CreateVM creates a new virtual machine based on the provided configuration
// specification.
func (d *VCenterDriver) CreateVM(config *CreateConfig) (VirtualMachine, error) {
	createSpec := types.VirtualMachineConfigSpec{
		Name:       config.Name,
		Annotation: config.Annotation,
		GuestId:    config.GuestOS,
	}
	if config.Version != 0 {
		createSpec.Version = fmt.Sprintf("%s%d", "vmx-", config.Version)
	}

	// Set firmware, if specified.
	if config.Firmware != "" {
		firmware := config.Firmware
		efiSecureBootEnabled := firmware == "efi-secure"
		if efiSecureBootEnabled {
			firmware = "efi"
		}
		createSpec.Firmware = firmware
		createSpec.BootOptions = &types.VirtualMachineBootOptions{
			EfiSecureBootEnabled: new(efiSecureBootEnabled),
		}
	}

	folder, err := d.FindFolder(config.Folder)
	if err != nil {
		return nil, err
	}

	resourcePool, err := d.FindResourcePool(config.Cluster, config.Host, config.ResourcePool)
	if err != nil {
		return nil, err
	}

	var host *object.HostSystem
	if config.Cluster != "" && config.Host != "" {
		h, err := d.FindHost(config.Host)
		if err != nil {
			return nil, err
		}
		host = h.host
	}

	datastore, err := d.FindDatastore(config.Datastore, config.Host)
	if err != nil {
		return nil, err
	}

	devices := object.VirtualDeviceList{}
	storageConfigSpec, err := config.StorageConfig.AddStorageDevices(devices)
	if err != nil {
		return nil, err
	}
	createSpec.DeviceChange = append(createSpec.DeviceChange, storageConfigSpec...)

	// Use the first disk's policy for the VM home so the entire VM lives on
	// compatible storage.
	for _, disk := range config.StorageConfig.Storage {
		if disk.StoragePolicyID != "" {
			createSpec.VmProfile = []types.BaseVirtualMachineProfileSpec{
				&types.VirtualMachineDefinedProfileSpec{
					ProfileId: disk.StoragePolicyID,
				},
			}
			break
		}
	}

	devices, err = addNetwork(d, devices, config)
	if err != nil {
		return nil, err
	}

	t := true
	for _, usbType := range config.USBController {
		var usb types.BaseVirtualDevice
		switch usbType {
		// handle "true" and "1" for backwards compatibility
		case "usb", "true", "1":
			usb = &types.VirtualUSBController{
				EhciEnabled: &t,
			}
		case "xhci":
			usb = new(types.VirtualUSBXHCIController)
		default:
			continue
		}

		devices = append(devices, usb)
	}

	devicesConfigSpec, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return nil, err
	}
	createSpec.DeviceChange = append(createSpec.DeviceChange, devicesConfigSpec...)

	createSpec.Files = &types.VirtualMachineFileInfo{
		VmPathName: fmt.Sprintf("[%s]", datastore.Name()),
	}

	task, err := folder.folder.CreateVM(d.Ctx, createSpec, resourcePool.pool, host)
	if err != nil {
		return nil, err
	}
	taskInfo, err := task.WaitForResult(d.Ctx, nil)
	if err != nil {
		return nil, err
	}

	vmRef, ok := taskInfo.Result.(types.ManagedObjectReference)
	if !ok {
		return nil, fmt.Errorf("something went wrong when creating the VM")
	}

	vm := d.NewVM(&vmRef)

	return vm, nil
}

// Info retrieves properties of the virtual machine object with optional filters
// specified  as parameters. If no parameters are provided, all properties are
// returned.
func (vm *VirtualMachineDriver) Info(params ...string) (*mo.VirtualMachine, error) {
	var p []string
	if len(params) == 0 {
		p = []string{"*"}
	} else {
		p = params
	}
	var info mo.VirtualMachine
	err := vm.vm.Properties(vm.driver.Ctx, vm.vm.Reference(), p, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// Devices returns a list of devices attached to the virtual machine.
func (vm *VirtualMachineDriver) Devices() (object.VirtualDeviceList, error) {
	vmInfo, err := vm.Info("config.hardware.device")
	if err != nil {
		return nil, err
	}

	return vmInfo.Config.Hardware.Device, nil
}

// FloppyDevices returns a list of floppy devices attached to the virtual
// machine.
func (vm *VirtualMachineDriver) FloppyDevices() (object.VirtualDeviceList, error) {
	device, err := vm.Devices()
	if err != nil {
		return device, err
	}
	floppies := device.SelectByType((*types.VirtualFloppy)(nil))
	return floppies, nil
}

// CdromDevices returns a list of all CD-ROM devices attached to the virtual
// machine.
func (vm *VirtualMachineDriver) CdromDevices() (object.VirtualDeviceList, error) {
	device, err := vm.Devices()
	if err != nil {
		return device, err
	}
	floppies := device.SelectByType((*types.VirtualCdrom)(nil))
	return floppies, nil
}

// Clone creates a new virtual machine by cloning an existing one.
func (vm *VirtualMachineDriver) Clone(ctx context.Context, config *CloneConfig) (VirtualMachine, error) {
	folder, err := vm.driver.FindFolder(config.Folder)
	if err != nil {
		return nil, fmt.Errorf("error finding folder: %s", err)
	}

	var relocateSpec types.VirtualMachineRelocateSpec

	pool, err := vm.driver.FindResourcePool(config.Cluster, config.Host, config.ResourcePool)
	if err != nil {
		return nil, fmt.Errorf("error finding resource pool: %s", err)
	}
	poolRef := pool.pool.Reference()
	relocateSpec.Pool = &poolRef

	datastore, err := vm.driver.FindDatastore(config.Datastore, config.Host)
	if err != nil {
		return nil, fmt.Errorf("error finding datastore: %s", err)
	}
	datastoreRef := datastore.Reference()
	relocateSpec.Datastore = &datastoreRef

	if config.Cluster != "" && config.Host != "" {
		h, err := vm.driver.FindHost(config.Host)
		if err != nil {
			return nil, err
		}
		hostRef := h.host.Reference()
		relocateSpec.Host = &hostRef
	}

	var cloneSpec types.VirtualMachineCloneSpec
	cloneSpec.Location = relocateSpec
	cloneSpec.PowerOn = false

	if config.LinkedClone {
		cloneSpec.Location.DiskMoveType = "createNewChildDiskBacking"

		tpl, err := vm.Info("snapshot")
		if err != nil {
			return nil, fmt.Errorf("error getting snapshot info for virtual machine: %s", err)
		}
		if tpl.Snapshot == nil {
			err = errors.New("`linked_clone=true`, but template has no snapshots")
			return nil, err
		}
		cloneSpec.Snapshot = tpl.Snapshot.CurrentSnapshot
	}

	var configSpec types.VirtualMachineConfigSpec
	cloneSpec.Config = &configSpec

	if config.Annotation != "" {
		configSpec.Annotation = config.Annotation
	}

	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return nil, err
	}

	if config.PrimaryDiskSize > 0 {
		deviceResizeSpec, err := vm.ResizeDisk(config.PrimaryDiskSize)
		if err != nil {
			return nil, fmt.Errorf("failed to resize primary disk: %s", err)
		}
		configSpec.DeviceChange = append(configSpec.DeviceChange, deviceResizeSpec...)
	}

	existingDevices := StorageExistingDevices(devices)

	storageConfigSpec, err := config.StorageConfig.AddStorageDevices(existingDevices)
	if err != nil {
		return nil, fmt.Errorf("failed to add storage devices: %s", err)
	}
	configSpec.DeviceChange = append(configSpec.DeviceChange, storageConfigSpec...)

	if config.Network != "" {
		network, err := vm.driver.FindNetwork(config.Network)
		if err != nil {
			return nil, fmt.Errorf("error finding network: %s", err)
		}
		backing, err := network.network.EthernetCardBackingInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("error finding ethernet card backing info: %s", err)
		}

		devices, err := vm.vm.Device(ctx)
		if err != nil {
			return nil, fmt.Errorf("error finding virtual machine devices: %s", err)
		}

		adapter, err := findNetworkAdapter(devices)
		if err != nil {
			return nil, fmt.Errorf("error finding network adapter: %s", err)
		}

		current := adapter.GetVirtualEthernetCard()
		current.Backing = backing

		if config.MacAddress != "" {
			current.AddressType = string(types.VirtualEthernetCardMacTypeManual)
			current.MacAddress = config.MacAddress
		}

		config := &types.VirtualDeviceConfigSpec{
			Device:    adapter.(types.BaseVirtualDevice),
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		}
		configSpec.DeviceChange = append(configSpec.DeviceChange, config)
	}

	vAppConfig, err := buildVAppConfigSpec(ctx, vm, false, config.VAppProperties)
	if err != nil {
		return nil, fmt.Errorf("error updating VAppConfig: %s", err)
	}
	configSpec.VAppConfig = vAppConfig

	task, err := vm.vm.Clone(vm.driver.Ctx, folder.folder, config.Name, cloneSpec)
	if err != nil {
		return nil, fmt.Errorf("error calling vm.vm.Clone task: %s", err)
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			err = task.Cancel(vm.driver.Ctx)
			return nil, err
		}

		return nil, fmt.Errorf("error waiting for virtual machine clone to complete: %s", err)
	}

	vmRef, ok := info.Result.(types.ManagedObjectReference)
	if !ok {
		log.Printf("[ERROR] unexpected result during cloning operation: %s", info.Result)
		return nil, fmt.Errorf("error occurred while cloning the virtual machine")
	}

	created := vm.driver.NewVM(&vmRef)
	return created, nil
}

// SetVAppProperties updates or creates vApp properties on the virtual machine.
func (vm *VirtualMachineDriver) SetVAppProperties(ctx context.Context, props map[string]string) error {
	config, err := buildVAppConfigSpec(ctx, vm, true, props)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	confSpec := types.VirtualMachineConfigSpec{VAppConfig: config}
	task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// AddPublicKeys adds public keys to the virtual machine vApp property.
func (vm *VirtualMachineDriver) AddPublicKeys(ctx context.Context, publicKeys string) error {
	newProps := map[string]string{"public-keys": publicKeys}
	config, err := buildVAppConfigSpec(ctx, vm, true, newProps)
	if err != nil {
		return fmt.Errorf("not possible to save temporary public key: %s", err)
	}
	if config == nil {
		return fmt.Errorf("not possible to save temporary public key: no vApp configuration updated")
	}

	confSpec := types.VirtualMachineConfigSpec{VAppConfig: config}
	task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// Properties retrieves the properties of a virtual machine.
func (vm *VirtualMachineDriver) Properties(ctx context.Context) (*mo.VirtualMachine, error) {
	log.Printf("fetching properties for VM %q", vm.vm.InventoryPath)
	var props mo.VirtualMachine
	if err := vm.vm.Properties(ctx, vm.vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// Destroy removes the virtual machine.
func (vm *VirtualMachineDriver) Destroy() error {
	task, err := vm.vm.Destroy(vm.driver.Ctx)
	if err != nil {
		return fmt.Errorf("error destroying virtual machine: %s", err)
	}
	if _, err = task.WaitForResult(vm.driver.Ctx, nil); err != nil {
		return fmt.Errorf("error destroying virtual machine: %s", err)
	}
	return nil
}

// Unregister removes the virtual machine from inventory without deleting its
// files. Used when Destroy fails because the backing files are already gone.
func (vm *VirtualMachineDriver) Unregister() error {
	return vm.vm.Unregister(vm.driver.Ctx)
}

// Configure modifies the configuration of an existing virtual machine based on
// the provided configuration specification.
func (vm *VirtualMachineDriver) Configure(config *HardwareConfig) error {
	var confSpec types.VirtualMachineConfigSpec
	confSpec.NumCPUs = config.CPUs
	if config.CpuCores != 0 {
		confSpec.NumCoresPerSocket = &config.CpuCores
	}
	confSpec.MemoryMB = config.RAM

	var cpuSpec types.ResourceAllocationInfo
	if config.CPUReservation != 0 {
		cpuSpec.Reservation = &config.CPUReservation
	}
	if config.CPULimit != 0 {
		cpuSpec.Limit = &config.CPULimit
	}
	confSpec.CpuAllocation = &cpuSpec

	var ramSpec types.ResourceAllocationInfo
	if config.RAMReservation != 0 {
		ramSpec.Reservation = &config.RAMReservation
	}
	confSpec.MemoryAllocation = &ramSpec

	if config.RAMReserveAll {
		confSpec.MemoryReservationLockedToMax = &config.RAMReserveAll
	}

	if config.NestedHV {
		confSpec.NestedHVEnabled = &config.NestedHV
	}

	if config.CpuHotAddEnabled {
		confSpec.CpuHotAddEnabled = &config.CpuHotAddEnabled
	}

	if config.MemoryHotAddEnabled {
		confSpec.MemoryHotAddEnabled = &config.MemoryHotAddEnabled
	}

	if config.VideoRAM != 0 || config.Displays != 0 {
		devices, err := vm.vm.Device(vm.driver.Ctx)
		if err != nil {
			return err
		}
		l := devices.SelectByType((*types.VirtualMachineVideoCard)(nil))
		if len(l) != 1 {
			return err
		}
		card := l[0].(*types.VirtualMachineVideoCard)

		if config.VideoRAM != 0 {
			card.VideoRamSizeInKB = config.VideoRAM
		}

		if config.Displays != 0 {
			card.NumDisplays = config.Displays
		}

		spec := &types.VirtualDeviceConfigSpec{
			Device:    card,
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		}
		confSpec.DeviceChange = append(confSpec.DeviceChange, spec)
	}

	if config.VGPUProfile != "" {
		devices, err := vm.vm.Device(vm.driver.Ctx)
		if err != nil {
			return err
		}

		pciDevices := devices.SelectByType((*types.VirtualPCIPassthrough)(nil))
		vGPUDevices := pciDevices.SelectByBackingInfo((*types.VirtualPCIPassthroughVmiopBackingInfo)(nil))
		var operation types.VirtualDeviceConfigSpecOperation
		if len(vGPUDevices) > 1 {
			return err
		} else if len(pciDevices) == 1 {
			operation = types.VirtualDeviceConfigSpecOperationEdit
		} else if len(pciDevices) == 0 {
			operation = types.VirtualDeviceConfigSpecOperationAdd
		}

		vGPUProfile := newVGPUProfile(config.VGPUProfile)
		spec := &types.VirtualDeviceConfigSpec{
			Device:    &vGPUProfile,
			Operation: operation,
		}
		log.Printf("Adding vGPU device with profile '%s'", config.VGPUProfile)
		confSpec.DeviceChange = append(confSpec.DeviceChange, spec)
	}

	if len(config.AllowedDevices) > 0 {
		VirtualPCIPassthroughAllowedDevice, err := newVirtualPCIPassthroughAllowedDevice(config.AllowedDevices)
		if err != nil {
			log.Printf("Failed to create VirtualPCIPassthrough: %s", err)
			return err
		}
		spec := &types.VirtualDeviceConfigSpec{
			Device:    &VirtualPCIPassthroughAllowedDevice,
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
		}
		confSpec.DeviceChange = append(confSpec.DeviceChange, spec)
	}

	firmware := config.Firmware
	efiSecureBootEnabled := firmware == "efi-secure"
	if efiSecureBootEnabled {
		firmware = "efi"
	}

	confSpec.Firmware = firmware

	if config.Firmware != "" || config.ForceBIOSSetup || config.BootDelay > 0 {
		confSpec.BootOptions = &types.VirtualMachineBootOptions{
			EnterBIOSSetup:       new(config.ForceBIOSSetup),
			EfiSecureBootEnabled: new(efiSecureBootEnabled),
			BootDelay:            config.BootDelay,
		}
	}

	task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	if err != nil {
		return err
	}

	devices, err := vm.Devices()
	if err != nil {
		return err
	}
	hasTPM := len(devices.SelectByType((*types.VirtualTPM)(nil))) > 0
	if config.VTPMEnabled != hasTPM {
		if !hasTPM {
			device := &types.VirtualTPM{}
			err = vm.addDevice(device)
		} else {
			err = vm.RemoveVTPM()
		}
	}
	if err != nil {
		return err
	}

	if config.VirtualPrecisionClock != "" && config.VirtualPrecisionClock != "none" {
		device := &types.VirtualPrecisionClock{
			VirtualDevice: types.VirtualDevice{
				Backing: &types.VirtualPrecisionClockSystemClockBackingInfo{
					Protocol: config.VirtualPrecisionClock,
				},
			},
		}
		err = vm.addDevice(device)
		if err != nil {
			return err
		}
	}

	return nil
}

// Reconfigure modifies the configuration of an existing virtual machine based
// on the provided configuration specification.
func (vm *VirtualMachineDriver) Reconfigure(confSpec types.VirtualMachineConfigSpec) error {
	task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// Customize applies the given CustomizationSpec to the virtual machine.
func (vm *VirtualMachineDriver) Customize(spec types.CustomizationSpec) error {
	task, err := vm.vm.Customize(vm.driver.Ctx, spec)
	if err != nil {
		return err
	}
	return task.Wait(vm.driver.Ctx)
}

// ResizeDisk adjusts the size of the virtual disk to the specified diskSize in
// KB. Returns a slice of configuration specifications to apply the change or
// an error if the operation fails.
// TODO: This method should be refactored to support resizing multiple disks.
func (vm *VirtualMachineDriver) ResizeDisk(diskSize int64) ([]types.BaseVirtualDeviceConfigSpec, error) {
	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return nil, err
	}

	disk, err := findDisk(devices)
	if err != nil {
		return nil, err
	}

	disk.CapacityInKB = diskSize * 1024
	disk.CapacityInBytes = disk.CapacityInKB * 1024

	return []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:    disk,
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		},
	}, nil
}

// PowerOn starts the virtual machine and waits for the operation to complete.
func (vm *VirtualMachineDriver) PowerOn() error {
	task, err := vm.vm.PowerOn(vm.driver.Ctx)
	if err != nil {
		return err
	}
	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// WaitForIP waits for a matching IP on a network adapter.
func (vm *VirtualMachineDriver) WaitForIP(ctx context.Context, ipNet *net.IPNet, adapterIndex *int) (string, error) {
	adapters, err := vm.NetworkAdapters(ctx)
	if err != nil {
		return "", err
	}
	if len(adapters) == 0 {
		return "", fmt.Errorf("no network adapters found")
	}

	if adapterIndex != nil {
		if *adapterIndex >= len(adapters) {
			return "", AdapterIndexOutOfRangeError(*adapterIndex, len(adapters), "network adapters")
		}
		adapters = []NetworkAdapter{adapters[*adapterIndex]}
	}

	return vm.waitForMatchingNetIP(ctx, ipNet, adapters)
}

// AdapterIndexOutOfRangeError reports that ip_wait_adapter_index is not valid
// for count adapters, named by noun (for example "network adapters").
func AdapterIndexOutOfRangeError(index, count int, noun string) error {
	if count <= 0 {
		return fmt.Errorf("ip_wait_adapter_index is %d, but no %s are defined", index, noun)
	}
	return fmt.Errorf("ip_wait_adapter_index is %d; valid indexes are 0 to %d (%d %s)", index, count-1, count, noun)
}

// NetworkAdapter is a virtual network adapter, ordered the same way as
// ip_wait_adapter_index (device key / network_adapters order).
type NetworkAdapter struct {
	Name string
	MAC  string
	Key  int32
}

func (vm *VirtualMachineDriver) waitForMatchingNetIP(ctx context.Context, ipNet *net.IPNet, adapters []NetworkAdapter) (string, error) {
	p := property.DefaultCollector(vm.vm.Client())
	var ip string
	err := property.Wait(ctx, p, vm.vm.Reference(), []string{"guest.net"}, func(pc []types.PropertyChange) bool {
		nics := guestNicsFromPropertyChange(pc)
		if candidate := selectIPByNetworkAdapters(adapters, nics, ipNet); candidate != "" {
			ip = candidate
			return true
		}
		return false
	})
	if err != nil {
		return "", err
	}

	return ip, nil
}

// NetworkAdapters returns the virtual machine's network adapters in
// ip_wait_adapter_index order, waiting until each has a MAC address.
func (vm *VirtualMachineDriver) NetworkAdapters(ctx context.Context) ([]NetworkAdapter, error) {
	p := property.DefaultCollector(vm.vm.Client())
	var adapters []NetworkAdapter

	err := property.Wait(ctx, p, vm.vm.Reference(), []string{"config.hardware.device"}, func(pc []types.PropertyChange) bool {
		found := false
		for _, c := range pc {
			if c.Op != types.PropertyChangeOpAssign {
				continue
			}

			devices := object.VirtualDeviceList(c.Val.(types.ArrayOfVirtualDevice).VirtualDevice)
			nics := networkAdaptersFromDevices(devices)
			if len(nics) == 0 {
				return false
			}
			for _, nic := range nics {
				if nic.MAC == "" {
					return false
				}
			}
			adapters = nics
			found = true
		}
		return found
	})
	if err != nil {
		return nil, err
	}

	return adapters, nil
}

func networkAdaptersFromDevices(devices object.VirtualDeviceList) []NetworkAdapter {
	var adapters []NetworkAdapter
	for _, d := range devices {
		nic, ok := d.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		card := nic.GetVirtualEthernetCard()
		adapters = append(adapters, NetworkAdapter{
			Name: devices.Name(d),
			MAC:  strings.ToLower(card.MacAddress),
			Key:  card.Key,
		})
	}
	// Device keys increase as NICs are added, which matches network_adapters
	// order. Unit number can be assigned out of that order and must not be used
	// as the index.
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Key < adapters[j].Key
	})
	return adapters
}

func guestNicsFromPropertyChange(pc []types.PropertyChange) []types.GuestNicInfo {
	var nics []types.GuestNicInfo
	for _, c := range pc {
		if c.Op != types.PropertyChangeOpAssign {
			continue
		}
		nics = append(nics, c.Val.(types.ArrayOfGuestNicInfo).GuestNicInfo...)
	}
	return nics
}

func selectIPByNetworkAdapters(adapters []NetworkAdapter, nics []types.GuestNicInfo, ipNet *net.IPNet) string {
	ipsByMAC := make(map[string][]string, len(nics))
	v4 := ipNetWantsIPv4(ipNet)
	for _, nic := range nics {
		mac := strings.ToLower(nic.MacAddress)
		if mac == "" || nic.IpConfig == nil {
			continue
		}
		for _, addr := range nic.IpConfig.IpAddress {
			if v4 && net.ParseIP(addr.IpAddress).To4() == nil {
				continue
			}
			ipsByMAC[mac] = append(ipsByMAC[mac], addr.IpAddress)
		}
	}

	for _, adapter := range adapters {
		for _, ip := range ipsByMAC[adapter.MAC] {
			if ipMatchesFilter(ip, ipNet) {
				return ip
			}
		}
	}
	return ""
}

func ipMatchesFilter(ip string, ipNet *net.IPNet) bool {
	parseIP := net.ParseIP(ip)
	if parseIP == nil {
		return false
	}
	if ipNet != nil && !ipNet.Contains(parseIP) {
		return false
	}
	if ipNet == nil && parseIP.To4() == nil {
		return false
	}
	return true
}

// ipNetWantsIPv4 reports whether the wait filter should ignore non-IPv4 addresses
// while waiting. Nil and IPv4 CIDRs (including the default 0.0.0.0/0) wait for IPv4
// so IPv6 link-local addresses do not complete the wait.
func ipNetWantsIPv4(ipNet *net.IPNet) bool {
	if ipNet == nil {
		return true
	}
	return ipNet.IP.To4() != nil
}

// PowerOff stops the virtual machine and waits for the operation to complete.
func (vm *VirtualMachineDriver) PowerOff() error {
	state, err := vm.vm.PowerState(vm.driver.Ctx)
	if err != nil {
		return err
	}

	if state == types.VirtualMachinePowerStatePoweredOff {
		return nil
	}

	task, err := vm.vm.PowerOff(vm.driver.Ctx)
	if err != nil {
		return err
	}
	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// IsPoweredOff checks if the virtual machine is powered off.
func (vm *VirtualMachineDriver) IsPoweredOff() (bool, error) {
	state, err := vm.vm.PowerState(vm.driver.Ctx)
	if err != nil {
		return false, err
	}

	return state == types.VirtualMachinePowerStatePoweredOff, nil
}

// StartShutdown initiates a guest shutdown operation.
func (vm *VirtualMachineDriver) StartShutdown() error {
	err := vm.vm.ShutdownGuest(vm.driver.Ctx)
	return err
}

// WaitForShutdown waits for the virtual machine to power off.
func (vm *VirtualMachineDriver) WaitForShutdown(ctx context.Context, timeout time.Duration) error {
	shutdownTimer := time.After(timeout)
	for {
		off, err := vm.IsPoweredOff()
		if err != nil {
			return err
		}
		if off {
			break
		}

		select {
		case <-shutdownTimer:
			err := errors.New("timeout while waiting for machine to shutdown")
			return err
		case <-ctx.Done():
			return nil
		default:
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

// CreateSnapshot creates a snapshot of the virtual machine.
func (vm *VirtualMachineDriver) CreateSnapshot(name string) error {
	task, err := vm.vm.CreateSnapshot(vm.driver.Ctx, name, "", false, false)
	if err != nil {
		return err
	}
	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// ConvertToTemplate converts the virtual machine to a template.
func (vm *VirtualMachineDriver) ConvertToTemplate() error {
	return vm.vm.MarkAsTemplate(vm.driver.Ctx)
}

// IsTemplate checks if the virtual machine is a template.
func (vm *VirtualMachineDriver) IsTemplate() (bool, error) {
	state, err := vm.vm.IsTemplate(vm.driver.Ctx)
	if err != nil {
		return false, err
	}

	return state, nil
}

// ConvertToVirtualMachine converts the template to a virtual machine.
func (vm *VirtualMachineDriver) ConvertToVirtualMachine(vsphereCluster string, vsphereHost string, vsphereResourcePool string) error {
	var host *object.HostSystem
	if vsphereCluster != "" && vsphereHost != "" {
		h, err := vm.driver.FindHost(vsphereHost)
		if err != nil {
			return err
		}
		host = h.host
	}

	resourcePool, err := vm.driver.FindResourcePool(vsphereCluster, vsphereHost, vsphereResourcePool)
	if err != nil {
		return err
	}

	return vm.vm.MarkAsVirtualMachine(vm.driver.Ctx, *resourcePool.pool, host)
}

// ImportOvfToContentLibrary imports the OVF to the content library.
func (vm *VirtualMachineDriver) ImportOvfToContentLibrary(ovf vcenter.OVF) error {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return err
	}

	l, err := vm.driver.FindContentLibraryByName(ovf.Target.LibraryID)
	if err != nil {
		log.Printf("cannot find content library: %v", err)
		vm.logout()
		return err
	}
	if l.library.Type != "LOCAL" {
		return fmt.Errorf("cannot deploy a VM to the content library %s of type %s; "+
			"the content library must be of type LOCAL", ovf.Target.LibraryID, l.library.Type)
	}

	item, err := vm.driver.FindContentLibraryItem(l.library.ID, ovf.Spec.Name)
	if err == nil {
		// Update the content library item if it exists.
		ovf.Target.LibraryItemID = item.ID
		if item.Description != nil && ovf.Spec.Description != *item.Description {
			err = vm.driver.UpdateContentLibraryItem(item, ovf.Spec.Name, ovf.Spec.Description)
			if err != nil {
				log.Printf("cannot update content library: %v", err)
				vm.logout()
				return err
			}
		}
	}

	ovf.Target.LibraryID = l.library.ID
	ovf.Source.Value = vm.vm.Reference().Value
	ovf.Source.Type = "VirtualMachine"

	vcm := vcenter.NewManager(vm.driver.RestClient.client)
	_, err = vcm.CreateOVF(vm.driver.Ctx, ovf)
	if err != nil {
		return err
	}

	return vm.driver.RestClient.Logout(vm.driver.Ctx)
}

// ImportToContentLibrary imports the virtual machine to the content library.
func (vm *VirtualMachineDriver) ImportToContentLibrary(template vcenter.Template) error {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return err
	}

	l, err := vm.driver.FindContentLibraryByName(template.Library)
	if err != nil {
		log.Printf("cannot find content library: %v", err)
		vm.logout()
		return err
	}
	if l.library.Type != "LOCAL" {
		return fmt.Errorf("cannot deploy a VM to the content library %s of type %s; "+
			"the content library must be of type LOCAL", template.Library, l.library.Type)
	}

	template.Library = l.library.ID
	template.SourceVM = vm.vm.Reference().Value

	if template.Placement.ResourcePool != "" {
		rp, err := vm.driver.FindResourcePool(template.Placement.Cluster, template.Placement.Host, template.Placement.ResourcePool)
		if err != nil {
			log.Printf("cannot find resource pool: %v", err)
			vm.logout()
			return err
		}
		template.Placement.ResourcePool = rp.pool.Reference().Value
	}
	if template.VMHomeStorage != nil {
		d, err := vm.driver.FindDatastore(template.VMHomeStorage.Datastore, template.Placement.Host)
		if err != nil {
			log.Printf("cannot find datastore: %v", err)
			vm.logout()
			return err
		}
		template.VMHomeStorage.Datastore = d.Reference().Value
	}

	if template.Placement.Cluster != "" {
		c, err := vm.driver.FindCluster(template.Placement.Cluster)
		if err != nil {
			return err
		}
		template.Placement.Cluster = c.cluster.Reference().Value
	}
	if template.Placement.Folder != "" {
		f, err := vm.driver.FindFolder(template.Placement.Folder)
		if err != nil {
			log.Printf("cannot find folder: %v", err)
			vm.logout()
			return err
		}
		template.Placement.Folder = f.folder.Reference().Value
	}
	if template.Placement.Host != "" {
		h, err := vm.driver.FindHost(template.Placement.Host)
		if err != nil {
			log.Printf("cannot find host: %v", err)
			vm.logout()
			return err
		}
		template.Placement.Host = h.host.Reference().Value
	}

	vcm := vcenter.NewManager(vm.driver.RestClient.client)
	_, err = vcm.CreateTemplate(vm.driver.Ctx, template)
	if err != nil {
		log.Printf("cannot create template: %v", err)
		vm.logout()
		return err
	}

	return vm.driver.RestClient.Logout(vm.driver.Ctx)
}

// GetDir returns the directory of the virtual machine. Returns an error if the
// operation fails.
func (vm *VirtualMachineDriver) GetDir() (string, error) {
	vmInfo, err := vm.Info("name", "layoutEx.file")
	if err != nil {
		return "", err
	}

	vmxName := fmt.Sprintf("/%s.vmx", vmInfo.Name)
	for _, file := range vmInfo.LayoutEx.File {
		if strings.Contains(file.Name, vmInfo.Name) {
			return RemoveDatastorePrefix(file.Name[:len(file.Name)-len(vmxName)]), nil
		}
	}
	return "", fmt.Errorf("cannot find '%s'", vmxName)
}

// addNetwork adds a network to the virtual machine. Returns a list of devices
// with the network added or an error if the operation fails.
func addNetwork(d *VCenterDriver, devices object.VirtualDeviceList, config *CreateConfig) (object.VirtualDeviceList, error) {
	for _, nic := range config.NICs {
		network, err := findNetwork(nic.Network, config.Host, d)
		if err != nil {
			return nil, err
		}

		backing, err := network.EthernetCardBackingInfo(d.Ctx)
		if err != nil {
			return nil, err
		}

		device, err := object.EthernetCardTypes().CreateEthernetCard(nic.NetworkCard, backing)
		if err != nil {
			return nil, err
		}

		card := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
		if nic.MacAddress != "" {
			card.AddressType = string(types.VirtualEthernetCardMacTypeManual)
			card.MacAddress = nic.MacAddress
		}
		card.UptCompatibilityEnabled = nic.Passthrough

		devices = append(devices, device)
	}
	return devices, nil
}

// findNetwork finds a network based on the network name and host.
func findNetwork(network string, host string, d *VCenterDriver) (object.NetworkReference, error) {
	if network != "" {
		var err error
		networks, err := d.FindNetworks(network)
		if err != nil {
			return nil, err
		}
		if len(networks) == 1 {
			return networks[0].network, nil
		}

		// If there are multiple networks, then try to match the host.
		if host != "" {
			h, err := d.FindHost(host)
			if err != nil {
				return nil, &MultipleNetworkFoundError{network, fmt.Sprintf("unable to match a network to the host %s: %s", host, err)}
			}
			for _, n := range networks {
				info, err := n.Info("host")
				if err != nil {
					continue
				}
				for _, host := range info.Host {
					if h.host.Reference().Value == host.Reference().Value {
						return n.network, nil
					}
				}
			}
			return nil, &MultipleNetworkFoundError{network, fmt.Sprintf("unable to match a network to the host %s", host)}
		}

		return nil, &MultipleNetworkFoundError{network, "specify the inventory path or id of the network"}
	}

	if host != "" {
		h, err := d.FindHost(host)
		if err != nil {
			return nil, err
		}

		i, err := h.Info("network")
		if err != nil {
			return nil, err
		}

		if len(i.Network) > 1 {
			return nil, fmt.Errorf("more than one network found on host %s, specify the network", host)
		}

		return object.NewNetwork(d.Client.Client, i.Network[0]), nil
	}

	return nil, fmt.Errorf("error finding network; 'host' and 'network' not specified. at least one of the two must be specified")
}

// newVirtualPCIPassthroughAllowedDevice creates a virtual PCI passthrough device.
func newVirtualPCIPassthroughAllowedDevice(devices []PCIPassthroughAllowedDevice) (types.VirtualPCIPassthrough, error) {
	allowedDevices := make([]types.VirtualPCIPassthroughAllowedDevice, len(devices))
	for i, device := range devices {
		deviceId, err := strconv.ParseInt(device.DeviceId, 16, 32)
		if err != nil {
			return types.VirtualPCIPassthrough{}, fmt.Errorf("error parsing DeviceId %s: %s", device.DeviceId, err)
		}
		vendorId, err := strconv.ParseInt(device.VendorId, 16, 32)
		if err != nil {
			return types.VirtualPCIPassthrough{}, fmt.Errorf("error parsing VendorId %s: %s", device.VendorId, err)
		}
		subVendorId, err := strconv.ParseInt(device.SubVendorId, 16, 32)
		if err != nil {
			return types.VirtualPCIPassthrough{}, fmt.Errorf("error parsing SubVendorId %s: %s", device.SubVendorId, err)
		}
		subDeviceId, err := strconv.ParseInt(device.SubDeviceId, 16, 32)
		if err != nil {
			return types.VirtualPCIPassthrough{}, fmt.Errorf("error parsing SubDeviceId %s: %s", device.SubDeviceId, err)
		}

		allowedDevices[i] = types.VirtualPCIPassthroughAllowedDevice{
			DeviceId:    int32(deviceId),
			VendorId:    int32(vendorId),
			SubVendorId: int32(subVendorId),
			SubDeviceId: int32(subDeviceId),
		}

		log.Printf("adding pci dynamic direct i/o passthrough device with device_id '%s',vendor_id '%s',subsystem_id '%s',subsystem_vendor_id '%s'",
			device.DeviceId,
			device.VendorId,
			device.SubDeviceId,
			device.SubVendorId)
	}

	return types.VirtualPCIPassthrough{
		VirtualDevice: types.VirtualDevice{
			DeviceInfo: &types.Description{
				Summary: "",
				Label:   "New PCI device",
			},
			Backing: &types.VirtualPCIPassthroughDynamicBackingInfo{
				AllowedDevice: allowedDevices,
			},
		},
	}, nil
}

// newVGPUProfile creates a vGPU profile.
func newVGPUProfile(vGPUProfile string) types.VirtualPCIPassthrough {
	return types.VirtualPCIPassthrough{
		VirtualDevice: types.VirtualDevice{
			DeviceInfo: &types.Description{
				Summary: "",
				Label:   fmt.Sprintf("New vGPU %v PCI device", vGPUProfile),
			},
			Backing: &types.VirtualPCIPassthroughVmiopBackingInfo{
				Vgpu: vGPUProfile,
			},
		},
	}
}

// MountCdrom mounts a CD-ROM to the virtual machine.
func (vm *VirtualMachineDriver) MountCdrom(controllerType string, datastoreIsoPath string, _cdrom types.BaseVirtualDevice) error {
	cdrom := _cdrom.(*types.VirtualCdrom)
	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return err
	}

	ds := &DatastoreIsoPath{path: datastoreIsoPath}
	if !ds.Validate() {
		return fmt.Errorf("%s is not a valid iso path", datastoreIsoPath)
	}
	if libPath, err := vm.driver.FindContentLibraryFileDatastorePath(ds.GetFilePath()); err == nil {
		datastoreIsoPath = libPath
	} else {
		log.Printf("Using %s as the datastore path", datastoreIsoPath)
	}

	devices.InsertIso(cdrom, datastoreIsoPath)

	err = devices.Connect(cdrom)
	if err != nil {
		return err
	}
	return nil
}

// AddCdrom adds a CD-ROM to the virtual machine.
func (vm *VirtualMachineDriver) AddCdrom(controllerType string, datastoreIsoPath string) error {
	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return err
	}

	var controller *types.VirtualController
	if controllerType == "sata" {
		c, err := vm.FindSATAController()
		if err != nil {
			return err
		}
		controller = c.GetVirtualController()
	} else {
		c, err := devices.FindIDEController("")
		if err != nil {
			return err
		}
		controller = c.GetVirtualController()
	}

	cdrom, err := vm.CreateCdrom(controller)
	if err != nil {
		return err
	}

	if datastoreIsoPath == "" {
		cdrom.Backing = &types.VirtualCdromRemotePassthroughBackingInfo{}
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	} else {
		err := vm.MountCdrom(controllerType, datastoreIsoPath, cdrom)
		if err != nil {
			return err
		}
	}

	log.Printf("Creating CD-ROM on controller '%v' with iso '%v'", controller, datastoreIsoPath)
	return vm.addDevice(cdrom)
}

// AddFloppy adds a floppy disk to the virtual machine.
func (vm *VirtualMachineDriver) AddFloppy(imgPath string) error {
	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return err
	}

	floppy, err := devices.CreateFloppy()
	if err != nil {
		return err
	}

	if imgPath != "" {
		floppy = devices.InsertImg(floppy, imgPath)
	}

	return vm.addDevice(floppy)
}

// SetBootOrder sets the boot order of the virtual machine.
func (vm *VirtualMachineDriver) SetBootOrder(order []string) error {
	devices, err := vm.vm.Device(vm.driver.Ctx)
	if err != nil {
		return err
	}

	bootOptions := types.VirtualMachineBootOptions{
		BootOrder: devices.BootOrder(order),
	}

	props, err := vm.Properties(vm.driver.Ctx)
	if err != nil {
		return err
	}
	if props.Config.BootOptions != nil {
		current := props.Config.BootOptions
		bootOptions.BootDelay = current.BootDelay
		bootOptions.EnterBIOSSetup = current.EnterBIOSSetup
		bootOptions.EfiSecureBootEnabled = current.EfiSecureBootEnabled
	}

	return vm.vm.SetBootOptions(vm.driver.Ctx, &bootOptions)
}

// RemoveDevice removes a device from the virtual machine.
func (vm *VirtualMachineDriver) RemoveDevice(keepFiles bool, device ...types.BaseVirtualDevice) error {
	return vm.vm.RemoveDevice(vm.driver.Ctx, keepFiles, device...)
}

// addDevice adds a device to the virtual machine.
func (vm *VirtualMachineDriver) addDevice(device types.BaseVirtualDevice) error {
	newDevices := object.VirtualDeviceList{device}
	confSpec := types.VirtualMachineConfigSpec{}
	var err error
	confSpec.DeviceChange, err = newDevices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return err
	}

	task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
	if err != nil {
		return err
	}

	_, err = task.WaitForResult(vm.driver.Ctx, nil)
	return err
}

// AddConfigParams adds configuration parameters to the virtual machine.
func (vm *VirtualMachineDriver) AddConfigParams(params map[string]string, info *types.ToolsConfigInfo) error {
	var confSpec types.VirtualMachineConfigSpec

	var ov []types.BaseOptionValue
	for k, v := range params {
		o := &types.OptionValue{
			Key:   k,
			Value: v,
		}
		ov = append(ov, o)
	}
	confSpec.ExtraConfig = ov

	confSpec.Tools = info

	if len(confSpec.ExtraConfig) > 0 || confSpec.Tools != nil {
		task, err := vm.vm.Reconfigure(vm.driver.Ctx, confSpec)
		if err != nil {
			return fmt.Errorf("failed to start reconfiguration task: %w", err)
		}

		_, err = task.WaitForResult(vm.driver.Ctx, nil)
		if err != nil {
			return fmt.Errorf("reconfiguration task failed: %w", err)
		}

		log.Println("[INFO] Reconfiguration task completed successfully.")

		// Retrieve the current configuration.
		var moVM mo.VirtualMachine
		err = vm.vm.Properties(vm.driver.Ctx, vm.vm.Reference(), []string{"config.extraConfig"}, &moVM)
		if err != nil {
			return fmt.Errorf("failed to retrieve current configuration: %w", err)
		}

		// Check for ignored parameters.
		var ignoredParams []string
		for k, v := range params {
			found := false
			for _, option := range moVM.Config.ExtraConfig {
				if optVal, ok := option.(*types.OptionValue); ok && optVal.Key == k && optVal.Value == v {
					found = true
					break
				}
			}
			if !found {
				ignoredParams = append(ignoredParams, fmt.Sprintf("%s = %v", k, v))
			}
		}

		if len(ignoredParams) > 0 {
			log.Printf("[INFO] Ignored the following parameters: [%s]", strings.Join(ignoredParams, " , "))
			log.Printf("[INFO] Some configuration keys were ignored due to conflicts with other fields in the ConfigSpec. Refer to VirtualMachineConfigSpec in the vSphere API documentation.")
		}
	}

	return nil
}

// AddFlag adds a flag to the virtual machine.
func (vm *VirtualMachineDriver) AddFlag(ctx context.Context, flagSpec *types.VirtualMachineFlagInfo) error {
	confSpec := types.VirtualMachineConfigSpec{
		Flags: flagSpec,
	}

	task, err := vm.vm.Reconfigure(ctx, confSpec)
	if err != nil {
		return err
	}

	err = task.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Export exports the virtual machine.
func (vm *VirtualMachineDriver) Export() (*nfc.Lease, error) {
	return vm.vm.Export(vm.driver.Ctx)
}

// CreateDescriptor creates a descriptor for the virtual machine used when exporting the virtual machine to an OVF.
func (vm *VirtualMachineDriver) CreateDescriptor(m *ovf.Manager, cdp types.OvfCreateDescriptorParams) (*types.OvfCreateDescriptorResult, error) {
	return m.CreateDescriptor(vm.driver.Ctx, vm.vm, cdp)
}

// NewOvfManager creates a new OVF manager instance.
func (vm *VirtualMachineDriver) NewOvfManager() *ovf.Manager {
	return ovf.NewManager(vm.vm.Client())
}

// GetOvfExportOptions retrieves the OVF export options for the virtual machine.
func (vm *VirtualMachineDriver) GetOvfExportOptions(m *ovf.Manager) ([]types.OvfOptionInfo, error) {
	var mgr mo.OvfManager
	err := property.DefaultCollector(vm.vm.Client()).RetrieveOne(vm.driver.Ctx, m.Reference(), nil, &mgr)
	if err != nil {
		return nil, err
	}
	return mgr.OvfExportOption, nil
}

// NewHost creates a new host instance.
func (vm *VirtualMachineDriver) NewHost(ref *types.ManagedObjectReference) *Host {
	return vm.driver.NewHost(ref)
}

// NewResourcePool creates a new resource pool instance.
func (vm *VirtualMachineDriver) NewResourcePool(ref *types.ManagedObjectReference) *ResourcePool {
	return vm.driver.NewResourcePool(ref)
}

// NewDatastore creates a new datastore instance.
func (vm *VirtualMachineDriver) NewDatastore(ref *types.ManagedObjectReference) Datastore {
	return vm.driver.NewDatastore(ref)
}

// NewNetwork creates a new network instance.
func (vm *VirtualMachineDriver) NewNetwork(ref *types.ManagedObjectReference) *Network {
	return vm.driver.NewNetwork(ref)
}

// Datacenter returns the datacenter of the virtual machine.
func (vm *VirtualMachineDriver) Datacenter() *object.Datacenter {
	return vm.driver.Datacenter
}

// FindContentLibraryItem finds a content library item by library name and item name.
// Returns the library item or an error if not found.
func (vm *VirtualMachineDriver) FindContentLibraryItem(libraryName string, itemName string) (*library.Item, error) {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return nil, err
	}

	l, err := vm.driver.FindContentLibraryByName(libraryName)
	if err != nil {
		log.Printf("cannot find content library: %v", err)
		vm.logout()
		return nil, err
	}

	item, err := vm.driver.FindContentLibraryItem(l.library.ID, itemName)
	if err != nil {
		vm.logout()
		return nil, err
	}

	_ = vm.driver.RestClient.Logout(vm.driver.Ctx)
	return item, nil
}

// DeleteContentLibraryItem deletes a content library item by its ID.
func (vm *VirtualMachineDriver) DeleteContentLibraryItem(itemID string) error {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return err
	}

	if err := vm.driver.DeleteContentLibraryItem(itemID); err != nil {
		log.Printf("cannot delete content library item: %v", err)
		vm.logout()
		return err
	}

	return vm.driver.RestClient.Logout(vm.driver.Ctx)
}

// FindContentLibraryItemUUID finds a content library item by name.
func (vm *VirtualMachineDriver) FindContentLibraryItemUUID(library string, name string) (string, error) {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return "", err
	}

	l, err := vm.driver.FindContentLibraryByName(library)
	if err != nil {
		log.Printf("cannot find content library: %v", err)
		vm.logout()
		return "", err
	}

	item, err := vm.driver.FindContentLibraryItemUUID(l.library.ID, name)
	if err != nil {
		log.Printf("cannot find content library item: %v", err)
		vm.logout()
		return "", err
	}

	return item, nil
}

// FindContentLibraryTemplateDatastoreName finds the datastore name of the content library template.
func (vm *VirtualMachineDriver) FindContentLibraryTemplateDatastoreName(library string) ([]string, error) {
	err := vm.driver.RestClient.Login(vm.driver.Ctx)
	if err != nil {
		return nil, err
	}

	l, err := vm.driver.FindContentLibraryByName(library)
	if err != nil {
		log.Printf("cannot find content library: %v", err)
		vm.logout()
		return nil, err
	}
	var datastores []string
	for _, storage := range l.library.Storage {
		name, err := vm.driver.GetDatastoreName(storage.DatastoreID)
		if err != nil {
			log.Printf("Failed to get Content Library datastore name: %s", err)
			continue
		}
		datastores = append(datastores, name)
	}
	return datastores, vm.driver.RestClient.Logout(vm.driver.Ctx)
}

// logout logs the user out of the vCenter.
func (vm *VirtualMachineDriver) logout() {
	if vm.driver.RestClient == nil {
		return
	}
	if err := vm.driver.RestClient.Logout(vm.driver.Ctx); err != nil {
		log.Printf("cannot logout: %s ", err)
	}
}

// Reference returns the managed object reference for the virtual machine.
func (vm *VirtualMachineDriver) Reference() types.ManagedObjectReference {
	return vm.vm.Reference()
}

// findNetworkAdapter finds a network adapter in the virtual machine.
func findNetworkAdapter(l object.VirtualDeviceList) (types.BaseVirtualEthernetCard, error) {
	c := l.SelectByType((*types.VirtualEthernetCard)(nil))
	if len(c) == 0 {
		return nil, errors.New("no network adapter device found")
	}

	return c[0].(types.BaseVirtualEthernetCard), nil
}

// RemoveNetworkAdapters removes all network adapters from the virtual machine.
func (vm *VirtualMachineDriver) RemoveNetworkAdapters() error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("error retrieving devices: %s", err)
	}

	networkAdapters := devices.SelectByType((*types.VirtualEthernetCard)(nil))
	if len(networkAdapters) == 0 {
		return nil
	}

	for _, adapter := range networkAdapters {
		err = vm.RemoveDevice(false, adapter)
		if err != nil {
			return fmt.Errorf("error removing network adapter: %s", err)
		}
	}

	return nil
}

// RemoveVTPM removes the virtual trusted platform module (vTPM) device from
// the virtual machine, if present.
func (vm *VirtualMachineDriver) RemoveVTPM() error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("error retrieving devices: %s", err)
	}

	tpms := devices.SelectByType((*types.VirtualTPM)(nil))
	if len(tpms) == 0 {
		return nil
	}

	if err = vm.RemoveDevice(false, tpms...); err != nil {
		return fmt.Errorf("error removing vTPM: %s", err)
	}

	return nil
}

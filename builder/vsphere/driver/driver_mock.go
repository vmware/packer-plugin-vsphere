// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25/types"
)

// DriverMock provides a mock implementation of the Driver interface for testing.
type DriverMock struct {
	FindDatastoreCalled bool
	DatastoreMock       *DatastoreMock
	FindDatastoreName   string
	FindDatastoreHost   string
	FindDatastoreErr    error

	PreCleanShouldFail bool
	PreCleanVMCalled   bool
	PreCleanForce      bool
	PreCleanVMPath     string

	CreateVMShouldFail bool
	CreateVMCalled     bool
	CreateConfig       *CreateConfig
	VM                 VirtualMachine

	FindVMCalled bool
	FindVMName   string

	// OVF deployment mock fields.
	DeployOvfCalled     bool
	DeployOvfConfig     *OvfDeployConfig
	DeployOvfShouldFail bool
	DeployOvfError      error
	DeployOvfVM         VirtualMachine

	GetOvfOptionsCalled        bool
	GetOvfOptionsURL           string
	GetOvfOptionsAuth          *OvfAuthConfig
	GetOvfOptionsLocale        string
	GetOvfOptionsSkipTlsVerify bool
	GetOvfOptionsShouldFail    bool
	GetOvfOptionsError         error
	GetOvfOptionsResult        []types.OvfOptionInfo
}

// NewDriverMock creates a new instance of DriverMock for testing.
func NewDriverMock() *DriverMock {
	return new(DriverMock)
}

func (d *DriverMock) FindDatastore(name string, host string) (Datastore, error) {
	d.FindDatastoreCalled = true
	if d.DatastoreMock == nil {
		d.DatastoreMock = new(DatastoreMock)
	}
	d.FindDatastoreName = name
	d.FindDatastoreHost = host
	return d.DatastoreMock, d.FindDatastoreErr
}

func (d *DriverMock) NewVM(ref *types.ManagedObjectReference) VirtualMachine {
	return nil
}

func (d *DriverMock) FindVM(name string) (VirtualMachine, error) {
	d.FindVMCalled = true
	if d.VM == nil {
		d.VM = new(VirtualMachineMock)
	}
	d.FindVMName = name
	return d.VM, d.FindDatastoreErr
}

func (d *DriverMock) FindCluster(name string) (*Cluster, error) {
	return nil, nil
}

func (d *DriverMock) PreCleanVM(ui packersdk.Ui, vmPath string, force bool, vsphereCluster string, vsphereHost string, vsphereResourcePool string) error {
	d.PreCleanVMCalled = true
	if d.PreCleanShouldFail {
		return fmt.Errorf("pre clean failed")
	}
	d.PreCleanForce = true
	d.PreCleanVMPath = vmPath
	return nil
}

func (d *DriverMock) CreateVM(config *CreateConfig) (VirtualMachine, error) {
	d.CreateVMCalled = true
	if d.CreateVMShouldFail {
		return nil, fmt.Errorf("create vm failed")
	}
	d.CreateConfig = config
	d.VM = new(VirtualMachineDriver)
	return d.VM, nil
}

func (d *DriverMock) NewDatastore(ref *types.ManagedObjectReference) Datastore { return nil }

func (d *DriverMock) GetDatastoreName(id string) (string, error) { return "", nil }

func (d *DriverMock) GetDatastoreFilePath(datastoreID, dir, filename string) (string, error) {
	return "", nil
}

func (d *DriverMock) NewFolder(ref *types.ManagedObjectReference) *Folder { return nil }

func (d *DriverMock) FindFolder(name string) (*Folder, error) { return nil, nil }

func (d *DriverMock) NewHost(ref *types.ManagedObjectReference) *Host { return nil }

func (d *DriverMock) FindHost(name string) (*Host, error) { return nil, nil }

func (d *DriverMock) NewNetwork(ref *types.ManagedObjectReference) *Network { return nil }

func (d *DriverMock) FindNetwork(name string) (*Network, error) { return nil, nil }

func (d *DriverMock) FindNetworks(name string) ([]*Network, error) { return nil, nil }

func (d *DriverMock) NewResourcePool(ref *types.ManagedObjectReference) *ResourcePool { return nil }

func (d *DriverMock) FindResourcePool(cluster string, host string, name string) (*ResourcePool, error) {
	return nil, nil
}

func (d *DriverMock) FindContentLibraryByName(name string) (*Library, error) { return nil, nil }

func (d *DriverMock) FindContentLibraryItem(libraryId string, name string) (*library.Item, error) {
	return nil, nil
}

func (d *DriverMock) FindContentLibraryFileDatastorePath(isoPath string) (string, error) {
	return "", nil
}

func (d *DriverMock) UpdateContentLibraryItem(item *library.Item, name string, description string) error {
	return nil
}

func (d *DriverMock) DeleteContentLibraryItem(itemID string) error {
	return nil
}

func (d *DriverMock) GetRestClient() *rest.Client {
	return nil
}

// DeployOvf mocks OVF deployment functionality for testing.
func (d *DriverMock) DeployOvf(ctx context.Context, config *OvfDeployConfig, ui packersdk.Ui) (VirtualMachine, error) {
	d.DeployOvfCalled = true
	d.DeployOvfConfig = config

	if d.DeployOvfShouldFail {
		if d.DeployOvfError != nil {
			return nil, d.DeployOvfError
		}
		return nil, fmt.Errorf("deploy OVF failed")
	}

	if d.DeployOvfVM == nil {
		d.DeployOvfVM = new(VirtualMachineMock)
	}
	return d.DeployOvfVM, nil
}

// GetOvfOptions mocks OVF options retrieval functionality for testing.
func (d *DriverMock) GetOvfOptions(ctx context.Context, url string, auth *OvfAuthConfig, locale string, skipTlsVerify bool) ([]types.OvfOptionInfo, error) {
	d.GetOvfOptionsCalled = true
	d.GetOvfOptionsURL = url
	d.GetOvfOptionsAuth = auth
	d.GetOvfOptionsLocale = locale
	d.GetOvfOptionsSkipTlsVerify = skipTlsVerify

	if d.GetOvfOptionsShouldFail {
		if d.GetOvfOptionsError != nil {
			return nil, d.GetOvfOptionsError
		}
		return nil, fmt.Errorf("get OVF options failed")
	}

	if d.GetOvfOptionsResult == nil {
		// Return default mock options.
		d.GetOvfOptionsResult = []types.OvfOptionInfo{
			{
				Option: "small",
				Description: types.LocalizableMessage{
					Message: "Small configuration",
				},
			},
			{
				Option: "medium",
				Description: types.LocalizableMessage{
					Message: "Medium configuration",
				},
			},
		}
	}

	return d.GetOvfOptionsResult, nil
}

func (d *DriverMock) Cleanup() (error, error) {
	return nil, nil
}

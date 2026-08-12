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
	FindDatastoreErr                    error
	VM                                  VirtualMachine
	DeployOvfError                      error
	DeployOvfVM                         VirtualMachine
	GetOvfOptionsError                  error
	ResolveContentLibraryItemError      error
	DeployContentLibraryItemError       error
	DeployContentLibraryItemVM          VirtualMachine
	FindStoragePolicyIDErr              error
	FindCompatibleDatastoreResult       Datastore
	FindCompatibleDatastoreErr          error
	DatastoreMock                       *DatastoreMock
	CreateConfig                        *CreateConfig
	DeployOvfConfig                     *OvfDeployConfig
	GetOvfOptionsConfig                 *OvfDeployConfig
	GetOvfOptionsAuth                   *OvfAuthConfig
	ResolveContentLibraryItemResult     *library.Item
	DeployContentLibraryItemConfig      *ContentLibraryDeployConfig
	FindStoragePolicyIDByName           map[string]string
	FindCompatibleDatastoreByPolicy     map[string]Datastore
	FindDatastoreName                   string
	FindDatastoreHost                   string
	PreCleanVMPath                      string
	FindVMName                          string
	GetOvfOptionsURL                    string
	GetOvfOptionsLocale                 string
	ResolveContentLibraryItemLibrary    string
	ResolveContentLibraryItemName       string
	FindStoragePolicyIDName             string
	FindStoragePolicyIDResult           string
	FindCompatibleDatastorePolicyID     string
	FindCompatibleDatastoreHost         string
	FindCompatibleDatastoreCluster      string
	GetOvfOptionsResult                 []types.OvfOptionInfo
	FindStoragePolicyIDCalls            []string
	FindCompatibleDatastoreCalls        []string
	FindDatastoreCalled                 bool
	PreCleanShouldFail                  bool
	PreCleanVMCalled                    bool
	PreCleanForce                       bool
	CreateVMShouldFail                  bool
	CreateVMCalled                      bool
	FindVMCalled                        bool
	DeployOvfCalled                     bool
	DeployOvfShouldFail                 bool
	GetOvfOptionsCalled                 bool
	GetOvfOptionsShouldFail             bool
	GetOvfOptionsSkipTlsVerify          bool
	ResolveContentLibraryItemCalled     bool
	ResolveContentLibraryItemShouldFail bool
	DeployContentLibraryItemCalled      bool
	DeployContentLibraryItemShouldFail  bool
	FindStoragePolicyIDCalled           bool
	FindCompatibleDatastoreCalled       bool
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
func (d *DriverMock) GetOvfOptions(ctx context.Context, config *OvfDeployConfig) ([]types.OvfOptionInfo, error) {
	d.GetOvfOptionsCalled = true
	d.GetOvfOptionsConfig = config
	if config != nil {
		d.GetOvfOptionsURL = config.URL
		d.GetOvfOptionsAuth = config.Authentication
		d.GetOvfOptionsLocale = config.Locale
		d.GetOvfOptionsSkipTlsVerify = config.SkipTlsVerify
	}

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

func (d *DriverMock) ResolveContentLibraryItem(libraryName, itemName string) (*library.Item, error) {
	d.ResolveContentLibraryItemCalled = true
	d.ResolveContentLibraryItemLibrary = libraryName
	d.ResolveContentLibraryItemName = itemName

	if d.ResolveContentLibraryItemShouldFail {
		if d.ResolveContentLibraryItemError != nil {
			return nil, d.ResolveContentLibraryItemError
		}
		return nil, fmt.Errorf("resolve content library item failed")
	}

	if d.ResolveContentLibraryItemResult == nil {
		d.ResolveContentLibraryItemResult = &library.Item{
			Name: itemName,
			Type: library.ItemTypeVMTX,
		}
	}

	return d.ResolveContentLibraryItemResult, nil
}

func (d *DriverMock) DeployContentLibraryItem(ctx context.Context, config *ContentLibraryDeployConfig, ui packersdk.Ui) (VirtualMachine, error) {
	d.DeployContentLibraryItemCalled = true
	d.DeployContentLibraryItemConfig = config

	if d.DeployContentLibraryItemShouldFail {
		if d.DeployContentLibraryItemError != nil {
			return nil, d.DeployContentLibraryItemError
		}
		return nil, fmt.Errorf("deploy content library item failed")
	}

	if d.DeployContentLibraryItemVM == nil {
		d.DeployContentLibraryItemVM = new(VirtualMachineMock)
	}
	return d.DeployContentLibraryItemVM, nil
}

func (d *DriverMock) FindStoragePolicyID(name string) (string, error) {
	d.FindStoragePolicyIDCalled = true
	d.FindStoragePolicyIDName = name
	d.FindStoragePolicyIDCalls = append(d.FindStoragePolicyIDCalls, name)
	if d.FindStoragePolicyIDErr != nil {
		return "", d.FindStoragePolicyIDErr
	}
	if d.FindStoragePolicyIDByName != nil {
		if id, ok := d.FindStoragePolicyIDByName[name]; ok {
			return id, nil
		}
		return "", fmt.Errorf("storage policy %q not found in mock", name)
	}
	return d.FindStoragePolicyIDResult, nil
}

func (d *DriverMock) FindCompatibleDatastore(policyID, host, cluster string) (Datastore, error) {
	d.FindCompatibleDatastoreCalled = true
	d.FindCompatibleDatastorePolicyID = policyID
	d.FindCompatibleDatastoreHost = host
	d.FindCompatibleDatastoreCluster = cluster
	d.FindCompatibleDatastoreCalls = append(d.FindCompatibleDatastoreCalls, policyID)
	if d.FindCompatibleDatastoreErr != nil {
		return nil, d.FindCompatibleDatastoreErr
	}
	if d.FindCompatibleDatastoreByPolicy != nil {
		if ds, ok := d.FindCompatibleDatastoreByPolicy[policyID]; ok {
			return ds, nil
		}
		return nil, fmt.Errorf("no compatible datastore mock for policy %q", policyID)
	}
	if d.FindCompatibleDatastoreResult == nil {
		d.FindCompatibleDatastoreResult = new(DatastoreMock)
	}
	return d.FindCompatibleDatastoreResult, nil
}

func (d *DriverMock) Cleanup() (error, error) {
	return nil, nil
}

// SelectDatastoresForDisks mocks multi-disk Storage DRS placement for testing.
func (d *DriverMock) SelectDatastoresForDisks(clusterName string, input StoragePlacementInput) ([]Datastore, string, error) {
	return nil, "", fmt.Errorf("SelectDatastoresForDisks not implemented in DriverMock")
}

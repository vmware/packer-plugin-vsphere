// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"net/url"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// Driver defines the interface for vSphere operations.
type Driver interface {
	NewVM(ref *types.ManagedObjectReference) VirtualMachine
	FindVM(name string) (VirtualMachine, error)
	FindCluster(name string) (*Cluster, error)
	PreCleanVM(ui packersdk.Ui, vmPath string, force bool, vsphereCluster string, vsphereHost string, vsphereResourcePool string) error
	CreateVM(config *CreateConfig) (VirtualMachine, error)

	NewDatastore(ref *types.ManagedObjectReference) Datastore
	FindDatastore(name string, host string) (Datastore, error)
	GetDatastoreName(id string) (string, error)
	GetDatastoreFilePath(datastoreID, dir, filename string) (string, error)

	NewFolder(ref *types.ManagedObjectReference) *Folder
	FindFolder(name string) (*Folder, error)
	NewHost(ref *types.ManagedObjectReference) *Host
	FindHost(name string) (*Host, error)
	NewNetwork(ref *types.ManagedObjectReference) *Network
	FindNetwork(name string) (*Network, error)
	FindNetworks(name string) ([]*Network, error)
	NewResourcePool(ref *types.ManagedObjectReference) *ResourcePool
	FindResourcePool(cluster string, host string, name string) (*ResourcePool, error)

	FindContentLibraryByName(name string) (*Library, error)
	FindContentLibraryItem(libraryId string, name string) (*library.Item, error)
	FindContentLibraryFileDatastorePath(isoPath string) (string, error)
	UpdateContentLibraryItem(item *library.Item, name string, description string) error
	GetRestClient() *rest.Client
	DeleteContentLibraryItem(itemID string) error
	DeployOvf(ctx context.Context, config *OvfDeployConfig, ui packersdk.Ui) (VirtualMachine, error)
	GetOvfOptions(ctx context.Context, config *OvfDeployConfig) ([]types.OvfOptionInfo, error)
	Cleanup() (error, error)
}

// VCenterDriver implements the Driver interface for vCenter operations.
type VCenterDriver struct {
	Ctx        context.Context
	Client     *govmomi.Client
	VimClient  *vim25.Client
	RestClient *RestClient
	Finder     *find.Finder
	Datacenter *object.Datacenter
}

func NewVCenterDriver(ctx context.Context, client *govmomi.Client, vimClient *vim25.Client, user *url.Userinfo, finder *find.Finder, datacenter *object.Datacenter) *VCenterDriver {
	return &VCenterDriver{
		Ctx:       ctx,
		Client:    client,
		VimClient: vimClient,
		RestClient: &RestClient{
			client:      rest.NewClient(vimClient),
			credentials: user,
		},
		Datacenter: datacenter,
		Finder:     finder,
	}
}

// ConnectConfig contains the configuration for connecting to vCenter.
type ConnectConfig struct {
	VCenterServer      string
	Username           string
	Password           string
	InsecureConnection bool
	Datacenter         string
}

func NewDriver(config *ConnectConfig) (Driver, error) {
	ctx := context.TODO()

	vcenterUrl, err := url.Parse(fmt.Sprintf("https://%v/sdk", config.VCenterServer))
	if err != nil {
		return nil, err
	}
	credentials := url.UserPassword(config.Username, config.Password)
	vcenterUrl.User = credentials

	soapClient := soap.NewClient(vcenterUrl, config.InsecureConnection)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = client.SessionManager.Login(ctx, credentials)
	if err != nil {
		return nil, err
	}

	restClient := &RestClient{
		client:      rest.NewClient(vimClient),
		credentials: credentials,
	}

	if err := restClient.Login(ctx); err != nil {
		return nil, fmt.Errorf("failed to login to REST API: %w", err)
	}

	finder := find.NewFinder(client.Client, false)
	datacenter, err := finder.DatacenterOrDefault(ctx, config.Datacenter)
	if err != nil {
		return nil, err
	}
	finder.SetDatacenter(datacenter)

	d := &VCenterDriver{
		Ctx:        ctx,
		Client:     client,
		VimClient:  vimClient,
		RestClient: restClient,
		Datacenter: datacenter,
		Finder:     finder,
	}
	return d, nil
}

func (d *VCenterDriver) GetRestClient() *rest.Client {
	return d.RestClient.client
}
func (d *VCenterDriver) Cleanup() (error, error) {
	return d.RestClient.client.Logout(d.Ctx), d.Client.SessionManager.Logout(d.Ctx)
}

// RestClient manages RESTful interactions with vCenter.
type RestClient struct {
	client      *rest.Client
	credentials *url.Userinfo
}

func (r *RestClient) Login(ctx context.Context) error {
	return r.client.Login(ctx, r.credentials)
}

func (r *RestClient) Logout(ctx context.Context) error {
	return r.client.Logout(ctx)
}

// Client returns the underlying rest.Client for direct API access.
func (r *RestClient) Client() *rest.Client {
	return r.client
}

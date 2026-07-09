// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common/utils"
)

// testUI provides a simple UI implementation for testing.
type testUI struct{}

func (ui *testUI) Ask(string) (string, error)                                      { return "", nil }
func (ui *testUI) Askf(format string, args ...interface{}) (string, error)         { return "", nil }
func (ui *testUI) Say(message string)                                              {}
func (ui *testUI) Sayf(format string, args ...interface{})                         {}
func (ui *testUI) Message(message string)                                          {}
func (ui *testUI) Messagef(format string, args ...interface{})                     {}
func (ui *testUI) Error(message string)                                            {}
func (ui *testUI) Errorf(format string, args ...interface{})                       {}
func (ui *testUI) Machine(string, ...string)                                       {}
func (ui *testUI) TrackProgress(string, int64, int64, io.ReadCloser) io.ReadCloser { return nil }

// newTestDriver creates a new driver instance for testing.
func newTestDriver(t *testing.T) Driver {
	vcenter := utils.GetenvOrDefault(utils.EnvVcenterServer, utils.DefaultVcenterServer)
	username := utils.GetenvOrDefault(utils.EnvVsphereUsername, utils.DefaultVsphereUsername)
	password := utils.GetenvOrDefault(utils.EnvVspherePassword, utils.DefaultVspherePassword)

	d, err := NewDriver(&ConnectConfig{
		VCenterServer:      vcenter,
		Username:           username,
		Password:           password,
		InsecureConnection: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return d
}

// newVMName generates a random VM name for testing.
func newVMName() string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	return fmt.Sprintf("test-%v", r.Intn(1000))
}

// VCenterSimulator provides a vCenter simulator for testing.
type VCenterSimulator struct {
	model  *simulator.Model
	server *simulator.Server
	driver *VCenterDriver
}

// NewCustomVCenterSimulator creates a new vCenter simulator with a custom model.
func NewCustomVCenterSimulator(model *simulator.Model) (*VCenterSimulator, error) {
	sim := new(VCenterSimulator)
	sim.model = model

	server, err := sim.NewSimulatorServer()
	if err != nil {
		sim.Close()
		return nil, err
	}
	sim.server = server

	driver, err := sim.NewSimulatorDriver()
	if err != nil {
		sim.Close()
		return nil, err
	}
	sim.driver = driver
	return sim, nil
}

// NewVCenterSimulator creates a new vCenter simulator with default VPX model.
func NewVCenterSimulator() (*VCenterSimulator, error) {
	model := simulator.VPX()
	model.Machine = 1
	return NewCustomVCenterSimulator(model)
}

// Close shuts down the simulator and cleans up resources.
func (s *VCenterSimulator) Close() {
	if s.model != nil {
		s.model.Remove()
	}
	if s.server != nil {
		s.server.Close()
	}
}

// Simulator shortcut to choose any pre-created virtual machine.
func (s *VCenterSimulator) ChooseSimulatorPreCreatedVM() (VirtualMachine, *simulator.VirtualMachine) {
	machine := s.model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	ref := machine.Reference()
	vm := s.driver.NewVM(&ref)
	return vm, machine
}

// Simulator shortcut to choose any pre-created datastore.
func (s *VCenterSimulator) ChooseSimulatorPreCreatedDatastore() (Datastore, *simulator.Datastore) {
	ds := s.model.Map().Any("Datastore").(*simulator.Datastore)
	ref := ds.Reference()
	datastore := s.driver.NewDatastore(&ref)
	return datastore, ds
}

// Simulator shortcut to choose any pre-created ESX host.
func (s *VCenterSimulator) ChooseSimulatorPreCreatedHost() (*Host, *simulator.HostSystem) {
	h := s.model.Map().Any("HostSystem").(*simulator.HostSystem)
	ref := h.Reference()
	host := s.driver.NewHost(&ref)
	return host, h
}

// NewSimulatorServer creates and configures a new simulator server.
func (s *VCenterSimulator) NewSimulatorServer() (*simulator.Server, error) {
	err := s.model.Create()
	if err != nil {
		return nil, err
	}

	s.model.Service.RegisterEndpoints = true
	s.model.Service.TLS = new(tls.Config)
	s.model.Service.ServeMux = http.NewServeMux()
	return s.model.Service.NewServer(), nil
}

// NewSimulatorDriver creates a new driver connected to the simulator.
func (s *VCenterSimulator) NewSimulatorDriver() (*VCenterDriver, error) {
	ctx := context.TODO()
	user := &url.Userinfo{}
	s.server.URL.User = user

	soapClient := soap.NewClient(s.server.URL, true)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = client.SessionManager.Login(ctx, user)
	if err != nil {
		return nil, err
	}

	finder := find.NewFinder(client.Client, false)
	datacenter, err := finder.DatacenterOrDefault(ctx, "")
	if err != nil {
		return nil, err
	}
	finder.SetDatacenter(datacenter)

	d := &VCenterDriver{
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
	return d, nil
}

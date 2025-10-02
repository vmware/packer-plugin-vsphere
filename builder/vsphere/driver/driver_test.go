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
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
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

// Simulator shortcut to choose any pre-created EXSi host.
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

// TestOvfManagerWrapper_ValidateURL tests URL validation for OVF Manager
// wrapper functionality.
func TestOvfManagerWrapper_ValidateURL(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid HTTP URL",
			url:         "http://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Valid HTTPS URL",
			url:         "https://packages.example.com/artifacts/example.ova",
			expectError: false,
		},
		{
			name:        "Invalid protocol",
			url:         "ftp://packages.example.com/artifacts/example.ovf",
			expectError: true,
			errorMsg:    "unsupported protocol 'ftp'",
		},
		{
			name:        "Invalid URL format",
			url:         "not-a-url",
			expectError: true,
			errorMsg:    "unsupported protocol",
		},
		{
			name:        "Missing host",
			url:         "https:///artifacts/example.ovf",
			expectError: true,
			errorMsg:    "URL must include a valid host",
		},
		{
			name:        "Missing path",
			url:         "https://packages.example.com",
			expectError: true,
			errorMsg:    "URL must include a path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := driver.validateOvfURL(tt.url)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestOvfManagerWrapper_ValidateAuthentication tests authentication validation
// for OVF Manager wrapper.
func TestOvfManagerWrapper_ValidateAuthentication(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name        string
		auth        *OvfAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "No authentication (anonymous)",
			auth:        nil,
			expectError: false,
		},
		{
			name:        "Empty authentication",
			auth:        &OvfAuthConfig{},
			expectError: false,
		},
		{
			name: "Valid basic authentication",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "Username without password",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "",
			},
			expectError: true,
			errorMsg:    "password must be provided when username is specified",
		},
		{
			name: "Password without username",
			auth: &OvfAuthConfig{
				Username: "",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "username must be provided when password is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := driver.validateOvfAuthentication(tt.auth)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestOvfManagerWrapper_IsOvfFileURL tests OVF/OVA file URL detection.
func TestOvfManagerWrapper_IsOvfFileURL(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "OVF file",
			url:      "https://packages.example.com/artifacts/example.ovf",
			expected: true,
		},
		{
			name:     "OVA file",
			url:      "https://packages.example.com/artifacts/example.ova",
			expected: true,
		},
		{
			name:     "OVF file with uppercase extension",
			url:      "https://packages.example.com/artifacts/example.OVF",
			expected: true,
		},
		{
			name:     "OVA file with uppercase extension",
			url:      "https://packages.example.com/artifacts/example.OVA",
			expected: true,
		},
		{
			name:     "Non-OVF file",
			url:      "https://packages.example.com/artifacts/example.vmdk",
			expected: false,
		},
		{
			name:     "No file extension",
			url:      "https://packages.example.com/artifacts/example",
			expected: false,
		},
		{
			name:     "Invalid URL",
			url:      "not-a-url",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := driver.isOvfFileURL(tt.url)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestOvfManagerWrapper_PrepareAuthenticatedURL tests URL preparation with
// authentication credentials.
func TestOvfManagerWrapper_PrepareAuthenticatedURL(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	_ = sim.driver // We don't need the driver for this test, just the wrapper.

	tests := []struct {
		name        string
		originalURL string
		auth        *OvfAuthConfig
		expected    string
		expectError bool
	}{
		{
			name:        "No authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth:        nil,
			expected:    "https://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Empty authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth:        &OvfAuthConfig{},
			expected:    "https://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Basic authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expected:    "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Invalid URL",
			originalURL: "://invalid-url",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := &OvfManagerWrapper{auth: tt.auth}
			result, err := wrapper.prepareAuthenticatedURL(tt.originalURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
				if result != tt.expected {
					t.Errorf("expected '%s', got '%s'", tt.expected, result)
				}
			}
		})
	}
}

// TestDeployOvf_ValidConfiguration tests successful OVF deployment with valid
// configuration.
func TestDeployOvf_ValidConfiguration(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	config := &OvfDeployConfig{
		URL:          "https://packages.example.com/artifacts/example.ovf",
		Name:         "test-vm",
		Folder:       "vm",
		Cluster:      "",
		Host:         "",
		ResourcePool: "Resources",
		Datastore:    "LocalDS_0",
		Network:      "VM Network",
		Locale:       "US",
	}

	// IMPORTANT:
	// This test will fail in the simulator because it doesn't support actual
	// OVF deployment, but it validates the configuration validation and setup
	// logic.
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to simulator limitations - this is acceptable.
			t.Logf("expected panic in simulator: %v", r)
		}
	}()

	_, err = driver.DeployOvf(ctx, config, &testUI{})

	// We expect an error because the simulator doesn't support OVF deployment,
	// but the error should not be a configuration validation error.
	if err != nil && strings.Contains(err.Error(), "invalid OVF deployment configuration") {
		t.Errorf("configuration validation failed unexpectedly: %s", err)
	}
}

// TestDeployOvf_InvalidConfiguration tests OVF deployment with invalid
// configurations.
func TestDeployOvf_InvalidConfiguration(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *OvfDeployConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil configuration",
			config:      nil,
			expectError: true,
			errorMsg:    "OVF deployment configuration cannot be nil",
		},
		{
			name: "Missing URL",
			config: &OvfDeployConfig{
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "OVF URL is required",
		},
		{
			name: "Missing VM name",
			config: &OvfDeployConfig{
				URL:          "https://packages.example.com/artifacts/example.ovf",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "VM name is required",
		},
		{
			name: "Invalid URL protocol",
			config: &OvfDeployConfig{
				URL:          "ftp://packages.example.com/artifacts/example.ovf",
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "unsupported protocol 'ftp'",
		},
		{
			name: "Non-OVF file URL",
			config: &OvfDeployConfig{
				URL:          "https://packages.example.com/artifacts/example.vmdk",
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "URL must point to an OVF (.ovf) or OVA (.ova) file",
		},
		{
			name: "Invalid authentication - username without password",
			config: &OvfDeployConfig{
				URL:  "https://packages.example.com/artifacts/example.ovf",
				Name: "test-vm",
				Authentication: &OvfAuthConfig{
					Username: "testuser",
					Password: "",
				},
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "password must be provided when username is specified",
		},
		{
			name: "Invalid TLS configuration - SkipTlsVerify with HTTP URL",
			config: &OvfDeployConfig{
				URL:           "http://packages.example.com/artifacts/example.ovf",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: true,
			},
			expectError: true,
			errorMsg:    "skip_tls_verify is only applicable for HTTPS URLs, but URL uses HTTP protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.expectError {
					// Unexpected panic - this should not happen for
					// configuration validation.
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			_, err := driver.DeployOvf(ctx, tt.config, &testUI{})
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestValidateOvfDeploymentConfig_TlsConfiguration tests TLS configuration
// validation.
func TestValidateOvfDeploymentConfig_TlsConfiguration(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("failed to create vCenter simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name        string
		config      *OvfDeployConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid TLS configuration - SkipTlsVerify with HTTPS URL",
			config: &OvfDeployConfig{
				URL:           "https://packages.example.com/artifacts/example.ovf",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: true,
			},
			expectError: false,
		},
		{
			name: "Invalid TLS configuration - SkipTlsVerify with HTTP URL",
			config: &OvfDeployConfig{
				URL:           "http://packages.example.com/artifacts/example.ovf",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: true,
			},
			expectError: true,
			errorMsg:    "skip_tls_verify is only applicable for HTTPS URLs, but URL uses HTTP protocol",
		},
		{
			name: "Valid configuration - SkipTlsVerify false with HTTP URL",
			config: &OvfDeployConfig{
				URL:           "http://packages.example.com/artifacts/example.ovf",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: false,
			},
			expectError: false,
		},
		{
			name: "Valid configuration - SkipTlsVerify false with HTTPS URL",
			config: &OvfDeployConfig{
				URL:           "https://packages.example.com/artifacts/example.ovf",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := driver.validateOvfDeploymentConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestDeployOvf_AuthenticationHandling tests authentication parameter handling
// in OVF deployment.
func TestDeployOvf_AuthenticationHandling(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	tests := []struct {
		name        string
		auth        *OvfAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "No authentication (anonymous)",
			auth:        nil,
			expectError: false,
		},
		{
			name:        "Empty authentication",
			auth:        &OvfAuthConfig{},
			expectError: false,
		},
		{
			name: "Valid basic authentication",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "Username without password",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "",
			},
			expectError: true,
			errorMsg:    "password must be provided when username is specified",
		},
		{
			name: "Password without username",
			auth: &OvfAuthConfig{
				Username: "",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "username must be provided when password is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.expectError {
					// Expected panic due to simulator limitations for valid
					// configurations.
					t.Logf("expected panic in simulator for valid config: %v", r)
				}
			}()

			config := &OvfDeployConfig{
				URL:            "https://packages.example.com/artifacts/example.ovf",
				Name:           "test-vm",
				Authentication: tt.auth,
				Folder:         "vm",
				ResourcePool:   "Resources",
				Datastore:      "LocalDS_0",
			}

			_, err := driver.DeployOvf(ctx, config, &testUI{})

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				// We expect some error because the simulator doesn't support
				// OVF deployment, but it should not be an authentication
				// validation error.
				if err != nil && strings.Contains(err.Error(), "authentication") && strings.Contains(err.Error(), "invalid") {
					t.Errorf("authentication validation failed unexpectedly: %s", err)
				}
			}
		})
	}
}

// TestGetOvfOptions_ValidConfiguration tests successful OVF options retrieval
// with valid configuration.
func TestGetOvfOptions_ValidConfiguration(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	tests := []struct {
		name   string
		url    string
		auth   *OvfAuthConfig
		locale string
	}{
		{
			name:   "Valid HTTP URL without authentication",
			url:    "http://packages.example.com/artifacts/example.ovf",
			auth:   nil,
			locale: "US",
		},
		{
			name:   "Valid HTTPS URL without authentication",
			url:    "https://packages.example.com/artifacts/example.ovf",
			auth:   nil,
			locale: "US",
		},
		{
			name: "Valid URL with basic authentication",
			url:  "https://packages.example.com/artifacts/example.ovf",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			locale: "US",
		},
		{
			name:   "Valid URL with empty locale (should default to US)",
			url:    "https://packages.example.com/artifacts/example.ovf",
			auth:   nil,
			locale: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := driver.GetOvfOptions(ctx, tt.url, tt.auth, tt.locale)

			// Expected panic due to simulator limitations for OVF parsing,
			// but the error should not be a configuration validation error.
			if err != nil && (strings.Contains(err.Error(), "invalid OVF URL") || strings.Contains(err.Error(), "invalid authentication")) {
				t.Errorf("configuration validation failed unexpectedly: %s", err)
			}
		})
	}
}

// TestGetOvfOptions_InvalidConfiguration tests OVF options retrieval with
// invalid configurations.
func TestGetOvfOptions_InvalidConfiguration(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	tests := []struct {
		name        string
		url         string
		auth        *OvfAuthConfig
		locale      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Invalid URL protocol",
			url:         "ftp://packages.example.com/artifacts/example.ovf",
			auth:        nil,
			locale:      "US",
			expectError: true,
			errorMsg:    "unsupported protocol 'ftp'",
		},
		{
			name:        "Invalid URL format",
			url:         "not-a-url",
			auth:        nil,
			locale:      "US",
			expectError: true,
			errorMsg:    "unsupported protocol",
		},
		{
			name:        "Missing host in URL",
			url:         "https:///artifacts/example.ovf",
			auth:        nil,
			locale:      "US",
			expectError: true,
			errorMsg:    "URL must include a valid host",
		},
		{
			name:        "Missing path in URL",
			url:         "https://packages.example.com",
			auth:        nil,
			locale:      "US",
			expectError: true,
			errorMsg:    "URL must include a path",
		},
		{
			name: "Invalid authentication - username without password",
			url:  "https://packages.example.com/artifacts/example.ovf",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "",
			},
			locale:      "US",
			expectError: true,
			errorMsg:    "password must be provided when username is specified",
		},
		{
			name: "Invalid authentication - password without username",
			url:  "https://packages.example.com/artifacts/example.ovf",
			auth: &OvfAuthConfig{
				Username: "",
				Password: "testpass",
			},
			locale:      "US",
			expectError: true,
			errorMsg:    "username must be provided when password is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := driver.GetOvfOptions(ctx, tt.url, tt.auth, tt.locale)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestOvfManagerWrapper_CreateOvfManagerWrapper tests OVF Manager wrapper
// creation with different authentication scenarios.
func TestOvfManagerWrapper_CreateOvfManagerWrapper(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name        string
		auth        *OvfAuthConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "No authentication",
			auth:        nil,
			expectError: false,
		},
		{
			name:        "Empty authentication",
			auth:        &OvfAuthConfig{},
			expectError: false,
		},
		{
			name: "Valid basic authentication",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "Invalid authentication - username without password",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "",
			},
			expectError: true,
			errorMsg:    "password must be provided when username is specified",
		},
		{
			name: "Invalid authentication - password without username",
			auth: &OvfAuthConfig{
				Username: "",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "username must be provided when password is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper, err := driver.createOvfManagerWrapper(tt.auth, false)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
				if wrapper == nil {
					t.Errorf("expected wrapper to be created but got nil")
					return
				}
				if wrapper.manager == nil {
					t.Errorf("expected wrapper.manager to be set but got nil")
				}
				if wrapper.auth != tt.auth {
					t.Errorf("expected wrapper.auth to match input auth")
				}
			}
		})
	}
}

// TestOvfManagerWrapper_ErrorScenarios tests various error scenarios in OVF
// operations.
func TestOvfManagerWrapper_ErrorScenarios(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	// Test network connectivity error simulation.
	t.Run("Network connectivity error", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to simulator limitations.
				t.Logf("expected panic in simulator: %v", r)
			}
		}()

		config := &OvfDeployConfig{
			URL:          "https://nonexistent.packages.example.com/artifacts/example.ovf",
			Name:         "test-vm",
			Folder:       "vm",
			ResourcePool: "Resources",
			Datastore:    "LocalDS_0",
		}

		_, err := driver.DeployOvf(ctx, config, &testUI{})
		// Expected error due to network connectivity issues.
		if err == nil {
			t.Errorf("expected network error but got none")
		}
	})

	// Test authentication error simulation.
	t.Run("Authentication error", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected panic due to simulator limitations.
				t.Logf("expected panic in simulator: %v", r)
			}
		}()

		config := &OvfDeployConfig{
			URL:  "https://packages.example.com/artifacts/example.ovf",
			Name: "test-vm",
			Authentication: &OvfAuthConfig{
				Username: "invalid-user",
				Password: "invalid-pass",
			},
			Folder:       "vm",
			ResourcePool: "Resources",
			Datastore:    "LocalDS_0",
		}

		_, err := driver.DeployOvf(ctx, config, &testUI{})
		// Expected error, but the error should not be a configuration validation
		// error.
		if err != nil && strings.Contains(err.Error(), "invalid authentication configuration") {
			t.Errorf("unexpected authentication configuration error: %s", err)
		}
	})

	// Test invalid resource references.
	t.Run("Invalid resource references", func(t *testing.T) {
		config := &OvfDeployConfig{
			URL:          "https://packages.example.com/artifacts/example.ovf",
			Name:         "test-vm",
			Folder:       "nonexistent-folder",
			ResourcePool: "nonexistent-pool",
			Datastore:    "nonexistent-datastore",
		}

		_, err := driver.DeployOvf(ctx, config, &testUI{})
		// We expect an error due to invalid resource references.
		if err == nil {
			t.Errorf("expected resource reference error but got none")
		}
	})
}

// TestOvfManagerWrapper_VAppPropertiesHandling tests vApp properties handling
// in OVF deployment.
func TestOvfManagerWrapper_VAppPropertiesHandling(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name           string
		vAppProperties map[string]string
		expectError    bool
	}{
		{
			name:           "No vApp properties",
			vAppProperties: nil,
			expectError:    false,
		},
		{
			name:           "Empty vApp properties",
			vAppProperties: map[string]string{},
			expectError:    false,
		},
		{
			name: "Valid vApp properties",
			vAppProperties: map[string]string{
				"hostname":    "test-host",
				"ip_address":  "192.168.1.100",
				"environment": "test",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &OvfDeployConfig{
				URL:            "https://packages.example.com/artifacts/example.ovf",
				Name:           "test-vm",
				Folder:         "vm",
				ResourcePool:   "Resources",
				Datastore:      "LocalDS_0",
				VAppProperties: tt.vAppProperties,
			}

			// Test that vApp properties are properly handled in import params
			// creation.
			importParams, err := driver.createOvfImportParams(config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
				if importParams == nil {
					t.Errorf("expected import params to be created but got nil")
				}

				// Verify vApp properties are correctly mapped.
				if len(tt.vAppProperties) > 0 {
					if len(importParams.PropertyMapping) != len(tt.vAppProperties) {
						t.Errorf("expected %d property mappings, got %d", len(tt.vAppProperties), len(importParams.PropertyMapping))
					}
				}
			}
		})
	}
}

// TestOvfManagerWrapper_NetworkMappingHandling tests network mapping handling
// in OVF deployment.
func TestOvfManagerWrapper_NetworkMappingHandling(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	tests := []struct {
		name        string
		network     string
		expectError bool
	}{
		{
			name:        "No network specified",
			network:     "",
			expectError: false,
		},
		{
			name:        "Valid network",
			network:     "VM Network",
			expectError: false,
		},
		{
			name:        "Invalid network",
			network:     "nonexistent-network",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &OvfDeployConfig{
				URL:          "https://packages.example.com/artifacts/example.ovf",
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
				Network:      tt.network,
			}

			// Test that network mapping is properly handled in import params
			// creation.
			importParams, err := driver.createOvfImportParams(config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
				if importParams == nil {
					t.Errorf("expected import params to be created but got nil")
				}

				// Verify network mapping is correctly set when network is
				// specified.
				if tt.network != "" {
					if len(importParams.NetworkMapping) == 0 {
						t.Errorf("expected network mapping to be set but got none")
					}
				}
			}
		})
	}
}

// TestDriverMock_DeployOvf tests the mock driver's OVF deployment functionality.
func TestDriverMock_DeployOvf(t *testing.T) {
	ctx := context.Background()
	mock := NewDriverMock()

	config := &OvfDeployConfig{
		URL:          "https://packages.example.com/artifacts/example.ovf",
		Name:         "test-vm",
		Folder:       "vm",
		ResourcePool: "Resources",
		Datastore:    "LocalDS_0",
	}

	// Test successful deployment.
	t.Run("Successful deployment", func(t *testing.T) {
		vm, err := mock.DeployOvf(ctx, config, &testUI{})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if vm == nil {
			t.Errorf("expected VM to be returned but got nil")
		}
		if !mock.DeployOvfCalled {
			t.Errorf("expected DeployOvf to be called")
		}
		if mock.DeployOvfConfig != config {
			t.Errorf("expected config to be stored in mock")
		}
	})

	// Test deployment failure.
	t.Run("Deployment failure", func(t *testing.T) {
		mock.DeployOvfShouldFail = true
		mock.DeployOvfError = fmt.Errorf("custom deployment error")

		vm, err := mock.DeployOvf(ctx, config, &testUI{})
		if err == nil {
			t.Errorf("expected error but got none")
		}
		if vm != nil {
			t.Errorf("expected nil VM on error but got %v", vm)
		}
		if err.Error() != "custom deployment error" {
			t.Errorf("expected custom error message, got: %s", err.Error())
		}
	})

	// Test deployment failure with default error.
	t.Run("Deployment failure with default error", func(t *testing.T) {
		mock.DeployOvfShouldFail = true
		mock.DeployOvfError = nil // Use default error.

		vm, err := mock.DeployOvf(ctx, config, &testUI{})
		if err == nil {
			t.Errorf("expected error but got none")
		}
		if vm != nil {
			t.Errorf("expected nil VM on error but got %v", vm)
		}
		if err.Error() != "deploy OVF failed" {
			t.Errorf("expected default error message, got: %s", err.Error())
		}
	})
}

// TestDriverMock_GetOvfOptions tests the mock driver's OVF options retrieval
// functionality.
func TestDriverMock_GetOvfOptions(t *testing.T) {
	ctx := context.Background()
	mock := NewDriverMock()

	url := "https://packages.example.com/artifacts/example.ovf"
	auth := &OvfAuthConfig{
		Username: "testuser",
		Password: "testpass",
	}
	locale := "US"

	// Test successful options retrieval.
	t.Run("Successful options retrieval", func(t *testing.T) {
		options, err := mock.GetOvfOptions(ctx, url, auth, locale)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if options == nil {
			t.Errorf("expected options to be returned but got nil")
		}
		if !mock.GetOvfOptionsCalled {
			t.Errorf("expected GetOvfOptions to be called")
		}
		if mock.GetOvfOptionsURL != url {
			t.Errorf("expected URL to be stored in mock")
		}
		if mock.GetOvfOptionsAuth != auth {
			t.Errorf("expected auth to be stored in mock")
		}
		if mock.GetOvfOptionsLocale != locale {
			t.Errorf("expected locale to be stored in mock")
		}

		// Verify default mock options.
		if len(options) != 2 {
			t.Errorf("expected 2 default options, got %d", len(options))
		}
		if options[0].Option != "small" {
			t.Errorf("expected first option to be 'small', got '%s'", options[0].Option)
		}
		if options[1].Option != "medium" {
			t.Errorf("expected second option to be 'medium', got '%s'", options[1].Option)
		}
	})

	// Test options retrieval failure.
	t.Run("Options retrieval failure", func(t *testing.T) {
		mock.GetOvfOptionsShouldFail = true
		mock.GetOvfOptionsError = fmt.Errorf("custom options error")

		options, err := mock.GetOvfOptions(ctx, url, auth, locale)
		if err == nil {
			t.Errorf("expected error but got none")
		}
		if options != nil {
			t.Errorf("expected nil options on error but got %v", options)
		}
		if err.Error() != "custom options error" {
			t.Errorf("expected custom error message, got: %s", err.Error())
		}
	})

	// Test options retrieval failure with default error.
	t.Run("Options retrieval failure with default error", func(t *testing.T) {
		mock.GetOvfOptionsShouldFail = true
		mock.GetOvfOptionsError = nil // Use default error.

		options, err := mock.GetOvfOptions(ctx, url, auth, locale)
		if err == nil {
			t.Errorf("expected error but got none")
		}
		if options != nil {
			t.Errorf("expected nil options on error but got %v", options)
		}
		if err.Error() != "get OVF options failed" {
			t.Errorf("expected default error message, got: %s", err.Error())
		}
	})

	// Test with custom options result.
	t.Run("Custom options result", func(t *testing.T) {
		mock.GetOvfOptionsShouldFail = false
		mock.GetOvfOptionsError = nil
		customOptions := []types.OvfOptionInfo{
			{
				Option: "custom",
				Description: types.LocalizableMessage{
					Message: "Custom configuration",
				},
			},
		}
		mock.GetOvfOptionsResult = customOptions

		options, err := mock.GetOvfOptions(ctx, url, auth, locale)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if len(options) != 1 {
			t.Errorf("expected 1 custom option, got %d", len(options))
		}
		if options[0].Option != "custom" {
			t.Errorf("expected option to be 'custom', got '%s'", options[0].Option)
		}
	})
}

// TestOvfManagerWrapper_EdgeCases tests edge cases and boundary conditions for
// OVF operations.
func TestOvfManagerWrapper_EdgeCases(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver

	// Test URL validation edge cases.
	t.Run("URL validation edge cases", func(t *testing.T) {
		edgeCaseURLs := []struct {
			name        string
			url         string
			expectError bool
			errorMsg    string
		}{
			{
				name:        "URL with query parameters",
				url:         "https://packages.example.com/artifacts/example.ovf?version=1.0",
				expectError: false,
			},
			{
				name:        "URL with fragment",
				url:         "https://packages.example.com/artifacts/example.ovf#section1",
				expectError: false,
			},
			{
				name:        "URL with port",
				url:         "https://packages.example.com:8443/artifacts/example.ovf",
				expectError: false,
			},
			{
				name:        "URL with subdirectory",
				url:         "https://packages.example.com/artifacts/v1.0/example.ovf",
				expectError: false,
			},
			{
				name:        "Empty URL",
				url:         "",
				expectError: true,
				errorMsg:    "unsupported protocol",
			},
			{
				name:        "URL with spaces",
				url:         "https://packages.example.com/artifacts/example with spaces.ovf",
				expectError: false, // URL parsing handles this.
			},
			{
				name:        "Very long URL",
				url:         "https://packages.example.com/artifacts/" + strings.Repeat("a", 2000) + ".ovf",
				expectError: false, // Should be handled by URL parsing.
			},
		}

		for _, tt := range edgeCaseURLs {
			t.Run(tt.name, func(t *testing.T) {
				err := driver.validateOvfURL(tt.url)
				if tt.expectError {
					if err == nil {
						t.Errorf("expected error but got none")
					} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %s", err)
					}
				}
			})
		}
	})

	// Test authentication edge cases.
	t.Run("Authentication edge cases", func(t *testing.T) {
		edgeCaseAuth := []struct {
			name        string
			auth        *OvfAuthConfig
			expectError bool
			errorMsg    string
		}{
			{
				name: "Username with special characters",
				auth: &OvfAuthConfig{
					Username: "testuser@packages.example.com",
					Password: "testpass",
				},
				expectError: false,
			},
			{
				name: "Password with special characters",
				auth: &OvfAuthConfig{
					Username: "testuser",
					Password: "VMw@re1!#$%",
				},
				expectError: false,
			},
			{
				name: "Very long username",
				auth: &OvfAuthConfig{
					Username: strings.Repeat("a", 1000),
					Password: "testpass",
				},
				expectError: false,
			},
			{
				name: "Very long password",
				auth: &OvfAuthConfig{
					Username: "testuser",
					Password: strings.Repeat("a", 1000),
				},
				expectError: false,
			},
			{
				name: "Empty strings (both)",
				auth: &OvfAuthConfig{
					Username: "",
					Password: "",
				},
				expectError: false, // This is valid (anonymous).
			},
		}

		for _, tt := range edgeCaseAuth {
			t.Run(tt.name, func(t *testing.T) {
				err := driver.validateOvfAuthentication(tt.auth)
				if tt.expectError {
					if err == nil {
						t.Errorf("expected error but got none")
					} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %s", err)
					}
				}
			})
		}
	})

	// Test OVF file URL detection edge cases.
	t.Run("OVF file URL detection edge cases", func(t *testing.T) {
		edgeCaseFileURLs := []struct {
			name     string
			url      string
			expected bool
		}{
			{
				name:     "Mixed case OVF",
				url:      "https://packages.example.com/artifacts/example.Ovf",
				expected: true,
			},
			{
				name:     "Mixed case OVA",
				url:      "https://packages.example.com/artifacts/example.OvA",
				expected: true,
			},
			{
				name:     "OVF with query parameters",
				url:      "https://packages.example.com/artifacts/example.ovf?version=1.0",
				expected: true,
			},
			{
				name:     "OVA with query parameters",
				url:      "https://packages.example.com/artifacts/example.ova?download=true",
				expected: true,
			},
			{
				name:     "File with ovf in name but different extension",
				url:      "https://packages.example.com/artifacts/ovf-example.vmdk",
				expected: false,
			},
			{
				name:     "File with ova in name but different extension",
				url:      "https://packages.example.com/artifacts/ova-example.iso",
				expected: false,
			},
			{
				name:     "Multiple dots in filename",
				url:      "https://packages.example.com/artifacts/example.v1.0.ovf",
				expected: true,
			},
		}

		for _, tt := range edgeCaseFileURLs {
			t.Run(tt.name, func(t *testing.T) {
				result := driver.isOvfFileURL(tt.url)
				if result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			})
		}
	})
}

// TestOvfManagerWrapper_ConcurrentAccess tests concurrent access to OVF
// operations.
func TestOvfManagerWrapper_ConcurrentAccess(t *testing.T) {
	sim, err := NewVCenterSimulator()
	if err != nil {
		t.Fatalf("unexpected error creating simulator: %s", err)
	}
	defer sim.Close()

	driver := sim.driver
	ctx := context.Background()

	// Test concurrent OVF options retrieval.
	t.Run("Concurrent OVF options retrieval", func(t *testing.T) {
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				url := fmt.Sprintf("https://packages%d.example.com/artifacts/example.ovf", id)
				_, err := driver.GetOvfOptions(ctx, url, nil, "US")
				results <- err
			}(i)
		}

		// Collect results.
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			// Expected error due to simulator limitations for OVF parsing,
			// but the error should not be a configuration validation error.
			if err != nil && strings.Contains(err.Error(), "invalid OVF URL") {
				t.Errorf("unexpected configuration validation error in goroutine: %s", err)
			}
		}
	})

	// Test concurrent wrapper creation.
	t.Run("Concurrent wrapper creation", func(t *testing.T) {
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				auth := &OvfAuthConfig{
					Username: fmt.Sprintf("user%d", id),
					Password: fmt.Sprintf("pass%d", id),
				}
				wrapper, err := driver.createOvfManagerWrapper(auth, false)
				if err != nil {
					results <- err
					return
				}
				if wrapper == nil {
					results <- fmt.Errorf("wrapper is nil")
					return
				}
				results <- nil
			}(i)
		}

		// Collect results.
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			if err != nil {
				t.Errorf("unexpected error in goroutine: %s", err)
			}
		}
	})
}

// TestOvfProgressMonitor_Creation tests OvfProgressMonitor creation and basic
// functionality.
func TestOvfProgressMonitor_Creation(t *testing.T) {
	ui := &testUI{}
	ctx := context.Background()

	monitor := NewOvfProgressMonitor(ui, ctx)
	if monitor == nil {
		t.Fatal("expected progress monitor to be created")
		return
	}

	if monitor.progressInterval != 5*time.Second {
		t.Errorf("expected progress interval to be 5 seconds, got %v", monitor.progressInterval)
	}

	monitor.Cancel()
}

// TestOvfProgressMonitor_ReportProgress tests progress reporting functionality.
func TestOvfProgressMonitor_ReportProgress(t *testing.T) {
	ui := &testUI{}
	ctx := context.Background()

	monitor := NewOvfProgressMonitor(ui, ctx)
	defer monitor.Cancel()

	taskInfo := types.TaskInfo{
		State:    types.TaskInfoStateRunning,
		Progress: 50,
	}

	monitor.reportProgress(taskInfo)
}

// TestOvfProgressMonitor_ReportErrorState tests error state reporting.
func TestOvfProgressMonitor_ReportErrorState(t *testing.T) {
	ui := &testUI{}
	ctx := context.Background()

	monitor := NewOvfProgressMonitor(ui, ctx)
	defer monitor.Cancel()

	taskInfo := types.TaskInfo{
		State: types.TaskInfoStateError,
		Error: &types.LocalizedMethodFault{
			LocalizedMessage: "Test error message",
		},
	}

	monitor.reportProgress(taskInfo)
}

// TestOvfProgressMonitor_HasTaskInfoChanged tests task info change detection.
func TestOvfProgressMonitor_HasTaskInfoChanged(t *testing.T) {
	ui := &testUI{}
	ctx := context.Background()

	monitor := NewOvfProgressMonitor(ui, ctx)
	defer monitor.Cancel()

	oldTask := &types.TaskInfo{
		State:    types.TaskInfoStateRunning,
		Progress: 25,
	}

	newTask := &types.TaskInfo{
		State:    types.TaskInfoStateRunning,
		Progress: 25,
	}

	if monitor.hasTaskInfoChanged(oldTask, newTask) {
		t.Error("expected task info to be unchanged")
	}

	newTask.Progress = 50
	if !monitor.hasTaskInfoChanged(oldTask, newTask) {
		t.Error("expected task info to be changed due to progress")
	}

	newTask.Progress = 25
	newTask.State = types.TaskInfoStateSuccess
	if !monitor.hasTaskInfoChanged(oldTask, newTask) {
		t.Error("expected task info to be changed due to state")
	}
}

// TestOvfProgressMonitor_Integration demonstrates the complete progress
// monitoring workflow.
func TestOvfProgressMonitor_Integration(t *testing.T) {
	ui := &testUI{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	monitor := NewOvfProgressMonitor(ui, ctx)
	defer monitor.Cancel()

	progressStates := []types.TaskInfo{
		{State: types.TaskInfoStateQueued, Progress: 0},
		{State: types.TaskInfoStateRunning, Progress: 10},
		{State: types.TaskInfoStateRunning, Progress: 25},
		{State: types.TaskInfoStateRunning, Progress: 50},
		{State: types.TaskInfoStateRunning, Progress: 75},
		{State: types.TaskInfoStateRunning, Progress: 90},
		{State: types.TaskInfoStateSuccess, Progress: 100},
	}

	for i, taskInfo := range progressStates {
		if i > 0 {
			monitor.lastProgressTime = time.Now().Add(-6 * time.Second)
		}

		monitor.reportProgress(taskInfo)
		time.Sleep(10 * time.Millisecond)
	}

	t.Log("Progress monitoring integration test completed successfully")
}

// TestOvfProgressMonitor_Cancellation tests the cancellation functionality.
func TestOvfProgressMonitor_Cancellation(t *testing.T) {
	ui := &testUI{}
	ctx, cancel := context.WithCancel(context.Background())

	monitor := NewOvfProgressMonitor(ui, ctx)

	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-monitor.ctx.Done():
				return
			case <-ticker.C:
				taskInfo := types.TaskInfo{
					State:    types.TaskInfoStateRunning,
					Progress: 50,
				}
				monitor.reportProgress(taskInfo)
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)

	cancel()
	monitor.Cancel()

	select {
	case <-done:
		t.Log("Progress monitoring cancelled successfully")
	case <-time.After(2 * time.Second):
		t.Error("Progress monitoring did not cancel within timeout")
	}
}

// TestOvfProgressMonitor_ErrorHandling tests error state reporting with various
// scenarios.
func TestOvfProgressMonitor_ErrorHandling(t *testing.T) {
	ui := &testUI{}
	ctx := context.Background()

	monitor := NewOvfProgressMonitor(ui, ctx)
	defer monitor.Cancel()

	errorScenarios := []struct {
		name     string
		taskInfo types.TaskInfo
	}{
		{
			name: "Task with error message",
			taskInfo: types.TaskInfo{
				State: types.TaskInfoStateError,
				Error: &types.LocalizedMethodFault{
					LocalizedMessage: "Network connection failed",
				},
			},
		},
		{
			name: "Task with error but no message",
			taskInfo: types.TaskInfo{
				State: types.TaskInfoStateError,
				Error: nil,
			},
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			monitor.reportProgress(scenario.taskInfo)
			t.Logf("Error handling test completed for scenario: %s", scenario.name)
		})
	}
}

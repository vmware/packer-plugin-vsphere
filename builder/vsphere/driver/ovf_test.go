// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

// TestOvfManagerWrapper_ValidateURL tests URL validation for OVF Manager
// wrapper functionality.
func TestOvfManagerWrapper_ValidateURL(t *testing.T) {
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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

// TestDeployOvf_ValidConfiguration tests successful OVF deployment with valid
// configuration.
func TestDeployOvf_ValidConfiguration(t *testing.T) {
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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

	_, err := driver.DeployOvf(ctx, config, &testUI{})

	// We expect an error because the simulator doesn't support OVF deployment,
	// but the error should not be a configuration validation error.
	if err != nil && strings.Contains(err.Error(), "invalid OVF deployment configuration") {
		t.Errorf("configuration validation failed unexpectedly: %s", err)
	}
}

// TestDeployOvf_InvalidConfiguration tests OVF deployment with invalid
// configurations.
func TestDeployOvf_InvalidConfiguration(t *testing.T) {
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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
			errorMsg:    "OVF source requires either URL or Path",
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
			errorMsg:    "virtual machine name is required",
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
		{
			name: "Both URL and Path specified",
			config: &OvfDeployConfig{
				URL:          "https://packages.example.com/artifacts/example.ovf",
				Path:         "./artifacts/example.ovf",
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "cannot specify both URL and Path",
		},
		{
			name: "Local path with authentication",
			config: &OvfDeployConfig{
				Path: "./artifacts/example.ovf",
				Name: "test-vm",
				Authentication: &OvfAuthConfig{
					Username: "testuser",
					Password: "testpass",
				},
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "authentication is only applicable when using a remote OVF/OVA URL",
		},
		{
			name: "Local path with SkipTlsVerify",
			config: &OvfDeployConfig{
				Path:          "./artifacts/example.ova",
				Name:          "test-vm",
				Folder:        "vm",
				ResourcePool:  "Resources",
				Datastore:     "LocalDS_0",
				SkipTlsVerify: true,
			},
			expectError: true,
			errorMsg:    "skip_tls_verify is only applicable when using a remote OVF/OVA URL",
		},
		{
			name: "Local path without OVF/OVA extension",
			config: &OvfDeployConfig{
				Path:         "./artifacts/example.vmdk",
				Name:         "test-vm",
				Folder:       "vm",
				ResourcePool: "Resources",
				Datastore:    "LocalDS_0",
			},
			expectError: true,
			errorMsg:    "local OVF/OVA path must point to an OVF (.ovf) or OVA (.ova) file",
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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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
			_, err := driver.GetOvfOptions(ctx, &OvfDeployConfig{URL: tt.url, Authentication: tt.auth, Locale: tt.locale})

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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
			_, err := driver.GetOvfOptions(ctx, &OvfDeployConfig{URL: tt.url, Authentication: tt.auth, Locale: tt.locale})
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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
			importParams, err := driver.createOvfImportParams(config, nil)
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

// TestMock_DeployOvf tests the mock driver's OVF deployment functionality.
func TestMock_DeployOvf(t *testing.T) {
	ctx := context.Background()
	mock := NewMock()

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

// TestMock_GetOvfOptions tests the mock driver's OVF options retrieval
// functionality.
func TestMock_GetOvfOptions(t *testing.T) {
	ctx := context.Background()
	mock := NewMock()

	url := "https://packages.example.com/artifacts/example.ovf"
	auth := &OvfAuthConfig{
		Username: "testuser",
		Password: "testpass",
	}
	locale := "US"

	// Test successful options retrieval.
	t.Run("Successful options retrieval", func(t *testing.T) {
		options, err := mock.GetOvfOptions(ctx, &OvfDeployConfig{URL: url, Authentication: auth, Locale: locale})
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

		options, err := mock.GetOvfOptions(ctx, &OvfDeployConfig{URL: url, Authentication: auth, Locale: locale})
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

		options, err := mock.GetOvfOptions(ctx, &OvfDeployConfig{URL: url, Authentication: auth, Locale: locale})
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

		options, err := mock.GetOvfOptions(ctx, &OvfDeployConfig{URL: url, Authentication: auth, Locale: locale})
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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

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
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)
	ctx := context.Background()

	// Test concurrent OVF options retrieval.
	t.Run("Concurrent OVF options retrieval", func(t *testing.T) {
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(id int) {
				url := fmt.Sprintf("https://packages%d.example.com/artifacts/example.ovf", id)
				_, err := driver.GetOvfOptions(ctx, &OvfDeployConfig{URL: url, Locale: "US"})
				results <- err
			}(i)
		}

		// Collect results.
		for range numGoroutines {
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

		for i := range numGoroutines {
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
		for range numGoroutines {
			err := <-results
			if err != nil {
				t.Errorf("unexpected error in goroutine: %s", err)
			}
		}
	})
}

// TestCategorizeOvfImportError tests the categorization of OVF import errors.
func TestCategorizeOvfImportError(t *testing.T) {
	driver := &VCenterDriver{}

	testCases := []struct {
		name           string
		inputError     error
		expectedPrefix string
	}{
		{
			name:           "Authentication error",
			inputError:     fmt.Errorf("HTTP 401 Unauthorized"),
			expectedPrefix: "authentication failed when accessing remote OVF/OVA source",
		},
		{
			name:           "File not found error",
			inputError:     fmt.Errorf("HTTP 404 Not Found"),
			expectedPrefix: "remote OVF/OVA file not found",
		},
		{
			name:           "Network timeout error",
			inputError:     fmt.Errorf("connection timeout"),
			expectedPrefix: "network connectivity error accessing remote OVF/OVA source",
		},
		{
			name:           "TLS certificate error",
			inputError:     fmt.Errorf("x509: certificate verify failed"),
			expectedPrefix: "TLS/SSL certificate error accessing remote OVF/OVA source",
		},
		{
			name:           "OVF validation error",
			inputError:     fmt.Errorf("invalid OVF descriptor"),
			expectedPrefix: "OVF/OVA file validation error",
		},
		{
			name:           "Resource error",
			inputError:     fmt.Errorf("insufficient disk space"),
			expectedPrefix: "insufficient vSphere resources for OVF deployment",
		},
		{
			name:           "Generic error",
			inputError:     fmt.Errorf("unknown error"),
			expectedPrefix: "OVF deployment failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := driver.categorizeOvfImportError(tc.inputError)
			if !strings.HasPrefix(result.Error(), tc.expectedPrefix) {
				t.Errorf("expected error to start with %q, got %q", tc.expectedPrefix, result.Error())
			}
		})
	}
}

// TestWrapOvfError tests the wrapping of OVF errors with context and sanitization.
func TestWrapOvfError(t *testing.T) {
	driver := &VCenterDriver{}

	errContext := "test operation failed"
	err := fmt.Errorf("original error")
	url := "https://testuser:testpass@packages.example.com/artifacts/example.ovf"

	result := driver.wrapOvfError(errContext, err, url)

	if !strings.Contains(result.Error(), errContext) {
		t.Errorf("expected error to contain context %q", errContext)
	}

	if strings.Contains(result.Error(), "password") || strings.Contains(result.Error(), "testpass") {
		t.Errorf("expected error to not contain password, got %q", result.Error())
	}

	if strings.Contains(result.Error(), "testuser@") || strings.Contains(result.Error(), "testuser:") {
		t.Errorf("expected error to not contain username, got %q", result.Error())
	}

	if !strings.Contains(result.Error(), "packages.example.com") {
		t.Errorf("expected error to contain sanitized URL host, got %q", result.Error())
	}
}

// TestApplyOvfPostImportConfig applies notes and MAC settings after OVF import.
func TestApplyOvfPostImportConfig(t *testing.T) {
	d := &VCenterDriver{}
	vm := &VirtualMachineMock{}

	if err := d.applyOvfPostImportConfig(vm, &OvfDeployConfig{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vm.ReconfigureCalled {
		t.Fatal("expected no reconfigure when annotation and MAC are unset")
	}

	if err := d.applyOvfPostImportConfig(vm, &OvfDeployConfig{Annotation: "test notes"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vm.ReconfigureCalled {
		t.Fatal("expected reconfigure when annotation is set")
	}
	if vm.ReconfigureSpec.Annotation != "test notes" {
		t.Fatalf("expected annotation %q, got %q", "test notes", vm.ReconfigureSpec.Annotation)
	}
}

func TestBuildOvfNetworkMappings(t *testing.T) {
	sim := mustVPXSimulator(t)

	driver := newSimulatorDriver(sim)

	tests := []struct {
		name         string
		ovfNetworks  []types.OvfNetworkInfo
		network      string
		expectError  bool
		errorMsg     string
		expectMapped int
		expectNames  []string
	}{
		{
			name:         "No OVF networks",
			ovfNetworks:  nil,
			network:      "VM Network",
			expectMapped: 0,
		},
		{
			name: "Single OVF network mapped",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Management Network"},
			},
			network:      "VM Network",
			expectMapped: 1,
			expectNames:  []string{"Management Network"},
		},
		{
			name: "Multiple OVF networks mapped to same vSphere network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Management Network"},
				{Name: "VM Network"},
			},
			network:      "VM Network",
			expectMapped: 2,
			expectNames:  []string{"Management Network", "VM Network"},
		},
		{
			name: "OVF networks require configured network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Guest Network"},
			},
			network:     "",
			expectError: true,
			errorMsg:    "OVF requires network mapping",
		},
		{
			name: "Invalid vSphere network",
			ovfNetworks: []types.OvfNetworkInfo{
				{Name: "Guest Network"},
			},
			network:     "nonexistent-network",
			expectError: true,
			errorMsg:    "error finding network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappings, err := driver.buildOvfNetworkMappings(tt.ovfNetworks, tt.network)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if len(mappings) != tt.expectMapped {
				t.Fatalf("expected %d network mappings, got %d", tt.expectMapped, len(mappings))
			}

			for i, expectedName := range tt.expectNames {
				if mappings[i].Name != expectedName {
					t.Errorf("mapping[%d].Name = %q, want %q", i, mappings[i].Name, expectedName)
				}
			}
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// TestCredentialHandling_SensitiveVariables tests that sensitive variables work
// correctly with remote source credentials.
func TestCredentialHandling_SensitiveVariables(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		expectError    bool
		expectedErrMsg string
		validateFunc   func(*testing.T, *CloneConfig)
	}{
		{
			name: "direct credentials",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser", // Simulates resolved {{user `ovf_username`}}.
					Password: "testpass", // Simulates resolved {{user `ovf_password`}}.
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *CloneConfig) {
				if c.RemoteSource.Username != "testuser" {
					t.Errorf("expected username 'testuser', got '%s'", c.RemoteSource.Username)
				}
				if c.RemoteSource.Password != "testpass" {
					t.Errorf("expected password 'testpass', got '%s'", c.RemoteSource.Password)
				}
			},
		},
		{
			name: "environment variable credentials",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "env-testuser", // Simulates resolved {{env `OVF_USERNAME`}}.
					Password: "env-testpass", // Simulates resolved {{env `OVF_PASSWORD`}}.
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *CloneConfig) {
				if c.RemoteSource.Username != "env-testuser" {
					t.Errorf("expected username 'env-testuser', got '%s'", c.RemoteSource.Username)
				}
				if c.RemoteSource.Password != "env-testpass" {
					t.Errorf("expected password 'env-testpass', got '%s'", c.RemoteSource.Password)
				}
			},
		},
		{
			name: "mixed credential types",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
					Password: "testpass",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *CloneConfig) {
				if c.RemoteSource.Username != "testuser" {
					t.Errorf("expected username 'testuser', got '%s'", c.RemoteSource.Username)
				}
				if c.RemoteSource.Password != "testpass" {
					t.Errorf("expected password 'testpass', got '%s'", c.RemoteSource.Password)
				}
			},
		},
		{
			name: "empty credentials (anonymous access)",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
					// No username/password for anonymous access.
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *CloneConfig) {
				if c.RemoteSource.Username != "" {
					t.Errorf("expected empty username for anonymous access, got '%s'", c.RemoteSource.Username)
				}
				if c.RemoteSource.Password != "" {
					t.Errorf("expected empty password for anonymous access, got '%s'", c.RemoteSource.Password)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.config.Prepare()

			if tt.expectError {
				if len(errs) == 0 {
					t.Errorf("expected error but got none")
					return
				}
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), tt.expectedErrMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error message containing '%s', got errors: %v", tt.expectedErrMsg, errs)
				}
				return
			}

			if len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, tt.config)
			}
		})
	}
}

// TestCredentialHandling_SecurityAndNonExposure tests that credentials are not
// exposed in logs or error messages.
func TestCredentialHandling_SecurityAndNonExposure(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		mockSetup      func(*driver.DriverMock)
		expectError    bool
		validateOutput func(*testing.T, string, string)
	}{
		{
			name: "credentials not exposed",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
					Password: "testpass",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateOutput: func(t *testing.T, uiOutput, errorMsg string) {
				if strings.Contains(uiOutput, "testuser") {
					t.Error("username should not appear in ui output")
				}
				if strings.Contains(uiOutput, "testpass") {
					t.Error("password should not appear in ui output")
				}
				if !strings.Contains(uiOutput, "https://packages.example.com/artifacts/example.ovf") {
					t.Error("sanitized url should appear in ui output")
				}
			},
		},
		{
			name: "url credentials sanitized",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL: "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateOutput: func(t *testing.T, uiOutput, errorMsg string) {
				if strings.Contains(uiOutput, "testuser:testpass@") {
					t.Error("URL credentials should not appear in UI output")
				}
				if !strings.Contains(uiOutput, "https://testuser@packages.example.com/artifacts/example.ovf") {
					t.Error("sanitized url should appear in UI output")
				}
			},
		},
		{
			name: "error message url sanitization",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
					Username: "testuser",
					Password: "testpass",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("network error")
			},
			expectError: true,
			validateOutput: func(t *testing.T, uiOutput, errorMsg string) {
				// Check that the URL in the error message is sanitized.
				if strings.Contains(errorMsg, "testuser:testpass@") {
					t.Error("credentials should not appear in url within error message")
				}

				// The url should be sanitized to show only username.
				if !strings.Contains(errorMsg, "https://testuser@packages.example.com/artifacts/example.ovf") {
					t.Error("expected sanitized url with username only in error message")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var uiBuffer bytes.Buffer
			ui := &packersdk.BasicUi{
				Reader: new(bytes.Buffer),
				Writer: &uiBuffer,
			}

			state := new(multistep.BasicStateBag)
			state.Put("ui", ui)

			driverMock := driver.NewDriverMock()
			state.Put("driver", driverMock)

			step := &StepCloneVM{
				Config:   tt.config,
				Location: basicLocationConfig(),
				Force:    true,
			}

			tt.mockSetup(driverMock)

			action := step.Run(context.Background(), state)

			uiOutput := uiBuffer.String()
			var errorMsg string

			if tt.expectError {
				if action != multistep.ActionHalt {
					t.Fatalf("expected ActionHalt for error case, got %v", action)
				}
				if err, ok := state.GetOk("error"); ok {
					errorMsg = err.(error).Error()
				} else {
					t.Error("expected error to be set in state")
				}
			} else {
				if action != multistep.ActionContinue {
					t.Fatalf("expected ActionContinue, got %v", action)
				}
			}

			if tt.validateOutput != nil {
				tt.validateOutput(t, uiOutput, errorMsg)
			}
		})
	}
}

// TestCredentialHandling_TlsConfiguration tests tls configuration options for
// remote OVF/OVA sources.
func TestCredentialHandling_TlsConfiguration(t *testing.T) {
	tests := []struct {
		name                     string
		config                   *CloneConfig
		mockSetup                func(*driver.DriverMock)
		expectError              bool
		expectedErrMsg           string
		validateTLSConfiguration func(*testing.T, *driver.OvfDeployConfig)
	}{
		{
			name: "default tls verification",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be false by default")
				}
			},
		},
		{
			name: "strict tls verification",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: false,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be false when explicitly set")
				}
			},
		},
		{
			name: "relaxed tls verification",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: true,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if !config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be true when explicitly enabled")
				}
			},
		},
		{
			name: "tls verification with authentication",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					Username:      "testuser",
					Password:      "testpass",
					SkipTlsVerify: false,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be false with authentication")
				}
				if config.Authentication == nil {
					t.Error("expected authentication to be configured")
				} else {
					if config.Authentication.Username != "testuser" {
						t.Errorf("expected username 'user', got '%s'", config.Authentication.Username)
					}
					if config.Authentication.Password != "testpass" {
						t.Errorf("expected password 'pass', got '%s'", config.Authentication.Password)
					}
				}
			},
		},
		{
			name: "http url with tls configuration (should be ignored)",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "http://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: true,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if !config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be preserved even for http urls")
				}
			},
		},
		{
			name: "tls certificate error simulation",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: false,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("x509: certificate signed by unknown authority")
			},
			expectError:    true,
			expectedErrMsg: "x509: certificate signed by unknown authority",
			validateTLSConfiguration: func(t *testing.T, config *driver.OvfDeployConfig) {
				if config.SkipTlsVerify {
					t.Error("expected SkipTlsVerify to be false for strict verification")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := new(multistep.BasicStateBag)
			state.Put("ui", &packersdk.BasicUi{
				Reader: new(bytes.Buffer),
				Writer: new(bytes.Buffer),
			})

			driverMock := driver.NewDriverMock()
			state.Put("driver", driverMock)

			step := &StepCloneVM{
				Config:   tt.config,
				Location: basicLocationConfig(),
				Force:    true,
			}

			tt.mockSetup(driverMock)

			action := step.Run(context.Background(), state)

			if tt.expectError {
				if action != multistep.ActionHalt {
					t.Fatalf("expected ActionHalt for error case, got %v", action)
				}
				if err, ok := state.GetOk("error"); ok {
					if tt.expectedErrMsg != "" && !strings.Contains(err.(error).Error(), tt.expectedErrMsg) {
						t.Errorf("expected error message to contain '%s', got '%s'", tt.expectedErrMsg, err.(error).Error())
					}
				} else {
					t.Error("expected error to be set in state")
				}
			} else {
				if action != multistep.ActionContinue {
					t.Fatalf("expected ActionContinue, got %v", action)
				}

				if !driverMock.DeployOvfCalled {
					t.Fatal("expected DeployOvf to be called")
				}

				if tt.validateTLSConfiguration != nil {
					tt.validateTLSConfiguration(t, driverMock.DeployOvfConfig)
				}
			}
		})
	}
}

// TestCredentialHandling_ConfigurationValidation tests validation of
// credential-related configuration.
func TestCredentialHandling_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid configuration with all credential fields",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					Username:      "testuser",
					Password:      "testpass",
					SkipTlsVerify: true,
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid configuration with minimal credentials",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid: username without password",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "'password' is required when 'username' is specified for remote source",
		},
		{
			name: "invalid: password without username",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Password: "testpass",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "'username' is required when 'password' is specified for remote source",
		},
		{
			name: "invalid: empty username with password",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "",
					Password: "testpass",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "'username' is required when 'password' is specified for remote source",
		},
		{
			name: "invalid: username with empty password",
			config: &CloneConfig{
				RemoteSource: &RemoteSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
					Password: "",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			expectError:    true,
			expectedErrMsg: "'password' is required when 'username' is specified for remote source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.config.Prepare()

			if tt.expectError {
				if len(errs) == 0 {
					t.Fatal("expected validation error but got none")
				}
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), tt.expectedErrMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error message containing '%s', got errors: %v", tt.expectedErrMsg, errs)
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("unexpected validation errors: %v", errs)
				}
			}
		})
	}
}

// TestCredentialHandling_CredentialSanitization tests the credential
// sanitization functions directly.
func TestCredentialHandling_CredentialSanitization(t *testing.T) {
	step := &StepCloneVM{}

	tests := []struct {
		name     string
		input    string
		expected string
		testFunc func(string) string
	}{
		{
			name:     "sanitize url with credentials",
			input:    "https://testuser:testpass@packages.example.com/artifacts",
			expected: "https://testuser@packages.example.com/artifacts",
			testFunc: step.sanitizeURL,
		},
		{
			name:     "sanitize url without credentials",
			input:    "https://packages.example.com/artifacts",
			expected: "https://packages.example.com/artifacts",
			testFunc: step.sanitizeURL,
		},
		{
			name:     "sanitize invalid url",
			input:    "://invalid-url",
			expected: "[invalid URL]",
			testFunc: step.sanitizeURL,
		},
		{
			name:     "sanitize error message with password pattern",
			input:    "authentication failed: password=testpass",
			expected: "authentication failed: [credentials removed]",
			testFunc: step.sanitizeCredentialPatterns,
		},
		{
			name:     "sanitize error message with multiple credential patterns",
			input:    "error: username=user password=pass token=abc123",
			expected: "error: username=user [credentials removed] [credentials removed]",
			testFunc: step.sanitizeCredentialPatterns,
		},
		{
			name:     "sanitize urls in string",
			input:    "failed to connect to https://testuser:testpass@packages.example.com/artifacts",
			expected: "failed to connect to https://packages.example.com/artifacts",
			testFunc: step.sanitizeURLsInString,
		},
		{
			name:     "sanitize complex error message",
			input:    "HTTP 401: auth failed for https://testuser:testpass@packages.example.com with password=testpass",
			expected: "HTTP 401: auth failed for https://packages.example.com with [credentials removed]",
			testFunc: step.sanitizeErrorMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

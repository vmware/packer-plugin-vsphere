// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"strings"
	"testing"
	"time"
)

func TestCloneConfig_MinimalConfig(t *testing.T) {
	c := new(Config)
	warns, errs := c.Prepare(minimalConfig())
	testConfigOk(t, warns, errs)
}

func TestCloneConfig_MandatoryParameters(t *testing.T) {
	params := []string{"vcenter_server", "username", "password", "template", "vm_name", "host"}
	for _, param := range params {
		raw := minimalConfig()
		raw[param] = ""
		c := new(Config)
		warns, err := c.Prepare(raw)
		testConfigErr(t, param, warns, err)
	}
}

func TestCloneConfig_Timeout(t *testing.T) {
	raw := minimalConfig()
	raw["shutdown_timeout"] = "3m"
	conf := new(Config)
	warns, err := conf.Prepare(raw)
	testConfigOk(t, warns, err)
	if conf.Timeout != 3*time.Minute {
		t.Fatalf("unexpected result: expected '3m', but returned '%v'", conf.Timeout)
	}
}

func TestCloneConfig_RAMReservation(t *testing.T) {
	raw := minimalConfig()
	raw["RAM_reservation"] = 1000
	raw["RAM_reserve_all"] = true
	c := new(Config)
	warns, err := c.Prepare(raw)
	testConfigErr(t, "RAM_reservation", warns, err)
}

func minimalConfig() map[string]interface{} {
	return map[string]interface{}{
		"vcenter_server": "vc01.example.com",
		"username":       "administrator@vsphere.local",
		"password":       "VMw@re1!",
		"template":       "example-template",
		"vm_name":        "vm-01",
		"host":           "esx01.example.com",
		"ssh_username":   "root",
		"ssh_password":   "VMw@re1!",
	}
}

func testConfigOk(t *testing.T, warns []string, err error) {
	if len(warns) > 0 {
		t.Errorf("unexpected warning: %#v", warns)
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func testConfigErr(t *testing.T, context string, warns []string, err error) {
	if len(warns) > 0 {
		t.Errorf("unexpected warning: %#v", warns)
	}
	if err == nil {
		t.Errorf("unexpected result: expected '%s', but returned 'nil'", context)
	}
}

// TestCloneConfig_RemoteSourceValidation tests the validation logic for remote source configurations
func TestCloneConfig_RemoteSourceValidation(t *testing.T) {
	testCases := []struct {
		name           string
		config         map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "Valid remote source with HTTP URL",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "http://packages.example.com/artifacts/example.ovf",
				},
			},
			expectError: false,
		},
		{
			name: "Valid remote source with HTTPS URL",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "https://packages.example.com/artifacts/example.ovf",
				},
			},
			expectError: false,
		},
		{
			name: "Valid remote source with basic authentication",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url":      "https://packages.example.com/artifacts/example.ovf",
					"username": "testuser",
					"password": "testpass",
				},
			},
			expectError: false,
		},
		{
			name: "Valid remote source with SkipTlsVerify",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url":             "https://packages.example.com/artifacts/example.ovf",
					"skip_tls_verify": true,
				},
			},
			expectError: false,
		},
		{
			name: "Invalid: both template and remote_source specified",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"template":       "example-template",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "https://packages.example.com/artifacts/example.ovf",
				},
			},
			expectError:    true,
			expectedErrMsg: "cannot specify both 'template' and 'remote_source' - choose one source type",
		},
		{
			name: "Invalid: neither template nor remote_source specified",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
			},
			expectError:    true,
			expectedErrMsg: "either 'template' or 'remote_source' must be specified",
		},
		{
			name: "Invalid: remote_source URL is empty",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "",
				},
			},
			expectError:    true,
			expectedErrMsg: "'url' is required when using 'remote_source'",
		},
		{
			name: "Invalid: remote_source URL missing",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"username": "testuser",
					"password": "testpass",
				},
			},
			expectError:    true,
			expectedErrMsg: "'url' is required when using 'remote_source'",
		},
		{
			name: "Invalid: remote_source URL with unsupported protocol",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "ftp://packages.example.com/artifacts/example.ovf",
				},
			},
			expectError:    true,
			expectedErrMsg: "'remote_source' URL must use HTTP or HTTPS protocol",
		},
		{
			name: "Invalid: remote_source URL with invalid format",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "://invalid-url-format",
				},
			},
			expectError:    true,
			expectedErrMsg: "invalid 'remote_source' URL format",
		},
		{
			name: "Invalid: remote_source username without password",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url":      "https://packages.example.com/artifacts/example.ovf",
					"username": "testuser",
				},
			},
			expectError:    true,
			expectedErrMsg: "'password' is required when 'username' is specified for remote source",
		},
		{
			name: "Invalid: remote_source password without username",
			config: map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url":      "https://packages.example.com/artifacts/example.ovf",
					"password": "testpass",
				},
			},
			expectError:    true,
			expectedErrMsg: "'username' is required when 'password' is specified for remote source",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := new(Config)
			warns, err := c.Prepare(tc.config)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tc.expectedErrMsg) {
					t.Errorf("expected error message to contain '%s', but got: %s", tc.expectedErrMsg, err.Error())
				}
			} else {
				if len(warns) > 0 {
					t.Errorf("unexpected warning: %#v", warns)
				}
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestCloneConfig_RemoteSourceMutualExclusivity tests that template and remote_source are mutually exclusive
func TestCloneConfig_RemoteSourceMutualExclusivity(t *testing.T) {
	testCases := []struct {
		name        string
		template    string
		remoteURL   string
		expectError bool
	}{
		{
			name:        "Only template specified - valid",
			template:    "example-template",
			remoteURL:   "",
			expectError: false,
		},
		{
			name:        "Only remote source specified - valid",
			template:    "",
			remoteURL:   "https://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Both template and remote source specified - invalid",
			template:    "example-template",
			remoteURL:   "https://packages.example.com/artifacts/example.ovf",
			expectError: true,
		},
		{
			name:        "Neither template nor remote source specified - invalid",
			template:    "",
			remoteURL:   "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
			}

			if tc.template != "" {
				config["template"] = tc.template
			}

			if tc.remoteURL != "" {
				config["remote_source"] = map[string]interface{}{
					"url": tc.remoteURL,
				}
			}

			c := new(Config)
			warns, err := c.Prepare(config)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if len(warns) > 0 {
					t.Errorf("unexpected warning: %#v", warns)
				}
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

// TestCloneConfig_RemoteSourceAuthenticationValidation tests authentication parameter validation
func TestCloneConfig_RemoteSourceAuthenticationValidation(t *testing.T) {
	testCases := []struct {
		name           string
		username       string
		password       string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:        "No authentication - valid",
			username:    "",
			password:    "",
			expectError: false,
		},
		{
			name:        "Both username and password - valid",
			username:    "testuser",
			password:    "testpass",
			expectError: false,
		},
		{
			name:           "Username without password - invalid",
			username:       "testuser",
			password:       "",
			expectError:    true,
			expectedErrMsg: "'password' is required when 'username' is specified for remote source",
		},
		{
			name:           "Password without username - invalid",
			username:       "",
			password:       "testpass",
			expectError:    true,
			expectedErrMsg: "'username' is required when 'password' is specified for remote source",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"vcenter_server": "vcenter.example.com",
				"username":       "administrator@vsphere.local",
				"password":       "VMw@re1!",
				"vm_name":        "vm-01",
				"host":           "esxi-01.example.com",
				"ssh_username":   "root",
				"ssh_password":   "VMw@re1!",
				"remote_source": map[string]interface{}{
					"url": "https://packages.example.com/artifacts/example.ovf",
				},
			}

			if tc.username != "" {
				config["remote_source"].(map[string]interface{})["username"] = tc.username
			}
			if tc.password != "" {
				config["remote_source"].(map[string]interface{})["password"] = tc.password
			}

			c := new(Config)
			warns, err := c.Prepare(config)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tc.expectedErrMsg) {
					t.Errorf("expected error message to contain '%s', but got: %s", tc.expectedErrMsg, err.Error())
				}
			} else {
				if len(warns) > 0 {
					t.Errorf("unexpected warning: %#v", warns)
				}
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			}
		})
	}
}

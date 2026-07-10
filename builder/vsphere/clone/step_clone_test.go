// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/mitchellh/mapstructure"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

func TestCloneConfig_Prepare(t *testing.T) {
	tc := []struct {
		name           string
		config         *CloneConfig
		fail           bool
		expectedErrMsg string
	}{
		{
			name: "Storage validate disk_size with disk_controller_type",
			config: &CloneConfig{
				Template: "template name",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"test"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 0,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0].'disk_size' is required",
		},
		{
			name: "Storage validate disk_size",
			config: &CloneConfig{
				Template: "template name",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:            0,
							DiskThinProvisioned: true,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0].'disk_size' is required",
		},
		{
			name: "Storage validate disk_controller_index",
			config: &CloneConfig{
				Template: "template name",
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskControllerIndex: 3,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "storage[0].'disk_controller_index' references an unknown disk controller",
		},
		{
			name: "Validate template is set",
			config: &CloneConfig{
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"test"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "either 'template' or 'ovf_source' must be specified",
		},
		{
			name: "Validate LinkedClone and DiskSize set at the same time",
			config: &CloneConfig{
				Template:    "template name",
				LinkedClone: true,
				DiskSize:    32768,
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"test"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "'linked_clone' and 'disk_size' cannot be used together",
		},
		{
			name: "Validate MacAddress and Network not set at the same time",
			config: &CloneConfig{
				Template:   "template name",
				MacAddress: "some mac address",
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"test"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "'network' is required when 'mac_address' is specified",
		},
		{
			name: "Validate template and ovf_source mutual exclusivity",
			config: &CloneConfig{
				Template: "template name",
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"test"},
					Storage: []common.DiskConfig{
						{
							DiskSize: 32768,
						},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "cannot specify both 'template' and 'ovf_source' - choose one source type",
		},
		{
			name: "Valid ovf_source config",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
			},
			fail: false,
		},
		{
			name: "Validate ovf_source URL is required",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{},
			},
			fail:           true,
			expectedErrMsg: "either 'url' or 'path' is required when using 'ovf_source'",
		},
		{
			name: "Validate ovf_source URL protocol",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "ftp://packages.example.com/artifacts/example.ovf",
				},
			},
			fail:           true,
			expectedErrMsg: "'ovf_source' url must use HTTP or HTTPS protocol",
		},
		{
			name: "Validate ovf_source URL extension",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovx",
				},
			},
			fail:           true,
			expectedErrMsg: "'ovf_source' url must point to an OVF (.ovf) or OVA (.ova) file",
		},
		{
			name: "Validate ovf_source username requires password",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
				},
			},
			fail:           true,
			expectedErrMsg: "'password' is required when 'username' is specified for OVF source",
		},
		{
			name: "Validate ovf_source password requires username",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Password: "testpass",
				},
			},
			fail:           true,
			expectedErrMsg: "'username' is required when 'password' is specified for OVF source",
		},
		{
			name: "Valid ovf_source with SkipTlsVerify for HTTPS",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: true,
				},
			},
			fail: false,
		},
		{
			name: "Validate disk_size cannot be used with ovf_source",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				DiskSize: 32768,
			},
			fail:           true,
			expectedErrMsg: "'disk_size' cannot be used with 'ovf_source'",
		},
		{
			name: "Validate linked_clone cannot be used with ovf_source",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				LinkedClone: true,
			},
			fail:           true,
			expectedErrMsg: "'linked_clone' cannot be used with 'ovf_source'",
		},
		{
			name: "Validate storage cannot be used with ovf_source",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					Storage: []common.DiskConfig{
						{DiskSize: 32768},
					},
				},
			},
			fail:           true,
			expectedErrMsg: "'storage' cannot be used with 'ovf_source'",
		},
		{
			name: "Validate disk_controller_type cannot be used with ovf_source",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
				},
			},
			fail:           true,
			expectedErrMsg: "'disk_controller_type' cannot be used with 'ovf_source'",
		},
		{
			name: "Valid ovf_source path config",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					Path: createTempOvfFile(t),
				},
			},
			fail: false,
		},
		{
			name: "Validate ovf_source cannot set both url and path",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:  "https://packages.example.com/artifacts/example.ovf",
					Path: createTempOvfFile(t),
				},
			},
			fail:           true,
			expectedErrMsg: "'ovf_source' cannot specify both 'url' and 'path'",
		},
		{
			name: "Validate ovf_source path must exist",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					Path: filepath.Join(t.TempDir(), "missing.ovf"),
				},
			},
			fail:           true,
			expectedErrMsg: "does not exist",
		},
		{
			name: "Validate ovf_source path extension",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					Path: filepath.Join(t.TempDir(), "example.vmdk"),
				},
			},
			fail:           true,
			expectedErrMsg: "'ovf_source' path must point to an OVF (.ovf) or OVA (.ova) file",
		},
		{
			name: "Validate auth not allowed with path",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					Path:     createTempOvfFile(t),
					Username: "user",
					Password: "pass",
				},
			},
			fail:           true,
			expectedErrMsg: "'username' and 'password' are only applicable when 'url' is set",
		},
		{
			name: "Validate skip_tls_verify not allowed with path",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					Path:          createTempOvfFile(t),
					SkipTlsVerify: true,
				},
			},
			fail:           true,
			expectedErrMsg: "'skip_tls_verify' is only applicable when 'url' is set",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			errs := c.config.Prepare()
			if c.fail {
				if len(errs) == 0 {
					t.Fatal("unexpected success: expected failure")
				}
				if c.expectedErrMsg != "" && !strings.Contains(errs[0].Error(), c.expectedErrMsg) {
					t.Fatalf("unexpected error: expected to contain '%s', but returned '%s'", c.expectedErrMsg, errs[0])
				}
			} else {
				if len(errs) != 0 {
					t.Fatalf("unexpected failure: expected success, but failed: %s", errs[0])
				}
			}
		})
	}
}

func TestCloneConfig_Prepare_OvfPathWithUsernameOnly(t *testing.T) {
	cfg := &CloneConfig{
		OvfSource: &OvfSourceConfig{
			Path:     createTempOvfFile(t),
			Username: "user",
		},
	}

	errs := cfg.Prepare()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "'username' and 'password' are only applicable when 'url' is set") {
		t.Fatalf("unexpected error: %s", errs[0])
	}
}

func createTempOvfFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "example.ovf")
	if err := os.WriteFile(path, []byte("<Envelope/>"), 0o644); err != nil {
		t.Fatalf("write temp ovf: %s", err)
	}
	return path
}

func TestStepCreateVM_Run(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	})
	driverMock := driver.NewDriverMock()
	state.Put("driver", driverMock)
	step := basicStepCloneVM()
	step.Force = true
	vmPath := path.Join(step.Location.Folder, step.Location.VMName)
	vmMock := new(driver.VirtualMachineMock)
	driverMock.VM = vmMock

	if action := step.Run(context.Background(), state); action == multistep.ActionHalt {
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Find VM
	if !driverMock.FindVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "FindVM")
	}

	// Pre clean VM
	if !driverMock.PreCleanVMCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "PreCleanVM")
	}
	if driverMock.PreCleanForce != step.Force {
		t.Fatalf("unexpected result: expected '%t', but returned '%t'", step.Force, driverMock.PreCleanForce)
	}
	if driverMock.PreCleanVMPath != vmPath {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", vmPath, driverMock.PreCleanVMPath)
	}

	// Clone VM
	if !vmMock.CloneCalled {
		t.Fatalf("unexpected result: expected '%s' to be called", "Clone")
	}
	if diff := cmp.Diff(vmMock.CloneConfig, driverCreateConfig(step.Config, step.Location)); diff != "" {
		t.Fatalf("unexpected result: '%s'", diff)
	}
	vm, ok := state.GetOk("vm")
	if !ok {
		t.Fatalf("unexpected state: '%s' not found", "vm")
	}
	if vm != driverMock.VM {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", driverMock.VM, vm)
	}
}

func TestStepCreateVM_Run_nilCloneResult(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	})
	driverMock := driver.NewDriverMock()
	state.Put("driver", driverMock)
	step := basicStepCloneVM()
	vmMock := new(driver.VirtualMachineMock)
	vmMock.CloneReturnNil = true
	driverMock.VM = vmMock

	action := step.Run(context.Background(), state)
	if action != multistep.ActionHalt {
		t.Fatalf("expected ActionHalt, got %v", action)
	}

	err, ok := state.GetOk("error")
	if !ok {
		t.Fatal("expected error in state")
	}
	if !strings.Contains(err.(error).Error(), "clone operation returned no VM and no error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func basicStepCloneVM() *StepCloneVM {
	step := &StepCloneVM{
		Config:   createConfig(),
		Location: basicLocationConfig(),
	}
	return step
}

func basicLocationConfig() *common.LocationConfig {
	return &common.LocationConfig{
		VMName:       "test-vm",
		Folder:       "test-folder",
		Cluster:      "test-cluster",
		Host:         "test-host",
		ResourcePool: "test-resource-pool",
		Datastore:    "test-datastore",
	}
}

func createConfig() *CloneConfig {
	return &CloneConfig{
		Template: "template name",
		StorageConfig: common.StorageConfig{
			DiskControllerType: []string{"pvscsi"},
			Storage: []common.DiskConfig{
				{
					DiskSize:            32768,
					DiskThinProvisioned: true,
				},
			},
		},
	}
}

func driverCreateConfig(config *CloneConfig, location *common.LocationConfig) *driver.CloneConfig {
	var disks []driver.Disk
	for _, disk := range config.StorageConfig.Storage {
		disks = append(disks, driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
		})
	}

	return &driver.CloneConfig{
		StorageConfig: driver.StorageConfig{
			DiskControllerType: config.StorageConfig.DiskControllerType,
			Storage:            disks,
		},
		Annotation:      config.Notes,
		Name:            location.VMName,
		Folder:          location.Folder,
		Cluster:         location.Cluster,
		Host:            location.Host,
		ResourcePool:    location.ResourcePool,
		Datastore:       location.Datastore,
		LinkedClone:     config.LinkedClone,
		Network:         config.Network,
		MacAddress:      strings.ToLower(config.MacAddress),
		VAppProperties:  config.VAppConfig.Properties,
		PrimaryDiskSize: config.DiskSize,
	}
}

// TestStepCloneVM_OvfSourceDetection tests that the step correctly detects remote source configuration and branches to the appropriate deployment method.
func TestStepCloneVM_OvfSourceDetection(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		expectTemplate bool
		expectRemote   bool
	}{
		{
			name: "Template source detection",
			config: &CloneConfig{
				Template: "template-name",
			},
			expectTemplate: true,
			expectRemote:   false,
		},
		{
			name: "Remote source detection",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
			},
			expectTemplate: false,
			expectRemote:   true,
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

			if tt.expectTemplate {
				driverMock.VM = new(driver.VirtualMachineMock)
			} else if tt.expectRemote {
				driverMock.DeployOvfVM = new(driver.VirtualMachineMock)
			}

			action := step.Run(context.Background(), state)
			if action != multistep.ActionContinue {
				t.Fatalf("expected ActionContinue, got %v", action)
			}

			if tt.expectTemplate {
				if !driverMock.FindVMCalled {
					t.Error("expected FindVM to be called for template source")
				}
				if driverMock.DeployOvfCalled {
					t.Error("expected DeployOvf NOT to be called for template source")
				}
			} else if tt.expectRemote {
				if !driverMock.DeployOvfCalled {
					t.Error("expected DeployOvf to be called for OVF source")
				}
				if driverMock.FindVMCalled {
					t.Error("expected FindVM NOT to be called for OVF source")
				}
			}
		})
	}
}

// TestStepCloneVM_OvfDeploymentUsesResolvedDatastore verifies that remote OVF deploy
// uses the datastore resolved by StepResolveDatastore (for example from datastore_cluster).
func TestStepCloneVM_OvfDeploymentUsesResolvedDatastore(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	})
	driverMock := driver.NewDriverMock()
	driverMock.DeployOvfVM = new(driver.VirtualMachineMock)
	state.Put("driver", driverMock)
	state.Put("datastore", &driver.DatastoreMock{NameReturn: "drs-selected-datastore"})

	location := basicLocationConfig()
	location.Datastore = ""
	location.DatastoreCluster = "test-datastore-cluster"

	step := &StepCloneVM{
		Config: &CloneConfig{
			OvfSource: &OvfSourceConfig{
				URL: "https://packages.example.com/artifacts/example.ovf",
			},
		},
		Location: location,
		Force:    true,
	}

	action := step.Run(context.Background(), state)
	if action != multistep.ActionContinue {
		t.Fatalf("expected ActionContinue, got %v; error: %v", action, state.Get("error"))
	}

	if !driverMock.DeployOvfCalled {
		t.Fatal("expected DeployOvf to be called")
	}
	if driverMock.DeployOvfConfig.Datastore != "drs-selected-datastore" {
		t.Errorf("expected datastore 'drs-selected-datastore', got '%s'", driverMock.DeployOvfConfig.Datastore)
	}
}

func TestStepCloneVM_OvfLocalPathDeploy(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	})
	driverMock := driver.NewDriverMock()
	driverMock.DeployOvfVM = new(driver.VirtualMachineMock)
	state.Put("driver", driverMock)

	localPath := createTempOvfFile(t)
	step := &StepCloneVM{
		Config: &CloneConfig{
			OvfSource: &OvfSourceConfig{
				Path: localPath,
			},
		},
		Location: basicLocationConfig(),
		Force:    true,
	}

	action := step.Run(context.Background(), state)
	if action != multistep.ActionContinue {
		t.Fatalf("expected ActionContinue, got %v; error: %v", action, state.Get("error"))
	}
	if !driverMock.DeployOvfCalled {
		t.Fatal("expected DeployOvf to be called")
	}
	if driverMock.DeployOvfConfig.Path != localPath {
		t.Errorf("expected Path %q, got %q", localPath, driverMock.DeployOvfConfig.Path)
	}
	if driverMock.DeployOvfConfig.URL != "" {
		t.Errorf("expected empty URL for local path deploy, got %q", driverMock.DeployOvfConfig.URL)
	}
}

// TestStepCloneVM_OvfDeploymentWithMockedCalls tests OVF deployment method with mocked vSphere calls.
func TestStepCloneVM_OvfDeploymentWithMockedCalls(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		location       *common.LocationConfig
		mockSetup      func(*driver.DriverMock)
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "Successful OVF deployment",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
			location: basicLocationConfig(),
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
		},
		{
			name: "OVF deployment with authentication",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
			location: basicLocationConfig(),
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfVM = new(driver.VirtualMachineMock)
			},
			expectError: false,
		},
		{
			name: "OVF deployment failure",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
			location: basicLocationConfig(),
			mockSetup: func(mock *driver.DriverMock) {
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("network error accessing remote OVF")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example.ovf': network error accessing remote OVF",
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
				Location: tt.location,
				Force:    true,
			}

			tt.mockSetup(driverMock)

			action := step.Run(context.Background(), state)

			if tt.expectError {
				if action != multistep.ActionHalt {
					t.Fatalf("expected ActionHalt for error case, got %v", action)
				}
				if err, ok := state.GetOk("error"); ok {
					if !strings.Contains(err.(error).Error(), tt.expectedErrMsg) {
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
					t.Error("expected DeployOvf to be called")
				}

				if driverMock.DeployOvfConfig.URL != tt.config.OvfSource.URL {
					t.Errorf("expected URL '%s', got '%s'", tt.config.OvfSource.URL, driverMock.DeployOvfConfig.URL)
				}

				if tt.config.OvfSource.Username != "" {
					if driverMock.DeployOvfConfig.Authentication == nil {
						t.Error("expected authentication config to be set")
					} else {
						if driverMock.DeployOvfConfig.Authentication.Username != tt.config.OvfSource.Username {
							t.Errorf("expected username '%s', got '%s'", tt.config.OvfSource.Username, driverMock.DeployOvfConfig.Authentication.Username)
						}
						if driverMock.DeployOvfConfig.Authentication.Password != tt.config.OvfSource.Password {
							t.Errorf("expected password '%s', got '%s'", tt.config.OvfSource.Password, driverMock.DeployOvfConfig.Authentication.Password)
						}
					}
				} else {
					if driverMock.DeployOvfConfig.Authentication != nil {
						t.Error("expected authentication config to be nil for anonymous access")
					}
				}

				if vm, ok := state.GetOk("vm"); !ok {
					t.Error("expected vm to be set in state")
				} else if vm != driverMock.DeployOvfVM {
					t.Error("expected vm in state to match mock VM")
				}
			}
		})
	}
}

// TestStepCloneVM_VAppPropertyIntegration tests vApp property integration for OVF deployment.
func TestStepCloneVM_VAppPropertyIntegration(t *testing.T) {
	tests := []struct {
		name               string
		vappConfig         common.VAppConfig
		expectedProperties map[string]string
		expectedOption     string
	}{
		{
			name: "Basic vApp properties",
			vappConfig: common.VAppConfig{
				Properties: map[string]string{
					"hostname":  "test-host",
					"user-data": "dGVzdCBkYXRh",
				},
			},
			expectedProperties: map[string]string{
				"hostname":  "test-host",
				"user-data": "dGVzdCBkYXRh",
			},
			expectedOption: "",
		},
		{
			name: "vApp properties with deployment option",
			vappConfig: common.VAppConfig{
				Properties: map[string]string{
					"hostname": "test-host",
					"domain":   "example.com",
				},
				DeploymentOption: "small",
			},
			expectedProperties: map[string]string{
				"hostname": "test-host",
				"domain":   "example.com",
			},
			expectedOption: "small",
		},
		{
			name: "Empty vApp properties",
			vappConfig: common.VAppConfig{
				Properties: map[string]string{},
			},
			expectedProperties: map[string]string{},
			expectedOption:     "",
		},
		{
			name: "Deployment option only",
			vappConfig: common.VAppConfig{
				DeploymentOption: "large",
			},
			expectedProperties: map[string]string{},
			expectedOption:     "large",
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

			config := &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				VAppConfig: tt.vappConfig,
				StorageConfig: common.StorageConfig{
					DiskControllerType: []string{"pvscsi"},
					Storage: []common.DiskConfig{
						{
							DiskSize:            32768,
							DiskThinProvisioned: true,
						},
					},
				},
			}

			step := &StepCloneVM{
				Config:   config,
				Location: basicLocationConfig(),
				Force:    true,
			}

			driverMock.DeployOvfVM = new(driver.VirtualMachineMock)

			if tt.vappConfig.DeploymentOption != "" {
				driverMock.GetOvfOptionsResult = []types.OvfOptionInfo{
					{
						Option: tt.vappConfig.DeploymentOption,
						Description: types.LocalizableMessage{
							Message: fmt.Sprintf("%s configuration", tt.vappConfig.DeploymentOption),
						},
					},
				}
			}

			action := step.Run(context.Background(), state)
			if action != multistep.ActionContinue {
				t.Fatalf("expected ActionContinue, got %v", action)
			}

			if !driverMock.DeployOvfCalled {
				t.Fatal("expected DeployOvf to be called")
			}

			if len(driverMock.DeployOvfConfig.VAppProperties) != len(tt.expectedProperties) {
				t.Errorf("expected %d vApp properties, got %d", len(tt.expectedProperties), len(driverMock.DeployOvfConfig.VAppProperties))
			}

			for key, expectedValue := range tt.expectedProperties {
				if actualValue, exists := driverMock.DeployOvfConfig.VAppProperties[key]; !exists {
					t.Errorf("expected vApp property '%s' to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("expected vApp property '%s' to be '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			if driverMock.DeployOvfConfig.DeploymentOption != tt.expectedOption {
				t.Errorf("expected deployment option '%s', got '%s'", tt.expectedOption, driverMock.DeployOvfConfig.DeploymentOption)
			}
		})
	}
}

// TestStepCloneVM_OvfValidationIntegration tests OVF validation integration during deployment.
func TestStepCloneVM_OvfValidationIntegration(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		mockSetup      func(*driver.DriverMock)
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "Valid deployment option",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				VAppConfig: common.VAppConfig{
					DeploymentOption: "small",
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
				mock.GetOvfOptionsResult = []types.OvfOptionInfo{
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
			},
			expectError: false,
		},
		{
			name: "Valid deployment option with skip TLS verify",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:           "https://packages.example.com/artifacts/example.ovf",
					SkipTlsVerify: true,
				},
				VAppConfig: common.VAppConfig{
					DeploymentOption: "small",
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
				mock.GetOvfOptionsResult = []types.OvfOptionInfo{
					{
						Option: "small",
						Description: types.LocalizableMessage{
							Message: "Small configuration",
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "Invalid deployment option",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example.ovf",
				},
				VAppConfig: common.VAppConfig{
					DeploymentOption: "invalid",
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
				mock.GetOvfOptionsResult = []types.OvfOptionInfo{
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
			},
			expectError:    true,
			expectedErrMsg: "deployment option 'invalid' not found in OVF. Available options: small, medium",
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
					if !strings.Contains(err.(error).Error(), tt.expectedErrMsg) {
						t.Errorf("expected error message to contain '%s', got '%s'", tt.expectedErrMsg, err.(error).Error())
					}
				} else {
					t.Error("expected error to be set in state")
				}
			} else {
				if action != multistep.ActionContinue {
					t.Fatalf("expected ActionContinue, got %v", action)
				}

				if tt.config.VAppConfig.DeploymentOption != "" {
					if !driverMock.GetOvfOptionsCalled {
						t.Error("expected GetOvfOptions to be called for deployment option validation")
					}
					if tt.config.OvfSource != nil && driverMock.GetOvfOptionsSkipTlsVerify != tt.config.OvfSource.SkipTlsVerify {
						t.Errorf("expected GetOvfOptionsSkipTlsVerify %v, got %v", tt.config.OvfSource.SkipTlsVerify, driverMock.GetOvfOptionsSkipTlsVerify)
					}
				}
			}
		})
	}
}

// TestStepCloneVM_CleanupTemplateSource tests that template-based cleanup does not depend on remote OVF state keys.
func TestStepCloneVM_CleanupTemplateSource(t *testing.T) {
	// Setup for template-based cloning (should not perform OVF cleanup)
	step := &StepCloneVM{
		Config: &CloneConfig{
			Template: "test-template",
		},
		Location: &common.LocationConfig{
			VMName: "test-vm",
			Folder: "test-folder",
		},
	}

	ui := &packersdk.BasicUi{
		Reader: new(bytes.Buffer),
		Writer: new(bytes.Buffer),
	}
	driverMock := driver.NewDriverMock()
	state := &multistep.BasicStateBag{}
	state.Put("ui", ui)
	state.Put("driver", driverMock)

	// Add some OVF-specific state that should NOT be cleaned up for template sources
	taskRef := &types.ManagedObjectReference{Type: "Task", Value: "task-123"}
	state.Put("ovf_task_ref", taskRef)

	// Execute cleanup
	step.Cleanup(state)

	// Verify cleanup does not remove unrelated state keys.
	if _, ok := state.GetOk("ovf_task_ref"); !ok {
		t.Error("expected ovf_task_ref to remain in state")
	}
}

// TestStepCloneVM_ErrorHandlingScenarios tests various error scenarios and error message formatting.
func TestStepCloneVM_ErrorHandlingScenarios(t *testing.T) {
	tests := []struct {
		name           string
		config         *CloneConfig
		mockSetup      func(*driver.DriverMock)
		expectError    bool
		expectedErrMsg string
		errorType      string
	}{
		{
			name: "Network connectivity error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("dial tcp: connection refused")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example.ovf': dial tcp: connection refused",
			errorType:      "network",
		},
		{
			name: "Authentication failure error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:      "https://packages.example.com/artifacts/example.ovf",
					Username: "testuser",
					Password: "wrongpass",
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
				mock.DeployOvfError = fmt.Errorf("HTTP 401 Unauthorized")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example.ovf': HTTP 401 Unauthorized",
			errorType:      "authentication",
		},
		{
			name: "File not found error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example-nonexistent.ovf",
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
				mock.DeployOvfError = fmt.Errorf("HTTP 404 Not Found")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example-nonexistent.ovf': HTTP 404 Not Found",
			errorType:      "not_found",
		},
		{
			name: "OVF validation error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example-invalid.ovf",
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
				mock.DeployOvfError = fmt.Errorf("invalid OVF descriptor: malformed XML")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example-invalid.ovf': invalid OVF descriptor: malformed XML",
			errorType:      "validation",
		},
		{
			name: "TLS certificate error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("x509: certificate signed by unknown authority")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example.ovf': x509: certificate signed by unknown authority",
			errorType:      "tls",
		},
		{
			name: "TLS certificate error with SkipTlsVerify enabled",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
				mock.DeployOvfShouldFail = false
				mock.DeployOvfVM = &driver.VirtualMachineMock{}
			},
			expectError: false,
		},
		{
			name: "Insufficient resources error",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL: "https://packages.example.com/artifacts/example-large.ovf",
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
				mock.DeployOvfError = fmt.Errorf("insufficient disk space on datastore")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example-large.ovf': insufficient disk space on datastore",
			errorType:      "resources",
		},
		{
			name: "Credential sanitization in error messages",
			config: &CloneConfig{
				OvfSource: &OvfSourceConfig{
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
				mock.DeployOvfShouldFail = true
				mock.DeployOvfError = fmt.Errorf("authentication failed with password=secretpassword for user testuser")
			},
			expectError:    true,
			expectedErrMsg: "OVF deployment failed for OVF source 'https://packages.example.com/artifacts/example.ovf'",
			errorType:      "credential_sanitization",
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
					errorMsg := err.(error).Error()
					if !strings.Contains(errorMsg, tt.expectedErrMsg) {
						t.Errorf("expected error message to contain '%s', got '%s'", tt.expectedErrMsg, errorMsg)
					}

					// Verify credential sanitization
					if tt.errorType == "credential_sanitization" {
						if strings.Contains(errorMsg, "secretpassword") {
							t.Errorf("Error message should not contain password, got '%s'", errorMsg)
						}
						if strings.Contains(errorMsg, "password=secretpassword") {
							t.Errorf("Error message should not contain password pattern, got '%s'", errorMsg)
						}
						if !strings.Contains(errorMsg, "password=[credentials removed]") {
							t.Errorf("expected sanitized password in error, got '%s'", errorMsg)
						}
					}
				} else {
					t.Error("expected error to be set in state")
				}
			} else {
				if action != multistep.ActionContinue {
					t.Fatalf("expected ActionContinue, got %v", action)
				}
			}
		})
	}
}

// TestStepCloneVM_ErrorMessageFormatting tests that error messages are properly formatted and sanitized.
func TestStepCloneVM_ErrorMessageFormatting(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		username         string
		password         string
		mockError        error
		expectedURL      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "URL with credentials sanitized",
			url:         "https://testuser:secret@packages.example.com/artifacts/example.ovf",
			username:    "testuser",
			password:    "testpass",
			mockError:   fmt.Errorf("connection failed"),
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			shouldContain: []string{
				"OVF deployment failed for OVF source",
				"https://packages.example.com/artifacts/example.ovf",
				"connection failed",
			},
			shouldNotContain: []string{"testpass", "secret", "testuser@", "testuser:secret"},
		},
		{
			name:        "Error message with password pattern sanitized",
			url:         "https://packages.example.com/artifacts/example.ovf",
			mockError:   fmt.Errorf("authentication failed: password=testpass"),
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			shouldContain: []string{
				"OVF deployment failed for OVF source",
				"https://packages.example.com/artifacts/example.ovf",
			},
			shouldNotContain: []string{"testpass", "password=testpass"},
		},
		{
			name:        "Error message with multiple credential patterns",
			url:         "https://packages.example.com/artifacts/example.ovf",
			mockError:   fmt.Errorf("failed with password=testpass and token=testtoken"),
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			shouldContain: []string{
				"OVF deployment failed for OVF source",
				"https://packages.example.com/artifacts/example.ovf",
			},
			shouldNotContain: []string{"testpass", "testtoken", "password=testpass", "token=testtoken"},
		},
		{
			name:        "Clean error message without credentials",
			url:         "https://packages.example.com/artifacts/example.ovf",
			mockError:   fmt.Errorf("network timeout occurred"),
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			shouldContain: []string{
				"OVF deployment failed for OVF source",
				"https://packages.example.com/artifacts/example.ovf",
				"network timeout occurred",
			},
			shouldNotContain: []string{},
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

			config := &CloneConfig{
				OvfSource: &OvfSourceConfig{
					URL:      tt.url,
					Username: tt.username,
					Password: tt.password,
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
			}

			step := &StepCloneVM{
				Config:   config,
				Location: basicLocationConfig(),
				Force:    true,
			}

			driverMock.DeployOvfShouldFail = true
			driverMock.DeployOvfError = tt.mockError

			action := step.Run(context.Background(), state)
			if action != multistep.ActionHalt {
				t.Fatalf("expected ActionHalt for error case, got %v", action)
			}

			if err, ok := state.GetOk("error"); ok {
				errorMsg := err.(error).Error()

				// Check that expected strings are present.
				for _, expected := range tt.shouldContain {
					if !strings.Contains(errorMsg, expected) {
						t.Errorf("expected error message to contain '%s', got '%s'", expected, errorMsg)
					}
				}

				// Check that sensitive strings are not present.
				for _, forbidden := range tt.shouldNotContain {
					if strings.Contains(errorMsg, forbidden) {
						t.Errorf("expected error message to NOT contain '%s', got '%s'", forbidden, errorMsg)
					}
				}
			} else {
				t.Error("expected error to be set in state")
			}
		})
	}
}

// TestOvfSourceConfig_MapstructureDecode verifies that ovf_source decodes from
// raw configuration maps using the struct's mapstructure tags.
func TestOvfSourceConfig_MapstructureDecode(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]interface{}
		want OvfSourceConfig
	}{
		{
			name: "url with credentials",
			raw: map[string]interface{}{
				"ovf_source": map[string]interface{}{
					"url":      "https://packages.example.com/artifacts/example.ovf",
					"username": "testuser",
					"password": "testpass",
				},
			},
			want: OvfSourceConfig{
				URL:      "https://packages.example.com/artifacts/example.ovf",
				Username: "testuser",
				Password: "testpass",
			},
		},
		{
			name: "url with skip tls verify",
			raw: map[string]interface{}{
				"ovf_source": map[string]interface{}{
					"url":             "https://packages.example.com/artifacts/example.ova",
					"skip_tls_verify": true,
				},
			},
			want: OvfSourceConfig{
				URL:           "https://packages.example.com/artifacts/example.ova",
				SkipTlsVerify: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg struct {
				OvfSource *OvfSourceConfig `mapstructure:"ovf_source"`
			}
			if err := mapstructure.Decode(tt.raw, &cfg); err != nil {
				t.Fatalf("mapstructure.Decode: %v", err)
			}
			if cfg.OvfSource == nil {
				t.Fatal("expected ovf_source to be decoded")
			}
			if diff := cmp.Diff(tt.want, *cfg.OvfSource); diff != "" {
				t.Errorf("decoded config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestOvfSourceConfig_CredentialSanitization verifies that URLs containing credentials
// are properly sanitized to prevent credential exposure in logs.
func TestOvfSourceConfig_CredentialSanitization(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with credentials",
			url:      "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expected: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "URL without credentials",
			url:      "https://packages.example.com/artifacts/example.ovf",
			expected: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "HTTP URL with credentials",
			url:      "http://admin:secret@internal.example.com/templates/vm.ova",
			expected: "http://internal.example.com/templates/vm.ova",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := driver.SanitizeOvfURL(tt.url)

			if sanitized != tt.expected {
				t.Errorf("SanitizeOvfURL() = %v, want %v", sanitized, tt.expected)
			}
		})
	}
}

// TestOvfSourceConfig_ErrorMessageSanitization verifies that error messages containing
// credential patterns are properly sanitized to prevent credential exposure.
func TestOvfSourceConfig_ErrorMessageSanitization(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{
			name:     "error with password",
			errMsg:   "authentication failed: password=testpass invalid",
			expected: "authentication failed: password=[credentials removed] invalid",
		},
		{
			name:     "error with URL credentials",
			errMsg:   "failed to connect to https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expected: "failed to connect to https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "error without credentials",
			errMsg:   "network timeout connecting to packages.example.com",
			expected: "network timeout connecting to packages.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := driver.SanitizeOvfErrorMessage(tt.errMsg)

			if sanitized != tt.expected {
				t.Errorf("SanitizeOvfErrorMessage() = %v, want %v", sanitized, tt.expected)
			}
		})
	}
}

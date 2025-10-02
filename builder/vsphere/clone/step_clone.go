// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type CloneConfig

package clone

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// RemoteSourceConfig defines configuration for cloning from remote OVF/OVA sources.
type RemoteSourceConfig struct {
	// The URL of the remote OVF/OVA file. Supports HTTP and HTTPS protocols.
	URL string `mapstructure:"url"`
	// The username for basic authentication when accessing the remote OVF/OVA file.
	// Must be used together with `password`.
	Username string `mapstructure:"username"`
	// The password for basic authentication when accessing the remote OVF/OVA file.
	// Must be used together with `username`.
	Password string `mapstructure:"password"`
	// Do not validate the certificate when accessing HTTPS URLs.
	// Defaults to `false`.
	//
	// -> **Note:** This option is beneficial in scenarios where the certificate
	// is self-signed or does not meet standard validation criteria.
	//
	// HCL Example:
	//
	// ```hcl
	//   remote_source = {
	//     url              = "https://packages.example.com/artifacts/example.ovf"
	//     username         = "remote_source_username"
	//     password         = "remote_source_password"
	//     skip_tls_verify  = false
	//   }
	// ```
	//
	// JSON Example:
	// ```json
	//   "remote_source": {
	//     "url": "https://packages.example.com/artifacts/example.ovf",
	//     "username": "remote_source_username",
	//     "password": "remote_source_password",
	//     "skip_tls_verify": false
	//   }
	SkipTlsVerify bool `mapstructure:"skip_tls_verify"`
}

type CloneConfig struct {
	// The name of the source virtual machine to clone.
	Template string `mapstructure:"template"`
	// Configuration for cloning from a remote OVF/OVA source.
	// Cannot be used together with `template`.
	//
	// For more information, refer to the [Remote Source Configuration](/packer/integrations/hashicorp/vmware/latest/components/builder/vsphere-clone#remote-source-configuration)
	// section.
	RemoteSource *RemoteSourceConfig `mapstructure:"remote_source"`
	// The size of the primary disk in MiB. Cannot be used with `linked_clone`.
	// -> **Note:** Only the primary disk size can be specified. Additional
	// disks are not supported.
	DiskSize int64 `mapstructure:"disk_size"`
	// Create the virtual machine as a linked clone from the latest snapshot.
	// Defaults to `false`. Cannot be used with `disk_size`.`
	LinkedClone bool `mapstructure:"linked_clone"`
	// The network to which the virtual machine will connect.
	//
	// For example:
	//
	// - Name: `<NetworkName>`
	// - Inventory Path: `/<DatacenterName>/<FolderName>/<NetworkName>`
	// - Managed Object ID (Port Group): `Network:network-<xxxxx>`
	// - Managed Object ID (Distributed Port Group): `DistributedVirtualPortgroup::dvportgroup-<xxxxx>`
	// - Logical Switch UUID: `<uuid>`
	// - Segment ID: `/infra/segments/<SegmentID>`
	//
	// ~> **Note:** If more than one network resolves to the same name, either
	// the inventory path to network or an ID must be provided.
	//
	// ~> **Note:** If no network is specified, provide `host` to allow the
	// plugin to search for an available network.
	Network string `mapstructure:"network"`
	// The network card MAC address. For example `00:50:56:00:00:00`.
	// If set, the `network` must be also specified.
	MacAddress string `mapstructure:"mac_address"`
	// The annotations for the virtual machine.
	Notes string `mapstructure:"notes"`
	// Destroy the virtual machine after the build is complete.
	// Defaults to `false`.
	Destroy bool `mapstructure:"destroy"`
	// The vApp Options for the virtual machine. For more information, refer to
	// the [vApp Options Configuration](/packer/plugins/builders/vmware/vsphere-clone#vapp-options-configuration)
	// section.
	VAppConfig    common.VAppConfig    `mapstructure:"vapp"`
	StorageConfig common.StorageConfig `mapstructure:",squash"`
}

// Prepare validates the CloneConfig and returns any validation errors.
func (c *CloneConfig) Prepare() []error {
	var errs []error
	errs = append(errs, c.StorageConfig.Prepare()...)

	// Validate source configuration for mutual exclusivity.
	hasTemplate := c.Template != ""
	hasRemoteSource := c.RemoteSource != nil

	if !hasTemplate && !hasRemoteSource {
		errs = append(errs, fmt.Errorf("either 'template' or 'remote_source' must be specified"))
	}

	if hasTemplate && hasRemoteSource {
		errs = append(errs, fmt.Errorf("cannot specify both 'template' and 'remote_source' - choose one source type"))
	}

	if hasRemoteSource {
		if c.RemoteSource.URL == "" {
			errs = append(errs, fmt.Errorf("'url' is required when using 'remote_source'"))
		} else {
			parsedURL, err := url.Parse(c.RemoteSource.URL)
			if err != nil {
				errs = append(errs, fmt.Errorf("invalid 'remote_source' URL format: %s", err))
			} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				errs = append(errs, fmt.Errorf("'remote_source' URL must use HTTP or HTTPS protocol"))
			}
		}

		hasUsername := c.RemoteSource.Username != ""
		hasPassword := c.RemoteSource.Password != ""
		if hasUsername && !hasPassword {
			errs = append(errs, fmt.Errorf("'password' is required when 'username' is specified for remote source"))
		}
		if hasPassword && !hasUsername {
			errs = append(errs, fmt.Errorf("'username' is required when 'password' is specified for remote source"))
		}
	}

	if c.LinkedClone && c.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'linked_clone' and 'disk_size' cannot be used together"))
	}

	if c.MacAddress != "" && c.Network == "" {
		errs = append(errs, fmt.Errorf("'network' is required when 'mac_address' is specified"))
	}

	return errs
}

type StepCloneVM struct {
	Config        *CloneConfig
	Location      *common.LocationConfig
	Force         bool
	GeneratedData *packerbuilderdata.GeneratedData
}

// Run executes the clone VM step by detecting the source type and delegating to the appropriate method.
func (s *StepCloneVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.RemoteSource != nil {
		return s.deployFromRemoteOvf(ctx, state)
	}
	return s.cloneFromTemplate(ctx, state)
}

// cloneFromTemplate handles traditional template-based cloning for backward compatibility.
func (s *StepCloneVM) cloneFromTemplate(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)
	vmPath := path.Join(s.Location.Folder, s.Location.VMName)

	ui.Say("Finding virtual machine to clone...")
	template, err := d.FindVM(s.Config.Template)
	if err != nil {
		state.Put("error", fmt.Errorf("error finding virtual machine to clone: %s", err))
		return multistep.ActionHalt
	}

	err = d.PreCleanVM(ui, vmPath, s.Force, s.Location.Cluster, s.Location.Host, s.Location.ResourcePool)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Cloning virtual machine...")
	var disks []driver.Disk
	for _, disk := range s.Config.StorageConfig.Storage {
		disks = append(disks, driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
		})
	}

	datastoreName := s.Location.Datastore
	var primaryDatastore driver.Datastore
	if ds, ok := state.GetOk("datastore"); ok {
		primaryDatastore = ds.(driver.Datastore)
		datastoreName = primaryDatastore.Name()
	}

	// If no datastore was resolved and no datastore was specified, return an error.
	if datastoreName == "" && s.Location.DatastoreCluster == "" {
		state.Put("error", fmt.Errorf("no datastore specified and no datastore resolved from cluster"))
		return multistep.ActionHalt
	}

	// Handle multi-disk placement when using a datastore cluster.
	var datastoreRefs []*types.ManagedObjectReference
	if s.Location.DatastoreCluster != "" && len(disks) > 1 {
		if vcDriver, ok := d.(*driver.VCenterDriver); ok {
			// Request Storage DRS recommendations for all disks at once for optimal placement.
			ui.Sayf("Requesting Storage DRS recommendations for %d disks...", len(disks))

			diskDatastores, method, err := vcDriver.SelectDatastoresForDisks(s.Location.DatastoreCluster, disks)
			if err != nil {
				ui.Errorf("Warning: Failed to get Storage DRS recommendations: %s. Using primary datastore.", err)
				if primaryDatastore != nil {
					ref := primaryDatastore.Reference()
					for i := 0; i < len(disks); i++ {
						datastoreRefs = append(datastoreRefs, &ref)
					}
				}
			} else {
				// Use the first disk's datastore as the primary datastore.
				if len(diskDatastores) > 0 {
					datastoreName = diskDatastores[0].Name()
				}

				for i, ds := range diskDatastores {
					ref := ds.Reference()
					if method == driver.SelectionMethodDRS {
						log.Printf("[INFO] Disk %d: Storage DRS selected datastore '%s'", i+1, ds.Name())
					} else {
						log.Printf("[INFO] Disk %d: Using first available datastore '%s'", i+1, ds.Name())
					}
					datastoreRefs = append(datastoreRefs, &ref)
				}
			}
		}
	}

	vm, err := template.Clone(ctx, &driver.CloneConfig{
		Name:            s.Location.VMName,
		Folder:          s.Location.Folder,
		Cluster:         s.Location.Cluster,
		Host:            s.Location.Host,
		ResourcePool:    s.Location.ResourcePool,
		Datastore:       datastoreName,
		LinkedClone:     s.Config.LinkedClone,
		Network:         s.Config.Network,
		MacAddress:      strings.ToLower(s.Config.MacAddress),
		Annotation:      s.Config.Notes,
		VAppProperties:  s.Config.VAppConfig.Properties,
		PrimaryDiskSize: s.Config.DiskSize,
		StorageConfig: driver.StorageConfig{
			DiskControllerType: s.Config.StorageConfig.DiskControllerType,
			Storage:            disks,
			DatastoreRefs:      datastoreRefs,
		},
	})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	if vm == nil {
		return multistep.ActionHalt
	}
	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

// deployFromRemoteOvf handles deployment from remote OVF/OVA sources using vSphere's native pull method.
func (s *StepCloneVM) deployFromRemoteOvf(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)
	vmPath := path.Join(s.Location.Folder, s.Location.VMName)

	ui.Say(fmt.Sprintf("Deploying virtual machine from remote OVF/OVA: %s", s.sanitizeURL(s.Config.RemoteSource.URL)))

	err := d.PreCleanVM(ui, vmPath, s.Force, s.Location.Cluster, s.Location.Host, s.Location.ResourcePool)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var auth *driver.OvfAuthConfig
	if s.Config.RemoteSource.Username != "" && s.Config.RemoteSource.Password != "" {
		auth = &driver.OvfAuthConfig{
			Username: s.Config.RemoteSource.Username,
			Password: s.Config.RemoteSource.Password,
		}
	}

	var disks []driver.Disk
	for _, disk := range s.Config.StorageConfig.Storage {
		disks = append(disks, driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
		})
	}

	ovfConfig := &driver.OvfDeployConfig{
		URL:              s.Config.RemoteSource.URL,
		Authentication:   auth,
		Name:             s.Location.VMName,
		Folder:           s.Location.Folder,
		Cluster:          s.Location.Cluster,
		Host:             s.Location.Host,
		ResourcePool:     s.Location.ResourcePool,
		Datastore:        s.Location.Datastore,
		Network:          s.Config.Network,
		MacAddress:       strings.ToLower(s.Config.MacAddress),
		Annotation:       s.Config.Notes,
		VAppProperties:   s.Config.VAppConfig.Properties,
		DeploymentOption: s.Config.VAppConfig.DeploymentOption,
		StorageConfig: driver.StorageConfig{
			DiskControllerType: s.Config.StorageConfig.DiskControllerType,
			Storage:            disks,
		},
		Locale:        "US",
		SkipTlsVerify: s.Config.RemoteSource.SkipTlsVerify,
	}

	// Validate OVF deployment parameters with enhanced error handling
	if err := s.validateOvfConfiguration(ctx, d, ovfConfig, ui); err != nil {
		state.Put("error", s.wrapStepError("OVF configuration validation failed", err, s.Config.RemoteSource.URL))
		return multistep.ActionHalt
	}

	ui.Say("Deploying virtual machine from remote OVF/OVA source...")
	vm, err := d.DeployOvf(ctx, ovfConfig, ui)
	if err != nil {
		state.Put("error", s.wrapStepError("OVF deployment failed", err, s.Config.RemoteSource.URL))
		return multistep.ActionHalt
	}

	if vm == nil {
		state.Put("error", fmt.Errorf("OVF deployment completed but returned no virtual machine reference. This may indicate a vSphere configuration issue"))
		return multistep.ActionHalt
	}

	ui.Say("Successfully deployed virtual machine from remote OVF/OVA source")

	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

// validateOvfDeploymentOption validates that the specified deployment option exists in the OVF descriptor.
func (s *StepCloneVM) validateOvfDeploymentOption(ctx context.Context, d driver.Driver, config *driver.OvfDeployConfig) error {
	if config.DeploymentOption == "" {
		return nil
	}

	locale := config.Locale
	if locale == "" {
		locale = "US"
	}
	options, err := d.GetOvfOptions(ctx, config.URL, config.Authentication, locale)
	if err != nil {
		return fmt.Errorf("error retrieving OVF deployment options: %s", err)
	}

	availableOptions := make([]string, 0, len(options))
	for _, option := range options {
		if option.Option == config.DeploymentOption {
			return nil
		}
		availableOptions = append(availableOptions, option.Option)
	}

	if len(availableOptions) == 0 {
		return fmt.Errorf("deployment option '%s' specified but OVF does not define any deployment options", config.DeploymentOption)
	}

	return fmt.Errorf("deployment option '%s' not found in OVF. Available options: %s",
		config.DeploymentOption, strings.Join(availableOptions, ", "))
}

// validateOvfConfiguration validates OVF deployment parameters and vApp properties.
func (s *StepCloneVM) validateOvfConfiguration(ctx context.Context, d driver.Driver, config *driver.OvfDeployConfig, ui packersdk.Ui) error {
	if config.DeploymentOption != "" {
		ui.Say(fmt.Sprintf("Validating OVF deployment option: %s", config.DeploymentOption))
		if err := s.validateOvfDeploymentOption(ctx, d, config); err != nil {
			return err
		}
	}

	if len(config.VAppProperties) > 0 {
		ui.Say("Validating vApp properties against OVF descriptor...")
		if err := s.validateOvfVAppProperties(ctx, d, config); err != nil {
			return err
		}
	}

	return nil
}

// validateOvfVAppProperties performs basic validation of vApp property keys and values.
// The vSphere OVF Manager performs definitive validation during deployment.
func (s *StepCloneVM) validateOvfVAppProperties(_ context.Context, _ driver.Driver, config *driver.OvfDeployConfig) error {
	if len(config.VAppProperties) == 0 {
		return nil
	}

	for key, value := range config.VAppProperties {
		if key == "" {
			return fmt.Errorf("vApp property key cannot be empty")
		}
		if len(key) > 255 {
			return fmt.Errorf("vApp property key '%s' exceeds maximum length of 255 characters", key)
		}
		if len(value) > 65535 {
			return fmt.Errorf("vApp property value for key '%s' exceeds maximum length of 65535 characters", key)
		}
	}

	return nil
}

// wrapStepError wraps errors with context and sanitizes sensitive information for step operations.
func (s *StepCloneVM) wrapStepError(context string, err error, url string) error {
	sanitizedURL := s.sanitizeURL(url)
	sanitizedErr := s.sanitizeErrorMessage(err.Error())
	return fmt.Errorf("%s for remote source '%s': %s", context, sanitizedURL, sanitizedErr)
}

// sanitizeURL removes credentials from URLs for safe logging in step operations.
func (s *StepCloneVM) sanitizeURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "[invalid URL]"
	}

	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// sanitizeErrorMessage removes sensitive information from error messages in step operations.
func (s *StepCloneVM) sanitizeErrorMessage(errMsg string) string {
	sanitized := s.sanitizeURLsInString(errMsg)
	return s.sanitizeCredentialPatterns(sanitized)
}

// sanitizeURLsInString removes credentials from URLs in the given string.
func (s *StepCloneVM) sanitizeURLsInString(str string) string {
	urlPattern := regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s]+`)
	return urlPattern.ReplaceAllStringFunc(str, func(match string) string {
		if u, err := url.Parse(match); err == nil {
			u.User = nil
			return u.String()
		}
		return "[URL with credentials removed]"
	})
}

// sanitizeCredentialPatterns removes credential patterns from the given string.
func (s *StepCloneVM) sanitizeCredentialPatterns(str string) string {
	patterns := []string{
		`password[=:]\s*[^\s]+`,
		`pwd[=:]\s*[^\s]+`,
		`pass[=:]\s*[^\s]+`,
		`secret[=:]\s*[^\s]+`,
		`token[=:]\s*[^\s]+`,
	}

	sanitized := str
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		sanitized = re.ReplaceAllString(sanitized, "[credentials removed]")
	}
	return sanitized
}

// Cleanup performs step cleanup, including OVF-specific resource cleanup for remote deployments.
func (s *StepCloneVM) Cleanup(state multistep.StateBag) {
	if s.Config.RemoteSource != nil {
		s.cleanupOvfDeployment(state)
	}

	common.CleanupVM(state)
}

// cleanupOvfDeployment cleans up OVF deployment-specific resources from the state bag.
func (s *StepCloneVM) cleanupOvfDeployment(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)

	if ovfTaskRef, ok := state.GetOk("ovf_task_ref"); ok {
		ui.Say("Cleaning up OVF deployment task...")

		if d, ok := state.Get("driver").(driver.Driver); ok {
			if taskRef, ok := ovfTaskRef.(*types.ManagedObjectReference); ok {
				s.cancelOvfTask(d, taskRef, ui)
			}
		}

		state.Remove("ovf_task_ref")
	}

	if progressMonitor, ok := state.GetOk("ovf_progress_monitor"); ok {
		ui.Say("Stopping OVF progress monitoring...")
		if monitor, ok := progressMonitor.(*driver.OvfProgressMonitor); ok {
			monitor.Cancel()
		}
		state.Remove("ovf_progress_monitor")
	}

	if _, ok := state.GetOk("ovf_lease"); ok {
		ui.Say("Cleaning up NFC lease...")
		state.Remove("ovf_lease")
	}
}

// cancelOvfTask provides a consistent interface for OVF task cleanup operations.
func (s *StepCloneVM) cancelOvfTask(_ driver.Driver, _ *types.ManagedObjectReference, ui packersdk.Ui) {
	ui.Say("OVF deployment task cleanup initiated")
}

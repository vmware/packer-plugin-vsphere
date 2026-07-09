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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// Specify either a local `path` or a remote `url`.
type OvfSourceConfig struct {
	// The path to a local OVF/OVA file on the host filesystem.
	// Conflicts with `url`.
	//
	// HCL Example:
	//
	// ```hcl
	// ovf_source {
	//   path = "./artifacts/example.ovf"
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "ovf_source": {
	//   "path": "./artifacts/example.ovf"
	// }
	// ```
	//
	// -> **Note:** The `path` must end in `.ovf` or `.ova`.
	Path string `mapstructure:"path"`
	// The URL of the remote OVF/OVA file. Supports HTTP and HTTPS protocols.
	// Conflicts with `path`.
	//
	// HCL Example:
	//
	// ```hcl
	// ovf_source {
	//   url = "https://packages.example.com/artifacts/example.ovf"
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "ovf_source": {
	//   "url": "https://packages.example.com/artifacts/example.ovf"
	// }
	// ```
	//
	// -> **Note:** Use `http://` or `https://`.
	//
	// -> **Note:** The `url` must end in `.ovf` or `.ova`.
	URL string `mapstructure:"url"`
	// The username for basic authentication when accessing the remote OVF/OVA file.
	// Must be used together with `password`. Only applicable when `url` is set.
	//
	// -> **Note:** For credentials, use variables marked `sensitive = true`.
	Username string `mapstructure:"username"`
	// The password for basic authentication when accessing the remote OVF/OVA file.
	// Must be used together with `username`. Only applicable when `url` is set.
	//
	// -> **Note:** For credentials, use variables marked `sensitive = true`.
	Password string `mapstructure:"password"`
	// Do not validate the certificate when accessing HTTPS URLs.
	// Defaults to `false`. Only applicable when `url` is set.
	//
	// -> **Note:** This option is beneficial in scenarios where the certificate
	// is self-signed or does not meet standard validation criteria.
	//
	// HCL Example:
	//
	// ```hcl
	// ovf_source {
	//   url             = "https://packages.example.com/artifacts/example.ova"
	//   username        = "ovf_source_username"
	//   password        = "ovf_source_password"
	//   skip_tls_verify = false
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "ovf_source": {
	//   "url": "https://packages.example.com/artifacts/example.ova",
	//   "username": "ovf_source_username",
	//   "password": "ovf_source_password",
	//   "skip_tls_verify": false
	// }
	// ```
	//
	// -> **Note:** When using a multi-file OVF, keep the descriptor and its disk
	// files in the same directory on the local and remote sources.
	SkipTlsVerify bool `mapstructure:"skip_tls_verify"`
}

type CloneConfig struct {
	// The name of the source virtual machine template to clone. Specify either
	// `template` or `ovf_source`, but not both.
	Template string `mapstructure:"template"`
	// Configuration for deploying from an OVF/OVA source. Specify either a
	// local `path` or a remote `url`. Conflicts with `template`. Refer to the
	// [OVF source configuration](#ovf-source-configuration) section for available
	// fields.
	OvfSource *OvfSourceConfig `mapstructure:"ovf_source"`
	// The size of the primary disk in MiB. Conflicts with `linked_clone`.
	//
	// -> **Note:** Only the primary disk size can be specified. Additional disks
	// are configured with [`storage`](#storage-configuration).
	//
	// ~> **Note:** Applies only when cloning from a `template`; rejected when
	// `ovf_source` is set.
	DiskSize int64 `mapstructure:"disk_size"`
	// Create the virtual machine as a linked clone from the latest snapshot.
	// Defaults to `false`. Conflicts with `disk_size`.
	//
	// ~> **Note:** Applies only when cloning from a `template`; rejected when
	// `ovf_source` is set.
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
	//
	// ~> **Note:** When deploying from an OVF/OVA source, each network
	// defined in the OVF descriptor is mapped to this network. If the OVF
	// defines multiple networks, they all use this same mapping.
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
	// the [vApp Options Configuration](#vapp-options-configuration) section.
	VAppConfig common.VAppConfig `mapstructure:"vapp"`
	// The storage configuration for the virtual machine. For more information,
	// refer to the [Storage Configuration](#storage-configuration) section.
	StorageConfig common.StorageConfig `mapstructure:",squash"`
}

// Prepare validates the CloneConfig and returns any validation errors.
func (c *CloneConfig) Prepare() []error {
	var errs []error

	hasTemplate := c.Template != ""
	hasOvfSource := c.OvfSource != nil

	if !hasTemplate && !hasOvfSource {
		errs = append(errs, fmt.Errorf("either 'template' or 'ovf_source' must be specified"))
	}

	if hasTemplate && hasOvfSource {
		errs = append(errs, fmt.Errorf("cannot specify both 'template' and 'ovf_source' - choose one source type"))
	}

	if hasOvfSource {
		errs = append(errs, c.prepareOvfSource()...)
	}

	errs = append(errs, c.StorageConfig.Prepare()...)

	if c.LinkedClone && c.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'linked_clone' and 'disk_size' cannot be used together"))
	}

	if c.MacAddress != "" && c.Network == "" {
		errs = append(errs, fmt.Errorf("'network' is required when 'mac_address' is specified"))
	}

	return errs
}

func (c *CloneConfig) prepareOvfSource() []error {
	var errs []error

	hasURL := c.OvfSource.URL != ""
	hasPath := c.OvfSource.Path != ""

	if hasURL && hasPath {
		errs = append(errs, fmt.Errorf("'ovf_source' cannot specify both 'url' and 'path'"))
	}
	if !hasURL && !hasPath {
		errs = append(errs, fmt.Errorf("either 'url' or 'path' is required when using 'ovf_source'"))
	}

	if hasURL {
		parsedURL, err := url.Parse(c.OvfSource.URL)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid 'ovf_source' url format: %s", err))
		} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errs = append(errs, fmt.Errorf("'ovf_source' url must use HTTP or HTTPS protocol"))
		} else {
			lower := strings.ToLower(parsedURL.Path)
			if !strings.HasSuffix(lower, ".ovf") && !strings.HasSuffix(lower, ".ova") {
				errs = append(errs, fmt.Errorf("'ovf_source' url must point to an OVF (.ovf) or OVA (.ova) file"))
			}
		}
	}

	if hasPath {
		cleaned := filepath.Clean(c.OvfSource.Path)
		c.OvfSource.Path = cleaned
		lower := strings.ToLower(cleaned)
		if !strings.HasSuffix(lower, ".ovf") && !strings.HasSuffix(lower, ".ova") {
			errs = append(errs, fmt.Errorf("'ovf_source' path must point to an OVF (.ovf) or OVA (.ova) file"))
		} else {
			info, err := os.Stat(cleaned)
			if err != nil {
				if os.IsNotExist(err) {
					errs = append(errs, fmt.Errorf("'ovf_source' path '%s' does not exist", cleaned))
				} else {
					errs = append(errs, fmt.Errorf("unable to access 'ovf_source' path '%s': %s", cleaned, err))
				}
			} else if info.IsDir() {
				errs = append(errs, fmt.Errorf("'ovf_source' path '%s' is a directory; specify an OVF or OVA file", cleaned))
			}
		}

		if c.OvfSource.Username != "" || c.OvfSource.Password != "" {
			errs = append(errs, fmt.Errorf("'username' and 'password' are only applicable when 'url' is set"))
		}
		if c.OvfSource.SkipTlsVerify {
			errs = append(errs, fmt.Errorf("'skip_tls_verify' is only applicable when 'url' is set"))
		}
	}

	hasUsername := c.OvfSource.Username != ""
	hasPassword := c.OvfSource.Password != ""
	if hasUsername && !hasPassword {
		errs = append(errs, fmt.Errorf("'password' is required when 'username' is specified for OVF source"))
	}
	if hasPassword && !hasUsername {
		errs = append(errs, fmt.Errorf("'username' is required when 'password' is specified for OVF source"))
	}

	if c.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'disk_size' cannot be used with 'ovf_source'"))
	}
	if c.LinkedClone {
		errs = append(errs, fmt.Errorf("'linked_clone' cannot be used with 'ovf_source'"))
	}
	if len(c.StorageConfig.DiskControllerType) > 0 {
		errs = append(errs, fmt.Errorf("'disk_controller_type' cannot be used with 'ovf_source'"))
	}
	if len(c.StorageConfig.Storage) > 0 {
		errs = append(errs, fmt.Errorf("'storage' cannot be used with 'ovf_source'"))
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
	if s.Config.OvfSource != nil {
		return s.deployFromOvf(ctx, state)
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

	datastoreName, primaryDatastore := s.resolveDatastore(state)

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

// deployFromOvf handles deployment from OVF/OVA sources. The
// plugin reads the descriptor and archive files on the Packer host and
// uploads disk content to vSphere over an NFC lease; vSphere does not fetch
// the source directly.
func (s *StepCloneVM) deployFromOvf(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)
	vmPath := path.Join(s.Location.Folder, s.Location.VMName)

	ui.Say("Deploying virtual machine from OVF/OVA...")

	err := d.PreCleanVM(ui, vmPath, s.Force, s.Location.Cluster, s.Location.Host, s.Location.ResourcePool)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	datastoreName, _ := s.resolveDatastore(state)
	if datastoreName == "" && s.Location.DatastoreCluster == "" {
		state.Put("error", fmt.Errorf("no datastore specified and no datastore resolved from cluster"))
		return multistep.ActionHalt
	}

	var auth *driver.OvfAuthConfig
	if s.Config.OvfSource.Username != "" && s.Config.OvfSource.Password != "" {
		auth = &driver.OvfAuthConfig{
			Username: s.Config.OvfSource.Username,
			Password: s.Config.OvfSource.Password,
		}
	}

	sourceLabel := driver.SanitizeOvfSource(s.Config.OvfSource.URL, s.Config.OvfSource.Path)

	ovfConfig := &driver.OvfDeployConfig{
		URL:              s.Config.OvfSource.URL,
		Path:             s.Config.OvfSource.Path,
		Authentication:   auth,
		Name:             s.Location.VMName,
		Folder:           s.Location.Folder,
		Cluster:          s.Location.Cluster,
		Host:             s.Location.Host,
		ResourcePool:     s.Location.ResourcePool,
		Datastore:        datastoreName,
		Network:          s.Config.Network,
		MacAddress:       strings.ToLower(s.Config.MacAddress),
		Annotation:       s.Config.Notes,
		VAppProperties:   s.Config.VAppConfig.Properties,
		DeploymentOption: s.Config.VAppConfig.DeploymentOption,
		Locale:           "US",
		SkipTlsVerify:    s.Config.OvfSource.SkipTlsVerify,
	}

	// Validate OVF deployment parameters with enhanced error handling
	if err := s.validateOvfConfiguration(ctx, d, ovfConfig); err != nil {
		state.Put("error", s.wrapStepError("OVF configuration validation failed", err, sourceLabel))
		return multistep.ActionHalt
	}

	vm, err := d.DeployOvf(ctx, ovfConfig, ui)
	if err != nil {
		state.Put("error", s.wrapStepError("OVF deployment failed", err, sourceLabel))
		return multistep.ActionHalt
	}

	if vm == nil {
		state.Put("error", fmt.Errorf("OVF deployment completed but returned no virtual machine reference"))
		return multistep.ActionHalt
	}

	ui.Say("Successfully deployed virtual machine from OVF/OVA source")

	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

// resolveDatastore returns the datastore name to use for clone or OVF deploy operations.
// When StepResolveDatastore has run (for example after Storage DRS selection from
// datastore_cluster), the resolved datastore in state takes precedence over the
// location configuration.
func (s *StepCloneVM) resolveDatastore(state multistep.StateBag) (string, driver.Datastore) {
	datastoreName := s.Location.Datastore
	var resolved driver.Datastore
	if ds, ok := state.GetOk("datastore"); ok {
		resolved = ds.(driver.Datastore)
		datastoreName = resolved.Name()
	}
	return datastoreName, resolved
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
	optionsConfig := *config
	optionsConfig.Locale = locale
	options, err := d.GetOvfOptions(ctx, &optionsConfig)
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
func (s *StepCloneVM) validateOvfConfiguration(ctx context.Context, d driver.Driver, config *driver.OvfDeployConfig) error {
	if config.DeploymentOption != "" {
		log.Printf("[INFO] Validating OVF deployment option: %s", config.DeploymentOption)
		if err := s.validateOvfDeploymentOption(ctx, d, config); err != nil {
			return err
		}
	}

	if len(config.VAppProperties) > 0 {
		log.Printf("[INFO] Validating vApp property key and value limits...")
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
func (s *StepCloneVM) wrapStepError(context string, err error, source string) error {
	sanitizedErr := driver.SanitizeOvfErrorMessage(err.Error())
	return fmt.Errorf("%s for OVF source '%s': %s", context, source, sanitizedErr)
}

// Cleanup performs step cleanup.
func (s *StepCloneVM) Cleanup(state multistep.StateBag) {
	common.CleanupVM(state)
}

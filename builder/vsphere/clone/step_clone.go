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
	"github.com/vmware/govmomi/vapi/library"
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

type ContentLibrarySourceConfig struct {
	// The name of the content library containing the source item.
	Library string `mapstructure:"library"`
	// The name of the content library item. Must be unique within the content
	// library.
	//
	// HCL Example:
	//
	// ```hcl
	// content_library_source {
	//   library = "Example Content Library"
	//   name    = "example-template"
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "content_library_source": {
	//   "library": "Example Content Library",
	//   "name": "example-template"
	// }
	// ```
	Name string `mapstructure:"name"`
}

type CloneConfig struct {
	// The name of the source virtual machine template to clone.
	Template string `mapstructure:"template"`
	// Configuration for deploying from an OVF/OVA source. Specify either a
	// local `path` or a remote `url`. Refer to the [OVF source configuration](#ovf-source-configuration)
	// section for available fields.
	OvfSource *OvfSourceConfig `mapstructure:"ovf_source"`
	// Configuration for deploying from a vSphere content library item. Refer to
	// the [content library source configuration](#content-library-source-configuration)
	// section for available fields.
	ContentLibrarySource *ContentLibrarySourceConfig `mapstructure:"content_library_source"`
	// The size of the primary disk in MiB. Conflicts with `linked_clone`.
	//
	// -> **Note:** Only the primary disk size can be specified. Additional disks
	// are configured with [`storage`](#storage-configuration).
	//
	// -> **Note:** Refer to the [Source Compatibility](#source-compatibility) section.
	DiskSize int64 `mapstructure:"disk_size"`
	// Create the virtual machine as a linked clone from the latest snapshot.
	// Defaults to `false`. Conflicts with `disk_size`.
	//
	// -> **Note:** Refer to the [Source Compatibility](#source-compatibility) section.
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
	// -> **Note:** Refer to the [Source Compatibility](#source-compatibility) section.
	//
	// ~> **Note:** If more than one network resolves to the same name, either
	// the inventory path to network or an ID must be provided.
	//
	// ~> **Note:** If no network is specified, provide `host` to allow the
	// plugin to search for an available network.
	//
	// -> **Note:** For `ovf_source` and `content_library_source` OVF templates,
	// every network in the OVF descriptor maps to this network.
	//
	// -> **Note:** For `template` and `content_library_source` VM templates, this
	// is applied to the primary network adapter.
	Network string `mapstructure:"network"`
	// The network card MAC address. For example `00:50:56:00:00:00`.
	// If set, the `network` must be also specified when cloning from `template`
	// or a `content_library_source` VM template.
	//
	// -> **Note:** Refer to the [Source Compatibility](#source-compatibility) section.
	//
	// -> **Note:** For `ovf_source` and `content_library_source` OVF templates,
	// `mac_address` can be set without `network`.
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

	sources := []struct {
		name string
		used bool
	}{
		{"template", c.Template != ""},
		{"ovf_source", c.OvfSource != nil},
		{"content_library_source", c.ContentLibrarySource != nil},
	}

	usedSources := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.used {
			usedSources = append(usedSources, source.name)
		}
	}

	switch len(usedSources) {
	case 0:
		errs = append(errs, fmt.Errorf("clone source is required - specify either 'template', 'ovf_source', or 'content_library_source'"))
	case 1:
		// valid
	default:
		errs = append(errs, fmt.Errorf("cannot specify both '%s' and '%s' - choose one source type", usedSources[0], usedSources[1]))
		if len(usedSources) > 2 {
			for i := 1; i < len(usedSources)-1; i++ {
				errs = append(errs, fmt.Errorf("cannot specify both '%s' and '%s' - choose one source type", usedSources[i], usedSources[i+1]))
			}
		}
	}

	if c.OvfSource != nil {
		errs = append(errs, c.prepareOvfSource()...)
	}

	if c.ContentLibrarySource != nil {
		errs = append(errs, c.prepareContentLibrarySource()...)
	}

	errs = append(errs, c.StorageConfig.Prepare()...)

	// disk_controller_unit is validated for both `template` and
	// `content_library_source` VM templates; a `content_library_source` OVF
	// template rejects `storage` entirely once the item type is resolved at
	// deploy time (see validateContentLibrarySourceOptions).
	if c.Template != "" || c.ContentLibrarySource != nil {
		errs = append(errs, c.prepareStorage()...)
	}

	if c.LinkedClone && c.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'linked_clone' and 'disk_size' cannot be used together"))
	}

	if c.MacAddress != "" && c.Network == "" && c.OvfSource == nil && c.ContentLibrarySource == nil {
		errs = append(errs, fmt.Errorf("'network' is required when 'mac_address' is specified"))
	}

	if c.Template != "" && c.VAppConfig.DeploymentOption != "" {
		errs = append(errs, fmt.Errorf("'vapp.deployment_option' cannot be used with 'template'"))
	}

	return errs
}

func (c *CloneConfig) prepareContentLibrarySource() []error {
	var errs []error

	if c.ContentLibrarySource.Library == "" {
		errs = append(errs, fmt.Errorf("'library' is required when using 'content_library_source'"))
	}
	if c.ContentLibrarySource.Name == "" {
		errs = append(errs, fmt.Errorf("'name' is required when using 'content_library_source'"))
	}
	if c.LinkedClone {
		errs = append(errs, fmt.Errorf("'linked_clone' cannot be used with 'content_library_source'"))
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
	if hasURL {
		if hasUsername && !hasPassword {
			errs = append(errs, fmt.Errorf("'password' is required when 'username' is specified for OVF source"))
		}
		if hasPassword && !hasUsername {
			errs = append(errs, fmt.Errorf("'username' is required when 'password' is specified for OVF source"))
		}
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
	switch {
	case s.Config.ContentLibrarySource != nil:
		return s.deployFromContentLibrary(ctx, state)
	case s.Config.OvfSource != nil:
		return s.deployFromOvf(ctx, state)
	default:
		return s.cloneFromTemplate(ctx, state)
	}
}

// disksFromStorageConfig converts the configured storage blocks into driver.Disk
// values, preserving both the legacy disk_controller_index and explicit
// disk_controller_unit addressing so callers don't have to duplicate (and risk
// diverging on) this mapping. Storage policy names are resolved to profile UUIDs.
func disksFromStorageConfig(d driver.Driver, storage []common.DiskConfig) ([]driver.Disk, error) {
	var disks []driver.Disk
	for _, disk := range storage {
		dd := driver.Disk{
			DiskSize:            disk.DiskSize,
			DiskEagerlyScrub:    disk.DiskEagerlyScrub,
			DiskThinProvisioned: disk.DiskThinProvisioned,
			ControllerIndex:     disk.DiskControllerIndex,
			ControllerUnit:      disk.DiskControllerUnit,
		}
		if disk.StoragePolicyName != "" {
			id, err := d.FindStoragePolicyID(disk.StoragePolicyName)
			if err != nil {
				return nil, fmt.Errorf("error resolving storage policy %q: %v", disk.StoragePolicyName, err)
			}
			dd.StoragePolicyID = id
		}
		disks = append(disks, dd)
	}
	return disks, nil
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
	disks, err := disksFromStorageConfig(d, s.Config.StorageConfig.Storage)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	templateDevices, err := template.Devices()
	if err != nil {
		state.Put("error", fmt.Errorf("error reading template devices: %s", err))
		return multistep.ActionHalt
	}

	datastoreName, primaryDatastore := s.resolveDatastore(state)

	// Handle multi-disk placement when using a datastore cluster.
	placementInput := driver.StoragePlacementInput{
		StorageConfig: driver.StorageConfig{
			DiskControllerType: s.Config.StorageConfig.DiskControllerType,
			Storage:            disks,
		},
		ExistingDevices: driver.StorageExistingDevices(templateDevices),
		PrimaryDiskSize: s.Config.DiskSize,
	}
	datastoreName, diskDatastores := common.ResolveMultiDiskDatastorePlacement(
		ui, d, s.Location.DatastoreCluster, placementInput, primaryDatastore, datastoreName,
	)

	if len(diskDatastores) == 0 {
		var resolveErr error
		datastoreName, diskDatastores, resolveErr = common.ResolveStoragePolicyDatastorePlacement(
			d, s.Location.Host, s.Location.Cluster, disks, primaryDatastore, datastoreName, s.Location.Datastore, s.Location.DatastoreCluster, common.StoragePolicyIDFromState(state),
		)
		if resolveErr != nil {
			state.Put("error", resolveErr)
			return multistep.ActionHalt
		}
	}

	if datastoreName == "" && s.Location.DatastoreCluster == "" {
		state.Put("error", fmt.Errorf("no datastore specified and no datastore resolved from cluster or storage policy"))
		return multistep.ActionHalt
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
			DiskDatastores:     diskDatastores,
		},
	})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	if vm == nil {
		state.Put("error", fmt.Errorf("clone operation returned no VM and no error"))
		return multistep.ActionHalt
	}
	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

// deployFromContentLibrary handles deployment from a vSphere content library item.
// vCenter deploys the item directly; Packer does not read OVF files from the library.
func (s *StepCloneVM) deployFromContentLibrary(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	d := state.Get("driver").(driver.Driver)
	vmPath := path.Join(s.Location.Folder, s.Location.VMName)
	source := s.Config.ContentLibrarySource

	err := d.PreCleanVM(ui, vmPath, s.Force, s.Location.Cluster, s.Location.Host, s.Location.ResourcePool)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	datastoreName, primaryDatastore := s.resolveDatastore(state)

	item, err := d.ResolveContentLibraryItem(source.Library, source.Name)
	if err != nil {
		state.Put("error", fmt.Errorf("error resolving content library source: %s", err))
		return multistep.ActionHalt
	}

	if err := s.validateContentLibrarySourceOptions(item); err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	disks, err := disksFromStorageConfig(d, s.Config.StorageConfig.Storage)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var diskDatastores []driver.DiskDatastore
	if item.Type == library.ItemTypeVMTX {
		placementInput := driver.StoragePlacementInput{
			StorageConfig: driver.StorageConfig{
				DiskControllerType: s.Config.StorageConfig.DiskControllerType,
				Storage:            disks,
			},
			PrimaryDiskSize: s.Config.DiskSize,
		}
		datastoreName, diskDatastores = common.ResolveMultiDiskDatastorePlacement(
			ui, d, s.Location.DatastoreCluster, placementInput, primaryDatastore, datastoreName,
		)
	}

	if len(diskDatastores) == 0 {
		var resolveErr error
		datastoreName, diskDatastores, resolveErr = common.ResolveStoragePolicyDatastorePlacement(
			d, s.Location.Host, s.Location.Cluster, disks, primaryDatastore, datastoreName, s.Location.Datastore, s.Location.DatastoreCluster, common.StoragePolicyIDFromState(state),
		)
		if resolveErr != nil {
			state.Put("error", resolveErr)
			return multistep.ActionHalt
		}
	}

	if datastoreName == "" && s.Location.DatastoreCluster == "" {
		state.Put("error", fmt.Errorf("no datastore specified and no datastore resolved from cluster or storage policy"))
		return multistep.ActionHalt
	}

	deployConfig := &driver.ContentLibraryDeployConfig{
		Item:             item,
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
		PrimaryDiskSize:  s.Config.DiskSize,
		StorageConfig: driver.StorageConfig{
			DiskControllerType: s.Config.StorageConfig.DiskControllerType,
			Storage:            disks,
			DiskDatastores:     diskDatastores,
		},
	}

	vm, err := d.DeployContentLibraryItem(ctx, deployConfig, ui)
	if err != nil {
		state.Put("error", fmt.Errorf("content library deployment failed for '%s/%s': %s",
			source.Library, source.Name, err))
		return multistep.ActionHalt
	}

	if vm == nil {
		state.Put("error", fmt.Errorf("content library deployment completed but returned no virtual machine reference"))
		return multistep.ActionHalt
	}

	ui.Say("Successfully deployed virtual machine from content library source")

	if s.Config.Destroy {
		state.Put("destroy_vm", s.Config.Destroy)
	}
	state.Put("vm", vm)
	return multistep.ActionContinue
}

func (s *StepCloneVM) validateContentLibrarySourceOptions(item *library.Item) error {
	if item.Type == library.ItemTypeVMTX {
		if s.Config.MacAddress != "" && s.Config.Network == "" {
			return fmt.Errorf("'network' is required when 'mac_address' is specified")
		}
		if s.Config.VAppConfig.DeploymentOption != "" {
			return fmt.Errorf("'vapp.deployment_option' cannot be used with VM template content library items")
		}
		return nil
	}

	if item.Type != library.ItemTypeOVF {
		return nil
	}

	var errs []error
	if s.Config.DiskSize != 0 {
		errs = append(errs, fmt.Errorf("'disk_size' cannot be used with OVF content library items"))
	}
	if len(s.Config.StorageConfig.DiskControllerType) > 0 {
		errs = append(errs, fmt.Errorf("'disk_controller_type' cannot be used with OVF content library items"))
	}
	if len(s.Config.StorageConfig.Storage) > 0 {
		errs = append(errs, fmt.Errorf("'storage' cannot be used with OVF content library items"))
	}

	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("%s (and %d more errors)", errs[0], len(errs)-1)
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

	options, err := d.GetOvfOptions(ctx, config)
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
func (s *StepCloneVM) wrapStepError(errContext string, err error, source string) error {
	sanitizedErr := driver.SanitizeOvfErrorMessage(err.Error())
	return fmt.Errorf("%s for OVF source '%s': %s", errContext, source, sanitizedErr)
}

// Cleanup performs step cleanup.
func (s *StepCloneVM) Cleanup(state multistep.StateBag) {
	common.CleanupVM(state)
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ContentLibraryDestinationConfig
package common

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// Create a content library item in a content library whose content is a VM
// template or an OVF template created from the virtual machine image after
// the build is complete.
//
// The template is stored in an existing or newly created library item.
type ContentLibraryDestinationConfig struct {
	// The name of the content library in which the new content library item
	// containing the template will be created or updated. The content library
	// must be of type Local to allow deploying virtual machines.
	Library string `mapstructure:"library"`
	// The name of the content library item that will be created or updated.
	//
	// For VM templates, the default is [vm_name](#vm_name) + timestamp when not set.
	// VM templates are always imported as new library items. If an item with the
	// specified name already exists, the import will fail unless [overwrite](#overwrite)
	// is set to `true`.
	//
	// For OVF templates, the name defaults to [vm_name](#vm_name) when not set.
	// If an item with the same name already exists, it will be updated with the
	// new OVF template, otherwise a new item will be created.
	//
	// ~> **Note:** VM templates cannot update existing content library items in-place
	// like OVF templates. When [overwrite](#overwrite) is enabled, the existing item
	// will be deleted before creating the new one.
	Name string `mapstructure:"name"`
	// A description for the content library item that will be created.
	// Defaults to "Packer imported [vm_name](#vm_name) VM template".
	Description string `mapstructure:"description"`
	// The cluster where the VM template will be placed.
	// If `cluster` and `resource_pool` are both specified, `resource_pool` must
	// belong to cluster. If `cluster` and `host` are both specified, the ESX
	// host must be a member of the cluster. This option is not used when
	// importing OVF templates. Defaults to [`cluster`](#cluster).
	Cluster string `mapstructure:"cluster"`
	// The virtual machine folder where the VM template will be placed.
	// This option is not used when importing OVF templates. Defaults to
	// the same folder as the source virtual machine.
	Folder string `mapstructure:"folder"`
	// The ESX host where the virtual machine template will be placed.
	// If `host` and `resource_pool` are both specified, `resource_pool` must
	// belong to host. If `host` and `cluster` are both specified, `host` must
	// be a member of the cluster. This option is not used when importing OVF
	// templates. Defaults to [`host`](#host).
	Host string `mapstructure:"host"`
	// The resource pool where the virtual machine template will be placed.
	// Defaults to [`resource_pool`](#resource_pool). If [`resource_pool`](#resource_pool)
	// is unset, the system will attempt to choose a suitable resource pool
	// for the VM template.
	ResourcePool string `mapstructure:"resource_pool"`
	// The datastore for the virtual machine template's configuration and log
	// files. This option is not used when importing OVF templates.
	// Defaults to the storage backing associated with the content library.
	Datastore string `mapstructure:"datastore"`
	// Destroy the virtual machine after the import to the content library.
	// Defaults to `false`.
	Destroy bool `mapstructure:"destroy"`
	// Import an OVF template to the content library item. Defaults to `false`.
	Ovf bool `mapstructure:"ovf"`
	// Skip the import to the content library item. Useful during a build test
	// stage. Defaults to `false`.
	SkipImport bool `mapstructure:"skip_import"`
	// Flags to use for OVF package creation. The supported flags can be
	// obtained using ExportFlag.list. If unset, no flags will be used.
	// Known values: `EXTRA_CONFIG`, `PRESERVE_MAC`.
	OvfFlags []string `mapstructure:"ovf_flags"`
	// Overwrite the content library item if it already exists. This only applies
	// to VM templates. For OVF templates, existing items are always updated.
	// When enabled for VM templates, the existing item will be deleted before
	// creating the new one. Defaults to `false`.
	//
	// **VM Template**
	//
	// HCL Example:
	//
	// ```hcl
	// content_library_destination {
	//   library = "Example Content Library"
	//   overwrite = true
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "content_library_destination": {
	//   "library": "Example Content Library",
	//   "overwrite": true
	// }
	// ```
	//
	// **OVF Template**
	//
	// HCL Example:
	//
	// ```hcl
	// content_library_destination {
	//   library = "Example Content Library"
	//   ovf     = true
	// }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// "content_library_destination": {
	//   "library": "Example Content Library",
	//   "ovf": true
	// }
	// ```
	Overwrite bool `mapstructure:"overwrite"`
}

func (c *ContentLibraryDestinationConfig) Prepare(lc *LocationConfig) []error {
	var errs *packersdk.MultiError

	if c.Library == "" {
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("a library name must be provided"))
	}

	if c.Overwrite {
		if c.Ovf {
			errs = packersdk.MultiErrorAppend(errs,
				fmt.Errorf("overwrite is only supported for VM template imports (set ovf to false)"))
		}
		if c.Name == "" {
			errs = packersdk.MultiErrorAppend(errs,
				fmt.Errorf("overwrite requires content_library_destination.name to be set"))
		}
	}

	if c.Ovf {
		if c.Name == "" {
			c.Name = lc.VMName
		}
	} else {

		if c.Name == "" {
			// Add timestamp to the name to differentiate from the original VM
			// otherwise vSphere won't be able to create the template which will be imported
			name, err := interpolate.Render(lc.VMName+"{{timestamp}}", nil)
			if err != nil {
				errs = packersdk.MultiErrorAppend(errs,
					fmt.Errorf("unable to parse content library VM template name: %s", err))
			}
			c.Name = name
		}
		if c.Cluster == "" {
			c.Cluster = lc.Cluster
		}
		if c.Host == "" {
			c.Host = lc.Host
		}
		if c.ResourcePool == "" {
			c.ResourcePool = lc.ResourcePool
		}

		if c.Name == lc.VMName {
			errs = packersdk.MultiErrorAppend(errs,
				fmt.Errorf("content_library_destination.name must differ from vm_name for VM template imports"))
		}
	}
	if c.Description == "" {
		c.Description = fmt.Sprintf("Packer imported %s VM template", lc.VMName)
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs.Errors
	}

	return nil
}

type StepImportToContentLibrary struct {
	ContentLibConfig *ContentLibraryDestinationConfig
}

func (s *StepImportToContentLibrary) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	if s.ContentLibConfig.SkipImport {
		ui.Say("Skipping import...")
		return multistep.ActionContinue
	}

	vm := state.Get("vm").(*driver.VirtualMachineDriver)
	var err error

	ui.Say("Clearing boot order...")
	err = vm.SetBootOrder([]string{"-"})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	vmTypeLabel := "VM"
	if s.ContentLibConfig.Ovf {
		vmTypeLabel = "VM OVF"
	}
	ui.Sayf("Importing %s template %s to Content Library '%s' as the item '%s' with the description '%s'...",
		vmTypeLabel, s.ContentLibConfig.Name, s.ContentLibConfig.Library, s.ContentLibConfig.Name, s.ContentLibConfig.Description)

	if s.ContentLibConfig.Ovf {
		err = s.importOvfTemplate(vm)
	} else {
		err = s.importVmTemplate(vm)
	}

	if err != nil {
		ui.Errorf("Failed to import template %s: %s", s.ContentLibConfig.Name, err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Add a tracer to the state to track if the Destroy parameter was used.
	if s.ContentLibConfig.Destroy {
		state.Put("destroy_vm", s.ContentLibConfig.Destroy)
	}

	// For HCP Packer metadata, save the content library item UUID in state.
	itemUuid, err := vm.FindContentLibraryItemUUID(s.ContentLibConfig.Library, s.ContentLibConfig.Name)
	if err != nil {
		ui.Errorf("Failed to get content library item uuid: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("content_library_item_uuid", itemUuid)

	// For HCP Packer metadata, save the content library datastore name in state.
	datastores, err := vm.FindContentLibraryTemplateDatastoreName(s.ContentLibConfig.Library)
	if err != nil {
		ui.Errorf("Failed to get content library datastore name: %s", err)
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("content_library_datastore", datastores)

	return multistep.ActionContinue
}

func (s *StepImportToContentLibrary) importOvfTemplate(vm *driver.VirtualMachineDriver) error {
	ovf := vcenter.OVF{
		Spec: vcenter.CreateSpec{
			Name:        s.ContentLibConfig.Name,
			Description: s.ContentLibConfig.Description,
			Flags:       s.ContentLibConfig.OvfFlags,
		},
		Target: vcenter.LibraryTarget{
			LibraryID: s.ContentLibConfig.Library,
		},
	}
	return vm.ImportOvfToContentLibrary(ovf)
}

func (s *StepImportToContentLibrary) importVmTemplate(vm *driver.VirtualMachineDriver) error {
	existingItem, err := vm.FindContentLibraryItem(s.ContentLibConfig.Library, s.ContentLibConfig.Name)
	if err == nil {
		// Item exists
		if !s.ContentLibConfig.Overwrite {
			return fmt.Errorf("content library item '%s' already exists; set overwrite to true to replace it", s.ContentLibConfig.Name)
		}
		log.Printf("Deleting existing content library item '%s' before re-import", s.ContentLibConfig.Name)
		if err := vm.DeleteContentLibraryItem(existingItem.ID); err != nil {
			return fmt.Errorf("failed to delete existing content library item: %s", err)
		}
	}
	// If err != nil the item was not found, which is the normal case — continue.

	template := vcenter.Template{
		Name:        s.ContentLibConfig.Name,
		Description: s.ContentLibConfig.Description,
		Library:     s.ContentLibConfig.Library,
		Placement: &vcenter.Placement{
			Cluster:      s.ContentLibConfig.Cluster,
			Folder:       s.ContentLibConfig.Folder,
			Host:         s.ContentLibConfig.Host,
			ResourcePool: s.ContentLibConfig.ResourcePool,
		},
	}

	if s.ContentLibConfig.Datastore != "" {
		template.VMHomeStorage = &vcenter.DiskStorage{
			Datastore: s.ContentLibConfig.Datastore,
		}
	}

	return vm.ImportToContentLibrary(template)
}

func (s *StepImportToContentLibrary) Cleanup(multistep.StateBag) {
}

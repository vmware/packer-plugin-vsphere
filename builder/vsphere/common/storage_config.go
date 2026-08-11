// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type StorageConfig,DiskConfig

package common

import (
	"fmt"
)

type DiskConfig struct {
	// The size of the disk in MiB.
	DiskSize int64 `mapstructure:"disk_size" required:"true"`
	// Enable thin provisioning for the disk.
	// Defaults to `false`.
	DiskThinProvisioned bool `mapstructure:"disk_thin_provisioned"`
	// Enable eager scrubbing for the disk.
	// Defaults to `false`.
	DiskEagerlyScrub bool `mapstructure:"disk_eagerly_scrub"`
	// The assigned disk controller for the disk.
	// Defaults to the first controller, `(0)`.
	// Mutually exclusive with `disk_controller_unit`.
	DiskControllerIndex int `mapstructure:"disk_controller_index"`
	// Explicit controller address for the disk when cloning from a `template`
	// or deploying a VM template `content_library_source`.
	// Format: `{type}{bus}:{unit}` such as `scsi0:1`, `nvme0:0`, or `sata0:2`.
	// Only supported by the `vsphere-clone` builder. Mutually exclusive with
	// `disk_controller_index`.
	DiskControllerUnit string `mapstructure:"disk_controller_unit"`
	// The name of the storage policy to apply to the disk. The storage policy
	// must already exist on the vCenter instance. If not specified, the default
	// storage policy of the target `datastore` or `datastore_cluster` is used.
	//
	// -> **Note:** When `datastore` and `datastore_cluster` are omitted, each
	// configured `storage_policy` is used to select a compliant datastore for
	// that disk and the first policy is used to place the virtual machine's home
	// files.
	StoragePolicyName string `mapstructure:"storage_policy"`
}

// Disk layout depends on the builder and clone source. For `vsphere-clone`,
// refer to the [Source Compatibility](#source-compatibility) section.
//
// When the source is an OVF descriptor (`ovf_source` or a
// `content_library_source` OVF template) and multiple deployment sizes are
// offered, use `vapp.deployment_option`. For more information, refer to the
// [vApp Options Configuration](#vapp-options-configuration) section.
//
// -> **Note:** Use `datastore` or `datastore_cluster` in the
// [Location Configuration](#location-configuration) to choose where imported
// disks are stored.
type StorageConfig struct {
	// The disk controller type. One of `lsilogic`, `lsilogic-sas`, `pvscsi`,
	// `nvme`, `scsi`, or `sata`. Defaults to `lsilogic`. Use a list to define
	// additional controllers. Refer to [SCSI, SATA, and NVMe Storage Controller
	// Conditions, Limitations, and Compatibility](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/scsi-controller-configurationvsphere-vm-admin.html)
	// for additional information.
	DiskControllerType []string `mapstructure:"disk_controller_type"`
	// Additional disks to attach to the virtual machine. Each `storage` block
	// defines one disk. Does not resize the primary disk; use `disk_size` for
	// that.
	Storage []DiskConfig `mapstructure:"storage"`
}

// FirstStoragePolicyName returns the first non-empty storage_policy across
// storage blocks. Used for VM-home / primary placement when no datastore is set.
func (c *StorageConfig) FirstStoragePolicyName() string {
	for _, storage := range c.Storage {
		if storage.StoragePolicyName != "" {
			return storage.StoragePolicyName
		}
	}
	return ""
}

func (c *StorageConfig) Prepare() []error {
	var errs []error

	if len(c.Storage) > 0 {
		for i, storage := range c.Storage {
			if storage.DiskSize == 0 {
				errs = append(errs, fmt.Errorf("storage[%d].'disk_size' is required", i))
			}
			if storage.DiskControllerUnit == "" && storage.DiskControllerIndex >= len(c.DiskControllerType) {
				errs = append(errs, fmt.Errorf("storage[%d].'disk_controller_index' references an unknown disk controller", i))
			}
		}
	}

	return errs
}

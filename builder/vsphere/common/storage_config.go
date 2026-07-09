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
	DiskControllerIndex int `mapstructure:"disk_controller_index"`
}

// When cloning from a `template`, the resulting virtual machine contains the
// source template's disks plus any newly configured disks and controllers.
// `storage {}`, `disk_controller_type`, and `disk_size` apply in this mode.
//
// When deploying from `ovf_source` from either an HTTP(S) `url` or a local
// filesystem `path`, the source OVF/OVA descriptor defines the configured disks
// and controllers. When the descriptor offers multiple deployment sizes,
// use `vapp.deployment_option` to select one. For more information, refer to
// the [vApp Options Configuration](#vapp-options-configuration) section.
//
// ~> **Note:** `storage {}`, `disk_controller_type`, and `disk_size` cannot be
// used with `ovf_source`.
//
// ~> **Note:** Use `datastore` or `datastore_cluster` in the
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
	// that when cloning from a `template`.
	Storage []DiskConfig `mapstructure:"storage"`
}

func (c *StorageConfig) Prepare() []error {
	var errs []error

	if len(c.Storage) > 0 {
		for i, storage := range c.Storage {
			if storage.DiskSize == 0 {
				errs = append(errs, fmt.Errorf("storage[%d].'disk_size' is required", i))
			}
			if storage.DiskControllerIndex >= len(c.DiskControllerType) {
				errs = append(errs, fmt.Errorf("storage[%d].'disk_controller_index' references an unknown disk controller", i))
			}
		}
	}

	return errs
}

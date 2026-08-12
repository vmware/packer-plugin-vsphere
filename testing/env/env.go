// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package env

import (
	"os"
	"strings"
)

// Default variable values for acceptance tests.

const (
	DefaultVcenterServer        = "vc01.example.com"
	DefaultVsphereUsername      = "administrator@vsphere.local"
	DefaultVspherePassword      = "VMw@re1!"
	DefaultDatacenter           = "dc01"
	DefaultVsphereHost          = "esx01.example.com"
	DefaultVsphereHostSecondary = "esx01.example.com"
	DefaultDatastore            = "nfs"
	DefaultDatastoreCluster     = "nfs-dsc01"
	DefaultCluster              = "cl01"
	DefaultClusterDRS           = "cl01"
	DefaultFolder               = "acc-test-fd01"
	DefaultResourcePool         = "acc-test-rp01"
	DefaultNetwork              = "VM Network"
	DefaultTemplate             = "alpine-vm-template"
	DefaultISOPath              = "[iso] iso/linux/alpine/3/amd64/alpine-standard-3.23.4-x86_64.iso"
	DefaultContentLibrary       = "lib01"
	DefaultContentLibraryVMTX   = "alpine-vm-template"
	DefaultContentLibraryOVF    = "alpine-ovf-template"
	DefaultTagCategory          = "color"
	DefaultTagA                 = "blue"
	DefaultTagB                 = "red"
	DefaultStoragePolicyA       = "blue"
	DefaultStoragePolicyB       = "green"
	DefaultStoragePolicyC       = "" // Optional; set VSPHERE_STORAGE_POLICY_C (e.g. red) for a third disk.
	DefaultNotes                = "Built by Acceptance Tests"
	DefaultOVFURL               = "" // OVF/OVA HTTPS URLs; Acceptance test skip when unset.
	DefaultOVAURL               = "" // OVF/OVA HTTPS URLs; Acceptance test skip when unset.
	DefaultOVFUsername          = "" // OVF/OVA HTTPS Username; Acceptance test skip when unset.
	DefaultOVFPassword          = "" // OVF/OVA HTTPS Password; Acceptance test skip when unset.
)

// Environment variable names for acceptance tests overrides.

const (
	VcenterServer        = "VSPHERE_VCENTER_SERVER"
	VsphereUsername      = "VSPHERE_USERNAME"
	VspherePassword      = "VSPHERE_PASSWORD"
	Datacenter           = "VSPHERE_DATACENTER"
	VsphereHost          = "VSPHERE_HOST"
	VsphereHostSecondary = "VSPHERE_HOST_SECONDARY"
	Datastore            = "VSPHERE_DATASTORE"
	DatastoreCluster     = "VSPHERE_DATASTORE_CLUSTER"
	Cluster              = "VSPHERE_CLUSTER"
	ClusterDRS           = "VSPHERE_CLUSTER_DRS"
	Folder               = "VSPHERE_FOLDER"
	ResourcePool         = "VSPHERE_RESOURCE_POOL"
	Network              = "VSPHERE_NETWORK"
	Template             = "VSPHERE_TEMPLATE"
	ISOPath              = "VSPHERE_ISO_PATH"
	ContentLibrary       = "VSPHERE_CONTENT_LIBRARY"
	ContentLibraryVMTX   = "VSPHERE_CL_VMTX_ITEM"
	ContentLibraryOVF    = "VSPHERE_CL_OVF_ITEM"
	TagCategory          = "VSPHERE_TAG_CATEGORY"
	TagA                 = "VSPHERE_TAG_A"
	TagB                 = "VSPHERE_TAG_B"
	StoragePolicyA       = "VSPHERE_STORAGE_POLICY_A"
	StoragePolicyB       = "VSPHERE_STORAGE_POLICY_B"
	StoragePolicyC       = "VSPHERE_STORAGE_POLICY_C"
	Notes                = "VSPHERE_NOTES"
	OVFURL               = "VSPHERE_OVF_URL"
	OVAURL               = "VSPHERE_OVA_URL"
	OVFUsername          = "VSPHERE_OVF_USERNAME"
	OVFPassword          = "VSPHERE_OVF_PASSWORD"
	OVFSkipTLSVerify     = "VSPHERE_OVF_SKIP_TLS_VERIFY"
	PackerAcc            = "PACKER_ACC"
)

// Truthy reports whether an environment variable is a common truthy value
// (1, true, yes, on). Empty or other values are false.
func Truthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetenvOrDefault returns the value of the environment variable named by key,
// or defaultValue when the variable is unset or empty.
func GetenvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

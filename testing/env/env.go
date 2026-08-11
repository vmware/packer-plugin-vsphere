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
	EnvVcenterServer        = "VSPHERE_VCENTER_SERVER"
	EnvVsphereUsername      = "VSPHERE_USERNAME"
	EnvVspherePassword      = "VSPHERE_PASSWORD"
	EnvDatacenter           = "VSPHERE_DATACENTER"
	EnvVsphereHost          = "VSPHERE_HOST"
	EnvVsphereHostSecondary = "VSPHERE_HOST_SECONDARY"
	EnvDatastore            = "VSPHERE_DATASTORE"
	EnvDatastoreCluster     = "VSPHERE_DATASTORE_CLUSTER"
	EnvCluster              = "VSPHERE_CLUSTER"
	EnvClusterDRS           = "VSPHERE_CLUSTER_DRS"
	EnvFolder               = "VSPHERE_FOLDER"
	EnvResourcePool         = "VSPHERE_RESOURCE_POOL"
	EnvNetwork              = "VSPHERE_NETWORK"
	EnvTemplate             = "VSPHERE_TEMPLATE"
	EnvISOPath              = "VSPHERE_ISO_PATH"
	EnvContentLibrary       = "VSPHERE_CONTENT_LIBRARY"
	EnvContentLibraryVMTX   = "VSPHERE_CL_VMTX_ITEM"
	EnvContentLibraryOVF    = "VSPHERE_CL_OVF_ITEM"
	EnvTagCategory          = "VSPHERE_TAG_CATEGORY"
	EnvTagA                 = "VSPHERE_TAG_A"
	EnvTagB                 = "VSPHERE_TAG_B"
	EnvStoragePolicyA       = "VSPHERE_STORAGE_POLICY_A"
	EnvStoragePolicyB       = "VSPHERE_STORAGE_POLICY_B"
	EnvStoragePolicyC       = "VSPHERE_STORAGE_POLICY_C"
	EnvNotes                = "VSPHERE_NOTES"
	EnvOVFURL               = "VSPHERE_OVF_URL"
	EnvOVAURL               = "VSPHERE_OVA_URL"
	EnvOVFUsername          = "VSPHERE_OVF_USERNAME"
	EnvOVFPassword          = "VSPHERE_OVF_PASSWORD"
	EnvOVFSkipTLSVerify     = "VSPHERE_OVF_SKIP_TLS_VERIFY"
	EnvPackerAcc            = "PACKER_ACC"
)

// EnvTruthy reports whether an environment variable is a common truthy value
// (1, true, yes, on). Empty or other values are false.
func EnvTruthy(key string) bool {
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

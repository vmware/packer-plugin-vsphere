// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package env

import (
	"testing"
)

func TestGetenvOrDefault(t *testing.T) {
	const key = "PACKER_PLUGIN_VSPHERE_TEST_ENV_KEY"
	t.Setenv(key, "")
	if got := GetenvOrDefault(key, "fallback"); got != "fallback" {
		t.Fatalf("empty env: got %q, want fallback", got)
	}

	t.Setenv(key, "set-value")
	if got := GetenvOrDefault(key, "fallback"); got != "set-value" {
		t.Fatalf("set env: got %q, want set-value", got)
	}
}

func TestAccFromEnv_defaults(t *testing.T) {
	clearAccEnv(t)

	acc := AccFromEnv()
	checks := map[string]string{
		"VCenterServer":      DefaultVcenterServer,
		"Username":           DefaultVsphereUsername,
		"Password":           DefaultVspherePassword,
		"Datacenter":         DefaultDatacenter,
		"Host":               DefaultVsphereHost,
		"HostSecondary":      DefaultVsphereHostSecondary,
		"Datastore":          DefaultDatastore,
		"DatastoreCluster":   DefaultDatastoreCluster,
		"Cluster":            DefaultCluster,
		"ClusterDRS":         DefaultClusterDRS,
		"Folder":             DefaultFolder,
		"ResourcePool":       DefaultResourcePool,
		"Network":            DefaultNetwork,
		"Template":           DefaultTemplate,
		"ISOPath":            DefaultISOPath,
		"ContentLibrary":     DefaultContentLibrary,
		"ContentLibraryVMTX": DefaultContentLibraryVMTX,
		"ContentLibraryOVF":  DefaultContentLibraryOVF,
		"TagCategory":        DefaultTagCategory,
		"TagA":               DefaultTagA,
		"TagB":               DefaultTagB,
		"StoragePolicyA":     DefaultStoragePolicyA,
		"StoragePolicyB":     DefaultStoragePolicyB,
		"StoragePolicyC":     DefaultStoragePolicyC,
		"Notes":              DefaultNotes,
		"OVFURL":             DefaultOVFURL,
		"OVAURL":             DefaultOVAURL,
		"OVFUsername":        DefaultOVFUsername,
		"OVFPassword":        DefaultOVFPassword,
	}

	got := map[string]string{
		"VCenterServer":      acc.VCenterServer,
		"Username":           acc.Username,
		"Password":           acc.Password,
		"Datacenter":         acc.Datacenter,
		"Host":               acc.Host,
		"HostSecondary":      acc.HostSecondary,
		"Datastore":          acc.Datastore,
		"DatastoreCluster":   acc.DatastoreCluster,
		"Cluster":            acc.Cluster,
		"ClusterDRS":         acc.ClusterDRS,
		"Folder":             acc.Folder,
		"ResourcePool":       acc.ResourcePool,
		"Network":            acc.Network,
		"Template":           acc.Template,
		"ISOPath":            acc.ISOPath,
		"ContentLibrary":     acc.ContentLibrary,
		"ContentLibraryVMTX": acc.ContentLibraryVMTX,
		"ContentLibraryOVF":  acc.ContentLibraryOVF,
		"TagCategory":        acc.TagCategory,
		"TagA":               acc.TagA,
		"TagB":               acc.TagB,
		"StoragePolicyA":     acc.StoragePolicyA,
		"StoragePolicyB":     acc.StoragePolicyB,
		"StoragePolicyC":     acc.StoragePolicyC,
		"Notes":              acc.Notes,
		"OVFURL":             acc.OVFURL,
		"OVAURL":             acc.OVAURL,
		"OVFUsername":        acc.OVFUsername,
		"OVFPassword":        acc.OVFPassword,
	}

	for name, want := range checks {
		if got[name] != want {
			t.Errorf("%s: got %q, want %q", name, got[name], want)
		}
	}
	if acc.OVFSkipTLSVerify {
		t.Errorf("OVFSkipTLSVerify: got true, want false by default")
	}
	if got := acc.StoragePolicies(); len(got) != 2 || got[0] != DefaultStoragePolicyA || got[1] != DefaultStoragePolicyB {
		t.Errorf("StoragePolicies defaults: got %#v", got)
	}
}

func TestAccFromEnv_overrides(t *testing.T) {
	clearAccEnv(t)
	t.Setenv(EnvVcenterServer, "vc-override.example.com")
	t.Setenv(EnvDatacenter, "dc-override")
	t.Setenv(EnvDatastore, "ds-override")
	t.Setenv(EnvDatastoreCluster, "dsc-override")
	t.Setenv(EnvFolder, "fd-override")
	t.Setenv(EnvResourcePool, "rp-override")
	t.Setenv(EnvNetwork, "net-override")
	t.Setenv(EnvTemplate, "tmpl-override")
	t.Setenv(EnvContentLibrary, "lib-override")
	t.Setenv(EnvContentLibraryVMTX, "vmtx-override")
	t.Setenv(EnvContentLibraryOVF, "ovf-override")
	t.Setenv(EnvISOPath, "[ds-override] ISO/custom.iso")
	t.Setenv(EnvTagCategory, "cat-override")
	t.Setenv(EnvTagA, "tag-blue-override")
	t.Setenv(EnvTagB, "tag-red-override")
	t.Setenv(EnvStoragePolicyA, "policy-a-override")
	t.Setenv(EnvStoragePolicyB, "policy-b-override")
	t.Setenv(EnvStoragePolicyC, "policy-c-override")
	t.Setenv(EnvNotes, "notes-override")
	t.Setenv(EnvOVFURL, "https://artifacts.example.com/alpine.ovf")
	t.Setenv(EnvOVAURL, "https://artifacts.example.com/alpine.ova")
	t.Setenv(EnvOVFUsername, "ovf-user")
	t.Setenv(EnvOVFPassword, "ovf-pass")
	t.Setenv(EnvOVFSkipTLSVerify, "true")

	acc := AccFromEnv()
	if acc.VCenterServer != "vc-override.example.com" {
		t.Fatalf("VCenterServer: got %q", acc.VCenterServer)
	}
	if acc.Datacenter != "dc-override" {
		t.Fatalf("Datacenter: got %q", acc.Datacenter)
	}
	if acc.Datastore != "ds-override" {
		t.Fatalf("Datastore: got %q", acc.Datastore)
	}
	if acc.DatastoreCluster != "dsc-override" {
		t.Fatalf("DatastoreCluster: got %q", acc.DatastoreCluster)
	}
	if acc.Folder != "fd-override" {
		t.Fatalf("Folder: got %q", acc.Folder)
	}
	if acc.ResourcePool != "rp-override" {
		t.Fatalf("ResourcePool: got %q", acc.ResourcePool)
	}
	if acc.Network != "net-override" {
		t.Fatalf("Network: got %q", acc.Network)
	}
	if acc.Template != "tmpl-override" {
		t.Fatalf("Template: got %q", acc.Template)
	}
	if acc.ContentLibrary != "lib-override" {
		t.Fatalf("ContentLibrary: got %q", acc.ContentLibrary)
	}
	if acc.ContentLibraryVMTX != "vmtx-override" {
		t.Fatalf("ContentLibraryVMTX: got %q", acc.ContentLibraryVMTX)
	}
	if acc.ContentLibraryOVF != "ovf-override" {
		t.Fatalf("ContentLibraryOVF: got %q", acc.ContentLibraryOVF)
	}
	if acc.ISOPath != "[ds-override] ISO/custom.iso" {
		t.Fatalf("ISOPath: got %q", acc.ISOPath)
	}
	if acc.TagCategory != "cat-override" {
		t.Fatalf("TagCategory: got %q", acc.TagCategory)
	}
	if acc.TagA != "tag-blue-override" {
		t.Fatalf("TagA: got %q", acc.TagA)
	}
	if acc.TagB != "tag-red-override" {
		t.Fatalf("TagB: got %q", acc.TagB)
	}
	if acc.StoragePolicyA != "policy-a-override" {
		t.Fatalf("StoragePolicyA: got %q", acc.StoragePolicyA)
	}
	if acc.StoragePolicyB != "policy-b-override" {
		t.Fatalf("StoragePolicyB: got %q", acc.StoragePolicyB)
	}
	if acc.StoragePolicyC != "policy-c-override" {
		t.Fatalf("StoragePolicyC: got %q", acc.StoragePolicyC)
	}
	if got := acc.StoragePolicies(); len(got) != 3 || got[2] != "policy-c-override" {
		t.Fatalf("StoragePolicies with C: got %#v", got)
	}
	if acc.Notes != "notes-override" {
		t.Fatalf("Notes: got %q", acc.Notes)
	}
	if acc.OVFURL != "https://artifacts.example.com/alpine.ovf" {
		t.Fatalf("OVFURL: got %q", acc.OVFURL)
	}
	if acc.OVAURL != "https://artifacts.example.com/alpine.ova" {
		t.Fatalf("OVAURL: got %q", acc.OVAURL)
	}
	if acc.OVFUsername != "ovf-user" {
		t.Fatalf("OVFUsername: got %q", acc.OVFUsername)
	}
	if acc.OVFPassword != "ovf-pass" {
		t.Fatalf("OVFPassword: got %q", acc.OVFPassword)
	}
	if !acc.OVFSkipTLSVerify {
		t.Fatalf("OVFSkipTLSVerify: got false, want true")
	}
}

func clearAccEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		EnvVcenterServer,
		EnvVsphereUsername,
		EnvVspherePassword,
		EnvDatacenter,
		EnvVsphereHost,
		EnvVsphereHostSecondary,
		EnvDatastore,
		EnvDatastoreCluster,
		EnvCluster,
		EnvClusterDRS,
		EnvFolder,
		EnvResourcePool,
		EnvNetwork,
		EnvTemplate,
		EnvISOPath,
		EnvContentLibrary,
		EnvContentLibraryVMTX,
		EnvContentLibraryOVF,
		EnvTagCategory,
		EnvTagA,
		EnvTagB,
		EnvStoragePolicyA,
		EnvStoragePolicyB,
		EnvStoragePolicyC,
		EnvNotes,
		EnvOVFURL,
		EnvOVAURL,
		EnvOVFUsername,
		EnvOVFPassword,
		EnvOVFSkipTLSVerify,
	} {
		t.Setenv(key, "")
	}
}

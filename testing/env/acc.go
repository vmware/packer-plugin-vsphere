// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package env

// AccConfig holds vSphere inventory names used by live acceptance tests.
// Values come from environment variables when set; otherwise package defaults.
type AccConfig struct {
	VCenterServer      string
	Username           string
	Password           string
	Datacenter         string
	Host               string
	HostSecondary      string
	Datastore          string
	DatastoreCluster   string
	Cluster            string
	ClusterDRS         string
	Folder             string
	ResourcePool       string
	Network            string
	Template           string
	ISOPath            string
	ContentLibrary     string
	ContentLibraryVMTX string
	ContentLibraryOVF  string
	TagCategory        string
	TagA               string
	TagB               string
	StoragePolicyA     string
	StoragePolicyB     string
	StoragePolicyC     string
	Notes              string
	OVFURL             string
	OVAURL             string
	OVFUsername        string
	OVFPassword        string
	OVFSkipTLSVerify   bool
}

// AccFromEnv returns an AccConfig populated from the environment, falling back
// to the package defaults when variables are unset.
func AccFromEnv() AccConfig {
	return AccConfig{
		VCenterServer:      GetenvOrDefault(VcenterServer, DefaultVcenterServer),
		Username:           GetenvOrDefault(VsphereUsername, DefaultVsphereUsername),
		Password:           GetenvOrDefault(VspherePassword, DefaultVspherePassword),
		Datacenter:         GetenvOrDefault(Datacenter, DefaultDatacenter),
		Host:               GetenvOrDefault(VsphereHost, DefaultVsphereHost),
		HostSecondary:      GetenvOrDefault(VsphereHostSecondary, DefaultVsphereHostSecondary),
		Datastore:          GetenvOrDefault(Datastore, DefaultDatastore),
		DatastoreCluster:   GetenvOrDefault(DatastoreCluster, DefaultDatastoreCluster),
		Cluster:            GetenvOrDefault(Cluster, DefaultCluster),
		ClusterDRS:         GetenvOrDefault(ClusterDRS, DefaultClusterDRS),
		Folder:             GetenvOrDefault(Folder, DefaultFolder),
		ResourcePool:       GetenvOrDefault(ResourcePool, DefaultResourcePool),
		Network:            GetenvOrDefault(Network, DefaultNetwork),
		Template:           GetenvOrDefault(Template, DefaultTemplate),
		ISOPath:            GetenvOrDefault(ISOPath, DefaultISOPath),
		ContentLibrary:     GetenvOrDefault(ContentLibrary, DefaultContentLibrary),
		ContentLibraryVMTX: GetenvOrDefault(ContentLibraryVMTX, DefaultContentLibraryVMTX),
		ContentLibraryOVF:  GetenvOrDefault(ContentLibraryOVF, DefaultContentLibraryOVF),
		TagCategory:        GetenvOrDefault(TagCategory, DefaultTagCategory),
		TagA:               GetenvOrDefault(TagA, DefaultTagA),
		TagB:               GetenvOrDefault(TagB, DefaultTagB),
		StoragePolicyA:     GetenvOrDefault(StoragePolicyA, DefaultStoragePolicyA),
		StoragePolicyB:     GetenvOrDefault(StoragePolicyB, DefaultStoragePolicyB),
		StoragePolicyC:     GetenvOrDefault(StoragePolicyC, DefaultStoragePolicyC),
		Notes:              GetenvOrDefault(Notes, DefaultNotes),
		OVFURL:             GetenvOrDefault(OVFURL, DefaultOVFURL),
		OVAURL:             GetenvOrDefault(OVAURL, DefaultOVAURL),
		OVFUsername:        GetenvOrDefault(OVFUsername, DefaultOVFUsername),
		OVFPassword:        GetenvOrDefault(OVFPassword, DefaultOVFPassword),
		OVFSkipTLSVerify:   Truthy(OVFSkipTLSVerify),
	}
}

// StoragePolicies returns the storage policy names used by the acceptance
// tests.
func (c AccConfig) StoragePolicies() []string {
	policies := []string{c.StoragePolicyA, c.StoragePolicyB}
	if c.StoragePolicyC != "" {
		policies = append(policies, c.StoragePolicyC)
	}
	return policies
}

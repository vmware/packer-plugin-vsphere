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
		VCenterServer:      GetenvOrDefault(EnvVcenterServer, DefaultVcenterServer),
		Username:           GetenvOrDefault(EnvVsphereUsername, DefaultVsphereUsername),
		Password:           GetenvOrDefault(EnvVspherePassword, DefaultVspherePassword),
		Datacenter:         GetenvOrDefault(EnvDatacenter, DefaultDatacenter),
		Host:               GetenvOrDefault(EnvVsphereHost, DefaultVsphereHost),
		HostSecondary:      GetenvOrDefault(EnvVsphereHostSecondary, DefaultVsphereHostSecondary),
		Datastore:          GetenvOrDefault(EnvDatastore, DefaultDatastore),
		DatastoreCluster:   GetenvOrDefault(EnvDatastoreCluster, DefaultDatastoreCluster),
		Cluster:            GetenvOrDefault(EnvCluster, DefaultCluster),
		ClusterDRS:         GetenvOrDefault(EnvClusterDRS, DefaultClusterDRS),
		Folder:             GetenvOrDefault(EnvFolder, DefaultFolder),
		ResourcePool:       GetenvOrDefault(EnvResourcePool, DefaultResourcePool),
		Network:            GetenvOrDefault(EnvNetwork, DefaultNetwork),
		Template:           GetenvOrDefault(EnvTemplate, DefaultTemplate),
		ISOPath:            GetenvOrDefault(EnvISOPath, DefaultISOPath),
		ContentLibrary:     GetenvOrDefault(EnvContentLibrary, DefaultContentLibrary),
		ContentLibraryVMTX: GetenvOrDefault(EnvContentLibraryVMTX, DefaultContentLibraryVMTX),
		ContentLibraryOVF:  GetenvOrDefault(EnvContentLibraryOVF, DefaultContentLibraryOVF),
		TagCategory:        GetenvOrDefault(EnvTagCategory, DefaultTagCategory),
		TagA:               GetenvOrDefault(EnvTagA, DefaultTagA),
		TagB:               GetenvOrDefault(EnvTagB, DefaultTagB),
		StoragePolicyA:     GetenvOrDefault(EnvStoragePolicyA, DefaultStoragePolicyA),
		StoragePolicyB:     GetenvOrDefault(EnvStoragePolicyB, DefaultStoragePolicyB),
		StoragePolicyC:     GetenvOrDefault(EnvStoragePolicyC, DefaultStoragePolicyC),
		Notes:              GetenvOrDefault(EnvNotes, DefaultNotes),
		OVFURL:             GetenvOrDefault(EnvOVFURL, DefaultOVFURL),
		OVAURL:             GetenvOrDefault(EnvOVAURL, DefaultOVAURL),
		OVFUsername:        GetenvOrDefault(EnvOVFUsername, DefaultOVFUsername),
		OVFPassword:        GetenvOrDefault(EnvOVFPassword, DefaultOVFPassword),
		OVFSkipTLSVerify:   EnvTruthy(EnvOVFSkipTLSVerify),
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

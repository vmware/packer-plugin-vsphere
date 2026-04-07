// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package utils

import (
	"os"
)

const (
	DefaultVcenterServer   = "vc01.example.com"
	DefaultVsphereUsername = "administrator@vsphere.local"
	DefaultVspherePassword = "VMw@re1!"
	DefaultVsphereHost     = "esx01.example.com"

	EnvVcenterServer   = "VSPHERE_VCENTER_SERVER"
	EnvVsphereUsername = "VSPHERE_USERNAME"
	EnvVspherePassword = "VSPHERE_PASSWORD"
	EnvVsphereHost     = "VSPHERE_HOST"
)

func GetenvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

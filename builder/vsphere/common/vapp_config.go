// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc mapstructure-to-hcl2 -type VAppConfig

package common

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
)

// VAppConfig configures vApp options and properties for a virtual machine.
// Refer to each builder's documentation for usage details.
type VAppConfig struct {
	Properties map[string]string `mapstructure:"properties"`
	// The deployment configuration to use when deploying from an OVF/OVA file.
	// This corresponds to deployment configurations defined in an OVF descriptor.
	// -> **Note:** Only applicable when using remote OVF/OVA sources.
	DeploymentOption string `mapstructure:"deployment_option"`
}

// Active reports whether vApp configuration should be applied.
// Omitting the vapp block, or a block with no properties, leaves vApp disabled.
func (c *VAppConfig) Active() bool {
	if c == nil {
		return false
	}
	return len(c.Properties) > 0
}

// NeedsEphemeralSSHKey reports whether the communicator will use a generated SSH key pair.
func (c *VAppConfig) NeedsEphemeralSSHKey(comm communicator.Config) bool {
	return comm.Type == "ssh" &&
		comm.SSHPassword == "" &&
		comm.SSHPrivateKeyFile == "" &&
		!comm.SSHAgentAuth
}

// PrepareSSH validates vApp settings required for ephemeral SSH key injection.
func (c *VAppConfig) PrepareSSH(comm communicator.Config) []error {
	if !c.NeedsEphemeralSSHKey(comm) {
		return nil
	}

	var errs []error
	if !c.Active() {
		errs = append(errs, fmt.Errorf(
			"'vapp.properties' must be set when using SSH without password, private key file, or agent auth"))
	}
	if c.Properties == nil {
		c.Properties = make(map[string]string)
	}
	if _, ok := c.Properties["public-keys"]; !ok {
		errs = append(errs, fmt.Errorf(
			"'vapp.properties' must include 'public-keys' when using ephemeral SSH keys"))
	}
	return errs
}

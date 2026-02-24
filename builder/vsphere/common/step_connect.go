// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ConnectConfig

package common

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

type ConnectConfig struct {
	// The fully qualified domain name or IP address of the vCenter instance.
	VCenterServer string `mapstructure:"vcenter_server"`
	// The username to authenticate with the vCenter instance.
	Username string `mapstructure:"username"`
	// The password to authenticate with the vCenter instance.
	Password string `mapstructure:"password"`
	// Do not validate the certificate of the vCenter instance.
	// Defaults to `false`.
	//
	// -> **Note:** This option is beneficial in scenarios where the certificate
	// is self-signed or does not meet standard validation criteria.
	InsecureConnection bool `mapstructure:"insecure_connection"`
	// The name of the datacenter object in the vSphere inventory.
	//
	// -> **Note:** Required if more than one datacenter object exists in the
	// vSphere inventory.
	Datacenter string `mapstructure:"datacenter"`
}

func (c *ConnectConfig) Prepare() []error {
	var errs []error

	if c.VCenterServer == "" {
		errs = append(errs, fmt.Errorf("'vcenter_server' is required"))
	}
	if c.Username == "" {
		errs = append(errs, fmt.Errorf("'username' is required"))
	}
	if c.Password == "" {
		errs = append(errs, fmt.Errorf("'password' is required"))
	}

	return errs
}

type StepConnect struct {
	Config *ConnectConfig
}

func (s *StepConnect) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	// Convert common.ConnectConfig to driver.ConnectConfig
	// Both structs have identical fields to avoid import cycles
	d, err := driver.NewDriver(&driver.ConnectConfig{
		VCenterServer:      s.Config.VCenterServer,
		Username:           s.Config.Username,
		Password:           s.Config.Password,
		InsecureConnection: s.Config.InsecureConnection,
		Datacenter:         s.Config.Datacenter,
	})
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}
	state.Put("driver", d)

	return multistep.ActionContinue
}

func (s *StepConnect) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	d, ok := state.GetOk("driver")
	if !ok {
		log.Printf("[INFO] No driver in state; nothing to cleanup.")
		return
	}

	driver, ok := d.(driver.Driver)
	if !ok {
		log.Printf("[ERROR] The object stored in the state under 'driver' key is of type '%s', not 'driver.Driver'. This could indicate a problem with the state initialization or management.", reflect.TypeOf(d))
		return
	}

	ui.Say("Closing sessions...")

	errorRestClient, errorSoapClient := driver.Cleanup()
	if errorRestClient != nil {
		log.Printf("[WARN] Failed to close REST client session. The session may already be closed: %s", errorRestClient.Error())
	}
	if errorSoapClient != nil {
		log.Printf("[WARN] Failed to close SOAP client session. The session may already be closed: %s", errorSoapClient.Error())
	}
}

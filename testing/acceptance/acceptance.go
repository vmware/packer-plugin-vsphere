// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func NewVmName() string {
	return fmt.Sprintf("acc-test-%s", time.Now().Format("20060102-150405"))
}

func RenderConfig(builderType string, config map[string]any) string {
	t := map[string][]map[string]any{
		"builders": {
			{"type": builderType},
		},
	}
	maps.Copy(t["builders"][0], config)

	j, _ := json.Marshal(t)
	return string(j)
}

func TestConn() (driver.Driver, error) {
	acc := env.AccFromEnv()

	d, err := driver.NewDriver(&driver.ConnectConfig{
		VCenterServer:      acc.VCenterServer,
		Username:           acc.Username,
		Password:           acc.Password,
		Datacenter:         acc.Datacenter,
		InsecureConnection: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to endpoint: %v", err)
	}
	return d, nil
}

func CleanupVm(d driver.Driver, name string) error {
	acc := env.AccFromEnv()
	vm, err := d.FindVM(name)
	if err != nil {
		// Plugin may have already destroyed the virtual machine on failure, or
		// the test was aborted before create completed.
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("error finding virtual machine: %v", err)
	}

	isTemplate, err := vm.IsTemplate()
	if err == nil && isTemplate {
		// Templates cannot be destroyed directly.
		if err := vm.ConvertToVirtualMachine(acc.Cluster, acc.Host, acc.ResourcePool); err != nil {
			// Backing may already be gone; drop the inventory entry.
			if err2 := vm.Unregister(); err2 != nil {
				return fmt.Errorf("error converting template (%v); unregister failed: %v", err, err2)
			}
			return nil
		}
	}

	_ = vm.PowerOff()

	if err := vm.Destroy(); err != nil {
		// Destroy deletes datastore files. If those are already gone, the
		// inventory object remains and must be unregistered.
		_ = vm.PowerOff()
		if err2 := vm.Destroy(); err2 == nil {
			return nil
		}
		if err2 := vm.Unregister(); err2 != nil {
			return fmt.Errorf("error destroying virtual machine (%v); unregister failed: %v", err, err2)
		}
	}
	return nil
}

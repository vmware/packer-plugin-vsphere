// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package virtualmachine

import (
	"fmt"
	"regexp"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	dscommon "github.com/vmware/packer-plugin-vsphere/datasource/common"
)

// filterVms removes virtual machines from vmList that do not match the datasource config filters.
func filterVms(vmList []*object.VirtualMachine, c Config, d *driver.VCenterDriver) ([]*object.VirtualMachine, error) {
	filterFuncs := make([]func(*object.VirtualMachine) (bool, error), 0)

	if c.NameRegex != "" {
		re := regexp.MustCompile(c.NameRegex)
		filterFuncs = append(filterFuncs, func(vm *object.VirtualMachine) (bool, error) {
			return re.MatchString(vm.Name()), nil
		})
	}

	if c.Template {
		filterFuncs = append(filterFuncs, func(vm *object.VirtualMachine) (bool, error) {
			isTemplate, err := vm.IsTemplate(d.Ctx)
			if err != nil {
				return false, fmt.Errorf("error checking if virtual machine is a template: %w", err)
			}
			return isTemplate, nil
		})
	}

	if c.Host != "" {
		hostVms, err := getHostVms(d, c.Host)
		if err != nil {
			return nil, err
		}

		filterFuncs = append(filterFuncs, func(vm *object.VirtualMachine) (bool, error) {
			vmName := vm.Name()
			for _, hostVm := range hostVms {
				if vmName == hostVm.Name {
					return true, nil
				}
			}
			return false, nil
		})
	}

	if c.Tags != nil {
		required := toCommonTags(c.Tags)
		filterFuncs = append(filterFuncs, func(vm *object.VirtualMachine) (bool, error) {
			return dscommon.ObjectHasAllTags(d, vm.Reference(), required)
		})
	}

	result := make([]*object.VirtualMachine, 0)
	for _, vm := range vmList {
		var ok bool
		var err error
		if len(filterFuncs) == 0 {
			ok = true
		}
		for _, vmPassedFilter := range filterFuncs {
			ok, err = vmPassedFilter(vm)
			if err != nil {
				return nil, fmt.Errorf("failed to filter vm: %w", err)
			}
			if !ok {
				break
			}
		}
		if ok {
			result = append(result, vm)
		}
	}

	return result, nil
}

// findLatestVM returns the most recently created virtual machine from vmList.
func findLatestVM(d *driver.VCenterDriver, vmList []*object.VirtualMachine) (*object.VirtualMachine, error) {
	return dscommon.SelectMax(vmList, func(elementVM *object.VirtualMachine) (int64, error) {
		var vmConfig mo.VirtualMachine
		err := elementVM.Properties(d.Ctx, elementVM.Reference(), []string{"config"}, &vmConfig)
		if err != nil {
			return 0, fmt.Errorf("error retrieving config properties for the virtual machine: %w", err)
		}
		if vmConfig.Config == nil || vmConfig.Config.CreateDate == nil {
			return 0, fmt.Errorf("virtual machine %s has no create date", elementVM.Name())
		}
		return vmConfig.Config.CreateDate.UnixNano(), nil
	})
}

// getHostVms retrieves all virtual machines on the specified host.
func getHostVms(d *driver.VCenterDriver, hostName string) ([]mo.VirtualMachine, error) {
	pc := property.DefaultCollector(d.Client.Client)
	obj, err := d.Finder.HostSystem(d.Ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("error finding defined host system: %w", err)
	}

	var host mo.HostSystem
	err = pc.RetrieveOne(d.Ctx, obj.Reference(), []string{"vm"}, &host)
	if err != nil {
		return nil, fmt.Errorf("error retrieving properties of host system: %w", err)
	}

	var hostVms []mo.VirtualMachine
	err = pc.Retrieve(d.Ctx, host.Vm, []string{"name"}, &hostVms)
	if err != nil {
		return nil, fmt.Errorf("failed to get properties for the virtual machine: %w", err)
	}
	return hostVms, nil
}

func toCommonTags(tagList []Tag) []dscommon.Tag {
	out := make([]dscommon.Tag, len(tagList))
	for i, tag := range tagList {
		out[i] = dscommon.Tag{Name: tag.Name, Category: tag.Category}
	}
	return out
}

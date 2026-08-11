// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25/types"
)

var vAppGuestInfoTransports = []string{"com.vmware.guestInfo", "iso"}

func defaultVAppPropertyInfo(key int32, id, value string) *types.VAppPropertyInfo {
	label := id
	description := ""
	switch id {
	case "public-keys":
		label = "SSH Public Keys"
		description = "SSH public keys for guest access"
	case "hostname":
		label = "Hostname"
		description = "Guest hostname"
	case "user-data":
		label = "User Data"
		description = "Cloud-init or guest customization user data"
	}

	return &types.VAppPropertyInfo{
		Key:              key,
		Id:               id,
		Label:            label,
		Type:             "string",
		UserConfigurable: new(true),
		Value:            value,
		Description:      description,
	}
}

func maxVAppPropertyKey(properties []types.VAppPropertyInfo) int32 {
	var maxKey int32
	for _, p := range properties {
		if p.Key > maxKey {
			maxKey = p.Key
		}
	}
	return maxKey
}

func buildVAppConfigSpec(ctx context.Context, vm *VirtualMachineDriver, enable bool, newProps map[string]string) (*types.VmConfigSpec, error) {
	if len(newProps) == 0 {
		return nil, nil
	}

	vProps, err := vm.Properties(ctx)
	if err != nil {
		return nil, err
	}

	var vAppInfo *types.VmConfigInfo
	if vProps.Config.VAppConfig != nil {
		vAppInfo = vProps.Config.VAppConfig.GetVmConfigInfo()
	}
	return buildVAppConfigSpecFromInfo(vAppInfo, enable, newProps)
}

func buildVAppConfigSpecFromInfo(vAppConfig *types.VmConfigInfo, enable bool, newProps map[string]string) (*types.VmConfigSpec, error) {
	if len(newProps) == 0 {
		return nil, nil
	}

	if vAppConfig == nil {
		if !enable {
			return nil, fmt.Errorf("no vApp configuration found; cannot set vApp properties")
		}
		return newVAppConfigSpec(newProps), nil
	}

	allProperties := vAppConfig.Property
	existing := make(map[string]types.VAppPropertyInfo, len(allProperties))
	for _, p := range allProperties {
		existing[p.Id] = p
	}

	nextKey := maxVAppPropertyKey(allProperties)
	var props []types.VAppPropertySpec

	keys := make([]string, 0, len(newProps))
	for id := range newProps {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	for _, id := range keys {
		userValue := newProps[id]
		p, found := existing[id]
		if !found {
			if !enable {
				continue
			}
			nextKey++
			props = append(props, types.VAppPropertySpec{
				ArrayUpdateSpec: types.ArrayUpdateSpec{
					Operation: types.ArrayUpdateOperationAdd,
				},
				Info: defaultVAppPropertyInfo(nextKey, id, userValue),
			})
			continue
		}

		if p.UserConfigurable != nil && !*p.UserConfigurable {
			return nil, fmt.Errorf("vApp property with userConfigurable=false specified in vapp.properties: %s", id)
		}

		props = append(props, types.VAppPropertySpec{
			ArrayUpdateSpec: types.ArrayUpdateSpec{
				Operation: types.ArrayUpdateOperationEdit,
			},
			Info: &types.VAppPropertyInfo{
				Key:              p.Key,
				Id:               p.Id,
				Value:            userValue,
				UserConfigurable: p.UserConfigurable,
			},
		})
	}

	if !enable {
		var unsupported []string
		for id := range newProps {
			if _, found := existing[id]; !found {
				unsupported = append(unsupported, id)
			}
		}
		if len(unsupported) > 0 {
			sort.Strings(unsupported)
			return nil, fmt.Errorf("unsupported vApp properties in vapp.properties: %v", unsupported)
		}
	}

	if len(props) == 0 {
		return nil, nil
	}

	spec := &types.VmConfigSpec{Property: props}
	if enable && len(allProperties) == 0 {
		spec.OvfEnvironmentTransport = vAppGuestInfoTransports
	}
	return spec, nil
}

func newVAppConfigSpec(newProps map[string]string) *types.VmConfigSpec {
	keys := make([]string, 0, len(newProps))
	for id := range newProps {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	var props []types.VAppPropertySpec
	for i, id := range keys {
		props = append(props, types.VAppPropertySpec{
			ArrayUpdateSpec: types.ArrayUpdateSpec{
				Operation: types.ArrayUpdateOperationAdd,
			},
			Info: defaultVAppPropertyInfo(int32(i+1), id, newProps[id]),
		})
	}

	return &types.VmConfigSpec{
		Property:                props,
		OvfEnvironmentTransport: vAppGuestInfoTransports,
	}
}

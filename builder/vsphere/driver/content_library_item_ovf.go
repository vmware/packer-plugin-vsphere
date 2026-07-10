// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/types"
)

func (d *VCenterDriver) deployOvfLibraryItem(ctx context.Context, item *library.Item, config *ContentLibraryDeployConfig, target *contentLibraryDeployTarget) (VirtualMachine, error) {
	filter, err := d.filterContentLibraryOvf(ctx, item.ID, target.Target)
	if err != nil {
		return nil, fmt.Errorf("error validating content library item OVF template: %s", err)
	}

	if err := validateContentLibraryOvfConfig(config, filter); err != nil {
		return nil, err
	}

	networkMappings, err := d.buildContentLibraryOvfNetworkMappings(config.Network, filter)
	if err != nil {
		return nil, err
	}

	deploy := vcenter.Deploy{
		DeploymentSpec: vcenter.DeploymentSpec{
			Name:               config.Name,
			DefaultDatastoreID: target.datastoreID,
			AcceptAllEULA:      true,
			Annotation:         config.Annotation,
			NetworkMappings:    networkMappings,
		},
		Target: target.Target,
	}

	var additionalParams []vcenter.AdditionalParams
	if config.DeploymentOption != "" {
		additionalParams = append(additionalParams, vcenter.AdditionalParams{
			Class:       vcenter.ClassDeploymentOptionParams,
			Type:        vcenter.TypeDeploymentOptionParams,
			SelectedKey: config.DeploymentOption,
		})
	}
	if len(config.VAppProperties) > 0 {
		properties := make([]vcenter.Property, 0, len(config.VAppProperties))
		for key, value := range config.VAppProperties {
			properties = append(properties, vcenter.Property{
				ID:    key,
				Value: value,
			})
		}
		additionalParams = append(additionalParams, vcenter.AdditionalParams{
			Class:      vcenter.ClassPropertyParams,
			Type:       vcenter.TypePropertyParams,
			Properties: properties,
		})
	}
	if len(additionalParams) > 0 {
		deploy.AdditionalParams = additionalParams
	}

	m := vcenter.NewManager(d.RestClient.client)
	ref, err := m.DeployLibraryItem(ctx, item.ID, deploy)
	if err != nil {
		return nil, fmt.Errorf("content library item OVF template deployment failed: %s", err)
	}

	vm := d.NewVM(ref)
	if err := d.applyContentLibraryOvfPostDeploy(vm, config); err != nil {
		return nil, fmt.Errorf("failed to apply post-deploy virtual machine configuration: %s", err)
	}

	log.Printf("[INFO] Successfully deployed virtual machine from content library item OVF template '%s'", item.Name)
	return vm, nil
}

func (d *VCenterDriver) buildContentLibraryOvfNetworkMappings(vsphereNetworkName string, filter vcenter.FilterResponse) ([]vcenter.NetworkMapping, error) {
	if len(filter.Networks) == 0 {
		return nil, nil
	}

	if vsphereNetworkName == "" {
		return nil, fmt.Errorf("OVF requires network mapping for %s; specify the network configuration option", strings.Join(filter.Networks, ", "))
	}

	net, err := d.FindNetwork(vsphereNetworkName)
	if err != nil {
		return nil, fmt.Errorf("error finding network: %s", err)
	}

	netRef := net.network.Reference().Value
	mappings := make([]vcenter.NetworkMapping, 0, len(filter.Networks))
	for _, ovfNet := range filter.Networks {
		mappings = append(mappings, vcenter.NetworkMapping{
			Key:   ovfNet,
			Value: netRef,
		})
	}

	return mappings, nil
}

func validateContentLibraryOvfConfig(config *ContentLibraryDeployConfig, filter vcenter.FilterResponse) error {
	if config.DeploymentOption != "" {
		if err := validateContentLibraryDeploymentOption(config.DeploymentOption, filter.AdditionalParams); err != nil {
			return err
		}
	}

	if len(config.VAppProperties) > 0 {
		for key, value := range config.VAppProperties {
			if key == "" {
				return fmt.Errorf("vApp property key cannot be empty")
			}
			if len(key) > 255 {
				return fmt.Errorf("vApp property key '%s' exceeds maximum length of 255 characters", key)
			}
			if len(value) > 65535 {
				return fmt.Errorf("vApp property value for key '%s' exceeds maximum length of 65535 characters", key)
			}
		}
	}

	return nil
}

func validateContentLibraryDeploymentOption(deploymentOption string, params []vcenter.AdditionalParams) error {
	for _, param := range params {
		if param.Class != vcenter.ClassDeploymentOptionParams {
			continue
		}
		for _, option := range param.DeploymentOptions {
			if option.Key == deploymentOption {
				return nil
			}
		}
		available := make([]string, 0, len(param.DeploymentOptions))
		for _, option := range param.DeploymentOptions {
			available = append(available, option.Key)
		}
		if len(available) == 0 {
			return fmt.Errorf("deployment option '%s' specified but OVF does not define any deployment options", deploymentOption)
		}
		return fmt.Errorf("deployment option '%s' not found in OVF. Available options: %s",
			deploymentOption, strings.Join(available, ", "))
	}

	return fmt.Errorf("deployment option '%s' specified but OVF does not define any deployment options", deploymentOption)
}

func (d *VCenterDriver) applyContentLibraryOvfPostDeploy(vm VirtualMachine, config *ContentLibraryDeployConfig) error {
	if config.MacAddress == "" {
		return nil
	}

	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("error finding virtual machine devices: %s", err)
	}

	adapter, err := findNetworkAdapter(devices)
	if err != nil {
		return fmt.Errorf("error finding network adapter: %s", err)
	}

	current := adapter.GetVirtualEthernetCard()
	current.AddressType = string(types.VirtualEthernetCardMacTypeManual)
	current.MacAddress = config.MacAddress

	return vm.Reconfigure(types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Device:    adapter.(types.BaseVirtualDevice),
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
			},
		},
	})
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

// buildOvfNetworkMappings maps OVF descriptor network names to a vSphere network.
// When the OVF defines multiple networks, each name is mapped to the same configured network.
func (d *VCenterDriver) buildOvfNetworkMappings(ovfNetworks []types.OvfNetworkInfo, vsphereNetworkName string) ([]types.OvfNetworkMapping, error) {
	if len(ovfNetworks) == 0 {
		return nil, nil
	}

	if vsphereNetworkName == "" {
		names := make([]string, 0, len(ovfNetworks))
		for _, ovfNet := range ovfNetworks {
			names = append(names, ovfNet.Name)
		}
		return nil, fmt.Errorf("OVF requires network mapping for %s; specify the network configuration option", strings.Join(names, ", "))
	}

	network, err := d.FindNetwork(vsphereNetworkName)
	if err != nil {
		return nil, fmt.Errorf("error finding network: %s", err)
	}

	netRef := network.network.Reference()
	mappings := make([]types.OvfNetworkMapping, 0, len(ovfNetworks))
	for _, ovfNet := range ovfNetworks {
		mappings = append(mappings, types.OvfNetworkMapping{
			Name:    ovfNet.Name,
			Network: netRef,
		})
	}

	return mappings, nil
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// StepHTTPIPDiscover is a resource for discovering an IP address over HTTP and optionally verifying it in a specific
// network.
type StepHTTPIPDiscover struct {
	HTTPIP  string
	Network *net.IPNet
}

func (s *StepHTTPIPDiscover) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ip, err := getHostIP(s.HTTPIP, s.Network)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("http_ip", ip)

	return multistep.ActionContinue
}

func (s *StepHTTPIPDiscover) Cleanup(state multistep.StateBag) {}

func getHostIP(s string, network *net.IPNet) (string, error) {
	if s != "" {
		if net.ParseIP(s) != nil {
			return s, nil
		}

		return "", fmt.Errorf("invalid IP address")
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	// look for an IP that is contained in the ip_wait_address range
	if network != nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if ok && !ipnet.IP.IsLoopback() {
				if network.Contains(ipnet.IP) {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	// fallback to an ipv4 address if an IP is not found in the range
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("IP not found")
}

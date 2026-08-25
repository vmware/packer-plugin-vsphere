// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"net"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func guestNic(mac, ip string) types.GuestNicInfo {
	nic := types.GuestNicInfo{MacAddress: mac}
	if ip != "" {
		nic.IpConfig = &types.NetIpConfigInfo{
			IpAddress: []types.NetIpConfigInfoIpAddress{{IpAddress: ip}},
		}
	}
	return nic
}

func TestSelectIPByNetworkAdapters(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.168.86.0/24")
	if err != nil {
		t.Fatal(err)
	}

	adapters := []NetworkAdapter{
		{Name: "ethernet-0", MAC: "00:50:56:81:9a:a5", Key: 4000},
		{Name: "ethernet-1", MAC: "00:50:56:81:01:d1", Key: 4001},
		{Name: "ethernet-2", MAC: "00:50:56:81:3e:82", Key: 4002},
		{Name: "ethernet-3", MAC: "00:50:56:81:71:a0", Key: 4003},
	}
	nics := []types.GuestNicInfo{
		guestNic("00:50:56:81:9a:a5", "192.168.86.201"),
		guestNic("00:50:56:81:01:d1", "192.168.86.202"),
		guestNic("00:50:56:81:71:a0", "192.168.86.203"),
		guestNic("00:50:56:81:3e:82", "192.168.86.204"),
	}

	if got := selectIPByNetworkAdapters(adapters, nics, cidr); got != "192.168.86.201" {
		t.Fatalf("any adapter = %q, want 192.168.86.201", got)
	}
	if got := selectIPByNetworkAdapters(adapters[2:3], nics, cidr); got != "192.168.86.204" {
		t.Fatalf("index 2 = %q, want 192.168.86.204 (MAC of ethernet-2)", got)
	}
	if got := selectIPByNetworkAdapters(adapters[2:3], nics[:2], cidr); got != "" {
		t.Fatalf("index 2 before that NIC has an IP = %q, want empty", got)
	}
}

func TestSelectIPByNetworkAdapters_skipsIPv6UntilMatch(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.168.86.0/24")
	if err != nil {
		t.Fatal(err)
	}
	adapters := []NetworkAdapter{{Name: "ethernet-0", MAC: "00:50:56:81:9a:a5"}}
	nics := []types.GuestNicInfo{
		{
			MacAddress: "00:50:56:81:9a:a5",
			IpConfig: &types.NetIpConfigInfo{
				IpAddress: []types.NetIpConfigInfoIpAddress{
					{IpAddress: "fe80::1"},
					{IpAddress: "192.168.86.201"},
				},
			},
		},
	}
	if got := selectIPByNetworkAdapters(adapters, nics, cidr); got != "192.168.86.201" {
		t.Fatalf("got %q, want 192.168.86.201", got)
	}
}

func TestIpMatchesFilter(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("192.168.0.0/16")

	if !ipMatchesFilter("192.168.1.1", cidr) {
		t.Fatal("expected match in cidr")
	}
	if ipMatchesFilter("10.0.0.1", cidr) {
		t.Fatal("expected no match outside cidr")
	}
	if !ipMatchesFilter("10.0.0.1", nil) {
		t.Fatal("expected ipv4 match with nil ipNet")
	}
	if ipMatchesFilter("2001:db8::1", nil) {
		t.Fatal("expected ipv6 skip with nil ipNet")
	}
}

func TestIpNetWantsIPv4(t *testing.T) {
	if !ipNetWantsIPv4(nil) {
		t.Fatal("expected nil ipNet to wait for ipv4")
	}

	_, v4, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatal(err)
	}
	if !ipNetWantsIPv4(v4) {
		t.Fatal("expected default ipv4 cidr to wait for ipv4")
	}

	_, v6, err := net.ParseCIDR("::/0")
	if err != nil {
		t.Fatal(err)
	}
	if ipNetWantsIPv4(v6) {
		t.Fatal("expected ipv6 cidr to wait for ipv6")
	}
}

func vmxnet3(key, unit int32, mac string) *types.VirtualVmxnet3 {
	u := unit
	return &types.VirtualVmxnet3{
		VirtualVmxnet: types.VirtualVmxnet{
			VirtualEthernetCard: types.VirtualEthernetCard{
				VirtualDevice: types.VirtualDevice{
					Key:        key,
					UnitNumber: &u,
				},
				MacAddress: mac,
			},
		},
	}
}

func TestNetworkAdaptersFromDevices_followsAddOrderNotUnit(t *testing.T) {
	// Third-added NIC has a higher unit number than the fourth. Index 2 must
	// still be the third network_adapters entry (key 4002), not unit order.
	devices := object.VirtualDeviceList{
		vmxnet3(4000, 7, "00:50:56:81:00:01"),
		vmxnet3(4001, 8, "00:50:56:81:00:02"),
		vmxnet3(4002, 10, "00:50:56:81:00:03"),
		vmxnet3(4003, 9, "00:50:56:81:00:04"),
	}

	got := networkAdaptersFromDevices(devices)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	wantMAC := []string{
		"00:50:56:81:00:01",
		"00:50:56:81:00:02",
		"00:50:56:81:00:03",
		"00:50:56:81:00:04",
	}
	for i, mac := range wantMAC {
		if got[i].MAC != mac {
			t.Errorf("index %d mac = %s, want %s (key %d)", i, got[i].MAC, mac, got[i].Key)
		}
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package iso

import (
	"strings"
	"testing"
)

func TestISOConfig_AdapterIndexOutOfRange(t *testing.T) {
	raw := minimalISOConfig()
	raw["ip_wait_adapter_index"] = 18
	raw["network_adapters"] = []any{
		map[string]any{"network": "VM Network", "network_card": "vmxnet3"},
		map[string]any{"network": "VM Network", "network_card": "vmxnet3"},
		map[string]any{"network": "VM Network", "network_card": "vmxnet3"},
		map[string]any{"network": "VM Network", "network_card": "vmxnet3"},
	}
	c := new(Config)
	_, err := c.Prepare(raw)
	if err == nil {
		t.Fatal("expected ip_wait_adapter_index error")
	}
	want := "ip_wait_adapter_index is 18; valid indexes are 0 to 3 (4 network adapters)"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got %q, want it to contain %q", err.Error(), want)
	}
}

func TestISOConfig_DisableIpWaitRequiresHost(t *testing.T) {
	raw := minimalISOConfig()
	raw["disable_ip_wait"] = true
	c := new(Config)
	_, err := c.Prepare(raw)
	if err == nil {
		t.Fatal("expected disable_ip_wait error when ssh_host/winrm_host is unset")
	}
	if !strings.Contains(err.Error(), "disable_ip_wait") {
		t.Fatalf("expected disable_ip_wait error, got: %s", err)
	}
}

func TestISOConfig_DisableIpWaitWithSSHHost(t *testing.T) {
	raw := minimalISOConfig()
	raw["disable_ip_wait"] = true
	raw["ssh_host"] = "192.168.1.10"
	c := new(Config)
	warns, err := c.Prepare(raw)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for _, w := range warns {
		if strings.Contains(w, "disable_ip_wait") {
			t.Fatalf("did not expect disable_ip_wait warning, got %#v", warns)
		}
	}
}

func TestISOConfig_DisableIpWaitWithCommunicatorNone(t *testing.T) {
	raw := minimalISOConfig()
	raw["disable_ip_wait"] = true
	raw["communicator"] = "none"
	delete(raw, "ssh_username")
	delete(raw, "ssh_password")
	c := new(Config)
	warns, err := c.Prepare(raw)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for _, w := range warns {
		if strings.Contains(w, "disable_ip_wait") {
			t.Fatalf("did not expect disable_ip_wait warning with communicator none, got %#v", warns)
		}
	}
}

func minimalISOConfig() map[string]any {
	return map[string]any{
		"vcenter_server": "vc01.example.com",
		"username":       "administrator@vsphere.local",
		"password":       "VMw@re1!",
		"vm_name":        "vm-01",
		"host":           "esx01.example.com",
		"storage": map[string]any{
			"disk_size": 1024,
		},
		"ssh_username": "root",
		"ssh_password": "VMw@re1!",
	}
}

// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
)

func TestVAppConfigActive(t *testing.T) {
	cfg := &VAppConfig{}
	if cfg.Active() {
		t.Fatal("expected inactive config")
	}
	cfg = &VAppConfig{Properties: map[string]string{"hostname": "h"}}
	if !cfg.Active() {
		t.Fatal("expected active when properties are set")
	}
}

func TestVAppConfigPrepareSSHWithPropertiesOnly(t *testing.T) {
	cfg := VAppConfig{Properties: map[string]string{"public-keys": ""}}
	comm := communicator.Config{Type: "ssh"}

	if errs := cfg.PrepareSSH(comm); len(errs) != 0 {
		t.Fatalf("expected no errors when properties are set, got %v", errs)
	}
}

func TestVAppConfigPrepareSSH(t *testing.T) {
	cfg := VAppConfig{}
	comm := communicator.Config{Type: "ssh", SSH: communicator.SSH{SSHPassword: "secret"}}

	errs := cfg.PrepareSSH(comm)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for password-based ssh, got %v", errs)
	}

	comm.SSHPassword = ""
	errs = cfg.PrepareSSH(comm)
	if len(errs) != 2 {
		t.Fatalf("expected properties and public-keys errors, got %v", errs)
	}

	cfg.Properties = map[string]string{"public-keys": ""}
	errs = cfg.PrepareSSH(comm)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

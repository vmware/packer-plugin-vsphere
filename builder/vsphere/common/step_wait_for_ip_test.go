// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"strings"
	"testing"
	"time"
)

func TestWaitIpConfig_Prepare_defaults(t *testing.T) {
	c := &WaitIpConfig{}
	errs := c.Prepare()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if c.WaitTimeout != 30*time.Minute {
		t.Fatalf("expected default wait timeout 30m, got %v", c.WaitTimeout)
	}
	if c.WaitAddress == nil || *c.WaitAddress != "0.0.0.0/0" {
		t.Fatalf("expected default ip_wait_address 0.0.0.0/0, got %v", c.WaitAddress)
	}
}

func TestWaitIpConfig_ValidateDisableIpWait(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		c := &WaitIpConfig{}
		warnings, errs := c.ValidateDisableIpWait("ssh", "")
		if len(warnings) > 0 || len(errs) > 0 {
			t.Fatalf("expected no warnings or errors, got warnings=%v errs=%v", warnings, errs)
		}
	})

	t.Run("requires communicator host", func(t *testing.T) {
		c := &WaitIpConfig{DisableIpWait: true}
		warnings, errs := c.ValidateDisableIpWait("ssh", "")
		if len(errs) != 1 {
			t.Fatalf("expected one error, got %v", errs)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected one warning, got %v", warnings)
		}
		if strings.Contains(warnings[0], "ip_wait_address") {
			t.Fatalf("warning should not claim ip_wait_address is ignored: %v", warnings[0])
		}
	})

	t.Run("ok with ssh host", func(t *testing.T) {
		c := &WaitIpConfig{DisableIpWait: true}
		warnings, errs := c.ValidateDisableIpWait("ssh", "192.168.1.10")
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "disable_ip_wait") {
			t.Fatalf("expected disable_ip_wait warning, got %v", warnings)
		}
	})

	t.Run("skipped for communicator none", func(t *testing.T) {
		c := &WaitIpConfig{DisableIpWait: true}
		warnings, errs := c.ValidateDisableIpWait("none", "")
		if len(warnings) > 0 || len(errs) > 0 {
			t.Fatalf("expected no warnings or errors for communicator none, got warnings=%v errs=%v", warnings, errs)
		}
	})
}

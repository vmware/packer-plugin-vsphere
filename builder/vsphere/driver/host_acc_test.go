// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

func TestHostAcc(t *testing.T) {
	t.Skip("Acceptance tests not configured yet.")
	d := newTestDriver(t)
	host, err := d.FindHost(env.DefaultVsphereHost)
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	info, err := host.Info("name")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if info.Name != env.DefaultVsphereHost {
		t.Errorf("unexpected result: expected '%s', but returned '%s'", env.DefaultVsphereHost, info.Name)
	}
}

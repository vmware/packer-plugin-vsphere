// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"testing"

	"github.com/vmware/govmomi/crypto"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// RequireKeyProvider skips the test unless vCenter has a Native Key Provider
// or KMIP cluster. Adding a vTPM encrypts VM home files and requires one.
func RequireKeyProvider(t *testing.T) {
	t.Helper()

	d, err := TestConn()
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}

	vc, ok := d.(*driver.VCenterDriver)
	if !ok {
		t.Fatalf("driver does not support key provider queries")
	}

	m, err := crypto.GetManagerKmip(vc.VimClient)
	if err != nil {
		t.Skipf("vTPM ACC skipped: CryptoManager is not available (%v); configure Native Key Provider or KMIP", err)
	}

	clusters, err := m.ListKmipServers(vc.Ctx, nil)
	if err != nil {
		t.Fatalf("list key providers: %v", err)
	}
	if len(clusters) == 0 {
		t.Skip("vTPM ACC skipped: no Native Key Provider or KMIP cluster configured on vCenter")
	}
}
